package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// A linked worktree keeps .git as a file, so anything that writes next to it has
// to ask git where this worktree's state lives.
func TestSnapshotWorksInALinkedWorktree(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	main := t.TempDir()
	env := []string{
		"HOME=" + main,
		"GIT_CONFIG_GLOBAL=" + main + "/.gitconfig",
		"GIT_CONFIG_SYSTEM=/dev/null",
		// git forks a repack of its own once a repository crosses a
		// threshold, and that child outlives the test: it races
		// t.TempDir's removal, and it takes the repository's locks
		// while another test is reading it.
		"GIT_CONFIG_COUNT=2",
		"GIT_CONFIG_KEY_0=gc.auto", "GIT_CONFIG_VALUE_0=0",
		"GIT_CONFIG_KEY_1=maintenance.auto", "GIT_CONFIG_VALUE_1=false",
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
	}
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir, cmd.Env = dir, env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run(main, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(main, "a.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(main, "add", "a.txt")
	run(main, "commit", "-m", "first")

	wt := filepath.Join(t.TempDir(), "wt")
	run(main, "worktree", "add", "-b", "side", wt)
	if info, err := os.Stat(filepath.Join(wt, ".git")); err != nil || info.IsDir() {
		t.Fatalf("linked worktree .git should be a file: err=%v", err)
	}
	if err := os.WriteFile(filepath.Join(wt, "a.txt"), []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Snapshot(context.Background(), wt, []Entry{{Path: "a.txt", Worktree: Modified}}, "discard"); err != nil {
		t.Fatalf("Snapshot in a linked worktree: %v", err)
	}
}
