package git

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// The snapshot refs are what undo restores from, and the pane drops the old
// ones on every start. A delete that fails and says nothing leaves the refs
// piling up in a repository nobody is told about.
func TestAFailedUndoRefDeleteIsReported(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	for i := range 101 {
		runGitTest(t, gitTestEnv(dir), dir, "update-ref", undoRefPrefix+strconv.Itoa(i), "HEAD")
	}

	// A read-only .git lets the refs be listed and not removed. The mode is put
	// back before the directory is taken away.
	gitDir := filepath.Join(dir, ".git")
	info, err := os.Stat(gitDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(gitDir, info.Mode()) })
	if err := os.Chmod(gitDir, 0o555); err != nil {
		t.Fatal(err)
	}
	probe := filepath.Join(gitDir, "permission-probe")
	if err := os.WriteFile(probe, nil, 0o600); err == nil {
		if err := os.Remove(probe); err != nil {
			t.Fatal(err)
		}
		t.Skip("process can write into a read-only git directory")
	}

	if err := Prune(context.Background(), dir); err == nil {
		t.Error("a delete that could not run was reported as done")
	}
}
