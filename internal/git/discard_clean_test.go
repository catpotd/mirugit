package git

import (
	"context"
	"testing"
)

// Discard runs up to three writes: it unstages, it restores, and it removes the
// untracked files with git clean. A write that fails and says nothing leaves
// the pane reporting a discard that did not happen, and the changes the reader
// asked to be rid of are still there.
func TestEveryWriteADiscardRunsReportsItsFailure(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	paths := []string{":(literal)u.txt"}
	for _, c := range []struct {
		name                                string
		stagedOnly, fullRestore, cleanPaths []string
		// A repository with no commit takes the removal branch of the restore;
		// one with a commit takes the branch that restores from it.
		repo bool
	}{
		{name: "the unstage", stagedOnly: paths},
		{name: "the removal a repository with no commit uses", fullRestore: paths},
		{name: "the restore from a commit", fullRestore: paths, repo: true},
		{name: "the clean", cleanPaths: paths},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			if c.repo {
				// A real repository, and a path it has never heard of.
				dir = newRepo(t)
			}
			err := runDiscardWrites(context.Background(), dir, c.stagedOnly, c.fullRestore, c.cleanPaths)
			if err == nil {
				t.Error("a write that could not run was reported as done")
			}
		})
	}
}
