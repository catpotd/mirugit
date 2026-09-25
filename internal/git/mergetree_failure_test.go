package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitThatFailsMergeTree puts a git ahead of the real one that passes every
// command through except merge-tree, which it fails without naming a path. The
// shim rather than a fake below it, for the reason gitThatDies gives: what is
// under test is what stashConflictPaths does with an exit status.
func gitThatFailsMergeTree(t *testing.T) {
	t.Helper()
	real, err := exec.LookPath("git")
	if err != nil {
		t.Skip("no git to stand in front of")
	}
	dir := t.TempDir()
	body := "#!/bin/sh\n" +
		"for a in \"$@\"; do\n" +
		"  if [ \"$a\" = merge-tree ]; then\n" +
		"    echo 'fatal: merge-tree refused' >&2\n" +
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

	paths, err := stashConflictPaths(context.Background(), dir, "stash@{0}")
	if err == nil {
		t.Fatalf("merge-tree failed and the answer was %v with no error", paths)
	}
	if !strings.Contains(err.Error(), "merge-tree refused") {
		t.Errorf("error = %q, want git's own line", err)
	}
}
