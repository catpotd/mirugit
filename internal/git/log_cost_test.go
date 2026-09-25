package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Log runs on every poll, so its cost must not grow with the repository. It
// used to start two subprocesses per commit (rev-parse --short and merge-base
// --is-ancestor) and read every commit, which took three seconds here.
func TestLogReadsAFixedNumberOfCommits(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := t.TempDir()
	env := []string{
		"HOME=" + dir, "GIT_CONFIG_GLOBAL=" + dir + "/.gitconfig",
		"GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@e.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@e.com",
		// Building a hundred and twenty commits crosses the threshold where git
		// forks a repack of its own. That child outlives the test and races
		// t.TempDir's removal, which then fails with "directory not empty".
		"GIT_CONFIG_COUNT=2",
		"GIT_CONFIG_KEY_0=gc.auto", "GIT_CONFIG_VALUE_0=0",
		"GIT_CONFIG_KEY_1=maintenance.auto", "GIT_CONFIG_VALUE_1=false",
	}
	run := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir, c.Env = dir, env
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	for i := range 120 {
		if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte{byte(i)}, 0o644); err != nil {
			t.Fatal(err)
		}
		run("add", "a.txt")
		run("commit", "-m", "c")
	}
	page, err := LogPage(context.Background(), dir, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Commits) != 100 {
		t.Errorf("LogPage returned %d commits, want the first 100", len(page.Commits))
	}
	if !page.HasMore {
		t.Error("the first page did not report the remaining 20 commits")
	}
	if page.Commits[0].ShortSHA == "" {
		t.Error("short SHA is empty; the format string should carry it")
	}
	if len(page.Commits[0].ShortSHA) >= len(page.Commits[0].SHA) {
		t.Errorf("short SHA %q is not shorter than %q", page.Commits[0].ShortSHA, page.Commits[0].SHA)
	}
	next, err := LogPage(context.Background(), dir, len(page.Commits))
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Commits) != 20 {
		t.Errorf("next page returned %d commits, want 20", len(next.Commits))
	}
	if next.HasMore {
		t.Error("the final page reported more commits")
	}
	if len(next.Commits) > 0 && next.Commits[0].SHA == page.Commits[len(page.Commits)-1].SHA {
		t.Error("the next page repeated the previous page's last commit")
	}
}
