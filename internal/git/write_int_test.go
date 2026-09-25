package git

import (
	"context"
	"os"
	"os/exec"
	"testing"
)

func newEmptyRepo(t *testing.T) string {
	t.Helper()
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
	cmd := exec.Command("git", "init", "-q", "-b", "main")
	cmd.Dir = dir
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	return dir
}

func TestStageRenamePassesBothPaths(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/old.txt", []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "old.txt")
	runGit(t, dir, "commit", "-q", "-m", "add old")
	if err := os.Rename(dir+"/old.txt", dir+"/new.txt"); err != nil {
		t.Fatal(err)
	}

	rename := Entry{Path: "new.txt", OldPath: "old.txt", Worktree: Untracked}
	out, err := Stage(context.Background(), dir, []Entry{rename})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Applied) != 1 {
		t.Fatalf("want one applied, got %+v", out)
	}

	entries, _, err := Status(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, e := range entries {
		if e.Path == "new.txt" && e.IsStaged() {
			found = true
		}
		if e.Path == "old.txt" && e.IsStaged() {
			t.Error("old path should not remain staged for deletion")
		}
	}
	if !found {
		t.Error("new path should be staged")
	}
}

func TestStageColonFilename(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	name := ":magic.txt"
	if err := os.WriteFile(dir+"/"+name, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e := Entry{Path: name, Worktree: Untracked}

	out, err := Stage(context.Background(), dir, []Entry{e})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Applied) != 1 {
		t.Fatalf("want one applied, got %+v", out)
	}

	entries, _, err := Status(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, got := range entries {
		if got.Path == name && got.IsStaged() {
			return
		}
	}
	t.Fatalf("want %q staged, got %+v", name, entries)
}

func TestStageAlreadyStagedIsIgnored(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/a.txt", []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "a.txt")
	e := Entry{Path: "a.txt", Index: Added}

	out, err := Stage(context.Background(), dir, []Entry{e})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Ignored) != 1 {
		t.Fatalf("want one ignored, got %+v", out)
	}
	if len(out.Applied) != 0 {
		t.Fatalf("want none applied, got %+v", out)
	}
}

func TestUnstageWithoutHEAD(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newEmptyRepo(t)
	if err := os.WriteFile(dir+"/a.txt", []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "a.txt")
	e := Entry{Path: "a.txt", Index: Added}

	out, err := Unstage(context.Background(), dir, []Entry{e})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Applied) != 1 {
		t.Fatalf("want one applied, got %+v", out)
	}

	entries, _, err := Status(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, got := range entries {
		if got.Path == "a.txt" && got.IsStaged() {
			t.Fatal("a.txt should not be staged")
		}
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
