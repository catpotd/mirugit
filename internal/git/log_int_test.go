package git

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestLogReadsTheGitAuthor(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	runGit(t, dir, "config", "user.name", "Taro")
	runGit(t, dir, "config", "user.email", "taro@example.com")
	runGit(t, dir, "commit", "-q", "--allow-empty", "-m", "first")
	runGit(t, dir, "config", "user.name", "Hanako")
	runGit(t, dir, "config", "user.email", "hanako@example.com")
	runGit(t, dir, "commit", "-q", "--allow-empty", "-m", "second")

	commits, err := Log(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) < 2 {
		t.Fatalf("want at least 2 commits, got %d", len(commits))
	}
	for i, want := range []string{"Hanako", "Taro"} {
		if got := commits[i].Author; got != want {
			t.Errorf("commit %d (%q) author = %q, want %q", i, commits[i].Subject, got, want)
		}
	}
}

// Without an upstream no remote holds anything, so every commit is unpushed and
// none of them has anywhere to go. The arrow and the push verb read both facts;
// undo reads only the first.
func TestLogWithoutUpstreamHasNowhereToPush(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	runGit(t, dir, "commit", "-q", "--allow-empty", "-m", "one")
	runGit(t, dir, "commit", "-q", "--allow-empty", "-m", "two")

	commits, err := Log(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range commits {
		if !c.Unpushed {
			t.Errorf("commit %d counted as reaching a remote there is none of", i)
		}
		if c.HasUpstream {
			t.Errorf("commit %d claims an upstream", i)
		}
	}
}

func TestLogMarksCommitsAheadOfUpstream(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	runGit(t, dir, "commit", "-q", "--allow-empty", "-m", "pushed")
	remote := t.TempDir()
	cmd := exec.Command("git", "init", "-q", "--bare", "-b", "main")
	cmd.Dir = remote
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, out)
	}
	runGit(t, dir, "remote", "add", "origin", remote)
	runGit(t, dir, "push", "-q", "-u", "origin", "main")
	runGit(t, dir, "commit", "-q", "--allow-empty", "-m", "local only")

	commits, err := Log(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) < 2 {
		t.Fatalf("want 2 commits, got %d", len(commits))
	}
	if !commits[0].Unpushed {
		t.Error("tip should be unpushed")
	}
	if commits[1].Unpushed {
		t.Error("pushed commit should not be unpushed")
	}
}

func TestResetSoftRemovesTipCommit(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	runGit(t, dir, "commit", "-q", "--allow-empty", "-m", "stay")
	runGit(t, dir, "commit", "-q", "--allow-empty", "-m", "gone")
	if err := ResetSoft(context.Background(), dir); err != nil {
		t.Fatal(err)
	}
	out, err := runWrite(context.Background(), dir, "log", "--format=%s", "-1")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(out)) != "stay" {
		t.Fatalf("HEAD = %q, want stay", out)
	}
}

// reset --soft names a revision, not paths. The safe-path wrapper appends
// --pathspec-from-file and git then cannot tell which is which.
func TestResetSoftMovesHeadBack(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	runGit(t, dir, "commit", "-q", "--allow-empty", "-m", "one")
	runGit(t, dir, "commit", "-q", "--allow-empty", "-m", "two")
	before, err := Log(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := ResetSoft(context.Background(), dir); err != nil {
		t.Fatalf("ResetSoft: %v", err)
	}
	after, err := Log(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before)-1 {
		t.Errorf("%d commits after undo, want %d", len(after), len(before)-1)
	}
}

func TestResetSoftOnTheRootCommitKeepsTheTreeStaged(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")
	if err := os.WriteFile(dir+"/a.txt", []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-q", "-m", "only")
	if err := ResetSoft(context.Background(), dir); err != nil {
		t.Fatal(err)
	}
	commits, err := Log(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 0 {
		t.Fatalf("want 0 commits after undoing the root, got %d", len(commits))
	}
	out, err := runWrite(context.Background(), dir, "diff", "--cached", "--name-only")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "a.txt") {
		t.Fatalf("index lost a.txt after root undo:\n%s", out)
	}
}

// The pane opens on a repository an agent has just created, which has no log.
func TestLogIsEmptyBeforeTheFirstCommit(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")
	commits, err := Log(context.Background(), dir)
	if err != nil {
		t.Fatalf("Log before the first commit: %v", err)
	}
	if len(commits) != 0 {
		t.Errorf("%d commits, want none", len(commits))
	}
}

func TestCommitFilesListsRootCommitFiles(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")
	if err := os.WriteFile(dir+"/a.txt", []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir+"/src", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/src/c.go", []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "init")

	commits, err := Log(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 1 {
		t.Fatalf("want 1 commit, got %d", len(commits))
	}
	entries, err := CommitFiles(context.Background(), dir, commits[0].SHA)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(entries))
	}
	var aTxt *Entry
	for i := range entries {
		switch entries[i].Path {
		case "a.txt":
			aTxt = &entries[i]
			if entries[i].Worktree != Added {
				t.Errorf("a.txt Worktree = %c, want A", entries[i].Worktree)
			}
		case "src/c.go":
			if entries[i].Worktree != Added {
				t.Errorf("src/c.go Worktree = %c, want A", entries[i].Worktree)
			}
		default:
			t.Errorf("unexpected path %q", entries[i].Path)
		}
	}
	if aTxt == nil {
		t.Fatal("a.txt not found")
	}
	if aTxt.WorktreeCount.Added != 1 || aTxt.WorktreeCount.Deleted != 0 {
		t.Errorf("a.txt counts = %+v, want Added:1 Deleted:0", aTxt.WorktreeCount)
	}
}

func TestCommitFilesKeepsRenameDetection(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/a.txt", []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "a.txt")
	runGit(t, dir, "commit", "-q", "-m", "add a")
	runGit(t, dir, "mv", "a.txt", "renamed.txt")
	runGit(t, dir, "commit", "-q", "-m", "rename")

	commits, err := Log(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := CommitFiles(context.Background(), dir, commits[0].SHA)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.OldPath != "a.txt" {
		t.Errorf("OldPath = %q, want a.txt", e.OldPath)
	}
	if e.Path != "renamed.txt" {
		t.Errorf("Path = %q, want renamed.txt", e.Path)
	}
	if e.Worktree != Renamed {
		t.Errorf("Worktree = %c, want R", e.Worktree)
	}
}
