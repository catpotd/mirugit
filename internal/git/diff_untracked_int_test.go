package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// A file git has never seen has no diff of its own, so the pane reads it
// against nothing to show what it holds. That stands in for the unstaged side
// only: nothing of an untracked file is staged, and drawing its lines under the
// staged heading says a commit would carry them.
func TestAnUntrackedFileHasNothingOnTheStagedSide(t *testing.T) {
	t.Parallel()
	dir := newRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "u.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	unstaged, err := Diff(context.Background(), dir, "u.txt", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(unstaged.Blocks) == 0 {
		t.Fatal("the unstaged side shows nothing, so this proves nothing")
	}
	if got := unstaged.Blocks[0].NewStart; got != 1 {
		t.Errorf("the first line of an untracked file starts at %d, want 1", got)
	}
	if got := unstaged.Blocks[0].LinePrefixLength; got != 1 {
		t.Errorf("the untracked file uses %d diff markers, want 1", got)
	}

	staged, err := Diff(context.Background(), dir, "u.txt", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(staged.Blocks) != 0 {
		t.Errorf("the staged side shows %d blocks of a file that was never added: %+v",
			len(staged.Blocks), staged.Blocks)
	}
}
