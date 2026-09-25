package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
	"github.com/fsnotify/fsnotify"
)

// The probe asks the terminal how wide an ambiguous glyph is. A terminal that
// never answers leaves the pane measuring with a guess, and the help says so:
// a reader whose columns look wrong has no other way to find out why.
func TestTheHelpSaysWhenTheWidthIsAGuess(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name    string
		settle  func(*Model)
		guessed bool
	}{
		{"the terminal answered", func(m *Model) {
			m.probe.replied, m.probe.before = true, 10
			_, _ = m.handleCursorPosition(tea.CursorPositionMsg{X: 12})
		}, false},
		{"the terminal said nothing", func(m *Model) {
			_, _ = m.handleWidthProbeTimeout()
		}, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := newParentModel(t)
			m.render.Widths = layout.Widths{}
			m.probe.settled = false
			c.settle(m)

			lines, _ := m.render.HelpOverlay(nil, state.TabChanges, 0, 90, 40)
			said := strings.Contains(strings.Join(lines, "\n"), "assumed")
			if said != c.guessed {
				t.Errorf("the help says the width is a guess %v, want %v\n%s",
					said, c.guessed, strings.Join(lines, "\n"))
			}
		})
	}
}

// Staging one block leaves the rest of the file unstaged, so the diff on screen
// is no longer what git holds. The pane reloads it; a whole-file verb has
// nothing left to show and does not.
func TestStagingOneBlockReloadsTheDiffAndAWholeFileVerbDoesNot(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name   string
		block  int
		reload bool
	}{
		{"one block", 0, true},
		{"the whole file", -1, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := newParentModel(t)
			m.state = state.Apply(m.state, state.StatusLoaded{
				Rows: []git.Entry{{Path: "a.txt", Worktree: git.Modified}},
				Head: git.Head{Branch: "main"}})
			m.state = state.Apply(m.state, state.DiffOpened{Path: "a.txt",
				Origin: state.WorkingTree(state.SectionUnstaged)})
			m.state = state.Apply(m.state, state.VerbRequested{
				Verb: state.VerbStage, Targets: []string{"a.txt"}, Block: c.block})

			if m.state.Open.Path == "" {
				t.Fatal("no diff was open, so this proves nothing")
			}
			_, cmd := m.handleVerbMsg(verbMsg{finished: state.VerbFinished{
				Verb: state.VerbStage, Applied: 1}})
			if cmd == nil {
				t.Fatal("the verb answered with no command at all")
			}
			// The tab reload always runs. The extra read of the open diff is
			// what tells the two apart, and it is the one that keeps the pane
			// showing what git now holds.
			if got := readsTheDiff(cmd); got != c.reload {
				t.Errorf("the diff was reloaded %v, want %v", got, c.reload)
			}
		})
	}
}

// readsTheDiff runs what the verb answered with and reports whether any of it
// asks git for the open file's diff. The commands run against a directory that
// is not a repository, so the read fails and still says it happened.
func readsTheDiff(cmd tea.Cmd) bool {
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		_, isDiff := msg.(diffMsg)
		return isDiff
	}
	for _, inner := range batch {
		if inner == nil {
			continue
		}
		if readsTheDiff(inner) {
			return true
		}
	}
	return false
}

// The pointer's row picks the side the wheel scrolls, the same way click and
// hover pick it. A notch over the list that moved the diff would make one
// pointer position mean two things.
func TestTheWheelScrollsTheSideThePointerIsOver(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name    string
		overRow func(*Model) int
		list    bool
	}{
		{"over the list", func(m *Model) int { return layout.HeaderRowsOf(m.state) }, true},
		{"over the diff", func(m *Model) int {
			rows, _, _ := m.windowRows()
			return layout.HeaderRowsOf(m.state) + rows
		}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := newParentModel(t)
			m.state = state.Apply(m.state, state.StatusLoaded{
				Rows: manyEntries(80), Head: git.Head{Branch: "main"}})
			m.state = state.Apply(m.state, state.DiffOpened{Path: "f0.txt",
				Origin: state.WorkingTree(state.SectionUnstaged)})
			m.state = state.Apply(m.state, state.DiffLoaded{Diff: manyBlocks(20)})

			_, _, _ = m.handleMouseWheel(tea.MouseWheelMsg{
				Button: tea.MouseWheelDown, Y: c.overRow(m)})

			movedList := m.state.ScrollTop > 0
			// The diff side moves the block the keys act on. The scroll follows
			// only when that block is off the area, so it says nothing here.
			movedDiff := m.state.Open.BlockCursor > 0
			if movedList == movedDiff {
				t.Fatalf("the notch moved list=%v diff=%v; exactly one side scrolls",
					movedList, movedDiff)
			}
			if movedList != c.list {
				t.Errorf("the list moved %v, want %v", movedList, c.list)
			}
		})
	}
}

func manyEntries(n int) []git.Entry {
	rows := make([]git.Entry, n)
	for i := range rows {
		rows[i] = git.Entry{Path: fmt.Sprintf("f%d.txt", i), Worktree: git.Modified}
	}
	return rows
}

func manyBlocks(n int) git.FileDiff {
	blocks := make([]git.Block, n)
	for i := range blocks {
		blocks[i] = git.Block{Header: fmt.Sprintf("@@ -%d +%d @@", i+1, i+1),
			Lines: []string{"+x"}, Hash: fmt.Sprintf("h%d", i)}
	}
	return git.FileDiff{Path: "f0.txt", Blocks: blocks}
}

// Closing a diff with the cursor still on its row has to stick. Hover reopens
// whatever the cursor names, so without remembering which file was closed the
// diff comes back on the next pointer move and the key looks ignored.
func TestHoverDoesNotReopenTheFileThatWasJustClosed(t *testing.T) {
	t.Parallel()
	m := newParentModel(t)
	m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabHistory})
	m.state = state.Apply(m.state, state.HistoryLoaded{Commits: []git.CommitInfo{
		{SHA: "aaa", ShortSHA: "aaa", Subject: "one", Age: "1m"}}})
	m.state = state.Apply(m.state, state.CommitExpanded{SHA: "aaa",
		Files: []git.Entry{{Path: "a.txt", Worktree: git.Modified}}})
	m.state = state.Apply(m.state, state.CursorMoved{By: 1})
	if row, ok := state.CursorRow(m.state); !ok || row.Kind() != state.RowFile {
		t.Fatalf("the cursor is not on a file row, so this proves nothing")
	}

	if cmd := m.followHistoryCursor(); cmd == nil {
		t.Fatal("hovering a file the pane has not opened read no diff")
	}
	m.state = state.Apply(m.state, state.DiffClosed{For: "a.txt"})
	if cmd := m.followHistoryCursor(); cmd != nil {
		t.Error("hover read the diff of the file the reader just closed")
	}

	// Closing one file says nothing about the next one. The refusal is tied to
	// the path that was closed, so moving off it opens diffs again.
	m.state = state.Apply(m.state, state.CommitExpanded{SHA: "aaa", Files: []git.Entry{
		{Path: "a.txt", Worktree: git.Modified},
		{Path: "b.txt", Worktree: git.Modified}}})
	m.state = state.Apply(m.state, state.CursorMoved{By: 1})
	if row, ok := state.CursorRow(m.state); !ok || row.Path() != "b.txt" {
		t.Fatalf("the cursor is not on b.txt, so this proves nothing")
	}
	if cmd := m.followHistoryCursor(); cmd == nil {
		t.Error("hover refused a file other than the one that was closed")
	}
}

// The block cursor steps through the blocks of the open diff. With no diff open
// there is nothing to step through, and moving anyway leaves a block cursor
// pointing into a file the pane is not showing.
func TestTheBlockCursorMovesOnlyThroughAnOpenDiff(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		open func(*Model)
		move bool
	}{
		{"no diff is open", func(*Model) {}, false},
		{"the open diff has no blocks", func(m *Model) {
			m.state = state.Apply(m.state, state.DiffOpened{Path: "f0.txt",
				Origin: state.WorkingTree(state.SectionUnstaged)})
		}, false},
		{"the open diff has blocks", func(m *Model) {
			m.state = state.Apply(m.state, state.DiffOpened{Path: "f0.txt",
				Origin: state.WorkingTree(state.SectionUnstaged)})
			m.state = state.Apply(m.state, state.DiffLoaded{Diff: manyBlocks(5)})
		}, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := newParentModel(t)
			m.state = state.Apply(m.state, state.StatusLoaded{
				Rows: manyEntries(3), Head: git.Head{Branch: "main"}})
			c.open(m)

			before := m.state.Open.BlockCursor
			_, _ = m.moveBlockCursor(1)
			if moved := m.state.Open.BlockCursor != before; moved != c.move {
				t.Errorf("the block cursor moved %v, want %v", moved, c.move)
			}
		})
	}
}

// The footer says how long ago the repository was fetched. A failed read is not
// evidence that no fetch ever happened, so the time it kept stands: zeroing it
// drew "never fetched" three minutes after a fetch that worked.
func TestAFailedFetchKeepsTheTimeOfTheLastGoodOne(t *testing.T) {
	t.Parallel()
	m := newParentModel(t)
	good := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	_, _ = m.handleFetchedMsg(fetchedMsg{at: good})
	if m.fetchedAt != good {
		t.Fatalf("the good read did not record its time: %v", m.fetchedAt)
	}

	_, _ = m.handleFetchedMsg(fetchedMsg{err: errors.New("no network")})
	if m.fetchedAt != good {
		t.Errorf("the failed read moved the time to %v, want %v", m.fetchedAt, good)
	}
	if m.state.Notice == "" || !m.state.NoticeFailed {
		t.Errorf("the failed read said nothing to the reader: %q failed=%v",
			m.state.Notice, m.state.NoticeFailed)
	}
}

// Each watched directory costs a descriptor on macOS and a watch against
// fs.inotify.max_user_watches on Linux, so the count is a budget rather than a
// target. A walk that takes one more than the bound is a walk that does not
// stop where the budget says.
func TestTheWatcherTakesNoMoreDirectoriesThanTheBound(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("opens one descriptor per watched directory")
	}
	root := t.TempDir()
	for i := range maxWatchedDirs + 20 {
		if err := os.Mkdir(filepath.Join(root, fmt.Sprintf("d%05d", i)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Close() }()

	if added := addWorktreeWatch(context.Background(), w, root); added != maxWatchedDirs {
		t.Errorf("the walk took %d directories, want the bound of %d", added, maxWatchedDirs)
	}
}

// The walk watches directories. A file is not one, and it goes past a directory
// it cannot read rather than ending there. The bound test above builds a tree
// of readable directories only, so neither case is drawn by it.
func TestTheWalkWatchesDirectoriesAndKeepsGoingPastOneItCannotRead(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("opens one descriptor per watched directory")
	}
	// zz comes after shut, so reaching it is what says the walk went on.
	build := func(t *testing.T, withShut bool) string {
		t.Helper()
		root := t.TempDir()
		for _, name := range []string{"one", "zz"} {
			if err := os.Mkdir(filepath.Join(root, name), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		for _, name := range []string{"a.txt", "one/b.txt"} {
			if err := os.WriteFile(filepath.Join(root, name), []byte("x\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if withShut {
			shut := filepath.Join(root, "shut")
			if err := os.Mkdir(shut, 0o000); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(shut, 0o755) })
			if _, err := os.ReadDir(shut); err == nil {
				t.Skip("process can read a mode-000 directory")
			}
		}
		return root
	}
	count := func(t *testing.T, root string) int {
		t.Helper()
		w, err := fsnotify.NewWatcher()
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = w.Close() }()
		return addWorktreeWatch(context.Background(), w, root)
	}

	// root, one, zz. The two files are not directories.
	plain := count(t, build(t, false))
	if plain != 3 {
		t.Errorf("a tree of three directories and two files was watched as %d", plain)
	}

	// The unreadable one cannot be watched, and the three that can still are.
	withShut := count(t, build(t, true))
	if withShut != plain {
		t.Errorf("adding a directory that cannot be read changed the count from %d to %d; "+
			"the walk ended there instead of going on", plain, withShut)
	}
}

// A directory made after the watcher started is not watched by it, so the pane
// would miss every file put inside. This adds one when a create event names a
// directory — and only then: a write or a remove names a path that is already
// watched or already gone, and adding on those spends the descriptor budget on
// paths the watcher is holding twice.
func TestOnlyACreatedDirectoryIsAddedToTheWatcher(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("opens one descriptor per watched directory")
	}
	root := t.TempDir()
	made := filepath.Join(root, "made")
	if err := os.Mkdir(made, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "a.txt")
	if err := os.WriteFile(file, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct {
		name  string
		ev    fsnotify.Event
		added bool
	}{
		{"a directory was created", fsnotify.Event{Name: made, Op: fsnotify.Create}, true},
		{"a directory was written to", fsnotify.Event{Name: made, Op: fsnotify.Write}, false},
		{"a directory was removed", fsnotify.Event{Name: made, Op: fsnotify.Remove}, false},
		{"a file was created", fsnotify.Event{Name: file, Op: fsnotify.Create}, false},
		{"a path that is not there was created",
			fsnotify.Event{Name: filepath.Join(root, "gone"), Op: fsnotify.Create}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			w, err := fsnotify.NewWatcher()
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = w.Close() }()

			maybeWatchNewDir(w, c.ev)
			if got := len(w.WatchList()) == 1; got != c.added {
				t.Errorf("the watcher holds %v, want it to hold the path = %v",
					w.WatchList(), c.added)
			}
		})
	}
}

// .git is watched by beginWatch as one directory. The walk has to leave the
// tree under it alone: objects/ and refs/ hold a directory per fanout byte and
// change on every write, so a repack would spend the whole budget on names that
// change nothing the pane draws. The name itself is skipped along with what is
// under it, which is one test because one condition answers for both.
func TestTheWalkWatchesNothingUnderDotGit(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("opens one descriptor per watched directory")
	}
	root := t.TempDir()
	for _, dir := range []string{
		".git", ".git/objects", ".git/objects/ab", ".git/refs", ".git/refs/heads",
		"app", "app/inner",
	} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Close() }()

	// root, app, app/inner.
	if added := addWorktreeWatch(context.Background(), w, root); added != 3 {
		t.Errorf("the walk took %d directories, want 3: %v", added, w.WatchList())
	}
	for _, path := range w.WatchList() {
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			t.Fatal(relErr)
		}
		if rel == ".git" || strings.HasPrefix(rel, ".git"+string(filepath.Separator)) {
			t.Errorf("the walk watched %q, which beginWatch already holds", rel)
		}
	}
}
