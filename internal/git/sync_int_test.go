package git

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func newSyncRepos(t *testing.T) (remoteDir, workDir string) {
	t.Helper()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	base := t.TempDir()
	env := gitTestEnv(base)
	remoteDir = filepath.Join(base, "remote.git")
	workDir = filepath.Join(base, "work")
	runGitTest(t, env, base, "init", "--bare", "-q", "-b", "main", remoteDir)
	runGitTest(t, env, base, "clone", "-q", remoteDir, "work")
	runGitTest(t, env, workDir, "commit", "-q", "--allow-empty", "-m", "base")
	runGitTest(t, env, workDir, "push", "-q", "-u", "origin", "main")
	return remoteDir, workDir
}

func cloneSyncWork(t *testing.T, remoteDir string) (env []string, workDir string) {
	t.Helper()
	base := filepath.Dir(remoteDir)
	env = gitTestEnv(base)
	workDir = filepath.Join(base, "work2")
	runGitTest(t, env, base, "clone", "-q", remoteDir, "work2")
	return env, workDir
}

func TestPullRefusesWhenHistoryWouldMerge(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	remoteDir, workDir := newSyncRepos(t)
	env, peerDir := cloneSyncWork(t, remoteDir)

	runGitTest(t, env, workDir, "commit", "-q", "--allow-empty", "-m", "local")
	runGitTest(t, env, peerDir, "commit", "-q", "--allow-empty", "-m", "remote")
	runGitTest(t, env, peerDir, "push", "-q")

	if err := Fetch(context.Background(), workDir); err != nil {
		t.Fatal(err)
	}
	behind, ahead, err := aheadBehind(context.Background(), workDir)
	if err != nil {
		t.Fatal(err)
	}
	if behind != 1 || ahead != 1 {
		t.Fatalf("got behind=%d ahead=%d, want 1 1", behind, ahead)
	}
	if err := pull(context.Background(), workDir); !errors.Is(err, ErrWouldMerge) {
		t.Fatalf("got %v, want ErrWouldMerge", err)
	}
}

func TestPullFastForwardsWhenUpstreamIsAhead(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	remoteDir, workDir := newSyncRepos(t)
	env, peerDir := cloneSyncWork(t, remoteDir)

	runGitTest(t, env, peerDir, "commit", "-q", "--allow-empty", "-m", "ahead")
	runGitTest(t, env, peerDir, "push", "-q")

	if err := Fetch(context.Background(), workDir); err != nil {
		t.Fatal(err)
	}
	behind, ahead, err := aheadBehind(context.Background(), workDir)
	if err != nil {
		t.Fatal(err)
	}
	if behind != 1 || ahead != 0 {
		t.Fatalf("got behind=%d ahead=%d, want 1 0", behind, ahead)
	}
	if err := pull(context.Background(), workDir); err != nil {
		t.Fatal(err)
	}
	behind, ahead, err = aheadBehind(context.Background(), workDir)
	if err != nil {
		t.Fatal(err)
	}
	if behind != 0 || ahead != 0 {
		t.Fatalf("after pull: behind=%d ahead=%d, want 0 0", behind, ahead)
	}
}

func TestSyncWithoutUpstreamDoesNotError(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := Fetch(context.Background(), dir); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if err := Sync(context.Background(), dir); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if err := pull(context.Background(), dir); err != nil {
		t.Fatalf("pull: %v", err)
	}
	if err := push(context.Background(), dir); err != nil {
		t.Fatalf("push: %v", err)
	}
}

// The ancestor check runs first, so a pull that reached git at all would
// already be fast-forwardable. This asserts the flag directly: without it a
// future change to the check would let git merge, which this program does not
// offer because it cannot clean up after one.
func TestPullNeverProducesAMergeCommit(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	remoteDir, workDir := newSyncRepos(t)
	env, peerDir := cloneSyncWork(t, remoteDir)

	// A reader who set pull.rebase=false gets a merge from a bare `git pull`;
	// git only refuses on its own while the setting is absent. The flag is
	// what holds regardless of their config.
	runGitTest(t, env, workDir, "config", "pull.rebase", "false")
	runGitTest(t, env, workDir, "commit", "-q", "--allow-empty", "-m", "local")
	runGitTest(t, env, peerDir, "commit", "-q", "--allow-empty", "-m", "remote")
	runGitTest(t, env, peerDir, "push", "-q")
	if err := Fetch(context.Background(), workDir); err != nil {
		t.Fatal(err)
	}

	before := headCount(t, workDir)
	_ = pull(context.Background(), workDir)
	if got := headCount(t, workDir); got != before {
		t.Errorf("history grew from %d to %d commits, so something was merged", before, got)
	}
	if _, err := runRead(context.Background(), workDir, "rev-parse", "--verify", "-q", "MERGE_HEAD"); err == nil {
		t.Error("a merge was left in progress")
	}
}

func headCount(t *testing.T, dir string) int {
	t.Helper()
	out, err := runRead(context.Background(), dir, "rev-list", "--count", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, c := range string(out) {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}

// Sync answers for the S key and the sync button. It used to decide whether a
// pull would merge on its own and then call pull, which decides the same thing
// again: two copies of the one rule that keeps a merge commit out of a pane
// that cannot finish one. Only pull was tested.
func TestSyncRefusesTheSameHistoryPullRefuses(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	remoteDir, workDir := newSyncRepos(t)
	env, peerDir := cloneSyncWork(t, remoteDir)

	runGitTest(t, env, workDir, "commit", "-q", "--allow-empty", "-m", "local")
	runGitTest(t, env, peerDir, "commit", "-q", "--allow-empty", "-m", "remote")
	runGitTest(t, env, peerDir, "push", "-q")

	if err := Sync(context.Background(), workDir); !errors.Is(err, ErrWouldMerge) {
		t.Fatalf("Sync got %v, want ErrWouldMerge", err)
	}
	// The refusal has to leave the branch where it was: a sync that half ran is
	// worse than one that did not.
	behind, ahead, err := aheadBehind(context.Background(), workDir)
	if err != nil {
		t.Fatal(err)
	}
	if behind != 1 || ahead != 1 {
		t.Errorf("after the refusal behind=%d ahead=%d, want 1 1", behind, ahead)
	}
}

// The other half of the same key. A branch cannot be both behind and ahead by a
// fast-forward — that is what divergence means — so the two ends are separate.
func TestSyncCatchesUpAndSendsOn(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}

	t.Run("only behind", func(t *testing.T) {
		t.Parallel()
		remoteDir, workDir := newSyncRepos(t)
		env, peerDir := cloneSyncWork(t, remoteDir)
		runGitTest(t, env, peerDir, "commit", "-q", "--allow-empty", "-m", "ahead")
		runGitTest(t, env, peerDir, "push", "-q")

		if err := Sync(context.Background(), workDir); err != nil {
			t.Fatal(err)
		}
		assertLevel(t, workDir)
	})

	t.Run("only ahead", func(t *testing.T) {
		t.Parallel()
		_, workDir := newSyncRepos(t)
		env := gitTestEnv(filepath.Dir(workDir))
		runGitTest(t, env, workDir, "commit", "-q", "--allow-empty", "-m", "mine")

		if err := Sync(context.Background(), workDir); err != nil {
			t.Fatal(err)
		}
		assertLevel(t, workDir)
	})
}

func assertLevel(t *testing.T, dir string) {
	t.Helper()
	behind, ahead, err := aheadBehind(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if behind != 0 || ahead != 0 {
		t.Errorf("after the sync behind=%d ahead=%d, want 0 0", behind, ahead)
	}
}

// A repository with no remote has nowhere to fetch from, which is not an error.
// A directory that answers nothing is a different case, and reading it as "no
// remote" made f and S do nothing and say nothing.
func TestFetchTellsNoRemoteApartFromNoRepository(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	if err := Fetch(context.Background(), newRepo(t)); err != nil {
		t.Errorf("a repository with no remote is not an error: %v", err)
	}
	if err := Fetch(context.Background(), t.TempDir()); err == nil {
		t.Error("a directory that is not a repository fetched successfully")
	}
}

// hasRemotes answers a caller that decides whether to reach the network. Its
// comment separates "this repository has no remote" from "this directory could
// not be read", and the error is what separates them — the bool has to stay
// false in the second case, because a caller that reads it without the error
// would fetch from a repository that answered nothing.
//
// Fetch, the one caller, returns on the error before it reads the bool, so
// nothing on screen changes if this is wrong. It is asked directly for that
// reason.
func TestHasRemotesDoesNotClaimARemoteForADirectoryItCouldNotRead(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	has, err := hasRemotes(context.Background(), t.TempDir())
	if err == nil {
		t.Fatal("a directory that is not a repository was read without error")
	}
	if has {
		t.Error("a directory that could not be read was answered as having a remote")
	}
}

// The two answers it does give, on a repository it can read.
func TestHasRemotesSeparatesOneRemoteFromNone(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	has, err := hasRemotes(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Error("a fresh repository was answered as having a remote")
	}

	runGit(t, dir, "remote", "add", "origin", "https://example.invalid/r.git")
	has, err = hasRemotes(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Error("a repository with one remote was answered as having none")
	}
}

// Sync sends nothing when there is nothing to send. A push with no commits
// behind it still reaches the remote to ask what it holds, so a sync that runs
// it anyway pays a network round trip on every press of the key — and fails
// outright where the remote can be read but not written to.
//
// receivepack names the program git runs on the other end to take a push, and
// a name that is not there fails whenever a push reaches the remote. Fetch does
// not use it, so the fixture blocks exactly one of the two.
func TestSyncWithNothingToSendDoesNotReachTheRemoteToPush(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	_, workDir := newSyncRepos(t)
	env := gitTestEnv(filepath.Dir(workDir))
	runGitTest(t, env, workDir, "config", "remote.origin.receivepack",
		"/nonexistent-receive-pack")

	if err := Sync(context.Background(), workDir); err != nil {
		t.Errorf("a sync with nothing to send pushed anyway: %v", err)
	}

	// The fixture has to be able to fail, or the line above proves nothing.
	runGitTest(t, env, workDir, "commit", "-q", "--allow-empty", "-m", "mine")
	if err := Sync(context.Background(), workDir); err == nil {
		t.Error("a sync with a commit to send did not push")
	}
}

// A branch that is behind is brought level by the fast-forward and holds what
// the upstream holds, so there is nothing left to send. A branch that is behind
// and ahead at once has diverged, and the pull refuses that instead: either way
// the sync is done when the pull is, and asking git what is ahead again is a
// subprocess whose answer cannot change the outcome.
func TestSyncAfterAFastForwardSendsNothing(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	remoteDir, workDir := newSyncRepos(t)
	env, peerDir := cloneSyncWork(t, remoteDir)
	runGitTest(t, env, peerDir, "commit", "-q", "--allow-empty", "-m", "theirs")
	runGitTest(t, env, peerDir, "push", "-q")

	// Reading the remote is allowed and writing to it is not, so a push that
	// should not happen fails rather than passing unseen.
	ourEnv := gitTestEnv(filepath.Dir(workDir))
	runGitTest(t, ourEnv, workDir, "config", "remote.origin.receivepack",
		"/nonexistent-receive-pack")

	if err := Sync(context.Background(), workDir); err != nil {
		t.Fatalf("a sync that only had to catch up pushed anyway: %v", err)
	}
	assertLevel(t, workDir)
}

// A read that failed says nothing about whether a pull would merge, and the
// answer that goes with it has to be the one that holds a pull back. Answering
// "yes, it fast-forwards" with an error beside it is what a caller reading the
// answer before the error would act on.
func TestAFastForwardCheckThatFailedAnswersNo(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	for _, c := range []struct {
		name string
		dir  func(t *testing.T) string
	}{
		{"a directory that is not a repository", func(t *testing.T) string { return t.TempDir() }},
		{"a repository with no upstream", newRepo},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			ff, err := canFastForward(context.Background(), c.dir(t))
			if err == nil {
				t.Fatal("the read answered without complaint, so this proves nothing")
			}
			if ff {
				t.Error("a read that failed answered that a pull fast-forwards")
			}
		})
	}
}
