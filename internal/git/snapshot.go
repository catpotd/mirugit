package git

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const undoRefPrefix = "refs/mirugit/undo/"

// Undo names commits that hold what a discard is about to remove, so the
// removal has somewhere to come back from.
type Undo struct {
	Ref    string
	Commit string
	// Paths is what the discard covered. The snapshot tree is HEAD with those
	// laid over it, so every other path in it holds HEAD's content and
	// restoring the whole tree would throw away edits the discard never saw.
	Paths []string
	// IndexCommit holds the index tree at snapshot time. Undiscard uses it to
	// put staged paths back the way discard had found them.
	IndexCommit string
	// IndexPaths lists paths whose index entry must be restored. HEAD-tracked
	// paths are included even when the index had no blob (staged deletion).
	IndexPaths []string
	// Unmerged holds the index entries of a path a merge left in conflict, one
	// per stage, in the form update-index --index-info reads. A tree cannot
	// carry them: a path sits in the index three times while it is unmerged,
	// and write-tree refuses an index in that state. So they travel as the
	// lines themselves and go back the way they came out.
	Unmerged []string
	// WorktreeAfter holds a digest of each path's worktree bytes right after
	// discard finished, so undo can tell post-discard edits from the tree
	// discard left behind.
	WorktreeAfter map[string]string
}

// undoRefName carries the commit as well as the second, because update-ref
// overwrites a name that is already taken and the ref is the only thing keeping
// the commit from being collected. Two windows on one repository can discard
// within the same second, and the second alone named them both the same. The
// commit follows from the content, so a discard repeated with nothing changed
// lands on the same name and overwrites a ref that names the same commit.
func undoRefName(second int64, commit string) string {
	return undoRefPrefix + strconv.FormatInt(second, 10) + "-" + commit[:min(8, len(commit))]
}

// Snapshot writes temporary-index commits under refs/mirugit/undo, because
// git stash create drops untracked files and fails on intent-to-add entries.
// The worktree commit holds working-tree content; a second commit holds the
// index so undo can restore staged paths.
func Snapshot(ctx context.Context, dir string, entries []Entry, reason string) (Undo, error) {
	paths, err := snapshotPaths(ctx, dir, entries)
	if err != nil {
		return Undo{}, err
	}
	if len(paths) == 0 {
		return Undo{}, fmt.Errorf("snapshot: no paths")
	}

	specs := pathspecs(paths)
	wtTree, headTracked, err := worktreeTree(ctx, dir, specs)
	if err != nil {
		return Undo{}, err
	}
	idxTree, staged, unmerged, err := indexTree(ctx, dir, specs)
	if err != nil {
		return Undo{}, err
	}
	// A path the merge left unmerged has no single index entry to restore from
	// the tree, so it is kept out of the list the tree answers for and put back
	// from its own stages instead.
	indexPaths := indexPathsFrom(paths, headTracked, staged, unmergedPaths(unmerged))

	commit, indexCommit, err := undoCommits(ctx, dir, wtTree, idxTree, indexPaths, reason)
	if err != nil {
		return Undo{}, err
	}

	ref := undoRefName(time.Now().Unix(), commit)
	if _, err := runWrite(ctx, dir, "update-ref", ref, commit); err != nil {
		return Undo{}, err
	}
	return Undo{
		Ref: ref, Commit: commit, Paths: paths,
		IndexCommit: indexCommit, IndexPaths: indexPaths,
		Unmerged: unmerged,
	}, nil
}

// GitDir names the directory git keeps this worktree's state in. A linked
// worktree's .git is a file holding "gitdir: ...", so anything that wants to
// watch or write next to the index has to ask rather than join a path.
func GitDir(ctx context.Context, dir string) (string, error) {
	out, err := runRead(ctx, dir, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func newTempIndex(ctx context.Context, dir string) (env []string, cleanup func(), err error) {
	// A linked worktree's .git is a file holding "gitdir: ...", not a directory,
	// so the index has to go where git itself keeps this worktree's state. The
	// system temp directory is not an option: git requires the index on the same
	// filesystem as the object store.
	gitDir, err := GitDir(ctx, dir)
	if err != nil {
		return nil, nil, err
	}
	indexFile, err := os.CreateTemp(gitDir, "mirugit-index-*")
	if err != nil {
		return nil, nil, err
	}
	indexPath := indexFile.Name()
	if err := indexFile.Close(); err != nil {
		_ = os.Remove(indexPath)
		return nil, nil, err
	}
	cleanup = func() { _ = os.Remove(indexPath) }
	return []string{"GIT_INDEX_FILE=" + indexPath}, cleanup, nil
}

func worktreeTree(ctx context.Context, dir string, specs []string) (tree string, headTracked []string, err error) {
	env, cleanup, err := newTempIndex(ctx, dir)
	if err != nil {
		return "", nil, err
	}
	defer cleanup()

	if hasHEAD(ctx, dir) {
		if _, err := runWithEnv(ctx, dir, env, "read-tree", "HEAD"); err != nil {
			return "", nil, err
		}
	} else if _, err := runWithEnv(ctx, dir, env, "read-tree", "--empty"); err != nil {
		return "", nil, err
	}

	lsArgs := append([]string{"ls-files", "-z", "--"}, specs...)
	out, err := runWithEnv(ctx, dir, env, lsArgs...)
	if err != nil {
		return "", nil, err
	}
	headTracked = splitNUL(out)

	if err := runWriteEnvPaths(ctx, dir, env, specs, "add"); err != nil {
		return "", nil, err
	}
	treeOut, err := runWithEnv(ctx, dir, env, "write-tree")
	if err != nil {
		return "", nil, err
	}
	return strings.TrimSpace(string(treeOut)), headTracked, nil
}

func indexTree(ctx context.Context, dir string, specs []string) (tree string, staged, unmerged []string, err error) {
	lsArgs := append([]string{"ls-files", "-s", "-z", "--"}, specs...)
	out, err := runRead(ctx, dir, lsArgs...)
	if err != nil {
		return "", nil, nil, err
	}
	infoLines, staged, unmerged := parseIndexInfo(out)

	env, cleanup, err := newTempIndex(ctx, dir)
	if err != nil {
		return "", nil, nil, err
	}
	defer cleanup()

	if _, err := runWithEnv(ctx, dir, env, "read-tree", "--empty"); err != nil {
		return "", nil, nil, err
	}
	if len(infoLines) > 0 {
		stdin := []byte(strings.Join(infoLines, "\x00") + "\x00")
		if _, err := execGit(ctx, dir, env, stdin, []string{"update-index", "-z", "--index-info"}); err != nil {
			return "", nil, nil, err
		}
	}
	treeOut, err := runWithEnv(ctx, dir, env, "write-tree")
	if err != nil {
		return "", nil, nil, err
	}
	return strings.TrimSpace(string(treeOut)), staged, unmerged, nil
}

func undoCommits(ctx context.Context, dir string, wtTree, idxTree string, indexPaths []string, reason string) (commit, indexCommit string, err error) {
	if len(indexPaths) > 0 {
		indexCommit, err = commitTree(ctx, dir, idxTree, nil, reason+" (index)")
		if err != nil {
			return "", "", err
		}
	}

	var parents []string
	if hasHEAD(ctx, dir) {
		head, err := runRead(ctx, dir, "rev-parse", "HEAD")
		if err != nil {
			return "", "", err
		}
		parents = append(parents, strings.TrimSpace(string(head)))
	}
	if indexCommit != "" {
		parents = append(parents, indexCommit)
	}
	commit, err = commitTree(ctx, dir, wtTree, parents, reason)
	if err != nil {
		return "", "", err
	}
	return commit, indexCommit, nil
}

func commitTree(ctx context.Context, dir string, tree string, parents []string, message string) (string, error) {
	args := make([]string, 0, 4+2*len(parents))
	args = append(args, "commit-tree", tree)
	for _, p := range parents {
		args = append(args, "-p", p)
	}
	args = append(args, "-m", message)
	out, err := runWrite(ctx, dir, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func indexPathsFrom(names, headTracked, staged, unmerged []string) []string {
	set := map[string]bool{}
	for _, p := range headTracked {
		set[p] = true
	}
	for _, p := range staged {
		set[p] = true
	}
	for _, p := range unmerged {
		delete(set, p)
	}
	var out []string
	for _, n := range names {
		if set[n] {
			out = append(out, n)
		}
	}
	return out
}

// parseIndexInfo splits the index listing three ways: the lines a tree can hold
// (stage 0), the paths those lines name, and the lines of a path a merge left
// unmerged, which a tree cannot hold at all.
func parseIndexInfo(out []byte) (lines, staged, unmerged []string) {
	for _, entry := range splitNUL(out) {
		tab := strings.IndexByte(entry, '\t')
		if tab < 0 {
			continue
		}
		meta, path := entry[:tab], entry[tab+1:]
		fields := strings.Fields(meta)
		if len(fields) != 3 || isZeroOID(fields[1]) {
			continue
		}
		if fields[2] != "0" {
			unmerged = append(unmerged, entry)
			continue
		}
		lines = append(lines, entry)
		staged = append(staged, path)
	}
	return lines, staged, unmerged
}

// unmergedPaths names the paths the unmerged lines are for, each once.
func unmergedPaths(lines []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, line := range lines {
		tab := strings.IndexByte(line, '\t')
		if tab < 0 {
			continue
		}
		path := line[tab+1:]
		if !seen[path] {
			seen[path] = true
			out = append(out, path)
		}
	}
	return out
}

func isZeroOID(oid string) bool {
	return strings.Trim(oid, "0") == ""
}

// undoMaxAge and undoKeepCount bound the snapshot refs, which are otherwise
// unbounded: every discard adds one. They are named because the README tells
// the reader these numbers about their own repository, and a test compares the
// two.
const (
	undoMaxAge    = 14 * 24 * time.Hour
	undoKeepCount = 100
)

type undoRef struct {
	ref string
	ts  int64
}

// newestFirst puts the refs in the order Prune keeps them in: the first
// undoKeepCount are the ones that stay. Two snapshots can carry one timestamp —
// they are taken to the second — so the order among equals decides which of
// them is dropped, and it has to be one the caller can predict. A comparator
// that answers true for a pair that is equal is not an order at all, and sort
// then arranges equals however its partitioning happens to fall.
func newestFirst(refs []undoRef) {
	sort.SliceStable(refs, func(i, j int) bool {
		if refs[i].ts != refs[j].ts {
			return refs[i].ts > refs[j].ts
		}
		return refs[i].ref < refs[j].ref
	})
}

// Prune drops undo refs past either bound.
func Prune(ctx context.Context, dir string) error {
	out, err := runRead(ctx, dir, "for-each-ref", "--format=%(refname) %(creatordate:unix)", undoRefPrefix)
	if err != nil {
		return err
	}
	var refs []undoRef
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		ts, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			continue
		}
		refs = append(refs, undoRef{ref: parts[0], ts: ts})
	}
	newestFirst(refs)

	cutoff := time.Now().Add(-undoMaxAge).Unix()
	for i, r := range refs {
		if staleUndoRef(i, r.ts, cutoff) {
			if _, err := runWrite(ctx, dir, "update-ref", "-d", r.ref); err != nil {
				return err
			}
		}
	}
	return nil
}

// staleUndoRef reports whether the ref in position i is past either bound: the
// number of snapshots kept, or the age. The clock is the caller's, so the age
// bound can be asked about at the second it falls on.
func staleUndoRef(i int, ts, cutoff int64) bool {
	return i >= undoKeepCount || ts < cutoff
}

func snapshotPaths(ctx context.Context, dir string, entries []Entry) ([]string, error) {
	all, _, err := Status(ctx, dir)
	if err != nil {
		return nil, err
	}

	selected := map[string]bool{}
	var raw []string
	add := func(p string) {
		if selected[p] {
			return
		}
		selected[p] = true
		raw = append(raw, p)
	}
	for _, e := range entries {
		add(e.Path)
		if e.OldPath != "" {
			add(e.OldPath)
		}
	}

	for _, u := range all {
		if u.Worktree != Untracked {
			continue
		}
		for s := range selected {
			if pathCollides(s, u.Path) {
				add(u.Path)
			}
		}
	}

	return raw, nil
}

func pathspecs(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = pathspec(n)
	}
	return out
}

func pathCollides(a, b string) bool {
	if a == b {
		return true
	}
	return strings.HasPrefix(a+"/", b+"/") || strings.HasPrefix(b+"/", a+"/")
}

func runWithEnv(ctx context.Context, dir string, env []string, args ...string) ([]byte, error) {
	return execGit(ctx, dir, env, nil, args)
}
