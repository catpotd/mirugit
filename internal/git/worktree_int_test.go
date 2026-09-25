package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func newWorktreeRemoteRepo(t *testing.T) (env []string, mainDir string) {
	t.Helper()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	base := t.TempDir()
	env = gitTestEnv(base)
	remoteDir := filepath.Join(base, "remote.git")
	mainDir = filepath.Join(base, "main")
	runGitTest(t, env, base, "init", "--bare", "-q", "-b", "main", remoteDir)
	runGitTest(t, env, base, "clone", "-q", remoteDir, "main")
	runGitTest(t, env, mainDir, "commit", "-q", "--allow-empty", "-m", "base")
	runGitTest(t, env, mainDir, "push", "-q", "-u", "origin", "main")
	return env, mainDir
}

func TestWorktreeListShowsEveryLinkedTree(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	env, mainDir := newWorktreeRemoteRepo(t)
	wtA := filepath.Join(filepath.Dir(mainDir), "wt-a")
	wtB := filepath.Join(filepath.Dir(mainDir), "wt-b")
	runGitTest(t, env, mainDir, "worktree", "add", "-q", wtA, "-b", "feat-a")
	runGitTest(t, env, mainDir, "worktree", "add", "-q", wtB, "-b", "feat-b")

	rows, base, err := WorktreeList(context.Background(), mainDir)
	if err != nil {
		t.Fatal(err)
	}
	if base != "main" {
		t.Errorf("base = %q, want main", base)
	}
	names := map[string]bool{}
	for _, row := range rows {
		names[row.Name] = true
	}
	for _, want := range []string{"main", "wt-a", "wt-b"} {
		if !names[want] {
			t.Errorf("missing worktree %q in %+v", want, rows)
		}
	}
}

// Removing the tree leaves the branch pointing at those commits, so the offer
// stands. TestWorktreeRemoveKeepsUnpushedCommits measures that directly.
func TestWorktreeWithUnpushedCommitsStillOffersRemove(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	env, mainDir := newWorktreeRemoteRepo(t)
	wtDir := filepath.Join(filepath.Dir(mainDir), "wt-unpushed")
	runGitTest(t, env, mainDir, "worktree", "add", "-q", wtDir, "-b", "feat-unpushed")
	runGitTest(t, env, wtDir, "commit", "-q", "--allow-empty", "-m", "local only")

	rows, base, err := WorktreeList(context.Background(), mainDir)
	if err != nil {
		t.Fatal(err)
	}
	var row *WorktreeRow
	for i := range rows {
		if rows[i].Name == "wt-unpushed" {
			row = &rows[i]
			break
		}
	}
	if row == nil {
		t.Fatalf("worktree wt-unpushed not listed: %+v", rows)
	}
	if err := FillWorktreeStatus(context.Background(), row, base); err != nil {
		t.Fatal(err)
	}
	if !row.CanRemove() {
		t.Fatal("remove withheld from a clean tree that only has unpushed commits")
	}
}

func TestFillWorktreeStatusSetsDirtyAndWrote(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	env, mainDir := newWorktreeRemoteRepo(t)
	wtDir := filepath.Join(filepath.Dir(mainDir), "wt-dirty")
	runGitTest(t, env, mainDir, "worktree", "add", "-q", wtDir, "-b", "feat-dirty")
	if err := os.WriteFile(filepath.Join(wtDir, "new.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rows, base, err := WorktreeList(context.Background(), mainDir)
	if err != nil {
		t.Fatal(err)
	}
	var row *WorktreeRow
	for i := range rows {
		if rows[i].Name == "wt-dirty" {
			row = &rows[i]
			break
		}
	}
	if row == nil {
		t.Fatalf("worktree wt-dirty missing from list: %+v", rows)
	}
	if err := FillWorktreeStatus(context.Background(), row, base); err != nil {
		t.Fatal(err)
	}
	if row.Dirty != 1 {
		t.Errorf("dirty = %d, want 1", row.Dirty)
	}
	if row.WroteAge == "" || row.WroteAge == "…" {
		t.Errorf("wrote age = %q", row.WroteAge)
	}
}

// Removing a worktree leaves its branch and its commits where they were, so
// unpushed work is not a reason to withhold the offer. What git does refuse is
// the main tree and a tree holding uncommitted work, and it exits 128 on both.
func TestWorktreeRemoveKeepsUnpushedCommits(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	base := t.TempDir()
	env := gitTestEnv(base)
	main := filepath.Join(base, "main")
	runGitTest(t, env, base, "init", "-q", "-b", "main", main)
	if err := os.WriteFile(filepath.Join(main, "a.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, env, main, "add", "-A")
	runGitTest(t, env, main, "commit", "-q", "-m", "base")

	side := filepath.Join(base, "side")
	runGitTest(t, env, main, "worktree", "add", "-q", side, "-b", "side")
	if err := os.WriteFile(filepath.Join(side, "b.txt"), []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, env, side, "add", "-A")
	runGitTest(t, env, side, "commit", "-q", "-m", "unpushed")

	want := runWorktreeGit(t, env, side, "rev-parse", "HEAD")
	runGitTest(t, env, main, "worktree", "remove", side)
	if got := runWorktreeGit(t, env, main, "rev-parse", "side"); got != want {
		t.Errorf("branch side moved to %q, want %q", got, want)
	}
}

func runWorktreeGit(t *testing.T, env []string, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = env
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

// worktree remove names a directory, not paths. The safe-path wrapper appends
// --pathspec-from-file and git rejects the whole command with exit 129.
func TestWorktreeRemoveActuallyRemoves(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	base := t.TempDir()
	env := gitTestEnv(base)
	main := filepath.Join(base, "main")
	runGitTest(t, env, base, "init", "-q", "-b", "main", main)
	runGitTest(t, env, main, "commit", "-q", "--allow-empty", "-m", "base")
	side := filepath.Join(base, "side")
	runGitTest(t, env, main, "worktree", "add", "-q", side, "-b", "side")

	if err := RemoveWorktree(context.Background(), main, side); err != nil {
		t.Fatalf("RemoveWorktree: %v", err)
	}
	if _, err := os.Stat(side); !os.IsNotExist(err) {
		t.Errorf("the worktree directory is still there: %v", err)
	}
}

// for-each-ref computes ahead-behind against a ref, and a repository with no
// commits has no branch ref to compare with. The pane opens on repositories an
// agent has just created.
func TestWorktreeListWorksBeforeTheFirstCommit(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	base := t.TempDir()
	env := gitTestEnv(base)
	dir := filepath.Join(base, "fresh")
	runGitTest(t, env, base, "init", "-q", "-b", "main", dir)

	rows, _, err := WorktreeList(context.Background(), dir)
	if err != nil {
		t.Fatalf("WorktreeList before the first commit: %v", err)
	}
	if len(rows) != 1 {
		t.Errorf("%d worktrees, want the main one", len(rows))
	}
}

func TestDiffReadsALinkedWorktreePath(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	env, mainDir := newWorktreeRemoteRepo(t)
	wtDir := filepath.Join(filepath.Dir(mainDir), "wt-side")
	runGitTest(t, env, mainDir, "worktree", "add", "-q", wtDir, "-b", "feat-side")
	const path = "only-here.txt"
	const body = "worktree change\n"
	if err := os.WriteFile(filepath.Join(wtDir, path), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	diff, err := Diff(context.Background(), wtDir, path, false)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, b := range diff.Blocks {
		for _, line := range b.Lines {
			if strings.Contains(line, "worktree change") {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("worktree diff = %+v, want %q", diff, body)
	}
}

// The merge verdict is the one thing about a tree the reader cannot work out
// from its name, and FillWorktreeStatus is where it is put on the row. The test
// above reads the two fields that come from status and stops before this one,
// so the line that carries the verdict across could be dropped and the row
// would still look filled in.
func TestFillWorktreeStatusSetsTheMergeVerdict(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	env, mainDir := newWorktreeRemoteRepo(t)
	wtDir := filepath.Join(filepath.Dir(mainDir), "wt-merge")
	runGitTest(t, env, mainDir, "worktree", "add", "-q", wtDir, "-b", "feat-merge")

	rows, base, err := WorktreeList(context.Background(), mainDir)
	if err != nil {
		t.Fatal(err)
	}
	var row *WorktreeRow
	for i := range rows {
		if rows[i].Name == "wt-merge" {
			row = &rows[i]
			break
		}
	}
	if row == nil {
		t.Fatalf("worktree wt-merge missing from list: %+v", rows)
	}
	if row.Merge != MergeUnknown {
		t.Fatalf("the row arrives with a verdict already on it, so this proves nothing: %v", row.Merge)
	}

	if err := FillWorktreeStatus(context.Background(), row, base); err != nil {
		t.Fatal(err)
	}
	if row.Merge == MergeUnknown {
		t.Error("the row has no merge verdict after its status was filled in")
	}

	want, wantConflicts, err := branchMergeStatus(context.Background(), row.Path, base, row.Branch)
	if err != nil {
		t.Fatal(err)
	}
	if row.Merge != want || row.Conflicts != wantConflicts {
		t.Errorf("the row says %v with %d conflicts; git says %v with %d",
			row.Merge, row.Conflicts, want, wantConflicts)
	}
}

// A tree whose merge verdict cannot be read is an error, not a row with no
// verdict on it: the caller draws what it is handed, and a row that quietly
// says "unknown" reads as an answer.
func TestFillWorktreeStatusReportsAVerdictItCouldNotRead(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	env, mainDir := newWorktreeRemoteRepo(t)
	wtDir := filepath.Join(filepath.Dir(mainDir), "wt-gone")
	runGitTest(t, env, mainDir, "worktree", "add", "-q", wtDir, "-b", "feat-gone")

	rows, _, err := WorktreeList(context.Background(), mainDir)
	if err != nil {
		t.Fatal(err)
	}
	var row *WorktreeRow
	for i := range rows {
		if rows[i].Name == "wt-gone" {
			row = &rows[i]
			break
		}
	}
	if row == nil {
		t.Fatalf("worktree wt-gone missing from list: %+v", rows)
	}

	// The tree's own directory, gone. A base that does not resolve is not the
	// case: branchMergeStatus answers MergeClean for it, because there is
	// nothing to merge onto before the first commit.
	if err := os.RemoveAll(row.Path); err != nil {
		t.Fatal(err)
	}
	if err := FillWorktreeStatus(context.Background(), row, "main"); err == nil {
		t.Errorf("a tree that is not on disk was answered with %v and no error", row.Merge)
	}
}

// The files a worktree changed carry their own counts, and a file that is only
// staged has them on the staged side. Reading the unstaged side twice leaves
// that file at zero, and the row says a file changed and nothing about by how
// much.
func TestAStagedFileInAWorktreeCarriesItsCounts(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	env, mainDir := newWorktreeRemoteRepo(t)
	wtDir := filepath.Join(filepath.Dir(mainDir), "wt-counts")
	runGitTest(t, env, mainDir, "worktree", "add", "-q", wtDir, "-b", "feat-counts")

	if err := os.WriteFile(filepath.Join(wtDir, "staged.txt"), []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, env, wtDir, "add", "staged.txt")
	if err := os.WriteFile(filepath.Join(wtDir, "loose.txt"), []byte("three\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := WorktreeFiles(context.Background(), wtDir)
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]Entry{}
	for _, f := range files {
		by[f.Path] = f
	}
	staged, ok := by["staged.txt"]
	if !ok {
		t.Fatalf("the staged file is not in %v", files)
	}
	if staged.IndexCount.Added != 2 {
		t.Errorf("the staged file added %d lines, want 2: %+v",
			staged.IndexCount.Added, staged)
	}
	// The other side is read by its own call, and a file that is not staged
	// carries its counts there.
	loose, ok := by["loose.txt"]
	if !ok {
		t.Fatalf("the unstaged file is not in %v", files)
	}
	if loose.WorktreeCount.Added != 1 {
		t.Errorf("the unstaged file added %d lines, want 1: %+v",
			loose.WorktreeCount.Added, loose)
	}
}

// The branches and where they stand are read in one for-each-ref. A read that
// failed leaves nothing to parse, and parsing nothing gives an empty set:
// every row would then say its branch has no upstream distance and no sha,
// which is what a repository with no commits looks like. The error has to
// reach the caller so the pane says it could not read rather than drawing a
// repository that is not there.
func TestReadingTheBranchesReportsAFailureRatherThanAnEmptySet(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	runGit(t, dir, "commit", "-q", "--allow-empty", "-m", "one")
	runGit(t, dir, "branch", "side")

	refs, err := branchRefs(context.Background(), dir, "main")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"main", "side"} {
		if _, ok := refs[want]; !ok {
			t.Errorf("%q is missing from %v", want, refs)
		}
	}

	// A directory git cannot read answers an error, and the set that comes
	// with it is not one the caller should draw.
	refs, err = branchRefs(context.Background(), t.TempDir(), "main")
	if err == nil {
		t.Errorf("a directory that is not a repository answered %v", refs)
	}
}

// A branch that merges cleanly says so. Reporting a conflict with a count of
// none draws "conflicts 0" on the row, which tells the reader to go and settle
// something that is not there.
func TestACleanBranchIsNotReportedAsConflicting(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "a.txt")
	runGit(t, dir, "commit", "-q", "-m", "base")
	runGit(t, dir, "checkout", "-q", "-b", "side")
	// A file of its own, so the two branches touch nothing in common.
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("side\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "b.txt")
	runGit(t, dir, "commit", "-q", "-m", "side")

	status, count, err := branchMergeStatus(context.Background(), dir, "main", "side")
	if err != nil {
		t.Fatal(err)
	}
	if status != MergeClean || count != 0 {
		t.Errorf("a branch that merges is %v with %d conflicts, want clean and none",
			status, count)
	}

	// A branch that does conflict is reported, so the answer above is not
	// "always clean".
	runGit(t, dir, "checkout", "-q", "main")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "commit", "-q", "-am", "main")
	runGit(t, dir, "checkout", "-q", "side")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("other\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "commit", "-q", "-am", "side again")

	status, count, err = branchMergeStatus(context.Background(), dir, "main", "side")
	if err != nil {
		t.Fatal(err)
	}
	if status != MergeConflicts || count == 0 {
		t.Errorf("a branch that conflicts is %v with %d conflicts", status, count)
	}
}
