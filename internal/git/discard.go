package git

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

// Discard removes working-tree changes against a snapshot, because callers must
// not reach discard without one.
func Discard(ctx context.Context, dir string, entries []Entry, undo Undo) (Outcome, error) {
	if undo.Commit == "" {
		return Outcome{}, errMissingUndo
	}
	if len(entries) == 0 {
		return Outcome{}, nil
	}

	before, _, err := Status(ctx, dir)
	if err != nil {
		return Outcome{}, err
	}
	beforeByPath := indexByPath(before)

	protected, err := protectedSnapshotPaths(ctx, dir, entries)
	if err != nil {
		return Outcome{}, err
	}
	stagedOnly, fullRestore, cleanPaths := classifyDiscardTargets(entries, beforeByPath, protected)

	if err := runDiscardWrites(ctx, dir, stagedOnly, fullRestore, cleanPaths); err != nil {
		return Outcome{}, err
	}

	after, _, err := Status(ctx, dir)
	if err != nil {
		return Outcome{}, err
	}
	return diffOutcome(entries, beforeByPath, indexByPath(after)), nil
}

func classifyDiscardTargets(entries []Entry, beforeByPath map[string]Entry, protected []string) (stagedOnly, fullRestore, cleanPaths []string) {
	seen := map[string]bool{}
	for _, e := range entries {
		if seen[e.Path] {
			continue
		}
		seen[e.Path] = true
		cur, ok := beforeByPath[e.Path]
		if !ok {
			continue
		}
		if isUntrackedOnly(cur) {
			cleanPaths = append(cleanPaths, pathspec(e.Path))
			continue
		}
		paths := e.Pathspecs()
		if entryCollidesProtected(e, protected) {
			stagedOnly = append(stagedOnly, paths...)
		} else {
			fullRestore = append(fullRestore, paths...)
		}
	}
	return stagedOnly, fullRestore, cleanPaths
}

func runDiscardWrites(ctx context.Context, dir string, stagedOnly, fullRestore, cleanPaths []string) error {
	if len(stagedOnly) > 0 {
		if err := runWritePaths(ctx, dir, stagedOnly, "restore", "--staged"); err != nil {
			return err
		}
	}
	if len(fullRestore) > 0 {
		if hasHEAD(ctx, dir) {
			if err := runWritePaths(ctx, dir, fullRestore, "restore", "--staged", "--worktree"); err != nil {
				return err
			}
		} else if err := runWritePaths(ctx, dir, fullRestore, "rm", "-f", "-r"); err != nil {
			return err
		}
	}
	if len(cleanPaths) > 0 {
		if _, err := runClean(ctx, dir, cleanPaths); err != nil {
			return err
		}
	}
	return nil
}

// Undiscard writes the snapshot trees back over the working tree and index,
// because discard removed both and undo must put each side back.
func Undiscard(ctx context.Context, dir string, undo Undo) (Outcome, error) {
	if undo.Commit == "" {
		return Outcome{}, errMissingUndo
	}

	// Only what the discard covered. Every other path in the snapshot tree
	// holds HEAD's content, so restoring the tree would revert edits the
	// discard never touched.
	names := undo.Paths
	if len(names) == 0 {
		return Outcome{}, nil
	}
	paths := pathspecs(names)

	before, _, err := Status(ctx, dir)
	if err != nil {
		return Outcome{}, err
	}
	beforeByPath := indexByPath(before)
	beforeWorktree := worktreeBytesFor(dir, names)

	if err := runWritePaths(ctx, dir, paths, "restore", "--source="+undo.Commit, "--worktree"); err != nil {
		return Outcome{}, err
	}
	if len(undo.IndexPaths) > 0 && undo.IndexCommit != "" {
		idxPaths := pathspecs(undo.IndexPaths)
		if err := runWritePaths(ctx, dir, idxPaths, "restore", "--source="+undo.IndexCommit, "--staged"); err != nil {
			return Outcome{}, err
		}
	}
	if err := restoreUnmerged(ctx, dir, undo.Unmerged); err != nil {
		return Outcome{}, err
	}

	after, _, err := Status(ctx, dir)
	if err != nil {
		return Outcome{}, err
	}

	return undiscardOutcome(dir, names, beforeWorktree, beforeByPath, indexByPath(after)), nil
}

// RecordWorktreeAfter stores each path's worktree digest after discard, so a
// later undo can tell edits made since from the tree discard left behind.
func RecordWorktreeAfter(dir string, undo Undo) Undo {
	undo.WorktreeAfter = make(map[string]string, len(undo.Paths))
	for _, path := range undo.Paths {
		// A path that cannot be read answers "". The undo side answers the same
		// way, so a path that stays unreadable does not turn into "edited".
		digest, err := worktreeDigest(dir, path)
		if err != nil {
			digest = ""
		}
		undo.WorktreeAfter[path] = digest
	}
	return undo
}

// PathsEditedAfterDiscard lists undo paths whose worktree bytes changed since
// discard finished. A discard with no recorded digests returns every path:
// undo overwrites the worktree, and "we cannot tell" must not read as
// "nothing was edited".
//
// It reads one file per path, so the context stops it between files rather than
// inside one. It takes a context even though it starts no git: a rule with an
// exception in it makes a reader open every signature to find out which kind
// they are looking at.
func PathsEditedAfterDiscard(ctx context.Context, dir string, undo Undo) ([]string, error) {
	if len(undo.WorktreeAfter) == 0 {
		return append([]string(nil), undo.Paths...), nil
	}
	var edited []string
	for _, path := range undo.Paths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		want := undo.WorktreeAfter[path]
		got, err := worktreeDigest(dir, path)
		if err != nil {
			return nil, err
		}
		if got != want {
			edited = append(edited, path)
		}
	}
	return edited, nil
}

// worktreeDigest returns "" for a path that is not there, because a discard
// that deleted the file records the same empty digest.
func worktreeDigest(dir, path string) (string, error) {
	b, err := os.ReadFile(filepath.Join(dir, path))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func readWorktreeBytes(dir, path string) ([]byte, bool) {
	b, err := os.ReadFile(filepath.Join(dir, path))
	if err != nil {
		return nil, false
	}
	return b, true
}

func worktreeBytesFor(dir string, paths []string) map[string][]byte {
	out := make(map[string][]byte, len(paths))
	for _, path := range paths {
		if b, ok := readWorktreeBytes(dir, path); ok {
			out[path] = b
		}
	}
	return out
}

func undiscardOutcome(dir string, names []string, beforeWorktree map[string][]byte, before, after map[string]Entry) Outcome {
	entries := make([]Entry, len(names))
	for i, name := range names {
		entries[i] = Entry{Path: name}
	}
	return outcomeByPath(entries, func(e Entry) bool {
		return worktreeRestored(dir, e.Path, beforeWorktree) ||
			entryChanged(before[e.Path], after[e.Path])
	})
}

func worktreeRestored(dir, path string, before map[string][]byte) bool {
	prev, hadPrev := before[path]
	cur, hasCur := readWorktreeBytes(dir, path)
	if hadPrev != hasCur {
		return true
	}
	return !bytes.Equal(prev, cur)
}

var errMissingUndo = &missingUndoError{}

type missingUndoError struct{}

func (e *missingUndoError) Error() string { return "discard: missing undo snapshot" }

func isUntrackedOnly(e Entry) bool {
	return e.Worktree == Untracked && !e.IsStaged()
}

func indexByPath(entries []Entry) map[string]Entry {
	m := make(map[string]Entry, len(entries))
	for _, e := range entries {
		m[e.Path] = e
	}
	return m
}

func diffOutcome(targets []Entry, before, after map[string]Entry) Outcome {
	return outcomeByPath(targets, func(e Entry) bool {
		return entryChanged(before[e.Path], after[e.Path])
	})
}

func runClean(ctx context.Context, dir string, paths []string) ([]byte, error) {
	args := append([]string{"clean", "-f"}, paths...)
	return execGit(ctx, dir, nil, nil, args)
}

func entryCollidesProtected(e Entry, protected []string) bool {
	for _, ps := range e.Pathspecs() {
		path := strings.TrimPrefix(ps, ":(literal)")
		for _, p := range protected {
			if pathCollides(path, p) {
				return true
			}
		}
	}
	return false
}

func protectedSnapshotPaths(ctx context.Context, dir string, entries []Entry) ([]string, error) {
	snapshotted, err := snapshotPathsList(ctx, dir, entries)
	if err != nil {
		return nil, err
	}
	selected := entryPathSet(entries)
	var out []string
	for _, p := range snapshotted {
		if !selected[p] {
			out = append(out, p)
		}
	}
	return out, nil
}

func snapshotPathsList(ctx context.Context, dir string, entries []Entry) ([]string, error) {
	specs, err := snapshotPaths(ctx, dir, entries)
	if err != nil {
		return nil, err
	}
	out := make([]string, len(specs))
	for i, ps := range specs {
		out[i] = strings.TrimPrefix(ps, ":(literal)")
	}
	return out, nil
}

func entryPathSet(entries []Entry) map[string]bool {
	m := map[string]bool{}
	for _, e := range entries {
		m[e.Path] = true
		if e.OldPath != "" {
			m[e.OldPath] = true
		}
	}
	return m
}

func entryChanged(before, after Entry) bool {
	if before.Path == "" && after.Path != "" {
		return true
	}
	if before.Path != "" && after.Path == "" {
		return true
	}
	return before.Index != after.Index || before.Worktree != after.Worktree
}

// restoreUnmerged puts a conflicted path's index entries back, one per merge
// stage. They cannot travel in a tree — write-tree refuses an index holding a
// path more than once — so they go back as the lines they came out as.
//
// The discard resolved the path to one entry, and update-index replaces it with
// the three only after that one is cleared: git rejects adding a staged entry
// beside an unmerged one for the same path.
func restoreUnmerged(ctx context.Context, dir string, lines []string) error {
	if len(lines) == 0 {
		return nil
	}
	clear := make([]string, 0, len(lines))
	seen := map[string]bool{}
	for _, line := range lines {
		tab := strings.IndexByte(line, '\t')
		if tab < 0 || seen[line[tab+1:]] {
			continue
		}
		seen[line[tab+1:]] = true
		clear = append(clear, clearLineFor(line))
	}
	stdin := []byte(strings.Join(append(clear, lines...), "\x00") + "\x00")
	_, err := execGit(ctx, dir, nil, stdin, []string{"update-index", "-z", "--index-info"})
	return err
}

// clearLineFor is the index-info line that removes every entry for a path, in
// the form the entry beside it is written: an object id is 40 characters in a
// SHA-1 repository and 64 in a SHA-256 one, and a zero id of the wrong length
// is one git refuses. The length comes off the entry being restored rather
// than from a constant, so nothing here has to know which kind this is.
func clearLineFor(line string) string {
	tab := strings.IndexByte(line, '\t')
	fields := strings.Fields(line[:tab])
	width := 40
	if len(fields) == 3 {
		width = len(fields[1])
	}
	return "0 " + strings.Repeat("0", width) + "\t" + line[tab+1:]
}
