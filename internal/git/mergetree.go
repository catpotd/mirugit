package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// stashStatusOf decides whether a stash can pop onto the current HEAD, because
// merge-tree alone misses untracked parents and working-tree edits.
func stashStatusOf(ctx context.Context, dir, ref string) (StashStatus, []string, error) {
	paths, err := stashConflictPaths(ctx, dir, ref)
	if err != nil {
		return StashUnknown, nil, err
	}
	if len(paths) > 0 {
		return StashConflicts, paths, nil
	}
	return StashApplies, nil, nil
}

// stashEditedConflictPaths names stash paths the working tree already changed.
// merge-tree compares the stash against HEAD and never sees the working tree,
// so it calls such a stash clean while the real pop refuses and keeps it.
func stashEditedConflictPaths(ctx context.Context, dir, ref string) ([]string, error) {
	out, err := runRead(ctx, dir, "stash", "show", "--name-only", "-z", ref)
	if err != nil {
		return nil, err
	}
	inStash := map[string]bool{}
	for _, p := range splitNULPaths(out) {
		inStash[p] = true
	}
	if len(inStash) == 0 {
		return nil, nil
	}
	entries, _, err := Status(ctx, dir)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, e := range entries {
		if inStash[e.Path] {
			paths = append(paths, e.Path)
		}
	}
	if len(paths) == 0 {
		return nil, nil
	}
	unchanged, err := pathsUnchangedByStash(ctx, dir, ref, paths)
	if err != nil {
		return nil, err
	}
	filtered := paths[:0]
	for _, p := range paths {
		if !unchanged[p] {
			filtered = append(filtered, p)
		}
	}
	return filtered, nil
}

// pathsUnchangedByStash reports paths whose index and working tree already hold
// what the stash recorded, so applying it would change nothing and pop would
// not refuse.
func pathsUnchangedByStash(ctx context.Context, dir, ref string, paths []string) (map[string]bool, error) {
	stash, err := stashTreeEntries(ctx, dir, ref, paths)
	if err != nil {
		return nil, err
	}
	index, err := indexStageZeroEntries(ctx, dir, paths)
	if err != nil {
		return nil, err
	}
	edited, err := worktreeEditedPaths(ctx, dir, paths)
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool)
	for _, p := range paths {
		recorded, ok := stash[p]
		if !ok {
			continue
		}
		if index[p] != recorded {
			continue
		}
		if edited[p] {
			continue
		}
		out[p] = true
	}
	return out, nil
}

func stashTreeEntries(ctx context.Context, dir, ref string, paths []string) (map[string]string, error) {
	if len(paths) == 0 {
		return map[string]string{}, nil
	}
	args := append([]string{"ls-tree", "-r", "-z", ref}, pathspecsFor(paths)...)
	out, err := runRead(ctx, dir, args...)
	if err != nil {
		return nil, err
	}
	return parseTabbedEntries(out, treeBlob), nil
}

// parseTabbedEntries reads a NUL-delimited listing whose record is three or
// more space-separated fields, a tab, then the path. valueOf answers what to
// store for a record and whether to store it at all.
//
// The field count is checked here rather than in valueOf because it was
// checked in both callers, and a mutation showed one of the two could be
// broken without a test noticing: valueOf reads fields[2], so a two-field
// record reaches it as a panic.
func parseTabbedEntries(out []byte, valueOf func(fields []string) (string, bool)) map[string]string {
	entries := map[string]string{}
	for _, rec := range splitNUL(out) {
		if rec == "" {
			continue
		}
		tab := strings.IndexByte(rec, '\t')
		if tab < 0 {
			continue
		}
		fields := strings.Fields(rec[:tab])
		if len(fields) < 3 {
			continue
		}
		if value, ok := valueOf(fields); ok {
			entries[rec[tab+1:]] = value
		}
	}
	return entries
}

// treeBlob reads `ls-tree -r -z`: "<mode> <type> <sha>". Only a blob is a file
// with contents to compare.
func treeBlob(fields []string) (string, bool) {
	return fields[0] + " " + fields[2], fields[1] == "blob"
}

// indexStageZero reads `ls-files -s -z`: "<mode> <sha> <stage>". Stage 0 is the
// entry a file has when it is not in a conflict.
func indexStageZero(fields []string) (string, bool) {
	return fields[0] + " " + fields[1], fields[2] == "0"
}

func indexStageZeroEntries(ctx context.Context, dir string, paths []string) (map[string]string, error) {
	if len(paths) == 0 {
		return map[string]string{}, nil
	}
	args := append([]string{"ls-files", "-s", "-z"}, pathspecsFor(paths)...)
	out, err := runRead(ctx, dir, args...)
	if err != nil {
		return nil, err
	}
	return parseTabbedEntries(out, indexStageZero), nil
}

func worktreeEditedPaths(ctx context.Context, dir string, paths []string) (map[string]bool, error) {
	if len(paths) == 0 {
		return map[string]bool{}, nil
	}
	args := append([]string{"diff", "--name-only", "-z"}, pathspecsFor(paths)...)
	out, err := runRead(ctx, dir, args...)
	if err != nil {
		return nil, err
	}
	edited := map[string]bool{}
	for _, p := range splitNULPaths(out) {
		edited[p] = true
	}
	return edited, nil
}

func pathspecsFor(paths []string) []string {
	specs := make([]string, len(paths))
	for i, p := range paths {
		specs[i] = pathspec(p)
	}
	return append([]string{"--"}, specs...)
}

func headSHA(ctx context.Context, dir string) (string, error) {
	out, err := runRead(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// mergeTreeResult holds one merge-tree run. Err is kept rather than returned so
// a caller that knows about collisions merge-tree cannot see can add them
// before deciding the run failed.
type mergeTreeResult struct {
	Paths []string
	Seen  map[string]bool
	Err   error
}

// runMergeTree names the paths two revisions cannot merge. The exit code does
// not separate a conflict from a failure: measured on git 2.50.1, a real
// conflict and an unresolvable revision ("nosuchref - not something we can
// merge") both exit 1, and bad options exit 129. The stage lines do separate
// them, because only a conflict lists them.
func runMergeTree(ctx context.Context, dir, base, other string) (mergeTreeResult, error) {
	out, err := runRead(ctx, dir, "merge-tree", "--write-tree", base, other)
	text := string(out)
	var exitErr error
	if err != nil {
		var ee *ExitError
		if !errors.As(err, &ee) {
			return mergeTreeResult{}, err
		}
		exitErr = err
		text += ee.Stderr
	}
	paths, seen := mergeTreeConflictPaths(text)
	return mergeTreeResult{Paths: paths, Seen: seen, Err: exitErr}, nil
}

// merge-tree lists one line per unmerged stage; stage 0 means merged.
func mergeTreeConflictPaths(text string) (paths []string, seen map[string]bool) {
	seen = map[string]bool{}
	for _, line := range strings.Split(text, "\n") {
		mode, rest, ok := strings.Cut(line, " ")
		if !ok || len(mode) != 6 {
			continue
		}
		_, rest, ok = strings.Cut(rest, " ")
		if !ok {
			continue
		}
		stage, path, ok := strings.Cut(rest, "\t")
		if !ok || stage == "0" || path == "" || seen[path] {
			continue
		}
		seen[path] = true
		paths = append(paths, path)
	}
	return paths, seen
}

func appendConflictPaths(seen map[string]bool, paths []string, more []string) []string {
	for _, p := range more {
		if !seen[p] {
			seen[p] = true
			paths = append(paths, p)
		}
	}
	return paths
}

// stashConflictPaths names the paths that would collide, so a row can say which
// file is the problem rather than only that there is one.
func stashConflictPaths(ctx context.Context, dir, ref string) ([]string, error) {
	headSHA, err := headSHA(ctx, dir)
	if err != nil {
		return nil, err
	}
	merge, err := runMergeTree(ctx, dir, headSHA, ref)
	if err != nil {
		return nil, err
	}
	paths, seen := merge.Paths, merge.Seen
	untracked, err := stashUntrackedConflictPaths(ctx, dir, ref)
	if err != nil {
		return nil, err
	}
	paths = appendConflictPaths(seen, paths, untracked)
	edited, err := stashEditedConflictPaths(ctx, dir, ref)
	if err != nil {
		return nil, err
	}
	paths = appendConflictPaths(seen, paths, edited)
	if merge.Err != nil && len(paths) == 0 {
		return nil, merge.Err
	}
	return paths, nil
}

// stashUntrackedConflictPaths names untracked paths from the stash that already
// exist in the working tree. merge-tree does not see the untracked parent, so
// those collisions are invisible to merge-tree alone.
func stashUntrackedConflictPaths(ctx context.Context, dir, ref string) ([]string, error) {
	paths, err := stashUntrackedPaths(ctx, dir, ref)
	if err != nil {
		return nil, err
	}
	var conflicts []string
	for _, path := range paths {
		if _, err := os.Stat(filepath.Join(dir, path)); err == nil {
			conflicts = append(conflicts, path)
		}
	}
	return conflicts, nil
}
