package tui

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// git init leaves no index behind: the file appears the first time something is
// staged. Binding the watcher to that one path meant the first thing a reader
// does with a new repository took the whole watcher down, and what they saw was
// a notice reading like a failure on a repository where nothing is wrong.
//
// The whole session then ran on the five-second poll, because a watcher is
// never rebound.
func TestAFreshRepositoryGetsAWatcher(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to git")
	}
	t.Parallel()
	dir := repoWithNoCommits(t)
	if _, err := os.Stat(filepath.Join(dir, ".git", "index")); !os.IsNotExist(err) {
		t.Fatalf("this repository was supposed to have no index yet: %v", err)
	}

	ready := beginWatch(context.Background(), dir)().(watchReadyMsg)
	if ready.err != nil {
		t.Fatalf("a repository with no commits got no watcher: %v", ready.err)
	}
	defer func() { _ = ready.watcher.Close() }()

	// Staging is what creates the index, and it is the change the reader makes
	// first. The pane has to hear about it without waiting for the poll.
	events := make(chan tea.Msg, 1)
	go func() { events <- waitWatchEvent(ready.watcher)() }()

	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitInRepo(t, dir, "add", "a.txt")

	select {
	case <-events:
	case <-time.After(5 * time.Second):
		t.Fatal("staging the first file did not wake the watcher")
	}
}

func repoWithNoCommits(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitInRepo(t, dir, "init", "-q", "-b", "main")
	return dir
}

func gitInRepo(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = []string{
		"HOME=" + dir,
		"GIT_CONFIG_GLOBAL=" + dir + "/.gitconfig",
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
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
