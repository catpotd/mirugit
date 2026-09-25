package git

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestRunReadDoesNotTakeTheIndexLock(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/a.txt", []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lock, err := os.Create(dir + "/.git/index.lock")
	if err != nil {
		t.Fatal(err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}

	out, err := runRead(context.Background(), dir, "status", "--porcelain")
	if err != nil {
		t.Fatalf("reading with the index locked failed: %v", err)
	}
	if !strings.Contains(string(out), "a.txt") {
		t.Fatalf("want a.txt in %q", out)
	}
}

func TestExitErrorCarriesStderr(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	_, err := runWrite(context.Background(), dir, "rev-parse", "--verify", "does-not-exist")

	var exit *ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("want *ExitError, got %T: %v", err, err)
	}
	if exit.Code == 0 {
		t.Error("want a non-zero exit code")
	}
	if exit.Stderr == "" {
		t.Error("want stderr to be carried on the error")
	}
}

func TestRunWriteDoesNotSetOptionalLocks(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/a.txt", []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runWritePaths(context.Background(), dir, []string{":(literal)a.txt"}, "add"); err != nil {
		t.Fatal(err)
	}
	out, err := runWrite(context.Background(), dir, "diff", "--cached", "--name-only")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "a.txt") {
		t.Fatalf("want a.txt staged, got %q", out)
	}
}

func TestRunWithPathsSendsPathsOnStdin(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	for _, name := range []string{"plain.txt", ":magic.txt", "sp ace.txt"} {
		if err := os.WriteFile(dir+"/"+name, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	paths := []string{":(literal)plain.txt", ":(literal):magic.txt", ":(literal)sp ace.txt"}
	if err := runWritePaths(context.Background(), dir, paths, "add"); err != nil {
		t.Fatalf("add: %v", err)
	}

	out, err := runWrite(context.Background(), dir, "diff", "--cached", "--name-only", "-z")
	if err != nil {
		t.Fatal(err)
	}
	staged := strings.Split(strings.TrimRight(string(out), "\x00"), "\x00")
	if len(staged) != 3 {
		t.Fatalf("want 3 staged paths, got %q", staged)
	}
}
