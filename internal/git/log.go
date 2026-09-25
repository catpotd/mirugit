package git

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// CommitInfo is one row in the history tab.
type CommitInfo struct {
	SHA      string
	ShortSHA string
	Subject  string
	Author   string
	// Unpushed means no remote is known to hold this commit. A branch with no
	// upstream makes this true for every commit: nothing has reached a remote.
	Unpushed bool
	// HasUpstream separates "not pushed yet" from "nowhere to push", which the
	// arrow and the push verb need and undo does not.
	HasUpstream bool
	Age         string
	Files       []string
}

// HistoryPage lets the history view load older commits on demand.
type HistoryPage struct {
	Commits []CommitInfo
	HasMore bool
}

const logPageSize = 100

func unpushedSHAs(ctx context.Context, dir string, upstream bool) (map[string]bool, error) {
	unpushed := map[string]bool{}
	if !upstream {
		return unpushed, nil
	}
	out, err := runRead(ctx, dir, "rev-list", "@{upstream}..HEAD")
	if err != nil {
		return nil, err
	}
	for _, sha := range strings.Fields(string(out)) {
		unpushed[sha] = true
	}
	return unpushed, nil
}

// Log keeps callers that need only the newest commits on a single call.
func Log(ctx context.Context, dir string) ([]CommitInfo, error) {
	page, err := LogPage(ctx, dir, 0)
	return page.Commits, err
}

// LogPage keeps history reads bounded as repositories grow.
func LogPage(ctx context.Context, dir string, skip int) (HistoryPage, error) {
	if skip < 0 {
		skip = 0
	}
	// A repository with no commits has no log, and git exits 128 rather than
	// printing nothing.
	if !hasHEAD(ctx, dir) {
		return HistoryPage{}, nil
	}
	// %h asks git for the short SHA in the same pass. Reading it per commit cost
	// one subprocess each, and this runs on every poll.
	args := []string{"log", fmt.Sprintf("--max-count=%d", logPageSize+1)}
	if skip > 0 {
		args = append(args, fmt.Sprintf("--skip=%d", skip))
	}
	args = append(args, "-z",
		"--format=%H%x00%h%x00%s%x00%an%x00%ct%x00")
	out, err := runRead(ctx, dir, args...)
	if err != nil {
		return HistoryPage{}, err
	}
	upstream := hasUpstream(ctx, dir)
	// One rev-list answers which commits have not reached upstream. Asking
	// merge-base once per commit would add a subprocess for every history row.
	unpushed, err := unpushedSHAs(ctx, dir, upstream)
	if err != nil {
		return HistoryPage{}, err
	}
	commits, err := parseLog(out, unpushed, upstream)
	if err != nil {
		return HistoryPage{}, err
	}
	page := HistoryPage{Commits: commits}
	if len(page.Commits) > logPageSize {
		page.HasMore = true
		page.Commits = page.Commits[:logPageSize]
	}
	return page, nil
}

// parseLog reads the five fields log prints per commit. Separated from the call
// that starts git so that the records a truncated read produces can be fed to
// it: the loop reads four past its own position, and a log that ends part way
// through a commit reaches past the end.
func parseLog(out []byte, unpushed map[string]bool, upstream bool) ([]CommitInfo, error) {
	records := strings.Split(string(out), "\x00")
	var commits []CommitInfo
	for i := 0; i < len(records); {
		sha := strings.TrimSpace(records[i])
		if sha == "" {
			i++
			continue
		}
		if i+4 >= len(records) {
			break
		}
		short := strings.TrimSpace(records[i+1])
		subject := records[i+2]
		author := records[i+3]
		when, err := strconv.ParseInt(strings.TrimSpace(records[i+4]), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("log: bad timestamp for %s: %w", sha, err)
		}
		commits = append(commits, CommitInfo{
			SHA:         sha,
			ShortSHA:    short,
			Subject:     subject,
			Author:      author,
			Unpushed:    !upstream || unpushed[sha],
			HasUpstream: upstream,
			Age:         formatAge(when),
		})
		i += 5
	}
	return commits, nil
}

// CommitFiles lists paths changed in a commit.
func CommitFiles(ctx context.Context, dir, sha string) ([]Entry, error) {
	counts, err := commitCounts(ctx, dir, sha)
	if err != nil {
		return nil, err
	}
	out, err := runRead(ctx, dir, "diff-tree", "--no-commit-id", "--name-status",
		"-M", "-r", "-z", "--root", sha)
	if err != nil {
		return nil, err
	}
	// A row that says M +0 −0 for every file cannot tell the reader which file
	// the commit was about, so the status letter and the counts come with it.
	return parseCommitFiles(out, counts), nil
}

// parseCommitFiles reads what diff-tree --name-status -z prints: a status field
// and a path for each file, and a second path for a rename. Separated from the
// call that starts git so that the fields a truncated read produces can be fed
// to it: the loop reads one field past its own position, and two for a rename.
func parseCommitFiles(out []byte, counts map[string]Count) []Entry {
	fields := splitNUL(out)
	var entries []Entry
	for i := 0; i < len(fields); i++ {
		status := fields[i]
		if status == "" || i+1 >= len(fields) {
			continue
		}
		i++
		path := fields[i]
		e := Entry{Path: path, Worktree: Kind(status[0])}
		// A rename reports the old path and then the new one.
		if status[0] == byte(Renamed) || status[0] == byte(Copied) {
			if i+1 >= len(fields) {
				continue
			}
			e.OldPath = path
			i++
			e.Path = fields[i]
		}
		// A status with no name after it is the same truncated read the bounds
		// above turn away, written one field longer. Kept, it becomes a row
		// with no name that the reader can arm and press a verb at.
		if e.Path == "" {
			continue
		}
		e.WorktreeCount = counts[e.Path]
		entries = append(entries, e)
	}
	return entries
}

func commitCounts(ctx context.Context, dir, sha string) (map[string]Count, error) {
	out, err := runRead(ctx, dir, "diff-tree", "--no-commit-id", "--numstat",
		"-M", "-r", "-z", "--root", sha)
	if err != nil {
		return nil, err
	}
	return parseNumstat(out), nil
}

// CommitDiff returns the patch a commit introduced for one path.
func CommitDiff(ctx context.Context, dir, sha, path string) (FileDiff, error) {
	parent, err := diffBase(ctx, dir, sha)
	if err != nil {
		return FileDiff{}, err
	}
	args := []string{"diff", parent, sha, "--", pathspec(path)}
	out, err := runRead(ctx, dir, args...)
	if err != nil {
		return FileDiff{}, err
	}
	files := parseDiff(out, path)
	if len(files) == 0 {
		return FileDiff{Path: path}, nil
	}
	return files[0], nil
}

// ResetSoft moves HEAD back one commit and keeps the tree staged.
// The root commit has no parent, so reset --soft HEAD^ fails; deleting the
// branch ref leaves the index and working tree as they were after that commit.
func ResetSoft(ctx context.Context, dir string) error {
	if !hasParent(ctx, dir, "HEAD") {
		_, err := runWrite(ctx, dir, "update-ref", "-d", "HEAD")
		return err
	}
	err := runWritePaths(ctx, dir, nil, "reset", "--soft", "HEAD^")
	return err
}

func hasParent(ctx context.Context, dir, sha string) bool {
	_, err := runRead(ctx, dir, "rev-parse", "--verify", sha+"^")
	return err == nil
}

// diffBase is what a commit is compared against to show what it changed: its
// first parent, or the empty tree for the commit that started the repository.
//
// git diff has no word for "against nothing". --root is git log's, and git diff
// takes it and ignores it: the reader who opened the commit that created a file
// was shown every change made to that file since, as one diff.
func diffBase(ctx context.Context, dir, sha string) (string, error) {
	if hasParent(ctx, dir, sha) {
		return sha + "^", nil
	}
	// Asked for rather than written out: the name of the empty tree is the hash
	// of nothing, and a repository can be built on a hash this one does not
	// know.
	out, err := runRead(ctx, dir, "hash-object", "-t", "tree", os.DevNull)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func formatAge(when int64) string {
	return FormatAge(time.Duration(time.Now().Unix()-when) * time.Second)
}

// FormatAge is how long ago something happened, in the one unit that fits. The
// commit rows and the fetch label are drawn on the same screen and answered
// this separately: a commit made ten seconds ago read "1m" while the header
// read "fetched 0m ago".
func FormatAge(elapsed time.Duration) string {
	if elapsed < 0 {
		elapsed = 0
	}
	switch {
	case elapsed >= 24*time.Hour:
		return fmt.Sprintf("%dd", int(elapsed/(24*time.Hour)))
	case elapsed >= time.Hour:
		return fmt.Sprintf("%dh", int(elapsed/time.Hour))
	default:
		return fmt.Sprintf("%dm", int(elapsed/time.Minute))
	}
}
