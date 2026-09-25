package tui

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

func applyRepoCmd(t *testing.T, m *Model, cmd tea.Cmd) *Model {
	t.Helper()
	if cmd == nil {
		return m
	}
	msg := cmd()
	switch v := msg.(type) {
	case repoMsg:
		next, _ := m.Update(v)
		return next.(*Model)
	case fetchedMsg:
		next, _ := m.Update(v)
		return next.(*Model)
	case worktreesMsg:
		// A reload reads the worktree list too, so the bar's fourth count
		// follows the repository like the other three.
		next, _ := m.Update(v)
		return next.(*Model)
	case tea.BatchMsg:
		for _, c := range v {
			if c == nil {
				continue
			}
			m = applyRepoCmd(t, m, c)
		}
		return m
	default:
		t.Fatalf("want a repo command, got %T", msg)
		return m
	}
}

func TestWatchStartupFailureShowsNotice(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	next, cmd := m.Update(watchReadyMsg{err: errors.New("too many open files")})
	m = next.(*Model)
	if cmd != nil {
		t.Fatal("watch failure should not schedule events")
	}
	if m.watch.fs != nil {
		t.Fatal("watcher should stay nil when startup fails")
	}
	want := "file watcher unavailable · polling only"
	if m.state.Notice != want {
		t.Errorf("got notice %q, want %q", m.state.Notice, want)
	}
}

func TestExternalFileChangeRefreshesTheList(t *testing.T) {
	t.Parallel()
	dir := testRepo(t)
	m, err := New(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.probe.settled = true
	m.watch.focused = true
	m.state.Width = 77
	m.state.Height = 53

	repo, err := git.Load(context.Background(), dir, mustGitDir(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	next, _ := m.Update(repoMsgFor(t, dir, repo))
	m = next.(*Model)

	ready := beginWatch(context.Background(), dir)().(watchReadyMsg)
	if ready.err != nil {
		t.Fatal(ready.err)
	}
	next, waitCmd := m.Update(ready)
	m = next.(*Model)

	go func() {
		if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("new\n"), 0o644); err != nil {
			t.Error(err)
		}
	}()

	if _, ok := waitCmd().(watchEventMsg); !ok {
		t.Fatalf("want a watch event, got %T", waitCmd())
	}

	next, _ = m.Update(watchEventMsg{})
	m = next.(*Model)
	next, reloadCmd := m.handleWatchDebounceDone(watchDebounceDoneMsg{gen: m.watch.debounceGen})
	m = next.(*Model)
	m = applyRepoCmd(t, m, reloadCmd)

	found := false
	for _, e := range m.state.Changes.Entries {
		if e.Path == "b.txt" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("want b.txt in entries, got %+v", m.state.Changes.Entries)
	}
}

func TestContinuousWritesProduceOneReload(t *testing.T) {
	t.Parallel()
	dir := testRepo(t)
	m, err := New(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.probe.settled = true
	m.watch.focused = true

	repo, err := git.Load(context.Background(), dir, mustGitDir(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	next, _ := m.Update(repoMsgFor(t, dir, repo))
	m = next.(*Model)

	ready := beginWatch(context.Background(), dir)().(watchReadyMsg)
	if ready.err != nil {
		t.Fatal(ready.err)
	}
	next, _ = m.Update(ready)
	m = next.(*Model)

	for i := range 3 {
		name := filepath.Join(dir, string(rune('a'+i))+".txt")
		go func() {
			if err := os.WriteFile(name, []byte("x\n"), 0o644); err != nil {
				t.Error(err)
			}
		}()
		if _, ok := waitWatchEvent(m.watch.fs)().(watchEventMsg); !ok {
			t.Fatal("want a watch event")
		}
		next, _ = m.Update(watchEventMsg{})
		m = next.(*Model)
	}

	_, reloadCmd := m.handleWatchDebounceDone(watchDebounceDoneMsg{gen: m.watch.debounceGen})
	if countRepoCmds(reloadCmd) != 1 {
		t.Fatalf("want one reload after coalesced events, got %d", countRepoCmds(reloadCmd))
	}
}

// Reading a file updates its access time, which macOS reports as CHMOD. The
// pane's own git reads would otherwise retrigger the watcher on every reload,
// and a debounce that restarts on each event never completes.
func TestReadingTheTreeDoesNotWakeTheWatcher(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("touches the filesystem")
	}
	dir := testRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ready := beginWatch(context.Background(), dir)().(watchReadyMsg)
	if ready.err != nil {
		t.Fatal(ready.err)
	}
	defer func() { _ = ready.watcher.Close() }()

	events := make(chan tea.Msg, 1)
	go func() { events <- waitWatchEvent(ready.watcher)() }()

	if _, err := os.ReadFile(filepath.Join(dir, "a.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filepath.Join(dir, "a.txt"), time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-events:
		t.Fatal("a metadata-only touch woke the watcher")
	case <-time.After(time.Second):
	}

	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case <-events:
	case <-time.After(3 * time.Second):
		t.Fatal("a write did not wake the watcher")
	}
}

// The tab's count comes from state, and a tab whose count is zero is not
// drawn. Loading worktrees only when the tab is opened therefore hides the
// tab that would open it.
func TestWorktreesTabAppearsWithoutOpeningIt(t *testing.T) {
	t.Parallel()
	dir := testRepo(t)
	env := []string{
		"HOME=" + dir, "GIT_CONFIG_GLOBAL=" + dir + "/.gitconfig",
		"GIT_CONFIG_SYSTEM=/dev/null",
		// git forks a repack of its own once a repository crosses a
		// threshold, and that child outlives the test: it races
		// t.TempDir's removal, and it takes the repository's locks
		// while another test is reading it.
		"GIT_CONFIG_COUNT=2",
		"GIT_CONFIG_KEY_0=gc.auto", "GIT_CONFIG_VALUE_0=0",
		"GIT_CONFIG_KEY_1=maintenance.auto", "GIT_CONFIG_VALUE_1=false",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@e.c",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@e.c",
	}
	second := filepath.Join(t.TempDir(), "second")
	cmd := exec.Command("git", "worktree", "add", "-q", second, "-b", "other")
	cmd.Dir = dir
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git worktree add: %v\n%s", err, out)
	}

	m, err := New(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.probe.settled = true
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 53})
	m = pumpInit(t, m)

	m.View()
	if !strings.Contains(m.frame.Lines[0], "worktrees") {
		t.Errorf("no worktrees tab on a repository with two worktrees: %q",
			m.frame.Lines[0])
	}
}

// pumpInit runs Init's commands the way the runtime would, so a test sees the
// state a reader sees on the first frame.
func pumpInit(t *testing.T, m *Model) *Model {
	t.Helper()
	// Init schedules the five-second poll, and this loop runs every command it
	// is handed. Without shortening the wait each call to pumpInit cost five
	// seconds of real time.
	m.watch.pollDelay, m.watch.debounceDelay = time.Nanosecond, time.Nanosecond
	queue := []tea.Cmd{m.Init()}
	for i := 0; i < 40 && len(queue) > 0; i++ {
		cmd := queue[0]
		queue = queue[1:]
		if cmd == nil {
			continue
		}
		msg := cmd()
		if msg == nil {
			continue
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			queue = append(queue, batch...)
			continue
		}
		switch msg.(type) {
		case watchEventMsg, watchPollTickMsg, watchReadyMsg, watchDebounceDoneMsg:
			continue
		}
		next, out := m.Update(msg)
		m = next.(*Model)
		queue = append(queue, out)
	}
	return m
}

func hasRepoCmd(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	msg := cmd()
	switch v := msg.(type) {
	case repoMsg:
		return true
	case tea.BatchMsg:
		for _, c := range v {
			if c != nil && hasRepoCmd(c) {
				return true
			}
		}
	}
	return false
}

func countRepoCmds(cmd tea.Cmd) int {
	if cmd == nil {
		return 0
	}
	msg := cmd()
	switch v := msg.(type) {
	case repoMsg:
		return 1
	case tea.BatchMsg:
		n := 0
		for _, c := range v {
			n += countRepoCmds(c)
		}
		return n
	}
	return 0
}

func TestRapidWatchEventsCoalesceToOneReload(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	m.watch.focused = true

	_, _ = m.Update(watchEventMsg{})
	_, _ = m.Update(watchEventMsg{})
	_, _ = m.Update(watchEventMsg{})

	gen := m.watch.debounceGen
	_, stale := m.Update(watchDebounceDoneMsg{gen: gen - 1})
	if countRepoCmds(stale) != 0 {
		t.Fatal("an older debounce should not reload")
	}

	_, fresh := m.Update(watchDebounceDoneMsg{gen: gen})
	if countRepoCmds(fresh) != 1 {
		t.Fatalf("want one reload, got %d", countRepoCmds(fresh))
	}
}

func TestPollDoesNotReloadWhileUnfocused(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	m.watch.focused = false

	_, cmd := m.Update(watchPollTickMsg{})
	if hasRepoCmd(cmd) {
		t.Fatal("poll should not reload while blurred")
	}
}

// The tick that a poll schedules is the one that keeps the pane current while
// nothing is typed and the watcher says nothing. Dropping the answer to a poll
// tick stops the polling: the reload and the next tick both go with it, and the
// pane sits on what it read last, with nothing on screen to say it has stopped.
func TestAPollWhileFocusedReloadsAndAsksForTheNextTick(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	m.watch.pollDelay = time.Nanosecond
	m.watch.focused = true
	m.watch.waitingForPoll = false

	_, cmd := m.Update(watchPollTickMsg{})
	if !hasRepoCmd(cmd) {
		t.Error("a poll while focused did not reload")
	}
	if !m.watch.waitingForPoll {
		t.Error("no next tick was asked for, so this was the last poll")
	}
	// The flag alone is not the tick: it is also what pollLater reads to decide
	// whether one is already on its way. Left set as the tick arrives, the flag
	// says yes and no tick is scheduled at all.
	if !hasPollTick(cmd) {
		t.Error("the answer carries no next tick, so the polling stops here")
	}
}

func hasPollTick(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	switch v := cmd().(type) {
	case watchPollTickMsg:
		return true
	case tea.BatchMsg:
		for _, c := range v {
			if hasPollTick(c) {
				return true
			}
		}
	}
	return false
}

func TestFocusReloadsOnce(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	m.watch.pollDelay = time.Nanosecond
	m.watch.focused = false

	_, cmd := m.Update(tea.FocusMsg{})
	if !hasRepoCmd(cmd) {
		t.Fatal("focus should reload once")
	}
	if countRepoCmds(cmd) != 1 {
		t.Fatalf("want one reload on focus, got %d", countRepoCmds(cmd))
	}
	// Focus is also what says the pane is being looked at, and the watcher and
	// the poll both read it before they reload. Left saying otherwise, the
	// window can be in front of the reader while the pane stops keeping up.
	if !m.watch.focused {
		t.Fatal("focus did not record that the pane is being looked at")
	}
	_, _ = m.Update(watchEventMsg{})
	gen := m.watch.debounceGen
	if _, cmd = m.Update(watchDebounceDoneMsg{gen: gen}); !hasRepoCmd(cmd) {
		t.Error("a change on disk after focus did not reload")
	}
}

func TestBlurCancelsPendingDebounce(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	m.watch.focused = true

	_, _ = m.Update(watchEventMsg{})
	gen := m.watch.debounceGen

	_, _ = m.Update(tea.BlurMsg{})
	_, cmd := m.Update(watchDebounceDoneMsg{gen: gen})
	if hasRepoCmd(cmd) {
		t.Fatal("blur should cancel a pending debounced reload")
	}
}

// Mouse motion arrives once per cell crossed, and following the cursor with a
// diff spawns git each time. Seventeen milliseconds a call is a stutter the
// moment the pointer moves across the list.
// Hover arms the row and nothing else. Opening a diff costs a git call and
// marks what it shows as read, and a pointer crossing the list is not a reader
// deciding to look at anything.
func TestHoverNeverOpensADiff(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	m.View()
	rowA, _ := hitOf(m, "app/a.swift")
	rowB, _ := hitOf(m, "app/b.swift")
	for _, row := range []int{rowA, rowB, rowA, rowB} {
		next, cmd := m.Update(tea.MouseMotionMsg{X: 20, Y: row})
		m = next.(*Model)
		if cmd != nil {
			t.Fatalf("hover on row %d produced a command", row)
		}
	}
}

func TestClickOpensTheDiff(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	m.View()
	row, col := hitOf(m, "app/a.swift")
	_, cmd := m.Update(tea.MouseClickMsg{X: col, Y: row, Button: tea.MouseLeft})
	if cmd == nil {
		t.Fatal("clicking a file name did not ask for its diff")
	}
}

// waitForWatch issues one reader and records that it did. Without the record
// every call issues another, and each one holds a goroutine on the watcher's
// channel: the readers pile up for as long as the pane runs, and the same
// event is then handled once per reader.
func TestOnlyOneWatcherReaderIsOutstanding(t *testing.T) {
	t.Parallel()
	dir := testRepo(t)
	m, err := New(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.probe.settled = true

	ready := beginWatch(context.Background(), dir)().(watchReadyMsg)
	if ready.err != nil {
		t.Fatal(ready.err)
	}
	next, _ := m.Update(ready)
	m = next.(*Model)
	if m.watch.fs == nil {
		t.Fatal("the watcher did not open, so this proves nothing")
	}
	m.watch.waitingForEvent = false

	if cmd := m.waitForWatch(); cmd == nil {
		t.Fatal("the first call issued no reader, so this proves nothing")
	}
	if !m.watch.waitingForEvent {
		t.Error("the first call did not record that a reader is out")
	}
	if cmd := m.waitForWatch(); cmd != nil {
		t.Error("a second reader was issued while the first is still out")
	}
}

// Closing the watcher wakes the goroutine reading from it, and that goroutine
// returns without asking for another event. The flag that says a reader is out
// has to go with it: left set, the next watcher opens and waitForWatch declines
// to read from it, so the pane stops seeing the files change with nothing on
// screen to say it has stopped.
func TestClosingTheWatcherLetsTheNextOneBeRead(t *testing.T) {
	t.Parallel()
	dir := testRepo(t)
	m, err := New(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.probe.settled = true

	ready := beginWatch(context.Background(), dir)().(watchReadyMsg)
	if ready.err != nil {
		t.Fatal(ready.err)
	}
	next, _ := m.Update(ready)
	m = next.(*Model)
	// Update issues the first reader, so the flag is already set here. That is
	// the state closeWatcher has to undo.
	if !m.watch.waitingForEvent {
		t.Fatal("no reader is out after the watcher opened, so this proves nothing")
	}

	m.closeWatcher()
	if m.watch.fs != nil {
		t.Error("the watcher was not released")
	}

	again := beginWatch(context.Background(), dir)().(watchReadyMsg)
	if again.err != nil {
		t.Fatal(again.err)
	}
	m.watch.fs = again.watcher
	if m.waitForWatch() == nil {
		t.Error("the watcher opened again and nothing reads from it")
	}
	m.closeWatcher()
}

// Losing focus stops the polling: the reader is looking at something else and
// the pane has no reason to keep asking git. Setting the flag the other way
// leaves a blurred pane reloading for as long as the program runs, which is the
// cost the flag exists to avoid.
func TestLosingFocusStopsThePolling(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	m.watch.pollDelay = time.Nanosecond
	m.watch.focused = true
	m.watch.waitingForPoll = false

	next, _ := m.Update(tea.BlurMsg{})
	m = next.(*Model)
	if m.watch.focused {
		t.Error("the pane still reads as focused after a blur")
	}

	_, cmd := m.Update(watchPollTickMsg{})
	if hasRepoCmd(cmd) {
		t.Error("a poll after the blur reloaded the repository")
	}
}

// ignoredDirs keeps the watcher off node_modules. A read it could not make is
// not an answer of none: the walk then watches every ignored tree and spends
// the descriptor budget the working tree needs.
func TestIgnoredDirsIsEmptyForADirectoryItCouldNotRead(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := testRepo(t)
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("node_modules/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "node_modules", "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "node_modules", "pkg", "a.js"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := ignoredDirs(context.Background(), dir); !got["node_modules"] {
		t.Errorf("the ignored directory is not in %v", got)
	}
	if got := ignoredDirs(context.Background(), t.TempDir()); len(got) != 0 {
		t.Errorf("a directory that is not a repository answered with %v", got)
	}
}
