package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTopLevelResolvesTheRootFromASubdirectory(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.MkdirAll(dir+"/sub", 0o755); err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := TopLevel(context.Background(), dir+"/sub")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("TopLevel = %q, want %q", got, want)
	}
}

func TestTopLevelOnAMissingDirectoryCarriesGitsStderr(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to git")
	}
	t.Setenv("LC_ALL", "C")
	_, err := TopLevel(context.Background(), t.TempDir()+"/no-such-dir")
	var exit *ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("want *ExitError, got %T: %v", err, err)
	}
	if !strings.Contains(exit.Stderr, "cannot change to") {
		t.Errorf("stderr = %q, want cannot change to", exit.Stderr)
	}
}

func TestTopLevelOutsideARepositoryCarriesGitsStderr(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", dir)
	t.Setenv("LC_ALL", "C")
	_, err := TopLevel(context.Background(), dir)
	var exit *ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("want *ExitError, got %T: %v", err, err)
	}
	if !strings.Contains(exit.Stderr, "not a git repository") {
		t.Errorf("stderr = %q, want not a git repository", exit.Stderr)
	}
}

// IsBare answers a question about the repository, so a run that could not ask
// it answers false and says why. A caller reading the bool before the error —
// which is the reading the name invites — would otherwise be told that a
// directory git refused to look at is a bare repository, and mirugit refuses
// to draw a bare repository.
func TestIsBareAnswersFalseWhenItCouldNotAsk(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	for _, c := range []struct {
		name string
		dir  func(t *testing.T) string
	}{
		{"a directory that is not a repository", func(t *testing.T) string {
			return t.TempDir()
		}},
		{"a directory that is not there", func(t *testing.T) string {
			return t.TempDir() + "/no-such-dir"
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			bare, err := IsBare(context.Background(), c.dir(t))
			if err == nil {
				t.Fatal("git answered for a directory it cannot read")
			}
			if bare {
				t.Error("a directory git refused to look at is reported bare")
			}
		})
	}

	// The other two answers, so the false above is not the only one this can give.
	dir := newRepo(t)
	bare, err := IsBare(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if bare {
		t.Error("a repository with a working tree is reported bare")
	}
	bareDir := t.TempDir() + "/bare.git"
	runGit(t, t.TempDir(), "init", "-q", "--bare", bareDir)
	bare, err = IsBare(context.Background(), bareDir)
	if err != nil {
		t.Fatal(err)
	}
	if !bare {
		t.Error("a repository git keeps without a working tree is not reported bare")
	}
}
