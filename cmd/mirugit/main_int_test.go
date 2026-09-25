package main

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// The three ways git refuses a directory at startup each get a sentence of
// mirugit's own. main writes "mirugit: " in front of whatever comes back, so a
// line still carrying git's "fatal: " names the same thing twice, and the
// reader is told to run a command that is not the one that failed.
func TestEveryStartupRefusalIsSaidInMirugitsOwnWords(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to git")
	}
	for _, c := range []struct {
		name string
		dir  func(t *testing.T) string
		says string
		// Three of the four sentences are mirugit's own and name the directory
		// it was pointed at. The fourth is git's, repeated with its prefix
		// taken off, and git wrote it without a path in it.
		namesTheDirectory bool
	}{
		{"a directory that is not there", func(t *testing.T) string {
			return t.TempDir() + "/no-such-dir"
		}, "no such directory: ", true},
		{"a directory outside any repository", func(t *testing.T) string {
			dir := t.TempDir()
			t.Setenv("GIT_CEILING_DIRECTORIES", dir)
			return dir
		}, "not a git repository: ", true},
		{"a bare repository", func(t *testing.T) string {
			dir := t.TempDir() + "/bare.git"
			cmd := exec.Command("git", "init", "-q", "--bare", dir)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git init --bare: %v\n%s", err, out)
			}
			return dir
		}, "bare repository", true},
		// The fourth way, and the only one where git's own sentence is what
		// mirugit repeats: the directory is inside a repository that has a
		// working tree, and is not itself in one. git answers the question
		// about a working tree with a refusal and the question about bareness
		// with "no", so neither sentence above fits.
		{"the .git directory itself", func(t *testing.T) string {
			dir := t.TempDir() + "/repo"
			cmd := exec.Command("git", "init", "-q", "-b", "main", dir)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git init: %v\n%s", err, out)
			}
			return dir + "/.git"
		}, "must be run in a work tree", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("LC_ALL", "C")
			dir := c.dir(t)
			_, err := topLevel(context.Background(), dir)
			if err == nil {
				t.Fatal("want an error")
			}
			msg := err.Error()
			if !strings.Contains(msg, c.says) {
				t.Errorf("error = %q, want it to say %q", msg, c.says)
			}
			// main writes "mirugit: " in front of this.
			if strings.Contains(msg, "fatal: ") {
				t.Errorf("error = %q still carries git's prefix", msg)
			}
			if got := strings.Contains(msg, dir); got != c.namesTheDirectory {
				t.Errorf("error = %q names the directory = %v, want %v",
					msg, got, c.namesTheDirectory)
			}
		})
	}
}

func TestTopLevelOutsideARepositoryNamesThePath(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", dir)
	t.Setenv("LC_ALL", "C")
	_, err := topLevel(context.Background(), dir)
	if err == nil {
		t.Fatal("want an error outside a repository")
	}
	want := "not a git repository: " + dir
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestOutsideARepositoryItSaysSoAndExitsNonZero(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("builds and runs the binary")
	}
	bin := t.TempDir() + "/mirugit"
	build := exec.Command("go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	cmd := exec.Command(bin)
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), "GIT_CEILING_DIRECTORIES="+cmd.Dir)
	out, err := cmd.CombinedOutput()

	if err == nil {
		t.Error("want a non-zero exit outside a repository")
	}
	if !strings.Contains(string(out), "not a git repository") {
		t.Errorf("want an explanation, got %q", out)
	}
}
