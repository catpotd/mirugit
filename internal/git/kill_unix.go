//go:build !windows

package git

import (
	"os/exec"
	"syscall"
	"time"
)

// killWholeTree ends git and the helpers it started. git runs helpers of its
// own, and killing only git leaves them holding the pipe: signaling the group
// and giving Wait a short grace period ends the whole tree. Without it a
// deadline took as long as the child did.
func killWholeTree(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}

// stopGroupOwned ends a process group this program owns, waiting on the
// goroutine that started it rather than on the kernel.
//
// The term comes first because git cleans up after it and does not after a
// kill. Measured with a slow clean filter holding .git/index.lock: after a
// TERM the lock was gone, after a KILL it was still there, and a lock left
// behind makes the reader's next git say "another git process seems to be
// running".
//
// The waiting is the part that has to be careful. Every git runs as the leader
// of its own group, so the group number is the process number, and the kernel
// may hand that number out again the moment the process is reaped. Asking
// `kill(-pid, 0)` whether the group is still there cannot tell "still mine"
// from "someone else's now": a shutdown that asked, and got yes about a number
// already given to another program's git, waited out the grace and killed that
// one. Until Wait returns, the process is a zombie at worst, and a zombie holds
// its number.
//
// The window is narrowed rather than closed. Reading the reap and sending the
// signal are two steps, and the reap can land between them; there is no way to
// hold a process number across a signal on macOS, which has no pidfd. What the
// old form waited out was the whole grace with a wrong answer in hand. What
// this one leaves is the microseconds between the last read and the send.
//
// What it gives up is narrower than it sounds. The kill is skipped only once
// git itself is reaped, and the reap happens after git has ended its own
// children: measured with a clean filter that ignores TERM, INT and HUP, the
// grace ran out, the kill went to the group, and nothing was left a moment
// later. What remains possible is a helper that outlives git and ignores every
// signal, and WaitDelay closes its pipes a second later. That is the trade for
// never signaling a process this program does not own.
//
// It answers whether the term was enough. A caller that needs to know what git
// had a chance to do — put its lock files back — has to know which of the two
// signals ended it, because only the first one asks.
func stopGroupOwned(pid int, reaped <-chan struct{}, grace time.Duration) (termWasEnough bool) {
	return stopGroupWith(syscall.Kill, pid, reaped, grace)
}

// stopGroupWith takes the signaling as an argument so a test can count what
// would have left this program. A package variable would be shared by every
// test in the package; see TestNoPackageVariableHoldsAFunction in tui.
func stopGroupWith(send func(int, syscall.Signal) error, pid int, reaped <-chan struct{},
	grace time.Duration) (termWasEnough bool) {
	select {
	case <-reaped:
		// Gone, and gone the way it chose. The number may be someone else's.
		return true
	default:
	}
	if err := send(-pid, syscall.SIGTERM); err != nil {
		return true
	}
	timer := time.NewTimer(grace)
	defer timer.Stop()
	select {
	case <-reaped:
		return true
	case <-timer.C:
	}
	// Once more before the kill: the grace may have expired in the same moment
	// the reap arrived, and a kill sent then goes to whoever holds the number
	// next.
	select {
	case <-reaped:
		return true
	default:
	}
	_ = send(-pid, syscall.SIGKILL)
	return false
}
