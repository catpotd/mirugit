package tui

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"
)

// watchingGoroutines counts the readers fsnotify has running. Each open watcher
// holds one goroutine and a handful of descriptors, and neither is returned
// until the watcher is closed.
func watchingGoroutines() int {
	buf := make([]byte, 1<<20)
	dump := string(buf[:runtime.Stack(buf, true)])
	n := 0
	for _, block := range strings.Split(dump, "\n\n") {
		if strings.Contains(block, "fsnotify") {
			n++
		}
	}
	return n
}

// A second watchReadyMsg used to overwrite the first watcher without closing
// it. Two rebinds in a row leave two beginWatch commands in flight, so this is
// reachable by pressing g twice.
func TestASecondWatcherDoesNotStrandTheFirst(t *testing.T) {
	dir := testRepo(t)
	m, err := New(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)

	before := watchingGoroutines()
	for range 5 {
		msg, ok := beginWatch(context.Background(), dir)().(watchReadyMsg)
		if !ok || msg.watcher == nil {
			t.Skip("this machine gave no watcher")
		}
		next, _ := m.handleWatchReady(msg)
		m = next.(*Model)
	}
	settle()
	if got := watchingGoroutines(); got > before+1 {
		t.Errorf("five watchers left %d readers running, want at most one",
			got-before)
	}
}

func TestCloseReleasesTheWatcher(t *testing.T) {
	dir := testRepo(t)
	m, err := New(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	msg, ok := beginWatch(context.Background(), dir)().(watchReadyMsg)
	if !ok || msg.watcher == nil {
		t.Skip("this machine gave no watcher")
	}
	next, _ := m.handleWatchReady(msg)
	m = next.(*Model)

	before := watchingGoroutines()
	m.Close()
	settle()
	if got := watchingGoroutines(); got >= before {
		t.Errorf("Close left %d readers running, was %d", got, before)
	}
	// Close twice is what a deferred call after an early return does.
	m.Close()
}

// settle gives the reader goroutine time to notice the close and return.
func settle() {
	for range 20 {
		runtime.Gosched()
		time.Sleep(10 * time.Millisecond)
	}
}
