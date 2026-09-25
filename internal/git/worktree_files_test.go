package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// A working tree has two sides and a file's change can be on either. The row
// drawn under a worktree carried the change on the working-tree side whichever
// side it came from, so opening a file whose change is only staged asked git
// for the diff of a file the working tree does not hold, and the pane showed
// nothing for a file that plainly changed.
func TestWorktreeFilesSayWhichSideTheChangeIsOn(t *testing.T) {
	t.Parallel()
	dir := newRepo(t)
	env := gitTestEnv(dir)
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("staged.txt", "one\n")
	write("both.txt", "one\n")
	runGitTest(t, env, dir, "add", "staged.txt", "both.txt")
	runGitTest(t, env, dir, "commit", "-q", "-m", "add")

	write("staged.txt", "one\ntwo\n")
	runGitTest(t, env, dir, "add", "staged.txt")
	write("both.txt", "one\ntwo\n")
	runGitTest(t, env, dir, "add", "both.txt")
	write("both.txt", "one\ntwo\nthree\n")
	write("worktree.txt", "new\n")

	files, err := WorktreeFiles(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	staged := map[string]bool{}
	for _, f := range files {
		staged[f.Path] = f.IsStaged() && !f.IsUnstaged()
	}

	for path, want := range map[string]bool{
		// The index holds it and the working tree does not: --cached answers.
		"staged.txt": true,
		// Both sides hold a change, so the working tree has one to show.
		"both.txt": false,
		// Never committed, so it is only in the working tree.
		"worktree.txt": false,
	} {
		if got, listed := staged[path]; !listed {
			t.Errorf("%s is missing from %v", path, staged)
		} else if got != want {
			t.Errorf("%s is staged=%v, want %v", path, got, want)
		}
	}

	// The side the row names has to be the side that has a diff to show.
	for path, useCached := range staged {
		d, err := Diff(context.Background(), dir, path, useCached)
		if err != nil {
			t.Fatal(err)
		}
		if len(d.Blocks) == 0 {
			t.Errorf("%s: the side the row names (staged=%v) has no diff to draw",
				path, useCached)
		}
	}
}
