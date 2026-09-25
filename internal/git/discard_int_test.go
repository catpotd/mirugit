package git

import (
	"context"
	"os"
	"testing"
)

func TestDiscardRequiresUndo(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/a.txt", []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	entry := Entry{Path: "a.txt", Worktree: Untracked}
	if _, err := Discard(context.Background(), dir, []Entry{entry}, Undo{}); err == nil {
		t.Fatal("want error without undo")
	}
}

func TestDiscardRemovesTrackedChanges(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/a.txt", []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "a.txt")
	runGit(t, dir, "commit", "-q", "-m", "add")
	if err := os.WriteFile(dir+"/a.txt", []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := Entry{Path: "a.txt", Worktree: Modified}
	undo, err := Snapshot(context.Background(), dir, []Entry{entry}, "test")
	if err != nil {
		t.Fatal(err)
	}
	out, err := Discard(context.Background(), dir, []Entry{entry}, undo)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Applied) != 1 {
		t.Fatalf("want one applied, got %+v", out)
	}
	data, err := os.ReadFile(dir + "/a.txt")
	if err != nil || string(data) != "x\n" {
		t.Fatalf("got %q err=%v", data, err)
	}
}

func TestPathsEditedAfterDiscardDetectsPostDiscardEdits(t *testing.T) {
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
	entry := Entry{Path: "a.txt", Worktree: Modified}
	undo, err := Snapshot(context.Background(), dir, []Entry{entry}, "discard")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Discard(context.Background(), dir, []Entry{entry}, undo); err != nil {
		t.Fatal(err)
	}
	undo = RecordWorktreeAfter(dir, undo)
	if edited, err := PathsEditedAfterDiscard(context.Background(), dir, undo); err != nil {
		t.Fatal(err)
	} else if len(edited) != 0 {
		t.Fatalf("right after discard: got %v, want none", edited)
	}
	if err := os.WriteFile(dir+"/a.txt", []byte("three\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	edited, err := PathsEditedAfterDiscard(context.Background(), dir, undo)
	if err != nil {
		t.Fatal(err)
	}
	if len(edited) != 1 || edited[0] != "a.txt" {
		t.Fatalf("after editing: got %v, want [a.txt]", edited)
	}
}

func TestUndiscardCountsRestoredBytesNotStatusKind(t *testing.T) {
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
	entry := Entry{Path: "a.txt", Worktree: Modified}
	undo, err := Snapshot(context.Background(), dir, []Entry{entry}, "discard")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Discard(context.Background(), dir, []Entry{entry}, undo); err != nil {
		t.Fatal(err)
	}
	undo = RecordWorktreeAfter(dir, undo)
	if err := os.WriteFile(dir+"/a.txt", []byte("three\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := Undiscard(context.Background(), dir, undo)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Applied) != 1 {
		t.Fatalf("applied = %d, want 1: %+v", len(out.Applied), out)
	}
	got, err := os.ReadFile(dir + "/a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "two\n" {
		t.Fatalf("worktree = %q, want two\\n", got)
	}
}

// The snapshot is HEAD with the discarded paths laid over it, so every other
// path in it holds HEAD's content. Restoring the whole tree therefore throws
// away edits the discard never touched.
func TestUndiscardLeavesUntouchedFilesAlone(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	for _, f := range []struct{ name, body string }{
		{"a.txt", "keep\nline\n"}, {"b.txt", "keep\n"},
	} {
		if err := os.WriteFile(dir+"/"+f.name, []byte(f.body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "base")
	if err := os.WriteFile(dir+"/a.txt", []byte("keep\nDISCARD ME\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	const bystander = "keep\nmine\n"
	if err := os.WriteFile(dir+"/b.txt", []byte(bystander), 0o644); err != nil {
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
	if _, err := Undiscard(context.Background(), dir, undo); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(dir + "/b.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != bystander {
		t.Errorf("b.txt = %q, want %q — undo reverted a file the discard never touched",
			got, bystander)
	}
	if a, _ := os.ReadFile(dir + "/a.txt"); string(a) != "keep\nDISCARD ME\n" {
		t.Errorf("a.txt = %q, want the discarded content back", a)
	}
}

func TestUndiscardRestagesAStagedFile(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/a.txt", []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "a.txt")
	runGit(t, dir, "commit", "-q", "-m", "add")
	if err := os.WriteFile(dir+"/a.txt", []byte("y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "a.txt")

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
	if _, err := Undiscard(context.Background(), dir, undo); err != nil {
		t.Fatal(err)
	}

	out, err := runWrite(context.Background(), dir, "status", "--porcelain")
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "M  a.txt\n" {
		t.Fatalf("status = %q, want staged modification", out)
	}
	idx, err := runWrite(context.Background(), dir, "show", ":a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(idx) != "y\n" {
		t.Fatalf("index = %q, want y\\n", idx)
	}
}

func TestUndiscardKeepsStagedAndWorktreeContentApart(t *testing.T) {
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
	data, err := os.ReadFile(dir + "/a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "three\n" {
		t.Fatalf("worktree = %q, want three\\n", data)
	}
	out, err := runWrite(context.Background(), dir, "status", "--porcelain")
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "MM a.txt\n" {
		t.Fatalf("status = %q, want MM a.txt", out)
	}
}

func TestUndiscardRestagesAnAddedFile(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
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

func TestUndiscardRestoresAStagedDeletion(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/a.txt", []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "a.txt")
	runGit(t, dir, "commit", "-q", "-m", "add")
	runGit(t, dir, "rm", "a.txt")

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
	if _, err := Undiscard(context.Background(), dir, undo); err != nil {
		t.Fatal(err)
	}

	out, err := runWrite(context.Background(), dir, "status", "--porcelain")
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "D  a.txt\n" {
		t.Fatalf("status = %q, want D  a.txt", out)
	}
	if _, err := os.Stat(dir + "/a.txt"); !os.IsNotExist(err) {
		t.Fatal("a.txt should stay deleted in the worktree")
	}
}

// A discard can carry both kinds of path at once: a tracked file to restore and
// an untracked one to remove. They are two git calls, and the second runs only
// if the first returned. Reversing that test stops after the restore, so the
// untracked file the reader asked to discard is still on disk with nothing
// saying it was kept.
func TestDiscardingATrackedAndAnUntrackedFileTogetherDoesBoth(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/tracked.txt", []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "tracked.txt")
	runGit(t, dir, "commit", "-q", "-m", "add")
	if err := os.WriteFile(dir+"/tracked.txt", []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/new.txt", []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entries := []Entry{
		{Path: "tracked.txt", Worktree: Modified},
		{Path: "new.txt", Worktree: Untracked},
	}
	undo, err := Snapshot(context.Background(), dir, entries, "discard")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Discard(context.Background(), dir, entries, undo); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(dir + "/tracked.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "one\n" {
		t.Errorf("the tracked file holds %q, want the committed content", got)
	}
	if _, err := os.Stat(dir + "/new.txt"); !os.IsNotExist(err) {
		t.Errorf("the untracked file is still there: %v", err)
	}
}
