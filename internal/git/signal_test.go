package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// gitThatDies puts a program named git ahead of the real one on PATH, so that a
// read can be made to end the way the failing runs ended: killed by a signal,
// with nothing on stderr. It records each time it is run, which is how a test
// tells one attempt from two.
//
// A shim rather than a fake at the Go level, because what is under test is the
// path from exec's wait status to the error a caller reads, and a fake would
// start below it.
func gitThatDies(t *testing.T, sleep time.Duration) (calls func() int) {
	t.Helper()
	dir := t.TempDir()
	tally := filepath.Join(dir, "calls")
	body := "#!/bin/sh\necho x >> " + tally + "\n"
	if sleep > 0 {
		body += "sleep " + strconv.FormatFloat(sleep.Seconds(), 'f', 2, 64) + "\n"
	}
	body += "kill -SEGV $$\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return func() int {
		b, err := os.ReadFile(tally)
		if os.IsNotExist(err) {
			return 0
		}
		if err != nil {
			t.Fatal(err)
		}
		return strings.Count(string(b), "x")
	}
}

// A signal means git never answered, so the same question is asked once more.
// Under -race on this machine, the child of a fork segfaults before it reaches
// exec about twice in eight runs of the tui suite: the crash report names the
// test binary, not git, because the process died still being a copy of the
// parent. The reader saw "exit -1: " with nothing after it.
func TestAReadKilledBySignalIsAskedOnceMore(t *testing.T) {
	if testing.Short() {
		t.Skip("starts a process")
	}
	calls := gitThatDies(t, 0)

	_, err := runRead(context.Background(), t.TempDir(), "rev-parse", "--absolute-git-dir")
	if err == nil {
		t.Fatal("a git that always dies returned no error")
	}
	if got := calls(); got != 2 {
		t.Errorf("git ran %d times, want the first and one retry", got)
	}
	if !strings.Contains(err.Error(), "segmentation fault") {
		t.Errorf("error = %q, want the signal named rather than exit -1", err)
	}
	if strings.Contains(err.Error(), "exit -1") {
		t.Errorf("error = %q still shows exit -1", err)
	}
}

// A write is not asked again. This cannot tell a fork that died before exec
// from a git that ran and was killed part way, and running a write twice is a
// second action rather than the same question.
func TestAWriteKilledBySignalIsNotAskedAgain(t *testing.T) {
	if testing.Short() {
		t.Skip("starts a process")
	}
	calls := gitThatDies(t, 0)

	if _, err := runWrite(context.Background(), t.TempDir(), "commit-tree", "HEAD^{tree}"); err == nil {
		t.Fatal("a git that always dies returned no error")
	}
	if got := calls(); got != 1 {
		t.Errorf("git ran %d times, want once", got)
	}
}

// signalThatEnded reads the wait status rather than the code, because a process
// that chose to exit 255 and one a signal ended can answer alike.
func TestAChosenExitCodeIsNotReadAsASignal(t *testing.T) {
	if testing.Short() {
		t.Skip("starts a process")
	}
	dir := t.TempDir()
	body := "#!/bin/sh\necho 'error: refused' >&2\nexit 1\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	_, err := runRead(context.Background(), t.TempDir(), "rev-parse", "--absolute-git-dir")
	var exit *ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("got %v, want an ExitError", err)
	}
	if exit.Killed() {
		t.Errorf("an exit code read as a signal: %v", exit.Signal)
	}
	if exit.Signal != syscall.Signal(0) {
		t.Errorf("Signal = %v, want none", exit.Signal)
	}
	if !strings.Contains(err.Error(), "error: refused") {
		t.Errorf("error = %q, want git's own line", err)
	}
}
