package tui

import (
	"context"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/layout"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/state"
	"github.com/fsnotify/fsnotify"
)

const (
	watchDebounce = 200 * time.Millisecond
	// Five seconds. The watcher is what keeps the pane current; this catches
	// what a watcher cannot be given — a directory past maxWatchedDirs, a
	// repository whose descriptor budget ran out part way through the walk, or
	// a worktree added under .git/worktrees, which the one-directory watch on
	// .git does not reach.
	watchPollInterval = 5 * time.Second
)

type watchReadyMsg struct {
	watcher *fsnotify.Watcher
	err     error
}

type watchEventMsg struct{}

type watchDebounceDoneMsg struct {
	gen int
}

type watchPollTickMsg struct{}

func beginWatch(ctx context.Context, dir string) tea.Cmd {
	return func() tea.Msg {
		w, err := fsnotify.NewWatcher()
		if err != nil {
			return watchReadyMsg{err: err}
		}
		// A linked worktree keeps .git as a file, so joining ".git/index" fails
		// there and used to take the whole watcher down with it.
		gitDir, err := git.GitDir(ctx, dir)
		if err != nil {
			_ = w.Close()
			return watchReadyMsg{err: err}
		}
		// The directory rather than .git/index: a repository with no commits has
		// no index yet, and binding to that one path meant git init followed by
		// mirugit reported a broken watcher and polled for the whole session.
		//
		// git writes the index by renaming index.lock over it, so the directory
		// hears every write to it and the creation of the first one. Measured on
		// macOS: staging into a fresh repository gave CREATE index, and staging
		// again gave CREATE index.lock, REMOVE index, CREATE index, RENAME.
		// The other names directly under .git — HEAD, MERGE_HEAD, the commit
		// message — are ones the pane should redraw for too.
		if err := w.Add(gitDir); err != nil {
			_ = w.Close()
			return watchReadyMsg{err: err}
		}
		// One directory the watcher cannot take does not make the rest useless.
		// A repository with node_modules hits the descriptor limit part way
		// through, and failing the whole watcher left only the five-second poll.
		addWorktreeWatch(ctx, w, dir)
		return watchReadyMsg{watcher: w}
	}
}

// maxWatchedDirs bounds how many directories the watcher takes. Linux counts
// them against fs.inotify.max_user_watches and macOS opens a file descriptor
// for each, so a repository with node_modules exhausts either one. Past the
// bound the five-second poll is what keeps the pane current.
const maxWatchedDirs = 2000

// addWorktreeWatch takes what it can and reports how many it took. A directory
// git ignores is skipped: watching node_modules costs the budget that the
// working tree needs.
func addWorktreeWatch(ctx context.Context, w *fsnotify.Watcher, root string) int {
	ignored := ignoredDirs(ctx, root)
	added := 0
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		// Returning the error would stop the walk. One unreadable directory is
		// not a reason to leave the rest of the tree unwatched.
		if err != nil || !d.IsDir() {
			return nil //nolint:nilerr // keep walking; see above
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil //nolint:nilerr // keep walking; see above
		}
		// .git is watched by beginWatch, as one directory rather than a tree.
		// The names under objects/ and refs/ change on every write and none of
		// them changes what the pane draws, so a repack would spend the whole
		// budget here.
		if rel == ".git" || strings.HasPrefix(rel, ".git"+string(filepath.Separator)) {
			return filepath.SkipDir
		}
		if ignored[rel] {
			return filepath.SkipDir
		}
		if added >= maxWatchedDirs {
			return filepath.SkipAll
		}
		if w.Add(path) == nil {
			added++
		}
		return nil
	})
	return added
}

// ignoredDirs names the directories git ignores, relative to root. It asks git
// once rather than reading .gitignore, because the rules compose across files
// and the answer has to match what the reader sees.
func ignoredDirs(ctx context.Context, root string) map[string]bool {
	paths, err := git.IgnoredDirs(ctx, root)
	if err != nil {
		return nil
	}
	out := make(map[string]bool, len(paths))
	for _, p := range paths {
		out[p] = true
	}
	return out
}

func waitWatchEvent(w *fsnotify.Watcher) tea.Cmd {
	return func() tea.Msg {
		for {
			select {
			case ev, ok := <-w.Events:
				if !ok {
					return watchEventMsg{}
				}
				// Reading a file bumps its access time, which macOS reports as
				// CHMOD. Reloading on that would make the pane's own git reads
				// schedule the next reload, and a debounce that restarts on
				// every event would never complete.
				if ev.Op == fsnotify.Chmod {
					continue
				}
				maybeWatchNewDir(w, ev)
				return watchEventMsg{}
			case _, ok := <-w.Errors:
				if !ok {
					return watchEventMsg{}
				}
				return watchEventMsg{}
			}
		}
	}
}

func maybeWatchNewDir(w *fsnotify.Watcher, ev fsnotify.Event) {
	if ev.Op&fsnotify.Create == 0 {
		return
	}
	stat, err := os.Stat(ev.Name)
	if err != nil || !stat.IsDir() {
		return
	}
	_ = w.Add(ev.Name)
}

func (m *Model) scheduleWatchDebounce(gen int) tea.Cmd {
	return tea.Tick(m.delay(m.watch.debounceDelay, watchDebounce), func(time.Time) tea.Msg {
		return watchDebounceDoneMsg{gen: gen}
	})
}

func (m *Model) schedulePoll() tea.Cmd {
	return tea.Tick(m.delay(m.watch.pollDelay, watchPollInterval), func(time.Time) tea.Msg {
		return watchPollTickMsg{}
	})
}

// delay lets a test shorten a wait on its own Model rather than on a package
// variable, which is what kept the tests from running in parallel. A Model
// built as a literal leaves the field at zero and gets the real interval.
func (m *Model) delay(set, standard time.Duration) time.Duration {
	if set > 0 {
		return set
	}
	return standard
}

func (m *Model) handleWatchReady(msg watchReadyMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.state = state.Apply(m.state, state.Failed{
			Line: "file watcher unavailable · polling only",
		})
		return m, nil
	}
	// Two rebinds in a row leave two beginWatch commands in flight, and the
	// second answer used to overwrite the first watcher: its goroutine and its
	// descriptors then lived as long as the program did.
	m.closeWatcher()
	m.watch.fs = msg.watcher
	return m, m.waitForWatch()
}

// closeWatcher releases the watcher and the goroutine reading from it. Closing
// wakes that reader, which returns without asking for another event, so the
// waiting flag has to go with it.
func (m *Model) closeWatcher() {
	if m.watch.fs == nil {
		return
	}
	_ = m.watch.fs.Close()
	m.watch.fs = nil
	m.watch.waitingForEvent = false
}

// Close releases what this model holds. It does not stop the git processes the
// program started: that is git.StopAll, which main calls on the way out. It
// belongs to the program rather than to a model, because a test builds several
// models and stopping git in one would stop it for all of them.
func (m *Model) Close() {
	m.closeWatcher()
}

// waitForWatch issues a reader only when none is outstanding.
func (m *Model) waitForWatch() tea.Cmd {
	if m.watch.fs == nil || m.watch.waitingForEvent {
		return nil
	}
	m.watch.waitingForEvent = true
	return waitWatchEvent(m.watch.fs)
}

// pollLater issues a tick only when none is outstanding.
func (m *Model) pollLater() tea.Cmd {
	if m.watch.waitingForPoll {
		return nil
	}
	m.watch.waitingForPoll = true
	return m.schedulePoll()
}

func (m *Model) handleWatchEvent() (tea.Model, tea.Cmd) {
	m.watch.waitingForEvent = false
	var cmds []tea.Cmd
	cmds = append(cmds, m.waitForWatch())
	if !m.watch.focused {
		return m, tea.Batch(cmds...)
	}
	m.watch.debounceGen++
	gen := m.watch.debounceGen
	cmds = append(cmds, m.scheduleWatchDebounce(gen))
	return m, tea.Batch(cmds...)
}

func (m *Model) handleWatchDebounceDone(msg watchDebounceDoneMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.watch.debounceGen || !m.watch.focused {
		return m, nil
	}
	return m, m.reloadCurrentTab()
}

func (m *Model) handleWatchPollTick() (tea.Model, tea.Cmd) {
	m.watch.waitingForPoll = false
	if !m.watch.focused {
		return m, nil
	}
	// "fetched 3m ago" ages on its own, so the label is recomputed here rather
	// than in View, which must not read the clock or the repository.
	m.state = state.Apply(m.state, state.FetchedAgoUpdated{
		Label: layout.FormatFetchedAgo(m.fetchedAt, time.Now())})
	return m, tea.Batch(m.reloadCurrentTab(), m.checkStale(), m.pollLater())
}

func (m *Model) handleFocus() (tea.Model, tea.Cmd) {
	m.watch.focused = true
	return m, tea.Batch(m.reloadCurrentTab(), m.waitForWatch(), m.pollLater())
}

func (m *Model) handleBlur() (tea.Model, tea.Cmd) {
	m.watch.focused = false
	m.watch.debounceGen++
	return m, m.waitForWatch()
}
