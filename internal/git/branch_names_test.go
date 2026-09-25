package git

import (
	"context"
	"testing"
)

// stashBranchName asks this what exists so it can pick a name git will accept,
// and treats an error as "nothing exists". A mutation that returned nothing on
// success as well survived the suite: no test read what BranchNames answers.
func TestBranchNamesListsEveryBranch(t *testing.T) {
	t.Parallel()
	dir := newRepo(t)
	env := gitTestEnv(dir)
	runGitTest(t, env, dir, "branch", "main-stash")
	runGitTest(t, env, dir, "branch", "feature/x")

	names, err := BranchNames(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"main", "main-stash", "feature/x"} {
		if !names[want] {
			t.Errorf("%q is missing from %v", want, names)
		}
	}
	if names["main-stash-2"] {
		t.Errorf("%v holds a branch that was never created", names)
	}
}

func TestBranchNamesReportsARepositoryItCannotRead(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	if _, err := BranchNames(context.Background(), t.TempDir()); err == nil {
		t.Error("a directory that is not a repository read as a branch listing")
	}
}
