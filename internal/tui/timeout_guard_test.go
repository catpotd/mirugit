package tui

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"
)

// The timeout the go tool passes ends this binary by panicking, and the panic
// prints every goroutine's stack: testing.go calls debug.SetTraceback("all")
// first. Measured, that is 84 KB for this package's forty-seven parallel tests.
// go test reads it through a pipe, a pipe holds 64 KB, and a write with nobody
// draining the other end blocks. The write happens after the timeout has
// already fired, so nothing fires again: one of these was found alive two days
// past its ten-minute timeout, and it was still holding a CPU.
//
// So this ends the run just before that, with a dump small enough for the pipe
// to hold whether or not anyone is reading it.
func TestMain(m *testing.M) {
	flag.Parse()
	undoDesktop, err := standInForTheDesktop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot stand in for the desktop programs:", err)
		os.Exit(1)
	}
	stop := stopBeforeTheTimeout()
	code := m.Run()
	stop()
	undoDesktop()
	os.Exit(code)
}

// guardDumpLimit is what the guard writes. A pipe holds 64 KB, and the sentence
// above the stacks has to fit beside them.
const guardDumpLimit = 32 << 10

// stopBeforeTheTimeout arms a timer a tenth of the run's own timeout early and
// returns the way to disarm it. A tenth rather than a fixed margin so that a
// five-second run in a reproduction is guarded the same as a ten-minute one in
// CI.
func stopBeforeTheTimeout() func() {
	f := flag.Lookup("test.timeout")
	if f == nil {
		return func() {}
	}
	limit, err := time.ParseDuration(f.Value.String())
	if err != nil || limit <= 0 {
		return func() {}
	}
	margin := min(limit/10, 30*time.Second)
	if margin < 100*time.Millisecond {
		return func() {}
	}
	timer := time.AfterFunc(limit-margin, func() {
		buf := make([]byte, guardDumpLimit)
		n := runtime.Stack(buf, true)
		fmt.Fprintf(os.Stderr,
			"test binary stopped %s before its timeout of %s; %d goroutines, "+
				"the first %d bytes of their stacks follow\n",
			margin, limit, runtime.NumGoroutine(), n)
		_, _ = os.Stderr.Write(buf[:n])
		os.Exit(1)
	})
	return func() { timer.Stop() }
}

// A dump larger than the pipe is what let the last one survive, so the size the
// guard writes is the thing to hold: it has to fit whether or not the reader is
// draining.
func TestTheGuardWritesLessThanAPipeHolds(t *testing.T) {
	t.Parallel()
	// The pipe a go test child writes through holds 64 KB on this platform.
	const pipeBuffer = 64 << 10
	sentence := len(fmt.Sprintf(
		"test binary stopped %s before its timeout of %s; %d goroutines, "+
			"the first %d bytes of their stacks follow\n",
		30*time.Second, 10*time.Minute, 999999, guardDumpLimit))
	if total := guardDumpLimit + sentence; total >= pipeBuffer {
		t.Errorf("the guard writes up to %d bytes, and a pipe holds %d", total, pipeBuffer)
	}
}

// A run with no timeout, or one too short to leave a margin, is left alone: the
// guard exists to beat a deadline, and without one there is nothing to beat.
func TestTheGuardStandsDownWhenThereIsNoDeadlineToBeat(t *testing.T) {
	t.Parallel()
	if f := flag.Lookup("test.timeout"); f == nil {
		t.Skip("this binary has no timeout flag")
	}
	for _, c := range []struct {
		name  string
		limit time.Duration
		armed bool
	}{
		{"no timeout at all", 0, false},
		{"too short to leave a margin", 500 * time.Millisecond, false},
		{"a second", time.Second, true},
		{"the go tool's default", 10 * time.Minute, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			margin := min(c.limit/10, 30*time.Second)
			armed := c.limit > 0 && margin >= 100*time.Millisecond
			if armed != c.armed {
				t.Errorf("timeout %s: armed = %v, want %v", c.limit, armed, c.armed)
			}
		})
	}
}
