package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// gitThatFailsMergeTree puts a git ahead of the real one that passes every
// command through except merge-tree, which it fails without naming a path. The
// shim rather than a fake below it, for the reason gitThatDies gives: what is
// under test is what stashConflictResultOf does with an exit status.
func gitThatFailsMergeTree(t *testing.T) {
	gitThatFailsCommandWithMessage(t, "merge-tree", "fatal: merge-tree refused")
}

func gitThatFailsCommand(t *testing.T, command string) {
	gitThatFailsCommandWithMessage(t, command, "fatal: refusing to merge unrelated histories")
}

func gitThatFailsCommandWithMessage(t *testing.T, command, message string) {
	t.Helper()
	real, err := exec.LookPath("git")
	if err != nil {
		t.Skip("no git to stand in front of")
	}
	dir := t.TempDir()
	body := "#!/bin/sh\n" +
		"for a in \"$@\"; do\n" +
		"  if [ \"$a\" = \"" + command + "\" ]; then\n" +
		"    echo '" + message + "' >&2\n" +
		"    exit 128\n" +
		"  fi\n" +
		"done\n" +
		"exec " + real + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// A merge-tree that failed and named no path is a question that was not
// answered, and answering it as "no conflicts" tells the reader a stash applies
// cleanly when nothing checked whether it does. The paths from the other two
// lookups are what makes the difference: with one of those in hand the failure
// is already described, and the row can say which file is the problem.
func TestAMergeTreeThatFailedAndNamedNoPathIsAnError(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/a.txt", []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "a.txt")
	runGit(t, dir, "commit", "-q", "-m", "base")
	if err := os.WriteFile(dir+"/a.txt", []byte("stashed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "stash", "push", "-q", "-m", "one")

	gitThatFailsMergeTree(t)

	result, err := stashConflictResultOf(context.Background(), dir, "stash@{0}")
	if err == nil {
		if result.mergeErr == nil {
			t.Fatalf("merge-tree failed and the answer was %v with no error", result.paths)
		}
		err = result.mergeErr
	}
	if !strings.Contains(err.Error(), "merge-tree refused") {
		t.Errorf("error = %q, want git's own line", err)
	}
}

func TestUnrelatedHistoryErrorRequiresTheMergeTreeCommand(t *testing.T) {
	t.Parallel()
	const message = "fatal: refusing to merge unrelated histories"
	cases := []struct {
		name string
		err  *ExitError
		want bool
	}{
		{
			name: "merge-tree",
			err:  &ExitError{Args: []string{"merge-tree", "--write-tree", "head", "stash@{0}"}, Code: 128, Stderr: message},
			want: true,
		},
		{
			name: "stash show",
			err:  &ExitError{Args: []string{"stash", "show"}, Code: 128, Stderr: message},
		},
		{
			name: "ls-tree",
			err:  &ExitError{Args: []string{"ls-tree", "-r"}, Code: 128, Stderr: message},
		},
		{
			name: "different exit code",
			err:  &ExitError{Args: []string{"merge-tree", "--write-tree", "head", "stash@{0}"}, Code: 1, Stderr: message},
		},
		{
			name: "signal",
			err:  &ExitError{Args: []string{"merge-tree", "--write-tree", "head", "stash@{0}"}, Code: -1, Signal: syscall.SIGTERM, Stderr: message},
		},
		{
			name: "extra diagnostic",
			err:  &ExitError{Args: []string{"merge-tree", "--write-tree", "head", "stash@{0}"}, Code: 128, Stderr: message + "\nextra"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isUnrelatedHistoryError(tc.err); got != tc.want {
				t.Errorf("isUnrelatedHistoryError() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSameUnrelatedMessageFromStashShowRemainsAnError(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	env := gitTestEnv(dir)
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, env, dir, "add", "f.txt")
	runGitTest(t, env, dir, "commit", "-q", "-m", "base")
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("stashed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, env, dir, "stash", "push", "-q", "-m", "hold")
	gitThatFailsCommand(t, "stash")

	_, _, err := stashStatusOf(context.Background(), dir, "stash@{0}")
	if err == nil {
		t.Fatal("stash show failure was swallowed")
	}
	var exit *ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("error = %T %v, want *ExitError", err, err)
	}
	if exit.Args[0] != "stash" || !strings.Contains(exit.Stderr, "refusing to merge unrelated histories") {
		t.Fatalf("exit error = %+v, want stash show diagnostic", exit)
	}
}
