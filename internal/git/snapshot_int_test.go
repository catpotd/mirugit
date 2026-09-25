package git

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestDiscardKeepsAnUntrackedFileThatCollidesWithADirectory(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.MkdirAll(dir+"/d", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/d/file.txt", []byte("tracked\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "d/file.txt")
	runGit(t, dir, "commit", "-q", "-m", "add d/file.txt")

	if err := os.RemoveAll(dir + "/d"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/d", []byte("untracked\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := Entry{Path: "d/file.txt", Index: Deleted}
	undo, err := Snapshot(context.Background(), dir, []Entry{entry}, "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Discard(context.Background(), dir, []Entry{entry}, undo); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(dir + "/d"); err != nil {
		t.Fatal("untracked file d should survive discard")
	}
	if _, err := os.Stat(dir + "/d/file.txt"); err == nil {
		t.Fatal("tracked path should be discarded")
	}
}

func TestDiscardKeepsAnUntrackedFileUnderAPathThatWasAFile(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/f", []byte("tracked\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "f")
	runGit(t, dir, "commit", "-q", "-m", "add f")

	if err := os.Remove(dir + "/f"); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir+"/f", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/f/new.txt", []byte("untracked\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := Entry{Path: "f", Index: Deleted}
	undo, err := Snapshot(context.Background(), dir, []Entry{entry}, "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Discard(context.Background(), dir, []Entry{entry}, undo); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(dir + "/f/new.txt"); err != nil {
		t.Fatal("untracked f/new.txt should survive discard")
	}
}

func TestSnapshotHoldsAnUntrackedFile(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/u.txt", []byte("untracked\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := Entry{Path: "u.txt", Worktree: Untracked}
	undo, err := Snapshot(context.Background(), dir, []Entry{entry}, "test")
	if err != nil {
		t.Fatal(err)
	}

	out, err := runWrite(context.Background(), dir, "show", undo.Commit+":u.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "untracked\n" {
		t.Fatalf("got %q", out)
	}
}

func TestSnapshotSurvivesAnIntentToAddIndex(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/n.txt", []byte("intent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "-N", "n.txt")

	entry := Entry{Path: "n.txt", Index: Added, Worktree: Untracked}
	undo, err := Snapshot(context.Background(), dir, []Entry{entry}, "test")
	if err != nil {
		t.Fatal(err)
	}
	if undo.Commit == "" {
		t.Fatal("want a snapshot commit")
	}
}

func TestSnapshotSurvivesGarbageCollection(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/a.txt", []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := Entry{Path: "a.txt", Worktree: Untracked}
	undo, err := Snapshot(context.Background(), dir, []Entry{entry}, "test")
	if err != nil {
		t.Fatal(err)
	}

	runGit(t, dir, "gc", "--prune=now")

	out, err := runWrite(context.Background(), dir, "cat-file", "-t", undo.Commit)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(out)) != "commit" {
		t.Fatalf("commit gone after gc: %q", out)
	}
}

func TestSnapshotWorksInARepositoryWithNoCommits(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newEmptyRepo(t)
	if err := os.WriteFile(dir+"/a.txt", []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := Entry{Path: "a.txt", Worktree: Untracked}
	undo, err := Snapshot(context.Background(), dir, []Entry{entry}, "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Undiscard(context.Background(), dir, undo); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(dir + "/a.txt"); err != nil || string(data) != "new\n" {
		t.Fatalf("got %q err=%v", data, err)
	}
}

func TestUndiscardPutsBackAnUntrackedFile(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/u.txt", []byte("gone\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := Entry{Path: "u.txt", Worktree: Untracked}
	undo, err := Snapshot(context.Background(), dir, []Entry{entry}, "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Discard(context.Background(), dir, []Entry{entry}, undo); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir + "/u.txt"); !os.IsNotExist(err) {
		t.Fatal("file should be gone after discard")
	}
	if _, err := Undiscard(context.Background(), dir, undo); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(dir + "/u.txt")
	if err != nil || string(data) != "gone\n" {
		t.Fatalf("got %q err=%v", data, err)
	}
}

func TestDiscardOfAnUntrackedPathUsesClean(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/u.txt", []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := Entry{Path: "u.txt", Worktree: Untracked}
	undo, err := Snapshot(context.Background(), dir, []Entry{entry}, "test")
	if err != nil {
		t.Fatal(err)
	}

	paths := []string{pathspec("u.txt")}
	if err := runWritePaths(context.Background(), dir, paths, "restore", "--staged", "--worktree"); err == nil {
		t.Fatal("restore should fail on an untracked path")
	}

	out, err := Discard(context.Background(), dir, []Entry{entry}, undo)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Applied) != 1 {
		t.Fatalf("want one applied, got %+v", out)
	}
	if _, err := os.Stat(dir + "/u.txt"); !os.IsNotExist(err) {
		t.Fatal("untracked file should be removed")
	}
}

func TestSnapshotWidensToCollidingUntrackedPaths(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/a.txt", []byte("tracked\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "a.txt")
	runGit(t, dir, "commit", "-q", "-m", "add")

	runGit(t, dir, "rm", "a.txt")
	if err := os.WriteFile(dir+"/a.txt", []byte("untracked\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := Entry{Path: "a.txt", Index: Deleted}
	undo, err := Snapshot(context.Background(), dir, []Entry{entry}, "test")
	if err != nil {
		t.Fatal(err)
	}
	out, err := runWrite(context.Background(), dir, "show", undo.Commit+":a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "untracked\n" {
		t.Fatalf("snapshot should hold the colliding untracked file, got %q", out)
	}
}

func TestPruneKeepsOnlyTheLatestHundredUndoRefs(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	for i := range 101 {
		ref := undoRefPrefix + strconv.Itoa(i)
		runGit(t, dir, "update-ref", ref, "HEAD")
	}
	if err := Prune(context.Background(), dir); err != nil {
		t.Fatal(err)
	}
	out, err := runWrite(context.Background(), dir, "for-each-ref", "--format=%(refname)", undoRefPrefix)
	if err != nil {
		t.Fatal(err)
	}
	refs := strings.Fields(strings.TrimSpace(string(out)))
	if len(refs) > 100 {
		t.Fatalf("got %d refs, want at most 100", len(refs))
	}
}

// Two mirugit windows on one repository can discard within the same second,
// and the worktrees tab makes running two of them an ordinary thing to do. The
// ref is the only thing keeping a snapshot commit from being collected, so a
// name that repeats within a second cost the earlier discard its undo: measured
// with two update-refs of one name, the first commit was left with no ref
// naming it.
func TestTwoSnapshotsInOneSecondAreNamedApart(t *testing.T) {
	t.Parallel()
	const second = 1788577222
	first := "aaaaaaaabbbbbbbbccccccccdddddddd11111111"
	second2 := "eeeeeeeeffffffff000000001111111122222222"

	a := undoRefName(second, first)
	if b := undoRefName(second, second2); a == b {
		t.Errorf("two snapshots share the name %q", a)
	}
	if again := undoRefName(second, first); again != a {
		t.Errorf("one snapshot got two names, %q and %q", a, again)
	}
	// README promises this prefix stays; only what follows it is ours to change.
	if !strings.HasPrefix(a, undoRefPrefix) {
		t.Errorf("%q is outside %q", a, undoRefPrefix)
	}
}

// The commit is what the name is taken from, so a reader holding a ref can see
// which snapshot it is without resolving it.
func TestSnapshotNamesItsRefAfterTheCommitItKeeps(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/a.txt", []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	undo, err := Snapshot(context.Background(), dir, []Entry{{Path: "a.txt", Worktree: Untracked}}, "test")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(undo.Ref, "-"+undo.Commit[:8]) {
		t.Errorf("ref %q does not name commit %q", undo.Ref, undo.Commit)
	}
	out, err := runWrite(context.Background(), dir, "rev-parse", undo.Ref)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(out)); got != undo.Commit {
		t.Errorf("%s points at %s, want %s", undo.Ref, got, undo.Commit)
	}
}

func TestUndiscardLeavesAnUntrackedFileUnstaged(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/u.txt", []byte("u\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := Entry{Path: "u.txt", Worktree: Untracked}
	undo, err := Snapshot(context.Background(), dir, []Entry{entry}, "discard")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Discard(context.Background(), dir, []Entry{entry}, undo); err != nil {
		t.Fatal(err)
	}
	if _, err := Undiscard(context.Background(), dir, undo); err != nil {
		t.Fatal(err)
	}

	out, err := runWrite(context.Background(), dir, "status", "--porcelain")
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "?? u.txt\n" {
		t.Fatalf("status = %q, want ?? u.txt", out)
	}
}

func TestUndiscardRestoresStagedContentAfterGC(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/a.txt", []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "a.txt")
	runGit(t, dir, "commit", "-q", "-m", "add")
	if err := os.WriteFile(dir+"/a.txt", []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "a.txt")
	if err := os.WriteFile(dir+"/a.txt", []byte("three\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entries, _, err := Status(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	var only []Entry
	for _, e := range entries {
		if e.Path == "a.txt" {
			only = append(only, e)
		}
	}
	undo, err := Snapshot(context.Background(), dir, only, "discard")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Discard(context.Background(), dir, only, undo); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "gc", "--prune=now")
	if _, err := Undiscard(context.Background(), dir, undo); err != nil {
		t.Fatal(err)
	}

	idx, err := runWrite(context.Background(), dir, "show", ":a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(idx) != "two\n" {
		t.Fatalf("index = %q, want two\\n", idx)
	}
}

func TestUndiscardRestagesInARepositoryWithNoCommits(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newEmptyRepo(t)
	if err := os.WriteFile(dir+"/n.txt", []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "n.txt")

	entry := Entry{Path: "n.txt", Index: Added}
	undo, err := Snapshot(context.Background(), dir, []Entry{entry}, "discard")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Discard(context.Background(), dir, []Entry{entry}, undo); err != nil {
		t.Fatal(err)
	}
	if _, err := Undiscard(context.Background(), dir, undo); err != nil {
		t.Fatal(err)
	}

	out, err := runWrite(context.Background(), dir, "status", "--porcelain")
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "A  n.txt\n" {
		t.Fatalf("status = %q, want A  n.txt", out)
	}
}

// The paths a snapshot covers are named once each. A rename carries two names
// and the file it was renamed from can be another row's name as well, so the
// same path reaches the list twice; an untracked file that sits under two of
// the selected paths reaches it once per path it sits under. Every repeat
// becomes another pathspec on the git command line, and the list is also what
// tells the caller which paths the snapshot protected.
func TestTheSnapshotNamesEachPathOnce(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	for _, name := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(dir, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Untracked, and under a directory that two of the selected paths name.
	if err := os.WriteFile(filepath.Join(dir, "pkg/new.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entries := []Entry{
		{Path: "a.txt", Worktree: Modified},
		{Path: "b.txt", OldPath: "a.txt", Index: Renamed},
		{Path: "pkg", Worktree: Modified},
		{Path: "pkg", Index: Modified},
	}
	paths, err := snapshotPaths(context.Background(), dir, entries)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for _, p := range paths {
		seen[p]++
	}
	for p, n := range seen {
		if n != 1 {
			t.Errorf("%q is named %d times in %v", p, n, paths)
		}
	}
	for _, want := range []string{"a.txt", "b.txt", "pkg"} {
		if seen[want] == 0 {
			t.Errorf("%q is not named at all in %v", want, paths)
		}
	}
}

// A conflicted path sits in the index three times — the base, ours and theirs —
// so the lines that name it are three. The snapshot needs the path itself, and
// it needs it once: the list becomes pathspecs on a git command line, and the
// caller reads its length as how many paths the conflict covers.
func TestTheUnmergedLinesNameEachPathOnce(t *testing.T) {
	t.Parallel()
	const mode = "100644 1111111111111111111111111111111111111111 "
	for _, c := range []struct {
		name  string
		lines []string
		want  []string
	}{
		{"one path in three stages", []string{
			mode + "1\tf.txt", mode + "2\tf.txt", mode + "3\tf.txt"}, []string{"f.txt"}},
		{"two paths in three stages each", []string{
			mode + "1\ta.txt", mode + "2\ta.txt", mode + "3\ta.txt",
			mode + "1\tb.txt", mode + "2\tb.txt", mode + "3\tb.txt"},
			[]string{"a.txt", "b.txt"}},
		{"stages that arrive interleaved", []string{
			mode + "1\ta.txt", mode + "1\tb.txt", mode + "2\ta.txt", mode + "2\tb.txt"},
			[]string{"a.txt", "b.txt"}},
		{"a line with no tab in it", []string{"no-tab-here", mode + "1\tf.txt"},
			[]string{"f.txt"}},
		{"nothing at all", nil, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := unmergedPaths(c.lines)
			if len(got) != len(c.want) {
				t.Fatalf("read %d paths, want %d: %v", len(got), len(c.want), got)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("path %d is %q, want %q", i, got[i], c.want[i])
				}
			}
		})
	}
}

// The index tree is committed only when some path needs its index entry put
// back. A snapshot of files that are changed in the working tree alone has
// none, and committing one anyway records a snapshot that claims to hold an
// index to restore: every discard then leaves a commit nothing points at, and
// the record carries a sha whose tree says nothing about what was discarded.
func TestASnapshotWithNothingStagedCommitsNoIndexTree(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "a.txt")
	runGit(t, dir, "commit", "-q", "-m", "base")
	// Untracked, so HEAD holds nothing for it and its index entry is nothing
	// to put back either.
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	all, _, err := Status(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	var entries []Entry
	for _, e := range all {
		if e.Path == "new.txt" {
			entries = append(entries, e)
		}
	}
	if len(entries) != 1 {
		t.Fatalf("the fixture found %d entries for new.txt", len(entries))
	}
	_ = entries
	undo, err := Snapshot(context.Background(), dir, entries, "discard")
	if err != nil {
		t.Fatal(err)
	}
	if len(undo.IndexPaths) != 0 {
		t.Fatalf("the snapshot lists %v as needing their index back", undo.IndexPaths)
	}
	if undo.IndexCommit != "" {
		t.Errorf("the snapshot holds an index commit %q with no path to restore",
			undo.IndexCommit)
	}

	// A staged path does need one, so the answer above is not "never".
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "b.txt")
	entries, _, err = Status(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	staged, err := Snapshot(context.Background(), dir, entries, "discard")
	if err != nil {
		t.Fatal(err)
	}
	if staged.IndexCommit == "" {
		t.Error("a snapshot with a staged path holds no index commit")
	}
}

// Undo puts the index back only when the snapshot holds both halves of the
// answer: which paths need their index entry restored, and the commit that
// holds it. A snapshot of an untracked file has neither, and asking git to
// restore from an empty source fails the whole undo — the working tree has
// already been put back by then, so the reader is left with a failure notice
// over a repository that was in fact restored.
func TestUndoWithNothingToRestoreInTheIndexSucceeds(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	runGit(t, dir, "commit", "-q", "--allow-empty", "-m", "base")
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	all, _, err := Status(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	undo, err := Snapshot(context.Background(), dir, all, "discard")
	if err != nil {
		t.Fatal(err)
	}
	if len(undo.IndexPaths) != 0 || undo.IndexCommit != "" {
		t.Fatalf("the snapshot holds %v and %q, so this proves nothing",
			undo.IndexPaths, undo.IndexCommit)
	}
	if _, err := Discard(context.Background(), dir, all, undo); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "new.txt")); err == nil {
		t.Fatal("the discard left the file, so the undo proves nothing")
	}

	if _, err := Undiscard(context.Background(), dir, undo); err != nil {
		t.Fatalf("undo failed with nothing to put back in the index: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "new.txt")); err != nil {
		t.Errorf("undo did not put the file back: %v", err)
	}
}
