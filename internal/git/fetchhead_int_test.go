package git

import (
	"context"
	"os"
	"testing"
	"time"
)

// The footer prints how long ago fetch ran, and it reads that from when
// FETCH_HEAD was written. A repository that has never fetched has no such file,
// and the zero time is what says so — so the two answers have to stay apart:
// reading a file that is there and answering zero reads as "never fetched" for
// a repository that fetched a moment ago.
func TestFetchHeadModTimeReadsTheFileAndAnswersZeroWithoutIt(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)

	at, err := FetchHeadModTime(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if !at.IsZero() {
		t.Errorf("a repository that never fetched answers %v, want the zero time", at)
	}

	written := time.Now().Add(-90 * time.Minute).Truncate(time.Second)
	path := dir + "/.git/FETCH_HEAD"
	if err := os.WriteFile(path, []byte("\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, written, written); err != nil {
		t.Fatal(err)
	}
	at, err = FetchHeadModTime(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if !at.Equal(written) {
		t.Errorf("FETCH_HEAD was written at %v and the answer is %v", written, at)
	}
}

// A linked worktree keeps a FETCH_HEAD of its own, and a fetch run from it
// writes that one. The refs it brings back are shared with every other
// worktree, so a fetch run from anywhere is when this repository last heard
// from the remote. Reading only the worktree's own file says "never fetched"
// in a worktree whose remote refs are minutes old.
func TestFetchHeadModTimeAnswersForTheWholeRepositoryFromAWorktree(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	env := gitTestEnv(dir)
	side := t.TempDir() + "/side"
	runGitTest(t, env, dir, "worktree", "add", "-q", side, "-b", "side")

	at, err := FetchHeadModTime(context.Background(), side)
	if err != nil {
		t.Fatal(err)
	}
	if !at.IsZero() {
		t.Fatalf("a worktree of a repository that never fetched answers %v", at)
	}

	// A fetch run from the main worktree, which is where its record goes.
	shared := time.Now().Add(-30 * time.Minute).Truncate(time.Second)
	common := dir + "/.git/FETCH_HEAD"
	if err := os.WriteFile(common, []byte("\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(common, shared, shared); err != nil {
		t.Fatal(err)
	}
	at, err = FetchHeadModTime(context.Background(), side)
	if err != nil {
		t.Fatal(err)
	}
	if !at.Equal(shared) {
		t.Errorf("a fetch from another worktree left %v and the worktree answers %v",
			shared, at)
	}

	// A fetch run from the worktree itself is newer, and it is the answer.
	own := time.Now().Add(-5 * time.Minute).Truncate(time.Second)
	ownPath := dir + "/.git/worktrees/side/FETCH_HEAD"
	if err := os.WriteFile(ownPath, []byte("\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(ownPath, own, own); err != nil {
		t.Fatal(err)
	}
	at, err = FetchHeadModTime(context.Background(), side)
	if err != nil {
		t.Fatal(err)
	}
	if !at.Equal(own) {
		t.Errorf("the worktree fetched at %v and answers %v", own, at)
	}
	// The main worktree is not moved by the worktree's own record.
	at, err = FetchHeadModTime(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if !at.Equal(shared) {
		t.Errorf("the main worktree fetched at %v and answers %v", shared, at)
	}
}
