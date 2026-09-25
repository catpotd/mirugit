package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MergeStatus is whether a branch would merge cleanly onto the base. Unknown
// means the check has not finished yet.
type MergeStatus int

const (
	MergeUnknown MergeStatus = iota
	MergeClean
	MergeConflicts
)

const worktreeLoading = -1

// WorktreeRow is one linked worktree in the worktrees tab.
type WorktreeRow struct {
	Path      string
	Name      string
	Branch    string
	SHA       string
	Ahead     int
	Behind    int
	Dirty     int // worktreeLoading until status returns
	WroteAge  string
	Merge     MergeStatus
	Conflicts int
	Main      bool // git refuses to remove the tree the repository was cloned into
}

// CanRemove reports whether remove may be offered. git refuses the main tree
// and a tree holding uncommitted work, exiting 128 on both, so the offer is
// withheld rather than shown and then rejected. Unpushed commits are not a
// reason: removing a tree leaves its branch and its commits in place.
func (r WorktreeRow) CanRemove() bool {
	return !r.Main && r.Dirty == 0
}

// defaultBase picks the branch worktrees are measured against. The remote
// default is the usual merge target; falling back to main then master keeps
// bare clones without origin usable.
func defaultBase(ctx context.Context, dir string) string {
	out, err := runRead(ctx, dir, "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	if err == nil {
		ref := strings.TrimSpace(string(out))
		return strings.TrimPrefix(ref, "origin/")
	}
	for _, name := range []string{"main", "master"} {
		if _, err := runRead(ctx, dir, "rev-parse", "--verify", name); err == nil {
			return name
		}
	}
	return "main"
}

// WorktreeList returns every linked worktree with branch distance from one
// for-each-ref call. Status fields stay at their loading sentinel until filled.
func WorktreeList(ctx context.Context, dir string) ([]WorktreeRow, string, error) {
	base := defaultBase(ctx, dir)
	byBranch, err := branchRefs(ctx, dir, base)
	if err != nil {
		return nil, "", err
	}
	trees, err := linkedWorktrees(ctx, dir)
	if err != nil {
		return nil, "", err
	}
	rows := make([]WorktreeRow, 0, len(trees))
	for i, wt := range trees {
		ref := byBranch[wt.branch]
		row := WorktreeRowSkeleton(wt.path, filepath.Base(wt.path), wt.branch)
		// git lists the main tree first, and refuses to remove it.
		row.Main = i == 0
		if ref.ok {
			row.SHA = ref.sha
			row.Ahead = ref.ahead
			row.Behind = ref.behind
		}
		rows = append(rows, row)
	}
	return rows, base, nil
}

type branchRef struct {
	sha    string
	ahead  int
	behind int
	ok     bool
}

func branchRefs(ctx context.Context, dir, base string) (map[string]branchRef, error) {
	// ahead-behind needs a ref to compare against, and a repository with no
	// commits has none. Without the distance the rest of the row still reads.
	distance := "%(ahead-behind:" + base + ")"
	if !refExists(ctx, dir, base) {
		distance = ""
	}
	format := "%(refname:short)%00" + distance + "%00%(objectname)%00%(worktreepath)"
	out, err := runRead(ctx, dir, "for-each-ref", "--format="+format, "refs/heads/")
	if err != nil {
		return nil, err
	}
	return parseBranchRefs(out), nil
}

func parseBranchRefs(out []byte) map[string]branchRef {
	refs := map[string]branchRef{}
	for _, line := range strings.Split(strings.TrimSuffix(string(out), "\n"), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\x00")
		if len(parts) < 3 {
			continue
		}
		branch := strings.TrimSpace(parts[0])
		if branch == "" {
			continue
		}
		ahead, behind := parseTwoCounts(parts[1])
		refs[branch] = branchRef{
			sha:    strings.TrimSpace(parts[2]),
			ahead:  ahead,
			behind: behind,
			ok:     true,
		}
	}
	return refs
}

type linkedWorktree struct {
	path   string
	branch string
}

func linkedWorktrees(ctx context.Context, dir string) ([]linkedWorktree, error) {
	out, err := runRead(ctx, dir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	return parseWorktreeList(out), nil
}

// parseWorktreeList reads what worktree list --porcelain prints: a block per
// tree, blocks separated by a blank line, and the last one ended by the end of
// the output rather than by a blank. Separated from the call that starts git so
// that the records a truncated read produces can be fed to it.
func parseWorktreeList(out []byte) []linkedWorktree {
	var trees []linkedWorktree
	var cur linkedWorktree
	for _, line := range strings.Split(string(out), "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			if cur.path != "" {
				trees = append(trees, cur)
			}
			cur = linkedWorktree{path: line[len("worktree "):]}
		case strings.HasPrefix(line, "branch "):
			ref := line[len("branch "):]
			cur.branch = strings.TrimPrefix(ref, "refs/heads/")
		case line == "detached":
			cur.branch = "(detached)"
		case line == "" && cur.path != "":
			trees = append(trees, cur)
			cur = linkedWorktree{}
		}
	}
	if cur.path != "" {
		trees = append(trees, cur)
	}
	return trees
}

// FillWorktreeStatus loads dirty count, last write and merge verdict for one
// tree. It is meant to run in the background after the list.
func FillWorktreeStatus(ctx context.Context, row *WorktreeRow, base string) error {
	entries, _, err := Status(ctx, row.Path)
	if err != nil {
		return err
	}
	row.Dirty = len(entries)
	if row.Dirty == 0 {
		row.WroteAge = ""
	} else if wrote, ok := latestWrite(row.Path, entries); ok {
		row.WroteAge = formatWrote(wrote)
	} else {
		row.WroteAge = ""
	}
	status, conflicts, err := branchMergeStatus(ctx, row.Path, base, row.Branch)
	if err != nil {
		return err
	}
	row.Merge = status
	row.Conflicts = conflicts
	return nil
}

func latestWrite(dir string, entries []Entry) (int64, bool) {
	var latest int64
	found := false
	for _, e := range entries {
		path := e.Path
		if e.OldPath != "" {
			path = e.OldPath
		}
		stat, err := os.Stat(filepath.Join(dir, path))
		if err != nil {
			continue
		}
		m := stat.ModTime().Unix()
		if !found || m > latest {
			latest = m
			found = true
		}
	}
	return latest, found
}

func formatWrote(when int64) string {
	return "wrote " + formatAge(when)
}

func branchMergeStatus(ctx context.Context, dir, base, branch string) (MergeStatus, int, error) {
	if branch == "" || branch == "(detached)" {
		return MergeClean, 0, nil
	}
	// Nothing to merge onto before the first commit, and rev-parse exits 128
	// rather than reporting an empty revision.
	if !refExists(ctx, dir, base) {
		return MergeClean, 0, nil
	}
	baseSHA, err := revParseIn(ctx, dir, base)
	if err != nil {
		return MergeUnknown, 0, err
	}
	branchSHA, err := revParseIn(ctx, dir, branch)
	if err != nil {
		return MergeUnknown, 0, err
	}
	if baseSHA == branchSHA {
		return MergeClean, 0, nil
	}
	merge, err := runMergeTree(ctx, dir, baseSHA, branchSHA)
	if err != nil {
		return MergeUnknown, 0, err
	}
	// A failed run used to be reported as a conflict, so a bad revision showed
	// the reader "conflicts 1" on a branch that merges.
	if merge.Err != nil && len(merge.Paths) == 0 {
		return MergeUnknown, 0, merge.Err
	}
	if len(merge.Paths) > 0 {
		return MergeConflicts, len(merge.Paths), nil
	}
	return MergeClean, 0, nil
}

func refExists(ctx context.Context, dir, ref string) bool {
	_, err := runRead(ctx, dir, "rev-parse", "--verify", "-q", ref)
	return err == nil
}

func revParseIn(ctx context.Context, dir, ref string) (string, error) {
	out, err := runRead(ctx, dir, "rev-parse", ref)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// RemoveWorktree deletes a linked worktree. It names a directory rather than
// paths, so it goes through runWrite: runWritePaths appends
// --pathspec-from-file, which git rejects here with exit 129. git refuses a dirty tree and the main tree on
// its own, both with exit 128.
func RemoveWorktree(ctx context.Context, repoDir, path string) error {
	_, err := runWrite(ctx, repoDir, "worktree", "remove", path)
	return err
}

// WorktreeRowSkeleton is the row a worktree gets before its status arrives.
// WorktreeList returns these and FillWorktreeStatus replaces the sentinels, so
// the pane draws the list one git call after asking for it rather than after
// one call per tree.
func WorktreeRowSkeleton(path, name, branch string) WorktreeRow {
	return WorktreeRow{
		Path:     path,
		Name:     name,
		Branch:   branch,
		Dirty:    worktreeLoading,
		WroteAge: "…",
		Merge:    MergeUnknown,
	}
}

// FormatWorktreeAheadBehind renders distance from base for the row.
func FormatWorktreeAheadBehind(ahead, behind int) string {
	var parts []string
	if ahead > 0 {
		parts = append(parts, fmt.Sprintf("↑%d", ahead))
	}
	if behind > 0 {
		parts = append(parts, fmt.Sprintf("↓%d", behind))
	}
	return strings.Join(parts, " ")
}

// WorktreeFiles lists what one linked tree has changed. It is read for the row
// under the cursor only, because a status call per tree would cost the list its
// speed on a repository with many trees.
func WorktreeFiles(ctx context.Context, path string) ([]Entry, error) {
	entries, _, err := Status(ctx, path)
	if err != nil {
		return nil, err
	}
	counts, err := Counts(ctx, path, false)
	if err != nil {
		return nil, err
	}
	staged, err := Counts(ctx, path, true)
	if err != nil {
		return nil, err
	}
	files := make([]Entry, 0, len(entries))
	for _, e := range entries {
		c, ok := counts[e.Path]
		if !ok {
			c = staged[e.Path]
		}
		// The change goes on the side it is on: asking git for the other side's
		// diff answers with nothing for a file that plainly changed.
		out := Entry{Path: e.Path}
		if e.IsStaged() && !e.IsUnstaged() {
			out.Index, out.IndexCount = kindOfEntry(e), c
		} else {
			out.Worktree, out.WorktreeCount = kindOfEntry(e), c
		}
		files = append(files, out)
	}
	return files, nil
}

func kindOfEntry(e Entry) Kind {
	if e.Worktree != Unchanged {
		return e.Worktree
	}
	return e.Index
}
