package git

import (
	"sync"
)

// running holds the git processes this program started and has not seen finish.
//
// bubbletea does not wait for a command to return before the program exits: it
// says so where it starts one, and leaks the goroutine instead. So pressing f
// and then q leaves git fetch behind with init as its parent, and the thirty
// second deadline dies with the program that was holding the timer. Measured:
// after quitting, two git fetch processes remained at ppid 1 and kept running.
//
// The set is package level rather than carried by a caller because the caller
// that has to end them is the one shutting the program down, and it does not
// know which of the reads and writes underway belong to which pane.
var running = struct {
	sync.Mutex
	// pids maps a process number to the channel its owner closes when Wait
	// returns. The number alone cannot say whether the process is still this
	// program's: see stopGroupOwned.
	pids map[int]chan struct{}
	// stopped records that the program is on its way out. A command goroutine
	// that reached Start after StopAll had already read the set would leave a
	// process nobody was left to end.
	stopped bool
}{pids: map[int]chan struct{}{}}

// addRunning records a process by its number rather than by its *exec.Cmd. The
// Cmd's Process field is written by Start and read by Wait on the goroutine
// that owns the command, so reading it from a shutdown on another goroutine is
// a data race — the detector said so.
//
// It answers whether the process should live: false means the program is
// already shutting down and the caller has to end what it just started.
func addRunning(pid int) (reaped chan struct{}, keep bool) {
	running.Lock()
	defer running.Unlock()
	if running.stopped {
		return nil, false
	}
	reaped = make(chan struct{})
	running.pids[pid] = reaped
	return reaped, true
}

// removeRunning is called when Wait has returned, which is what closing the
// channel says: from here the number may be anyone's.
func removeRunning(pid int, reaped chan struct{}) {
	running.Lock()
	delete(running.pids, pid)
	running.Unlock()
	close(reaped)
}

// StopAll ends every git this program started and is still waiting on. It is
// for shutdown: a caller that wants one command to stop should not start it.
//
// A write is ended the same as a read. Leaving one running past the program
// that started it is worse than ending it: the reader can see a half-applied
// index while the pane is gone, and cannot see anything at all while a git they
// did not know about is still writing.
// It answers whether every git ended on the term alone. One that had to be
// killed did not get to put its lock files back, and a caller checking that
// they are gone has to know the difference: under load the term handler can be
// starved past the grace, and then the lock is left through no fault of this
// program. Only a test asks; the program on its way out has nothing to do with
// the answer.
func StopAll() (everyTermWasEnough bool) {
	running.Lock()
	running.stopped = true
	type owned struct {
		pid    int
		reaped chan struct{}
	}
	live := make([]owned, 0, len(running.pids))
	for pid, reaped := range running.pids {
		live = append(live, owned{pid, reaped})
	}
	running.pids = map[int]chan struct{}{}
	running.Unlock()

	everyTermWasEnough = true
	for _, o := range live {
		if !stopGroupOwned(o.pid, o.reaped, termGrace) {
			everyTermWasEnough = false
		}
	}
	return everyTermWasEnough
}

// resumeForTest lets a test that called StopAll start git again. Production
// never resumes: StopAll runs once, on the way out.
func resumeForTest() {
	running.Lock()
	running.stopped = false
	running.Unlock()
}
