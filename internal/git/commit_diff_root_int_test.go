package git

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A commit's diff is what that commit changed. The first commit of a repository
// has no parent to compare against, and the empty tree is what it added its
// files to. Comparing it with the working tree instead answers with whatever
// has happened since — the reader opens the commit that created a file and is
// shown an edit made this morning.
func TestTheFirstCommitsDiffIsWhatItAdded(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := t.TempDir()
	env := gitTestEnv(dir)
	runGitTest(t, env, dir, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, env, dir, "add", "a.txt")
	runGitTest(t, env, dir, "commit", "-q", "-m", "first")
	root, err := runRead(context.Background(), dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	// The file moves on after the commit, both in the index and on disk.
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\nlater\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	d, err := CommitDiff(context.Background(), dir, strings.TrimSpace(string(root)), "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Blocks) != 1 {
		t.Fatalf("the commit's diff holds %d blocks, want 1: %+v", len(d.Blocks), d.Blocks)
	}
	b := d.Blocks[0]
	if b.Added != 2 || b.Deleted != 0 {
		t.Errorf("the commit added two lines and removed none; the diff says +%d −%d: %v",
			b.Added, b.Deleted, b.Lines)
	}
	for _, line := range b.Lines {
		if line == "+later" || line == "-two" {
			t.Errorf("the diff carries a change made after the commit: %v", b.Lines)
		}
	}

	// A commit with a parent is compared against that parent, and the diff is
	// the lines it changed rather than every line of the file.
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\nsecond\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, env, dir, "add", "a.txt")
	runGitTest(t, env, dir, "commit", "-q", "-m", "second")
	tip, err := runRead(context.Background(), dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	d, err = CommitDiff(context.Background(), dir, strings.TrimSpace(string(tip)), "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Blocks) != 1 {
		t.Fatalf("the second commit's diff holds %d blocks, want 1", len(d.Blocks))
	}
	if b := d.Blocks[0]; b.Added != 1 || b.Deleted != 1 {
		t.Errorf("the second commit changed one line; the diff says +%d −%d: %v",
			b.Added, b.Deleted, b.Lines)
	}
}
