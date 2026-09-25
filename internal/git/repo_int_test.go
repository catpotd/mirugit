package git

import (
	"context"
	"os"
	"os/exec"
	"testing"
)

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestLoadPutsTheCountsOnTheRightSide(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/staged.txt", []byte("a\nb\nc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, dir, "add", "staged.txt")
	if err := os.WriteFile(dir+"/loose.txt", []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	repo, err := Load(context.Background(), dir, mustGitDir(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]Entry{}
	for _, e := range repo.Entries {
		byPath[e.Path] = e
	}
	if got := byPath["staged.txt"].IndexCount; got.Added != 3 {
		t.Errorf("staged side = %+v, want 3 added", got)
	}
	if got := byPath["loose.txt"].IndexCount; got.Added != 0 {
		t.Errorf("an untracked file has nothing staged: %+v", got)
	}
}

// git diff --numstat refuses the whole repository when one path will not open,
// and the counts are a decoration on a row: the reader asked which files
// changed, and answering "none" because one of them cannot be measured is a
// wrong answer to the question that was asked.
func TestLoadKeepsTheListWhenOneFileCannotBeRead(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	if os.Geteuid() == 0 {
		t.Skip("root opens a file whose mode is 000")
	}
	dir := newRepo(t)
	env := gitTestEnv(dir)
	for _, name := range []string{"secret.txt", "other.txt"} {
		if err := os.WriteFile(dir+"/"+name, []byte("before\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runGitTest(t, env, dir, "add", ".")
	runGitTest(t, env, dir, "commit", "-q", "-m", "one")
	for _, name := range []string{"secret.txt", "other.txt"} {
		if err := os.WriteFile(dir+"/"+name, []byte("after\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(dir+"/secret.txt", 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir+"/secret.txt", 0o644) })

	repo, err := Load(context.Background(), dir, mustGitDir(t, dir))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	paths := make([]string, 0, len(repo.Entries))
	byPath := map[string]Entry{}
	for _, e := range repo.Entries {
		paths = append(paths, e.Path)
		byPath[e.Path] = e
	}
	if len(repo.Entries) != 2 {
		t.Errorf("changed files = %v, want both", paths)
	}
	// The one path git cannot open is the one that loses its figures. Drawing
	// the other as +0 −0 says it did not change, which is a different answer
	// from "this could not be counted".
	if got := byPath["other.txt"].WorktreeCount; got.Added != 1 || got.Deleted != 1 {
		t.Errorf("the readable file counts %+v, want 1 added and 1 deleted", got)
	}
	if got := byPath["secret.txt"].WorktreeCount; got != (Count{}) {
		t.Errorf("the unreadable file counts %+v, want nothing", got)
	}
	if repo.CountsErr == nil {
		t.Error("a count is absent and nothing says why")
	}
}
