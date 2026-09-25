//go:build !windows

package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// Every git runs as the leader of its own process group, so its group number is
// its process number. A number is the kernel's to hand out again the moment the
// process is reaped, and the goroutine that started the git is reaping it while
// the shutdown is signaling it.
//
// Asking the kernel whether the group is still there cannot tell "still mine"
// from "someone else's now". A shutdown that asked, and got yes about a number
// the kernel had already handed to another program's git, waited out the grace
// and then killed that one. It happened here: a scenario test drew
// "git stash list ... failed", with nothing on stderr because the child had
// been signaled.
//
// The one answer that cannot be wrong is the goroutine that owns the command:
// while it has not returned from Wait, the process is a zombie at worst and the
// number is still ours.
func TestAKillGoesOnlyToAProcessThisProgramHasNotReaped(t *testing.T) {
	if testing.Short() {
		t.Skip("starts a process")
	}
	t.Parallel()

	// A process that ignores the term, so the grace runs out and the kill is
	// the only way it ends.
	// The shell touches the file after installing the trap, so the test waits
	// on that rather than on a sleep: a sleep long enough on an idle machine is
	// short on a loaded one, which is the shape of the flake this suite already
	// had once.
	ready := filepath.Join(t.TempDir(), "trapped")
	cmd := exec.Command("sh", "-c", "trap '' TERM; : > \"$1\"; sleep 30", "sh", ready)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	reaped := make(chan struct{})
	go func() { _ = cmd.Wait(); close(reaped) }()
	t.Cleanup(func() { _ = syscall.Kill(-pid, syscall.SIGKILL) })
	if !waitFor(func() bool { _, err := os.Stat(ready); return err == nil }, 10*time.Second) {
		t.Fatal("the shell never reported its trap installed")
	}

	if termWasEnough := stopGroupOwned(pid, reaped, 200*time.Millisecond); termWasEnough {
		t.Error("a process that ignores the term was reported as ending on it")
	}
	select {
	case <-reaped:
	case <-time.After(5 * time.Second):
		t.Error("the kill did not end it")
	}
}

// A process that is already reaped is one whose number the kernel may have
// given to someone else, so nothing is sent to it.
func TestNothingIsSentToANumberAlreadyReaped(t *testing.T) {
	if testing.Short() {
		t.Skip("starts a process")
	}
	t.Parallel()

	cmd := exec.Command("sh", "-c", "exit 0")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	reaped := make(chan struct{})
	go func() { _ = cmd.Wait(); close(reaped) }()
	<-reaped

	// Nothing is sent at all. A term to a number the kernel has handed on is
	// a term to someone else, so the reap is read before the signal and not
	// only after it.
	sent := 0
	count := func(int, syscall.Signal) error { sent++; return nil }

	start := time.Now()
	if termWasEnough := stopGroupWith(count, pid, reaped, 5*time.Second); !termWasEnough {
		t.Error("a process that had already gone was reported as needing a kill")
	}
	if sent != 0 {
		t.Errorf("%d signal(s) went to a number this program no longer owns", sent)
	}
	if took := time.Since(start); took > time.Second {
		t.Errorf("stopping an already reaped process took %v; it waited on the "+
			"kernel rather than on the goroutine that owns it", took)
	}
}

// The ordinary case: git takes the term, ends, and is reaped inside the grace.
// Neither of the two tests above covers it — one never reaps and the other is
// reaped before anything is sent — and it is the one that runs every time a
// reader presses q with a git in flight.
func TestATermThatIsTakenSendsNoKill(t *testing.T) {
	t.Parallel()
	reaped := make(chan struct{})
	var sent []syscall.Signal
	send := func(_ int, sig syscall.Signal) error {
		sent = append(sent, sig)
		// The term lands and the process ends: what the owner's Wait would do.
		if sig == syscall.SIGTERM {
			close(reaped)
		}
		return nil
	}

	if termWasEnough := stopGroupWith(send, 424242, reaped, 5*time.Second); !termWasEnough {
		t.Error("a term that was taken was reported as not enough")
	}
	if len(sent) != 1 || sent[0] != syscall.SIGTERM {
		t.Errorf("sent %v; a term that is taken is the only signal that goes", sent)
	}
}

// The term itself cannot be sent. On this system that means the group is not
// there — the only process this program signals is one it started, and a group
// it cannot reach has already gone. Sending a kill after it reaches the same
// nothing, and reporting the term as not enough makes the caller wait out the
// grace for a process that ended before it was asked to.
func TestATermThatCannotBeSentNeedsNoKill(t *testing.T) {
	t.Parallel()
	reaped := make(chan struct{})
	var sent []syscall.Signal
	send := func(_ int, sig syscall.Signal) error {
		sent = append(sent, sig)
		return syscall.ESRCH
	}

	if termWasEnough := stopGroupWith(send, 424242, reaped, 5*time.Second); !termWasEnough {
		t.Error("a term that could not be sent was reported as not enough, so a kill follows")
	}
	if len(sent) != 1 || sent[0] != syscall.SIGTERM {
		t.Errorf("sent %v; nothing follows a term that could not be sent", sent)
	}
}

// Cancel is what the deadline runs, and it runs whether or not the command was
// ever started: a context that was already over when exec looked at it, or a
// start that failed, both leave Cancel with no process to signal. Reading the
// process id of a command that has none is a nil dereference inside the
// deadline, which is a crash of the pane rather than a git that took too long.
func TestTheDeadlineOnACommandThatNeverStartedSignalsNothing(t *testing.T) {
	t.Parallel()
	cmd := exec.CommandContext(context.Background(), "sleep", "5")
	killWholeTree(cmd)
	if cmd.Cancel == nil {
		t.Fatal("killWholeTree left no cancel to run")
	}
	if err := cmd.Cancel(); err != nil {
		t.Errorf("canceling a command that never started answered %v", err)
	}

	// The same cancel ends a command that did start, or the answer above is
	// simply what it always gives.
	started := exec.CommandContext(context.Background(), "sleep", "5")
	killWholeTree(started)
	if err := started.Start(); err != nil {
		t.Fatal(err)
	}
	if err := started.Cancel(); err != nil {
		t.Fatalf("canceling a running command answered %v", err)
	}
	err := started.Wait()
	if err == nil {
		t.Fatal("the command outlived its cancel")
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("the command ended with %v, want a signal", err)
	}
	if got := signalThatEnded(exit); got != syscall.SIGKILL {
		t.Errorf("the command was ended by %v, want SIGKILL", got)
	}
}
