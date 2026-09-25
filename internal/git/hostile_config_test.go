package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The reader's own git config reaches this process. color.ui=always makes every
// line start with an escape, which left parseDiff finding no files and the pane
// silently empty; diff.external replaces the output wholesale.
func TestReaderConfigCannotBreakParsing(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to git")
	}
	for _, cfg := range [][2]string{
		{"color.ui", "always"},
		{"color.diff", "always"},
		{"diff.external", "/bin/echo"},
		{"core.quotePath", "true"},
	} {
		dir := t.TempDir()
		env := []string{
			"HOME=" + dir, "GIT_CONFIG_GLOBAL=" + dir + "/.gitconfig",
			"GIT_CONFIG_SYSTEM=/dev/null",
			// git forks a repack of its own once a repository crosses a
			// threshold, and that child outlives the test: it races
			// t.TempDir's removal, and it takes the repository's locks
			// while another test is reading it.
			"GIT_CONFIG_COUNT=2",
			"GIT_CONFIG_KEY_0=gc.auto", "GIT_CONFIG_VALUE_0=0",
			"GIT_CONFIG_KEY_1=maintenance.auto", "GIT_CONFIG_VALUE_1=false",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@e.com",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@e.com",
		}
		run := func(a ...string) {
			t.Helper()
			c := exec.Command("git", a...)
			c.Dir, c.Env = dir, env
			if o, err := c.CombinedOutput(); err != nil {
				t.Fatalf("%v: %v\n%s", a, err, o)
			}
		}
		run("init", "-b", "main")
		name := "日本語.txt"
		if err := os.WriteFile(filepath.Join(dir, name), []byte("a\nb\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		run("add", ".")
		run("commit", "-m", "one")
		if err := os.WriteFile(filepath.Join(dir, name), []byte("a\nc\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		run("config", "--global", cfg[0], cfg[1])

		t.Setenv("GIT_CONFIG_GLOBAL", dir+"/.gitconfig")
		d, err := Diff(context.Background(), dir, name, false)
		known, kerr := KnownPaths(context.Background(), dir)

		if err != nil || len(d.Blocks) == 0 {
			t.Errorf("%s=%s: Diff blocks=%d err=%v", cfg[0], cfg[1], len(d.Blocks), err)
		}
		if kerr != nil || !known[name] {
			t.Errorf("%s=%s: KnownPaths に %q が無い err=%v", cfg[0], cfg[1], name, kerr)
		}
	}
}
