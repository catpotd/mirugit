package git

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A file both sides of a merge changed sits in the index three times: the base,
// ours and theirs. Discarding it collapses those to one, and the snapshot that
// undo restores from could hold only one blob per path — so it held none, and
// undo staged a deletion of a file that was on disk.
//
// The reader who pressed U then had a repository where committing recorded the
// file as removed. The README promises the discard is always recoverable.
func TestUndoingADiscardedConflictPutsTheConflictBack(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to git")
	}
	t.Parallel()
	dir := conflictedRepo(t)

	if stages := indexStagesFor(t, dir, "f.txt"); stages != 3 {
		t.Fatalf("this test needs a conflicted file; the index holds %d stages", stages)
	}

	entries, _, err := Status(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	undo, err := Snapshot(context.Background(), dir, entries, "discard")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Discard(context.Background(), dir, entries, undo); err != nil {
		t.Fatal(err)
	}
	if stages := indexStagesFor(t, dir, "f.txt"); stages != 1 {
		t.Fatalf("the discard left %d index stages, want the resolved one", stages)
	}

	if _, err := Undiscard(context.Background(), dir, undo); err != nil {
		t.Fatal(err)
	}

	if stages := indexStagesFor(t, dir, "f.txt"); stages != 3 {
		t.Errorf("undo left %d index stages; the conflict is not back", stages)
	}
	body, err := os.ReadFile(filepath.Join(dir, "f.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "<<<<<<<") {
		t.Errorf("undo did not put the conflict markers back:\n%s", body)
	}

	// The state a reader is left in has to be one they can act on. A path the
	// index calls deleted while it sits on disk records a removal on the next
	// commit.
	after, _, err := Status(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range after {
		if e.Path == "f.txt" && e.Index == Deleted {
			t.Errorf("undo staged a deletion of a file that is on disk: %+v", e)
		}
	}
}

func indexStagesFor(t *testing.T, dir, path string) int {
	t.Helper()
	out, err := runRead(context.Background(), dir, "ls-files", "-s", "--", path)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(line) != "" {
			n++
		}
	}
	return n
}

// A repository can name its objects with SHA-256 instead of SHA-1, and then an
// object id is 64 characters rather than 40. The zero id that clears an index
// entry has to be as long as the ids beside it, so it is measured from the
// entry being restored rather than written out.
func TestClearingAnIndexEntryUsesTheRepositoryOwnIDLength(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name  string
		line  string
		zeros int
	}{
		{"sha-1", "100644 " + strings.Repeat("a", 40) + " 2\tf.txt", 40},
		{"sha-256", "100644 " + strings.Repeat("a", 64) + " 2\tf.txt", 64},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := clearLineFor(c.line)
			want := "0 " + strings.Repeat("0", c.zeros) + "\tf.txt"
			if got != want {
				t.Errorf("clearing that entry writes\n  %q\nand the ids beside it are "+
					"%d characters:\n  %q", got, c.zeros, want)
			}
		})
	}
}

// A discard covers what the reader selected, and a selection can hold both
// kinds at once: files a merge left unmerged and files it never touched. Undo
// has to put back each in its own way — three index entries for one, one entry
// for the other — from a single snapshot.
func TestUndoingADiscardOfConflictedAndOrdinaryFilesTogether(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to git")
	}
	t.Parallel()
	dir := conflictedRepo(t)
	// A file the merge never saw, edited while it is unmerged: the discard
	// takes both. It is written rather than committed because a merge in
	// progress refuses a commit.
	write(t, dir, "plain.txt", "written during the merge\n")

	entries, _, err := Status(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("this test needs one conflicted and one ordinary file, got %d", len(entries))
	}
	undo, err := Snapshot(context.Background(), dir, entries, "discard")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Discard(context.Background(), dir, entries, undo); err != nil {
		t.Fatal(err)
	}
	if _, err := Undiscard(context.Background(), dir, undo); err != nil {
		t.Fatal(err)
	}

	if stages := indexStagesFor(t, dir, "f.txt"); stages != 3 {
		t.Errorf("the conflict came back with %d index stages", stages)
	}
	body, err := os.ReadFile(filepath.Join(dir, "plain.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "written during the merge\n" {
		t.Errorf("the ordinary file came back as %q", body)
	}
	if stages := indexStagesFor(t, dir, "plain.txt"); stages > 1 {
		t.Errorf("the ordinary file has %d index stages; it was never unmerged", stages)
	}
}

// Resolving a conflict and committing is how a merge ends, and the commit that
// ends it has two parents. Nothing in mirugit runs `git merge`, so this is the
// one place its own commit has to behave like git's: MERGE_HEAD is what tells
// git to record the second parent, and a commit that ignored it would leave the
// branch looking like the merge never happened.
func TestCommittingAResolvedConflictEndsTheMerge(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to git")
	}
	t.Parallel()
	dir := conflictedRepo(t)
	env := gitTestEnv(dir)

	write(t, dir, "f.txt", "one\nRESOLVED\n")
	runGitTest(t, env, dir, "add", "f.txt")

	if _, err := os.Stat(filepath.Join(dir, ".git", "MERGE_HEAD")); err != nil {
		t.Fatalf("this test needs a merge in progress: %v", err)
	}
	if _, err := Commit(context.Background(), dir, "end the merge"); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dir, ".git", "MERGE_HEAD")); !os.IsNotExist(err) {
		t.Errorf("the merge is still in progress after the commit that ends it: %v", err)
	}
	out, err := runRead(context.Background(), dir, "log", "--format=%P", "-1")
	if err != nil {
		t.Fatal(err)
	}
	if parents := len(strings.Fields(string(out))); parents != 2 {
		t.Errorf("the commit that ends a merge has %d parent(s), and a merge has two",
			parents)
	}
	if state, err := OperationInProgress(filepath.Join(dir, ".git")); err != nil {
		t.Fatal(err)
	} else if state != NothingInProgress {
		t.Errorf("the pane would still say %q is in progress", state.Verb())
	}
}
