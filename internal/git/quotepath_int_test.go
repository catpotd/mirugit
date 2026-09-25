package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// core.quotePath escapes a non-ASCII name unless -z is given. KnownPaths feeds
// Prune, which drops the read marks of anything it does not recognize, so an
// escaped name loses its marks on every startup.
func TestKnownPathsReturnsNonASCIINamesUnescaped(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := t.TempDir()
	env := []string{
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
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir, cmd.Env = dir, env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	tracked := "日本語.txt"
	logged := "コミット済み.txt"
	for _, name := range []string{tracked, logged} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("add", ".")
	run("commit", "-m", "first")
	run("rm", "--quiet", logged)
	run("commit", "-m", "second")

	// log に載らない tracked file。ls-files だけが答えるので、そこの -z が
	// 抜けているとこの1件だけが escape された形で返る
	onlyInIndex := "索引だけ.txt"
	if err := os.WriteFile(filepath.Join(dir, onlyInIndex), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", onlyInIndex)

	untracked := "未追跡.txt"
	if err := os.WriteFile(filepath.Join(dir, untracked), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	known, err := KnownPaths(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{tracked, logged, untracked, onlyInIndex} {
		if !known[want] {
			t.Errorf("KnownPaths に %q が無い(得たもの: %v)", want, known)
		}
	}
}
