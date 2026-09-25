package git

import (
	"context"
	"os"
	"testing"
)

// merge-tree refuses unrelated histories with exit 128 and prints no stage
// lines. Treating every non-zero exit as a conflict turned that refusal into
// "conflicts 1" on the worktrees tab, so git's own failure was shown to the
// reader as a fact about the branch.
func TestUnrelatedHistoriesAreNotReportedAsConflicts(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	env := gitTestEnv(dir)
	if err := os.WriteFile(dir+"/f.txt", []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, env, dir, "add", "f.txt")
	runGitTest(t, env, dir, "commit", "-q", "-m", "f")
	runGitTest(t, env, dir, "checkout", "-q", "--orphan", "other")
	runGitTest(t, env, dir, "rm", "-q", "-rf", ".")
	if err := os.WriteFile(dir+"/z.txt", []byte("z\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, env, dir, "add", "z.txt")
	runGitTest(t, env, dir, "commit", "-q", "-m", "orphan")

	status, n, err := branchMergeStatus(context.Background(), dir, "main", "other")
	if err == nil {
		t.Fatalf("エラーを返さなかった: status=%v conflicts=%d", status, n)
	}
	if status != MergeUnknown {
		t.Errorf("status = %v, want MergeUnknown", status)
	}
	if n != 0 {
		t.Errorf("conflicts = %d, want 0", n)
	}
}

// A real conflict still reports its count, now in the same unit the stash side
// uses: the paths merge-tree could not merge.
func TestConflictingBranchesCountTheirPaths(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	env := gitTestEnv(dir)
	for _, name := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(dir+"/"+name, []byte("base\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runGitTest(t, env, dir, "add", ".")
	runGitTest(t, env, dir, "commit", "-q", "-m", "base files")
	runGitTest(t, env, dir, "checkout", "-q", "-b", "side")
	for _, name := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(dir+"/"+name, []byte("side\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runGitTest(t, env, dir, "commit", "-q", "-am", "side")
	runGitTest(t, env, dir, "checkout", "-q", "main")
	for _, name := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(dir+"/"+name, []byte("main\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runGitTest(t, env, dir, "commit", "-q", "-am", "main")

	status, n, err := branchMergeStatus(context.Background(), dir, "main", "side")
	if err != nil {
		t.Fatal(err)
	}
	if status != MergeConflicts {
		t.Fatalf("status = %v, want MergeConflicts", status)
	}
	if n != 2 {
		t.Errorf("conflicts = %d, want 2", n)
	}
}
