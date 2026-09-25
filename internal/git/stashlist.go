package git

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// StashStatus is whether a stash can pop onto the current HEAD. Unknown means
// the untracked side has not been checked yet, because merge-tree alone lies.
type StashStatus int

const (
	StashUnknown StashStatus = iota
	StashApplies
	StashConflicts
)

// StashRow is one entry in the stashed tab.
type StashRow struct {
	Ref       string
	SHA       string
	Message   string
	Branch    string
	FileCount int
	Age       string
	Status    StashStatus
	// Collides names the paths this stash would land on top of. It sits on the
	// stash rather than on each file because it is a fact about this stash
	// against the working tree as it is now, not about the file. The status is
	// read off the same list, so the two cannot disagree.
	Collides []string
}

// CollidesWith reports whether restoring this stash would land on path.
func (s StashRow) CollidesWith(path string) bool {
	for _, p := range s.Collides {
		if p == path {
			return true
		}
	}
	return false
}

// StashList returns stashes newest first, at one git for the whole list. What
// each stash holds is left for FillStashStatus: FileCount is -1 and Status is
// StashUnknown until then.
//
// The split is WorktreeList and FillWorktreeStatus again, for the same reason.
// Every reload reads this list so the bar's count follows the repository, and
// reading three or four gits per stash on the way took 2.5 s at ten stashes
// against 0.34 s for the whole of Load.
func StashList(ctx context.Context, dir string) ([]StashRow, error) {
	out, err := runRead(ctx, dir, "stash", "list", "--format=%H%x00%gd%x00%s%x00%ct%x00", "-z")
	if err != nil {
		return nil, err
	}
	return parseStashList(out)
}

// parseStashList reads the four fields stash list prints per entry. Separated
// from the call that starts git so that the records a truncated read produces
// can be fed to it: the loop indexes three past its own position, and a list
// that ends one field short reaches past the end.
func parseStashList(out []byte) ([]StashRow, error) {
	records := splitNUL(out)
	var rows []StashRow
	for i := 0; i < len(records); {
		sha := strings.TrimSpace(records[i])
		if sha == "" {
			i++
			continue
		}
		if i+3 >= len(records) {
			break
		}
		ref := records[i+1]
		subject := records[i+2]
		when, err := strconv.ParseInt(strings.TrimSpace(records[i+3]), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("stash list: bad timestamp for %s: %w", ref, err)
		}
		branch, message := parseStashSubject(subject)
		rows = append(rows, StashRow{
			Ref:       ref,
			SHA:       sha,
			Message:   message,
			Branch:    branch,
			FileCount: -1,
			Age:       formatAge(when),
			Status:    StashUnknown,
		})
		i += 4
	}
	return rows, nil
}

// FillStashStatus reads what one stash holds: how many files it has, and
// whether popping it onto the tree as it is now would conflict. The stashed tab
// is the only pane that draws either.
//
// head decides the second answer, so a repository with no commits gets the
// count and leaves the verdict at StashUnknown.
func FillStashStatus(ctx context.Context, dir string, row *StashRow, head Head) error {
	count, err := stashFileCount(ctx, dir, row.Ref)
	if err != nil {
		return err
	}
	row.FileCount = count
	if head.Initial {
		return nil
	}
	status, collides, err := stashStatusOf(ctx, dir, row.Ref)
	if err != nil {
		return err
	}
	row.Status, row.Collides = status, collides
	return nil
}

// StashPop applies a stash and removes it from the list. These three name a
// stash rather than paths, so they go through runWrite: runWritePaths appends
// --pathspec-from-file, which git rejects here with exit 129.
func StashPop(ctx context.Context, dir, ref string) error {
	_, err := runWrite(ctx, dir, "stash", "pop", ref)
	return err
}

// StashBranch checks out the stash's base commit on a new branch and applies the
// stash there, which succeeds even when pop would conflict on the current HEAD.
func StashBranch(ctx context.Context, dir, ref, branch string) error {
	_, err := runWrite(ctx, dir, "stash", "branch", branch, ref)
	return err
}

// stashDrop removes a stash without applying it.
func stashDrop(ctx context.Context, dir, ref string) error {
	_, err := runWrite(ctx, dir, "stash", "drop", ref)
	return err
}

// StashDropBySHA drops the stash whose commit SHA matches sha. Position specs
// like stash@{N} point at whatever occupies that slot after a list reload, so
// the ref is looked up from the current list right before drop.
func StashDropBySHA(ctx context.Context, dir, sha string) error {
	out, err := runRead(ctx, dir, "stash", "list", "--format=%H %gd")
	if err != nil {
		return err
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		if parts[0] == sha {
			return stashDrop(ctx, dir, parts[1])
		}
	}
	short := sha
	if len(short) > 7 {
		short = short[:7]
	}
	return fmt.Errorf("stash %s is gone; nothing dropped", short)
}

func parseStashSubject(subject string) (branch, message string) {
	for _, prefix := range []string{"WIP on ", "On "} {
		if !strings.HasPrefix(subject, prefix) {
			continue
		}
		rest := subject[len(prefix):]
		if i := strings.Index(rest, ": "); i >= 0 {
			return rest[:i], rest[i+2:]
		}
	}
	return "", subject
}

func stashFileCount(ctx context.Context, dir, ref string) (int, error) {
	out, err := runRead(ctx, dir, "diff", "--no-renames", "--name-only", "-z", ref+"^", ref)
	if err != nil {
		return 0, err
	}
	tracked := len(splitNULPaths(out))
	untracked, err := stashUntrackedPaths(ctx, dir, ref)
	if err != nil {
		return 0, err
	}
	return tracked + len(untracked), nil
}

// StashFiles lists what a stash holds. The tab exists because a count does not
// remind anyone what they put down, so the contents are read on demand for the
// row under the cursor rather than for every stash at once.
func StashFiles(ctx context.Context, dir, ref string) ([]Entry, error) {
	tracked, err := stashTrackedFiles(ctx, dir, ref)
	if err != nil {
		return nil, err
	}
	untracked, err := stashUntrackedFiles(ctx, dir, ref)
	if err != nil {
		return nil, err
	}
	// Concat rather than append: append writes into tracked's array when it has
	// room, and the sort below then rewrites the slice the caller of
	// stashTrackedFiles is holding.
	files := slices.Concat(tracked, untracked)
	byPath(files)
	return files, nil
}

// byPath puts the files of a stash in the order the tab draws them. A path can
// arrive from both halves — the same name tracked in the stash and untracked
// beside it — and a comparator that answers true for a pair that is equal is
// not an order at all: sort then arranges the equals however its partitioning
// happens to fall, and the row the reader opens is a different one each run.
func byPath(files []Entry) {
	sort.SliceStable(files, func(i, j int) bool {
		if files[i].Path != files[j].Path {
			return files[i].Path < files[j].Path
		}
		return files[i].OldPath < files[j].OldPath
	})
}

func stashTrackedFiles(ctx context.Context, dir, ref string) ([]Entry, error) {
	statusOut, err := runRead(ctx, dir, "diff", "--no-renames", "--name-status", "-z", ref+"^", ref)
	if err != nil {
		return nil, err
	}
	numstatOut, err := runRead(ctx, dir, "diff", "--no-renames", "--numstat", "-z", ref+"^", ref)
	if err != nil {
		return nil, err
	}
	return parseStashTrackedFiles(statusOut, parseNumstat(numstatOut)), nil
}

// parseStashTrackedFiles reads the status-and-path pairs of a stash's diff.
// Separated from the call that starts git so that a read cut anywhere can be
// fed to it: the first character of the status is what names the change, and a
// pair whose status arrived empty has no such character.
func parseStashTrackedFiles(statusOut []byte, counts map[string]Count) []Entry {
	var files []Entry
	fields := strings.Split(strings.TrimSuffix(string(statusOut), "\x00"), "\x00")
	for i := 0; i+1 < len(fields); i += 2 {
		status, path := fields[i], fields[i+1]
		if status == "" || path == "" {
			continue
		}
		// A stash holds one state, so its change sits on the working-tree side.
		files = append(files, Entry{
			Path: path, Worktree: Kind(status[0]), WorktreeCount: counts[path],
		})
	}
	return files
}

// stashUntrackedPaths lists paths held in a stash's untracked parent. Stashes
// without one return nil so callers do not treat a missing parent as an error.
func stashUntrackedPaths(ctx context.Context, dir, ref string) ([]string, error) {
	out, err := runRead(ctx, dir, "ls-tree", "-r", "--name-only", "-z", ref+"^3")
	if err != nil {
		var ee *ExitError
		if errors.As(err, &ee) {
			return nil, nil
		}
		return nil, err
	}
	return splitNULPaths(out), nil
}

// StashDiff returns the patch one path contributed to a stash commit. sha is
// the stash commit itself, not a position spec like stash@{N}, because that
// slot can point at a different commit after the list reloads.
func StashDiff(ctx context.Context, dir, sha, path string) (FileDiff, error) {
	untracked, err := stashUntrackedPaths(ctx, dir, sha)
	if err != nil {
		return FileDiff{}, err
	}
	for _, p := range untracked {
		if p == path {
			args := []string{"diff", "--root", sha + "^3", "--", pathspec(path)}
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
	}
	args := []string{"diff", sha + "^", sha, "--", pathspec(path)}
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

func stashUntrackedFiles(ctx context.Context, dir, ref string) ([]Entry, error) {
	paths, err := stashUntrackedPaths(ctx, dir, ref)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, nil
	}
	numstatOut, err := runRead(ctx, dir, "diff-tree", "-r", "--root", "--numstat", "-z", ref+"^3")
	if err != nil {
		return nil, err
	}
	counts := parseNumstat(numstatOut)
	files := make([]Entry, len(paths))
	for i, path := range paths {
		files[i] = Entry{Path: path, Worktree: Untracked, WorktreeCount: counts[path]}
	}
	return files, nil
}

// FreeBranchName answers base, or base with a number after it when base is
// taken. Which names are taken is a question about the repository, so it is
// answered here rather than by the caller reading the list and counting.
//
// A repository that cannot be read answers base: git refuses a name that
// exists, and the reader sees that refusal, which is better than a key that
// does nothing while the list is unavailable.
func FreeBranchName(ctx context.Context, dir, base string) string {
	taken, err := BranchNames(ctx, dir)
	if err != nil || !taken[base] {
		return base
	}
	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s-%d", base, n)
		if !taken[candidate] {
			return candidate
		}
	}
}

// BranchNames is what already exists, so a new branch can be given a name git
// will accept.
func BranchNames(ctx context.Context, dir string) (map[string]bool, error) {
	out, err := runRead(ctx, dir, "for-each-ref", "--format=%(refname:short)", "refs/heads/")
	if err != nil {
		return nil, err
	}
	names := map[string]bool{}
	for _, n := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if n != "" {
			names[n] = true
		}
	}
	return names, nil
}
