package tui

import (
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
)

// Each outstanding reader and tick blocks a goroutine until it fires. focus and
// blur used to issue one without consuming one, so a focus/blur round trip
// leaked two, and a focus within five seconds of a blur started a second poll
// chain that doubled the reload rate.
func TestFocusAndBlurDoNotStackWaiters(t *testing.T) {
	t.Parallel()
	w, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	m := &Model{
		state:  state.State{Tab: state.TabChanges, Width: 77, Height: 24},
		render: layout.Renderer{}, read: emptyRead(), dir: t.TempDir(),
		watch: watchState{fs: w},
	}
	// One of each is outstanding, as after startup.
	m.watch.waitingForPoll, m.watch.waitingForEvent = true, true
	for range 10 {
		next, _ := m.handleBlur()
		m = next.(*Model)
		next, _ = m.handleFocus()
		m = next.(*Model)
	}
	// waitForWatch and pollLater return nil while one is outstanding, so ten
	// round trips must have issued nothing.
	if got := m.waitForWatch(); got != nil {
		t.Error("未消費の reader があるのに、もう1つ出した")
	}
	if got := m.pollLater(); got != nil {
		t.Error("未消費の tick があるのに、もう1つ出した")
	}
	// Consuming one lets exactly one more out.
	next, _ := m.handleWatchPollTick()
	m = next.(*Model)
	m.watch.waitingForPoll = false
	if got := m.pollLater(); got == nil {
		t.Error("消費した後に次の tick を出していない")
	}
}

// A Model built as a literal leaves pollDelay and debounceDelay at zero, which
// is how a test asks for the real interval rather than a short one. Reading
// zero as a delay of zero gives tea.Tick no wait at all: the poll fires as fast
// as the loop can run it, and the pane redraws without pause.
func TestAnUnsetDelayIsTheStandardInterval(t *testing.T) {
	t.Parallel()
	var m Model
	if got := m.delay(m.watch.pollDelay, watchPollInterval); got != watchPollInterval {
		t.Errorf("an unset poll delay is %v, want %v", got, watchPollInterval)
	}
	if got := m.delay(m.watch.debounceDelay, watchDebounce); got != watchDebounce {
		t.Errorf("an unset debounce delay is %v, want %v", got, watchDebounce)
	}
	m.watch.pollDelay = 3 * time.Millisecond
	if got := m.delay(m.watch.pollDelay, watchPollInterval); got != 3*time.Millisecond {
		t.Errorf("a delay the test set is %v, want 3ms", got)
	}
}

// Every watcher event is read one at a time: the read is issued again as soon
// as one arrives, and the flag that says "a read is outstanding" has to be
// cleared first or the next read is skipped. Skipping it leaves the pane
// blind — the watcher has more to say and nobody is listening — and the
// five-second poll becomes the only way it notices anything.
func TestAWatchEventIssuesTheNextRead(t *testing.T) {
	t.Parallel()
	fs, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = fs.Close() }()

	// Blurred, so the wait before a reload is not issued and the only command
	// the batch can hold is the next read.
	var m Model
	m.watch.fs = fs
	m.watch.waitingForEvent = true
	m.watch.focused = false

	_, cmd := m.handleWatchEvent()
	if cmd == nil {
		t.Error("the watcher was not read again after an event")
	}
	if !m.watch.waitingForEvent {
		t.Error("the model does not record that a read is outstanding")
	}
}
