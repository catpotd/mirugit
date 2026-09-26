package git

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestUntrackedStashDoesNotApplyWhenTheFileExists(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/new.txt", []byte("stashed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "stash", "push", "-q", "-u", "-m", "untracked")
	if err := os.WriteFile(dir+"/new.txt", []byte("blocking\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	status, _, err := stashStatusOf(context.Background(), dir, "stash@{0}")
	if err != nil {
		t.Fatal(err)
	}
	if status != StashConflicts {
		t.Fatalf("want conflicts, got %v", status)
	}
}

func TestStashWithoutUntrackedParentDoesNotFatalOnThirdParent(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/a.txt", []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "a.txt")
	runGit(t, dir, "commit", "-q", "-m", "add")
	if err := os.WriteFile(dir+"/a.txt", []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "stash", "push", "-q", "-m", "tracked only")
	status, _, err := stashStatusOf(context.Background(), dir, "stash@{0}")
	if err != nil {
		t.Fatal(err)
	}
	if status != StashApplies {
		t.Fatalf("want applies, got %v", status)
	}
}

func TestConflictingStashIsMarkedConflicts(t *testing.T) {
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
	runGit(t, dir, "stash", "push", "-q", "-m", "hold")
	if err := os.WriteFile(dir+"/a.txt", []byte("local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "a.txt")
	runGit(t, dir, "commit", "-q", "-m", "diverge")
	status, _, err := stashStatusOf(context.Background(), dir, "stash@{0}")
	if err != nil {
		t.Fatal(err)
	}
	if status != StashConflicts {
		t.Fatalf("want conflicts, got %v", status)
	}
}

// merge-tree compares the stash against HEAD and never looks at the working
// tree, so it calls a stash clean whose own paths are already edited. The real
// pop refuses and keeps the stash. The design says a false verdict is worse
// than none, so the working tree is checked before applies is claimed.
func TestStashDoesNotApplyOverItsOwnEditedPaths(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/a.txt", []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "base")
	if err := os.WriteFile(dir+"/a.txt", []byte("base\nSTASHED\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "stash", "-q")
	if err := os.WriteFile(dir+"/a.txt", []byte("base\nlocal\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	status, _, err := stashStatusOf(context.Background(), dir, "stash@{0}")
	if err != nil {
		t.Fatal(err)
	}
	if status == StashApplies {
		t.Error("claimed applies for a stash whose path is edited in the tree")
	}
}

func TestStashLeavingAStagedFileAloneStillApplies(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/a.txt", []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/c.txt", []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "base")
	if err := os.WriteFile(dir+"/a.txt", []byte("base\nEDIT\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/c.txt", []byte("staged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "c.txt")
	runGit(t, dir, "stash", "push", "-q", "-m", "partial", "--", "a.txt")

	status, _, err := stashStatusOf(context.Background(), dir, "stash@{0}")
	if err != nil {
		t.Fatal(err)
	}
	if status != StashApplies {
		t.Fatalf("want applies, got %v", status)
	}
}

func TestStashConflictPathsSkipsThePathAlreadyAtStashContent(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/a.txt", []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/c.txt", []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "base")
	if err := os.WriteFile(dir+"/a.txt", []byte("base\nEDIT\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/c.txt", []byte("staged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "c.txt")
	runGit(t, dir, "stash", "push", "-q", "-m", "partial", "--", "a.txt")

	result, err := stashConflictResultOf(context.Background(), dir, "stash@{0}")
	if err != nil {
		t.Fatal(err)
	}
	if result.mergeErr != nil && len(result.paths) == 0 {
		t.Fatal(result.mergeErr)
	}
	paths := result.paths
	if len(paths) != 0 {
		t.Fatalf("paths = %v, want none", paths)
	}
}

func TestStashConflictPathsNamesThePathEditedInTheTree(t *testing.T) {
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
	runGit(t, dir, "stash", "push", "-q", "-m", "hold")
	if err := os.WriteFile(dir+"/a.txt", []byte("local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := stashConflictResultOf(context.Background(), dir, "stash@{0}")
	if err != nil {
		t.Fatal(err)
	}
	if result.mergeErr != nil && len(result.paths) == 0 {
		t.Fatal(result.mergeErr)
	}
	paths := result.paths
	if len(paths) != 1 || paths[0] != "a.txt" {
		t.Fatalf("paths = %v, want [a.txt]", paths)
	}
}

func TestStashConflictPathsNamesTheUntrackedFileThatExists(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/new.txt", []byte("stashed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "stash", "push", "-q", "-u", "-m", "untracked")
	if err := os.WriteFile(dir+"/new.txt", []byte("blocking\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := stashConflictResultOf(context.Background(), dir, "stash@{0}")
	if err != nil {
		t.Fatal(err)
	}
	if result.mergeErr != nil && len(result.paths) == 0 {
		t.Fatal(result.mergeErr)
	}
	paths := result.paths
	if len(paths) != 1 || paths[0] != "new.txt" {
		t.Fatalf("paths = %v, want [new.txt]", paths)
	}
}

func TestStashConflictPathsListsAPathOnce(t *testing.T) {
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
	runGit(t, dir, "stash", "push", "-q", "-m", "hold")
	if err := os.WriteFile(dir+"/a.txt", []byte("local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "a.txt")
	runGit(t, dir, "commit", "-q", "-m", "diverge")
	if err := os.WriteFile(dir+"/a.txt", []byte("local2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := stashConflictResultOf(context.Background(), dir, "stash@{0}")
	if err != nil {
		t.Fatal(err)
	}
	if result.mergeErr != nil && len(result.paths) == 0 {
		t.Fatal(result.mergeErr)
	}
	paths := result.paths
	if len(paths) != 1 || paths[0] != "a.txt" {
		t.Fatalf("paths = %v, want [a.txt]", paths)
	}
}

func TestMergeTreeFailureWithoutConflictIsNotStashConflicts(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	ref := strings.Repeat("deadbeef", 5)
	status, _, err := stashStatusOf(context.Background(), dir, ref)
	if err == nil {
		t.Fatal("want error for invalid merge ref")
	}
	if status == StashConflicts {
		t.Fatalf("want not StashConflicts on merge-tree failure, got %v", status)
	}
}

func TestUnrelatedStashHistoryIsUnrelated(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := repoWithUnrelatedStash(t, false)

	status, paths, err := stashStatusOf(context.Background(), dir, "stash@{0}")
	if err != nil {
		t.Fatal(err)
	}
	if status != StashUnrelated {
		t.Fatalf("status = %v, want StashUnrelated", status)
	}
	if len(paths) != 0 {
		t.Fatalf("paths = %v, want none", paths)
	}
}

func TestUnrelatedStashHistoryWithEditedPathIsUnrelated(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := repoWithUnrelatedStash(t, true)

	status, paths, err := stashStatusOf(context.Background(), dir, "stash@{0}")
	if err != nil {
		t.Fatal(err)
	}
	if status != StashUnrelated {
		t.Fatalf("status = %v, want StashUnrelated", status)
	}
	if len(paths) != 0 {
		t.Fatalf("paths = %v, want none", paths)
	}
}

func repoWithUnrelatedStash(t *testing.T, keepPath bool) string {
	t.Helper()
	dir := newRepo(t)
	env := gitTestEnv(dir)
	if err := os.WriteFile(dir+"/f.txt", []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, env, dir, "add", "f.txt")
	runGitTest(t, env, dir, "commit", "-q", "-m", "base")
	if err := os.WriteFile(dir+"/f.txt", []byte("stashed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, env, dir, "stash", "push", "-q", "-m", "hold")
	runGitTest(t, env, dir, "checkout", "-q", "--orphan", "other")
	runGitTest(t, env, dir, "read-tree", "--empty")
	if keepPath {
		if err := os.WriteFile(dir+"/f.txt", []byte("local\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	} else if err := os.Remove(dir + "/f.txt"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/z.txt", []byte("other\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, env, dir, "add", "z.txt")
	runGitTest(t, env, dir, "commit", "-q", "-m", "other")
	return dir
}

func TestStashConflictsAlwaysNamesAFile(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}

	t.Run("untracked exists", func(t *testing.T) {
		dir := newRepo(t)
		if err := os.WriteFile(dir+"/new.txt", []byte("stashed\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		runGit(t, dir, "stash", "push", "-q", "-u", "-m", "untracked")
		if err := os.WriteFile(dir+"/new.txt", []byte("blocking\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		assertStashConflictsNameAFile(t, dir)
	})

	t.Run("edited in tree", func(t *testing.T) {
		dir := newRepo(t)
		if err := os.WriteFile(dir+"/a.txt", []byte("base\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		runGit(t, dir, "add", "a.txt")
		runGit(t, dir, "commit", "-q", "-m", "add")
		if err := os.WriteFile(dir+"/a.txt", []byte("stashed\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		runGit(t, dir, "stash", "push", "-q", "-m", "hold")
		if err := os.WriteFile(dir+"/a.txt", []byte("local\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		assertStashConflictsNameAFile(t, dir)
	})

	t.Run("diverged", func(t *testing.T) {
		dir := newRepo(t)
		if err := os.WriteFile(dir+"/a.txt", []byte("base\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		runGit(t, dir, "add", "a.txt")
		runGit(t, dir, "commit", "-q", "-m", "add")
		if err := os.WriteFile(dir+"/a.txt", []byte("stashed\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		runGit(t, dir, "stash", "push", "-q", "-m", "hold")
		if err := os.WriteFile(dir+"/a.txt", []byte("local\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		runGit(t, dir, "add", "a.txt")
		runGit(t, dir, "commit", "-q", "-m", "diverge")
		assertStashConflictsNameAFile(t, dir)
	})
}

func assertStashConflictsNameAFile(t *testing.T, dir string) {
	t.Helper()
	status, _, err := stashStatusOf(context.Background(), dir, "stash@{0}")
	if err != nil {
		t.Fatal(err)
	}
	if status != StashConflicts {
		t.Fatalf("want conflicts, got %v", status)
	}
	// The status and the list of colliding paths come off one answer, so a
	// stash that conflicts has to be able to name where.
	files, err := StashFiles(context.Background(), dir, "stash@{0}")
	if err != nil {
		t.Fatal(err)
	}
	rows := stashListFilled(t, dir, Head{})
	if len(rows) == 0 {
		t.Fatal("no stash was listed")
	}
	for _, f := range files {
		if rows[0].CollidesWith(f.Path) {
			return
		}
	}
	t.Fatalf("the stash conflicts and names no file: %v", rows[0].Collides)
}

// A path can hold the stash's own content in the index and something else in
// the working tree: staging what was stashed and then editing the file again.
// The index alone says the stash would change nothing, and merge-tree compares
// the stash against HEAD and never looks at the working tree, so neither of
// them sees the edit. The stash still lands on a file that was changed after
// it was taken, which is what the row names.
func TestStashConflictPathsNamesThePathStagedAtStashContentAndEditedAfter(t *testing.T) {
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
	runGit(t, dir, "stash", "push", "-q", "-m", "hold")

	// The index now holds what the stash recorded, and the working tree holds
	// one more line on top of it.
	if err := os.WriteFile(dir+"/a.txt", []byte("stashed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "a.txt")
	if err := os.WriteFile(dir+"/a.txt", []byte("stashed\nEDIT\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := stashConflictResultOf(context.Background(), dir, "stash@{0}")
	if err != nil {
		t.Fatal(err)
	}
	if result.mergeErr != nil && len(result.paths) == 0 {
		t.Fatal(result.mergeErr)
	}
	paths := result.paths
	if len(paths) != 1 || paths[0] != "a.txt" {
		t.Fatalf("paths = %v, want [a.txt]", paths)
	}
}
