package git

import (
	"context"
	"errors"
	"testing"
	"time"
)

// bubbletea does not wait for a command before the program exits, so a git
// started just before q keeps running with init as its parent — and the
// deadline that would have ended it died with the program holding the timer.
// Measured before this: two git fetch processes at ppid 1, still running.
func testStopAllEndsAGitThatIsStillRunning(t *testing.T) {
	if testing.Short() {
		t.Skip("starts a process")
	}
	dir := newRepo(t)
	// The result travels on a channel: the goroutine writing an err the test
	// reads is a race, which is what -race said.
	done := make(chan error, 1)
	go func() {
		// A git that would not end on its own inside the test's lifetime.
		_, err := runRead(context.Background(), dir, "-c", "alias.wait=!sleep 60", "wait")
		done <- err
	}()

	// The process has to be running before it can be stopped.
	deadline := time.Now().Add(5 * time.Second)
	for {
		running.Lock()
		started := len(running.pids) > 0
		running.Unlock()
		if started {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("git never started")
		}
		time.Sleep(10 * time.Millisecond)
	}

	StopAll()
	select {
	case err := <-done:
		if err == nil {
			t.Error("the killed git reported success")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the git was still running ten seconds after StopAll")
	}
}

// A finished git is not left in the set: the program would otherwise carry
// every command it ever ran until it exits.
func testAFinishedGitLeavesTheRunningSet(t *testing.T) {
	if testing.Short() {
		t.Skip("starts a process")
	}
	dir := newRepo(t)
	if _, err := runRead(context.Background(), dir, "rev-parse", "--show-toplevel"); err != nil {
		t.Fatal(err)
	}

	running.Lock()
	left := len(running.pids)
	running.Unlock()
	if left != 0 {
		t.Errorf("%d git processes are still recorded after they finished", left)
	}
}

// StopAll with nothing running is what a program that never ran git does on the
// way out.
func testStopAllWithNothingRunning(t *testing.T) {
	if testing.Short() {
		t.Skip("touches package state")
	}
	// The answer says whether every git ended on the term alone. With none to
	// end, none had to be killed. A caller reads it to decide whether a lock
	// file left behind is this program's doing; an answer of no sends it looking
	// for a lock that nothing took.
	if !StopAll() {
		t.Error("with no git running, StopAll said one had to be killed")
	}
}

// A command goroutine that reaches git after StopAll has read the set would
// leave a process with nobody to end it: the pane is gone and the deadline died
// with the program holding it. bubbletea leaks those goroutines rather than
// waiting, so this is the ordinary case on the way out, not a rare one.
func testAGitStartedAfterTheStopIsEndedAtOnce(t *testing.T) {
	if testing.Short() {
		t.Skip("starts a process")
	}
	dir := newRepo(t)
	StopAll()

	start := time.Now()
	_, err := runRead(context.Background(), dir, "-c", "alias.wait=!sleep 60", "wait")
	if !errors.Is(err, ErrStopped) {
		t.Fatalf("err = %v, want ErrStopped", err)
	}
	// The point is that it does not wait out the sleep.
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Errorf("took %v; the process was not ended", elapsed.Round(time.Millisecond))
	}

	running.Lock()
	left := len(running.pids)
	running.Unlock()
	if left != 0 {
		t.Errorf("%d processes are recorded after the stop", left)
	}
}

// The answer says whether the term alone was enough for every git. A git that
// does not take the term has to be killed, and a kill is what leaves a lock
// file behind: the caller reads this to decide whether the lock the reader's
// next git complains about is this program's doing. Reporting that the term was
// enough sends them looking for a lock nothing took.
func testStopAllSaysWhenAGitHadToBeKilled(t *testing.T) {
	if testing.Short() {
		t.Skip("starts a process")
	}
	dir := newRepo(t)
	done := make(chan error, 1)
	go func() {
		// A git whose child ignores the term, so only the kill ends it.
		_, err := runRead(context.Background(), dir, "-c", "alias.wait=!trap '' TERM; sleep 20", "wait")
		done <- err
	}()

	deadline := time.Now().Add(5 * time.Second)
	for {
		running.Lock()
		started := len(running.pids) > 0
		running.Unlock()
		if started {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("git never started")
		}
		time.Sleep(10 * time.Millisecond)
	}
	// The shell has to have installed its trap before the term arrives, or the
	// term ends it and the answer is the other one.
	time.Sleep(200 * time.Millisecond)

	if StopAll() {
		t.Error("a git that ignored the term was reported as ended by it")
	}
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the git was still running ten seconds after StopAll")
	}
}

// The shutdown is package state: one flag and one set that every git in the
// program passes through. The checks run in order rather than in parallel,
// because a StopAll in one would be a StopAll in all of them.
func TestStoppingGit(t *testing.T) {
	for _, c := range []struct {
		name string
		run  func(*testing.T)
	}{
		{"a finished git leaves the set", testAFinishedGitLeavesTheRunningSet},
		{"stopping with nothing running", testStopAllWithNothingRunning},
		{"stopping one that is running", testStopAllEndsAGitThatIsStillRunning},
		{"one that had to be killed", testStopAllSaysWhenAGitHadToBeKilled},
		{"one started after the stop", testAGitStartedAfterTheStopIsEndedAtOnce},
		{"no lock is left behind", testStoppingGitLeavesNoLockBehind},
	} {
		t.Run(c.name, func(t *testing.T) { c.run(t) })
		resumeForTest()
	}
}
