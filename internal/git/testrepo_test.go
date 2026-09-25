package git

import (
	"os/exec"
	"testing"
)

func gitTestEnv(base string) []string {
	return []string{
		"HOME=" + base,
		"GIT_CONFIG_GLOBAL=" + base + "/.gitconfig",
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
}

func runGitTest(t *testing.T, env []string, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
}

// Isolated from the developer's own git configuration so that hooks, templates
// and aliases cannot reach the test.
func newRepo(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := t.TempDir()
	env := gitTestEnv(dir)
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"commit", "-q", "--allow-empty", "-m", "base"},
	} {
		runGitTest(t, env, dir, args...)
	}
	return dir
}
