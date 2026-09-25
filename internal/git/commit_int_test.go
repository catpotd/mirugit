package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestCommitRefusesWhileAnEntryIsUnmerged(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/a.txt", []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "a.txt")
	runGit(t, dir, "commit", "-q", "-m", "add")
	if err := os.WriteFile(dir+"/a.txt", []byte("stashed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "stash", "push", "-q", "-m", "s")
	if err := os.WriteFile(dir+"/a.txt", []byte("local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "a.txt")
	stashPopConflict(t, dir)
	if _, err := os.Stat(dir + "/.git/MERGE_HEAD"); err == nil {
		t.Fatal("MERGE_HEAD should not exist after a conflicted stash pop")
	}
	entries, _, err := Status(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	conflicted := false
	for _, e := range entries {
		if e.IsConflicted() {
			conflicted = true
		}
	}
	if !conflicted {
		t.Fatal("want a conflicted entry from stash pop")
	}
	_, err = Commit(context.Background(), dir, "msg")
	if !errors.Is(err, ErrUnmerged) {
		t.Fatalf("got %v, want ErrUnmerged", err)
	}
}

func TestCommitRefusesAnEmptyMessage(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/a.txt", []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "a.txt")
	if _, err := Commit(context.Background(), dir, ""); !errors.Is(err, ErrEmptyMessage) {
		t.Fatalf("got %v, want ErrEmptyMessage", err)
	}
}

func TestCommitRefusesWithNothingStaged(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/a.txt", []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Commit(context.Background(), dir, "msg"); !errors.Is(err, ErrNothingStaged) {
		t.Fatalf("got %v, want ErrNothingStaged", err)
	}
}

func TestCommitReturnsTheNewSha(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/a.txt", []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "a.txt")
	sha, err := Commit(context.Background(), dir, "add a")
	if err != nil {
		t.Fatal(err)
	}
	want, err := runGitOut(t, dir, "rev-parse", "--short", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if sha != want {
		t.Fatalf("got %q, want %q", sha, want)
	}
}

func stashPopConflict(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "stash", "pop")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("stash pop: want conflict, got success\n%s", out)
	}
}

func runGitOut(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	out, err := runWrite(context.Background(), dir, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
