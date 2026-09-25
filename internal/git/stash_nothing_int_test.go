package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// The pane asks for a stash from the status it read a moment ago, and the file
// may be back to what it was by the time the key is pressed. git then stashes
// nothing, and a notice saying the file was stashed sends the reader looking
// for it in a stash that does not exist.
func TestStashingAFileWithNoChangeReportsItIgnored(t *testing.T) {
	t.Parallel()
	dir := newRepo(t)
	env := gitTestEnv(dir)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, env, dir, "add", "a.txt")
	runGitTest(t, env, dir, "commit", "-q", "-m", "add")

	// The entry says the file changed; the working tree says it did not.
	entries := []Entry{{Path: "a.txt", Worktree: Modified}}
	out, err := Stash(context.Background(), dir, entries)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Ignored) != 1 || out.Ignored[0].Path != "a.txt" {
		t.Errorf("Ignored = %+v, want the file that was not stashed", out.Ignored)
	}
	if len(out.Applied) != 0 {
		t.Errorf("Changed = %+v, want nothing", out.Applied)
	}

	stashes, err := StashList(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(stashes) != 0 {
		t.Errorf("the repository holds %d stashes, so this proves nothing", len(stashes))
	}
}
