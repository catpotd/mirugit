package state

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func pruneTestRepo(t *testing.T) (dir string, env []string) {
	t.Helper()
	dir = t.TempDir()
	env = []string{
		"HOME=" + dir,
		"GIT_CONFIG_GLOBAL=" + dir + "/.gitconfig",
		"GIT_CONFIG_SYSTEM=/dev/null",
		// git forks a repack of its own once a repository crosses a
		// threshold, and that child outlives the test: it races
		// t.TempDir's removal, and it takes the repository's locks
		// while another test is reading it.
		"GIT_CONFIG_COUNT=2",
		"GIT_CONFIG_KEY_0=gc.auto", "GIT_CONFIG_VALUE_0=0",
		"GIT_CONFIG_KEY_1=maintenance.auto", "GIT_CONFIG_VALUE_1=false",
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
	}
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("commit", "-q", "--allow-empty", "-m", "base")
	return dir, env
}

func loadTemp(t *testing.T) *ReadState {
	t.Helper()
	repo, _ := pruneTestRepo(t)
	rs, err := LoadRead(context.Background(), t.TempDir(), repo)
	if err != nil {
		t.Fatal(err)
	}
	return rs
}

func TestAFileNeverOpenedIsUnread(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	rs := loadTemp(t)
	if got := rs.MarkIn(WorkingTree(SectionUnstaged), "a.txt", []string{"h1"}); got != Unread {
		t.Errorf("got %v, want Unread", got)
	}
}

func TestAFileReadAllTheWayThroughIsRead(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	rs := loadTemp(t)
	rs.MarkFileIn(WorkingTree(SectionUnstaged), "a.txt", []string{"h1", "h2"})
	if got := rs.MarkIn(WorkingTree(SectionUnstaged), "a.txt", []string{"h1", "h2"}); got != Read {
		t.Errorf("got %v, want Read", got)
	}
}

func TestAFileReadThroughThenChangedIsChanged(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	rs := loadTemp(t)
	rs.MarkFileIn(WorkingTree(SectionUnstaged), "a.txt", []string{"h1"})
	if got := rs.MarkIn(WorkingTree(SectionUnstaged), "a.txt", []string{"h1", "h2"}); got != Changed {
		t.Errorf("got %v, want Changed", got)
	}
}

func TestAFilePartlyReadStaysUnreadRatherThanInventAFourthState(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	// Reading part of a file and closing it leaves it unread. Only MarkFile
	// finishes one, so a fourth state for partly read is never recorded.
	rs := loadTemp(t)
	rs.MarkBlock("b.txt", "h1")
	if got := rs.MarkIn(WorkingTree(SectionUnstaged), "b.txt", []string{"h1", "h2"}); got != Unread {
		t.Errorf("got %v, want Unread", got)
	}
	if got := rs.MarkIn(WorkingTree(SectionUnstaged), "b.txt", []string{"h1"}); got != Unread {
		t.Errorf("every block read but the file never finished: got %v, want Unread", got)
	}
}

func TestTheSameContentInAnotherFileIsStillUnread(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	// Keyed by content alone, an agent applying one edit to ten files would
	// mark nine of them read that nobody opened.
	rs := loadTemp(t)
	rs.MarkFileIn(WorkingTree(SectionUnstaged), "a.txt", []string{"same"})
	if got := rs.MarkIn(WorkingTree(SectionUnstaged), "b.txt", []string{"same"}); got != Unread {
		t.Errorf("got %v, want Unread", got)
	}
}

func TestAPathContainingAColonKeepsItsOwnMarks(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	// The key separator must be a byte a path cannot hold.
	rs := loadTemp(t)
	rs.MarkFileIn(WorkingTree(SectionUnstaged), "a:b.txt", []string{"h1"})
	rs.Rename("a", "x")
	if got := rs.MarkIn(WorkingTree(SectionUnstaged), "a:b.txt", []string{"h1"}); got != Read {
		t.Errorf("renaming \"a\" disturbed \"a:b.txt\": got %v", got)
	}
}

func TestReadStateSurvivesARename(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	rs := loadTemp(t)
	rs.MarkFileIn(WorkingTree(SectionUnstaged), "old.txt", []string{"h1"})
	rs.Rename("old.txt", "new.txt")
	if got := rs.MarkIn(WorkingTree(SectionUnstaged), "new.txt", []string{"h1"}); got != Read {
		t.Errorf("got %v, want Read", got)
	}
}

func TestReadStateRoundTripsThroughDisk(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := t.TempDir()
	repo, env := pruneTestRepo(t)
	if err := os.WriteFile(repo+"/a.txt", []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", "a.txt")
	cmd.Dir = repo
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	rs, err := LoadRead(context.Background(), dir, repo)
	if err != nil {
		t.Fatal(err)
	}
	rs.MarkFileIn(WorkingTree(SectionUnstaged), "a.txt", []string{"h1"})
	if err := rs.Save(); err != nil {
		t.Fatal(err)
	}
	again, err := LoadRead(context.Background(), dir, repo)
	if err != nil {
		t.Fatal(err)
	}
	if got := again.MarkIn(WorkingTree(SectionUnstaged), "a.txt", []string{"h1"}); got != Read {
		t.Errorf("got %v, want Read", got)
	}
}

func TestLoadReadOfAMissingFileIsNotAnError(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	repo, _ := pruneTestRepo(t)
	if _, err := LoadRead(context.Background(), filepath.Join(t.TempDir(), "nope"), repo); err != nil {
		t.Errorf("a first run must not fail: %v", err)
	}
}

func TestMarkFileReadWithoutOpeningThenChangedShowsChanged(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	rs := loadTemp(t)
	rs.MarkFileIn(WorkingTree(SectionUnstaged), "a.txt", []string{"h1"})
	if got := rs.MarkIn(WorkingTree(SectionUnstaged), "a.txt", []string{"h1"}); got != Read {
		t.Errorf("after r read: got %v, want Read", got)
	}
	if got := rs.MarkIn(WorkingTree(SectionUnstaged), "a.txt", []string{"h1", "h2"}); got != Changed {
		t.Errorf("after the file changed: got %v, want Changed", got)
	}
}

func TestHashesForShowsChangedOnAClosedFile(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	rs := loadTemp(t)
	rs.MarkFileIn(WorkingTree(SectionUnstaged), "a.txt", []string{"old"})
	rs.worktreeHashes = map[string][]string{"a.txt": {"old", "new"}}
	if got := rs.MarkIn(WorkingTree(SectionUnstaged), "a.txt", rs.HashesIn(WorkingTree(SectionUnstaged), "a.txt")); got != Changed {
		t.Errorf("got %v, want Changed", got)
	}
}

func TestMarkShownLeavesAPartlySeenFileUnread(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	rs := loadTemp(t)
	all := []string{"h1", "h2", "h3", "h4", "h5", "h6"}
	rs.MarkShownIn(WorkingTree(SectionUnstaged), "a.txt", []string{"h1", "h2"}, all)
	if got := rs.MarkIn(WorkingTree(SectionUnstaged), "a.txt", all); got != Unread {
		t.Errorf("got %v, want Unread", got)
	}
}

func TestMarkShownFinishesWhenEveryBlockWasSeen(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	rs := loadTemp(t)
	all := []string{"h1", "h2"}
	rs.MarkShownIn(WorkingTree(SectionUnstaged), "a.txt", all, all)
	if got := rs.MarkIn(WorkingTree(SectionUnstaged), "a.txt", all); got != Read {
		t.Errorf("got %v, want Read", got)
	}
}

func TestStashMarkUsesTheCommitSHA(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	rs := loadTemp(t)
	if got := rs.StashMark("deadbeef"); got != Unread {
		t.Fatalf("got %v, want Unread", got)
	}
	rs.MarkStash("deadbeef")
	if got := rs.StashMark("deadbeef"); got != Read {
		t.Fatalf("got %v, want Read", got)
	}
}

func TestWorktreeMarkUsesPathAndSHA(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	rs := loadTemp(t)
	if got := rs.WorktreeMark("/wt", "deadbeef"); got != Unread {
		t.Fatalf("got %v, want Unread", got)
	}
	rs.MarkWorktree("/wt", "deadbeef")
	if got := rs.WorktreeMark("/wt", "deadbeef"); got != Read {
		t.Fatalf("got %v, want Read", got)
	}
}

func TestPruneDropsPathsGoneFromTheTreeAndHistory(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir, _ := pruneTestRepo(t)
	stateDir := t.TempDir()
	rs, err := LoadRead(context.Background(), stateDir, dir)
	if err != nil {
		t.Fatal(err)
	}
	rs.MarkFileIn(WorkingTree(SectionUnstaged), "vanished.txt", []string{"h1"})
	if err := rs.Save(); err != nil {
		t.Fatal(err)
	}
	again, err := LoadRead(context.Background(), stateDir, dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := again.MarkIn(WorkingTree(SectionUnstaged), "vanished.txt", []string{"h1"}); got != Unread {
		t.Errorf("pruned path: got %v, want Unread", got)
	}
}

func TestPruneKeepsReadMarksForUntrackedFiles(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir, env := pruneTestRepo(t)
	if err := os.WriteFile(dir+"/tracked.txt", []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/fresh.txt", []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("add", "tracked.txt")
	stateDir := t.TempDir()
	rs, err := LoadRead(context.Background(), stateDir, dir)
	if err != nil {
		t.Fatal(err)
	}
	rs.MarkFileIn(WorkingTree(SectionUnstaged), "tracked.txt", []string{"h1"})
	rs.MarkFileIn(WorkingTree(SectionUnstaged), "fresh.txt", []string{"h2"})
	if err := rs.Save(); err != nil {
		t.Fatal(err)
	}
	again, err := LoadRead(context.Background(), stateDir, dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := again.MarkIn(WorkingTree(SectionUnstaged), "fresh.txt", []string{"h2"}); got != Read {
		t.Errorf("untracked path after reload: got %v, want Read", got)
	}
	if !again.Finished[fileKey(WorkingTree(SectionUnstaged), "fresh.txt")] {
		t.Errorf("finished missing %q", fileKey(WorkingTree(SectionUnstaged), "fresh.txt"))
	}
}

func TestPruneKeepsPathsStillInTheWorkingTree(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir, env := pruneTestRepo(t)
	if err := os.WriteFile(dir+"/kept.txt", []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", "kept.txt")
	cmd.Dir = dir
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	stateDir := t.TempDir()
	rs, err := LoadRead(context.Background(), stateDir, dir)
	if err != nil {
		t.Fatal(err)
	}
	rs.MarkFileIn(WorkingTree(SectionUnstaged), "kept.txt", []string{"h1"})
	if err := rs.Save(); err != nil {
		t.Fatal(err)
	}
	again, err := LoadRead(context.Background(), stateDir, dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := again.MarkIn(WorkingTree(SectionUnstaged), "kept.txt", []string{"h1"}); got != Read {
		t.Errorf("working-tree path: got %v, want Read", got)
	}
}

func TestPruneKeepsPathsInTheLastHundredCommits(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir, env := pruneTestRepo(t)
	if err := os.WriteFile(dir+"/old.txt", []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("add", "old.txt")
	run("commit", "-q", "-m", "add old")
	run("rm", "old.txt")
	run("commit", "-q", "-m", "remove old")
	stateDir := t.TempDir()
	rs, err := LoadRead(context.Background(), stateDir, dir)
	if err != nil {
		t.Fatal(err)
	}
	rs.MarkFileIn(WorkingTree(SectionUnstaged), "old.txt", []string{"h1"})
	if err := rs.Save(); err != nil {
		t.Fatal(err)
	}
	again, err := LoadRead(context.Background(), stateDir, dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := again.MarkIn(WorkingTree(SectionUnstaged), "old.txt", []string{"h1"}); got != Read {
		t.Errorf("history path: got %v, want Read", got)
	}
}

// A partly read file records blocks but not the file, so pruning has to keep
// the block map for a live path; dropping it silently restarts the reading.
func TestPruneKeepsBlockMarksForALivePath(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir, env := pruneTestRepo(t)
	if err := os.WriteFile(dir+"/kept.txt", []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", "kept.txt")
	cmd.Dir = dir
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	stateDir := t.TempDir()
	rs, err := LoadRead(context.Background(), stateDir, dir)
	if err != nil {
		t.Fatal(err)
	}
	both := []string{"h1", "h2"}
	rs.MarkShownIn(WorkingTree(SectionUnstaged), "kept.txt", []string{"h1"}, both)
	if got := rs.MarkIn(WorkingTree(SectionUnstaged), "kept.txt", both); got != Unread {
		t.Fatalf("one block of two: got %v, want Unread", got)
	}
	if err := rs.Save(); err != nil {
		t.Fatal(err)
	}

	again, err := LoadRead(context.Background(), stateDir, dir)
	if err != nil {
		t.Fatal(err)
	}
	again.MarkShownIn(WorkingTree(SectionUnstaged), "kept.txt", []string{"h2"}, both)
	if got := again.MarkIn(WorkingTree(SectionUnstaged), "kept.txt", both); got != Read {
		t.Errorf("second block after a reload: got %v, want Read", got)
	}
}

// A record written with an older key layout matches nothing, and a file the
// reader read comes back as ● rather than as read. Dropping it costs the marks
// once; keeping it lies about every file.
func sha256Sum(s string) []byte {
	sum := sha256.Sum256([]byte(s))
	return sum[:]
}

func TestARecordFromAnOlderKeyLayoutIsDropped(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := t.TempDir()
	repo, env := pruneTestRepo(t)
	if err := os.WriteFile(repo+"/a.txt", []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", "a.txt")
	cmd.Dir = repo
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	rs, err := LoadRead(context.Background(), dir, repo)
	if err != nil {
		t.Fatal(err)
	}
	rs.MarkFileIn(WorkingTree(SectionUnstaged), "a.txt", []string{"h1"})
	if err := rs.Save(); err != nil {
		t.Fatal(err)
	}
	// A record from before the field existed has no version at all, which is
	// the case an explicit zero would not reach.
	raw, err := os.ReadFile(filepath.Join(dir, hex.EncodeToString(sha256Sum(repo))[:16], "read.json"))
	if err != nil {
		t.Fatal(err)
	}
	var loose map[string]any
	if err := json.Unmarshal(raw, &loose); err != nil {
		t.Fatal(err)
	}
	delete(loose, "version")
	out, err := json.Marshal(loose)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, hex.EncodeToString(sha256Sum(repo))[:16], "read.json"), out, 0o644); err != nil {
		t.Fatal(err)
	}

	again, err := LoadRead(context.Background(), dir, repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Finished) != 0 || len(again.Blocks) != 0 {
		t.Errorf("kept %v / %v", again.Finished, again.Blocks)
	}
	if again.Version != readVersion {
		t.Errorf("version = %d, want %d", again.Version, readVersion)
	}
}

// Prune writes only when it dropped something. It runs at startup on every
// repository, and writing each time replaces the file for no reason: the write
// is the moment a crash can lose the marks, and the record is rewritten even
// when it did not change.
func TestPruneWritesOnlyWhenItDroppedSomething(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	repo, env := pruneTestRepo(t)
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(repo, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir, cmd.Env = repo, env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write("alive.txt", "x\n")
	run("add", "alive.txt")
	run("commit", "-q", "-m", "add")

	stateDir := t.TempDir()
	rs, err := LoadRead(context.Background(), stateDir, repo)
	if err != nil {
		t.Fatal(err)
	}
	rs.MarkFileIn(WorkingTree(SectionUnstaged), "alive.txt", []string{"h1"})
	if err := rs.Save(); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(rs.path)
	if err != nil {
		t.Fatal(err)
	}

	// Nothing to drop: every mark names a path the repository still has.
	if err := rs.Prune(context.Background(), repo); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(rs.path)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(before.ModTime()) || after.Size() != before.Size() {
		t.Error("a prune that dropped nothing wrote the record again")
	}

	// One to drop. It goes to disk first: a prune that drops a mark in memory
	// and does not write leaves the record on disk still holding it, and the
	// next start reads it back.
	rs.MarkFileIn(WorkingTree(SectionUnstaged), "gone.txt", []string{"h1"})
	if err := rs.Save(); err != nil {
		t.Fatal(err)
	}
	if err := rs.Prune(context.Background(), repo); err != nil {
		t.Fatal(err)
	}
	// The bytes on disk rather than a reload: LoadRead prunes what it read, so
	// a prune that dropped the mark in memory and never wrote reads back the
	// same either way.
	onDisk, err := os.ReadFile(rs.path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(onDisk), "gone.txt") {
		t.Error("the record on disk still holds a mark for a path the repository does not have")
	}
	if !strings.Contains(string(onDisk), "alive.txt") {
		t.Error("the prune dropped a mark for a path the repository still has")
	}

	// A row finished with no blocks behind it — a file whose diff was empty, or
	// one marked read without opening. It is the only kind of mark the block
	// half of the prune cannot notice, so a record holding nothing else is
	// where a prune that drops it without writing shows.
	rs.MarkFileIn(WorkingTree(SectionUnstaged), "vanished.txt", nil)
	if err := rs.Save(); err != nil {
		t.Fatal(err)
	}
	if err := rs.Prune(context.Background(), repo); err != nil {
		t.Fatal(err)
	}
	onDisk, err = os.ReadFile(rs.path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(onDisk), "vanished.txt") {
		t.Error("the record on disk still holds a finished row for a path the " +
			"repository does not have")
	}

	// A stash row and a worktree row are named by what they are — a stash sha,
	// a tree and its tip — and neither is a path in this working tree. Pruned
	// by the path check, every stash the reader has seen comes back unread on
	// the next start.
	rs.MarkStash("stashsha")
	rs.MarkWorktree("/wt", "treesha")
	if err := rs.Save(); err != nil {
		t.Fatal(err)
	}
	if err := rs.Prune(context.Background(), repo); err != nil {
		t.Fatal(err)
	}
	if rs.StashMark("stashsha") != Read {
		t.Error("the prune dropped the mark of a stash the reader had seen")
	}
	if rs.WorktreeMark("/wt", "treesha") != Read {
		t.Error("the prune dropped the mark of a worktree the reader had seen")
	}
	onDisk, err = os.ReadFile(rs.path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"stashsha", "treesha"} {
		if !strings.Contains(string(onDisk), want) {
			t.Errorf("the record on disk lost the mark naming %q", want)
		}
	}
}
