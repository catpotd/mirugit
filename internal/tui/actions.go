package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
)

func (m *Model) open(path string) (tea.Model, tea.Cmd) {
	if path == "" {
		return m, nil
	}
	if state.Facts[m.state.Tab].DiffPeekWhenClosed &&
		!layout.CanPeekDiff(m.state.Height-layout.HeaderRowsOf(m.state)-1) {
		m.state = state.Apply(m.state, state.DiffBlocked{})
		return m, nil
	}
	save := m.finishReading()
	// The diff closes two ways: the reader presses esc, or the list grows past
	// what the pane holds. d has to reopen it in both cases, and only the first
	// was reaching here.
	peek := m.peekFor(path)
	sec := m.sectionFor(path)
	m.state = state.Apply(m.state, state.DiffOpened{Path: path, Peek: peek,
		Origin: state.WorkingTree(sec)})
	return m, tea.Batch(save, loadDiff(m.startDiffRead(), m.dir, path, sec == state.SectionStaged))
}

// Closing a diff is what finishes a file, so a file scrolled past without being
// opened keeps its unread mark.
func (m *Model) finishReading() tea.Cmd {
	if m.state.Open.Path == "" || len(m.state.Open.Diff.Blocks) == 0 {
		return nil
	}
	path := m.state.Open.Path
	m.recordShownBlocks()
	shown := make([]string, 0, len(m.shown.hashes))
	for hash := range m.shown.hashes {
		shown = append(shown, hash)
	}
	all := make([]string, len(m.state.Open.Diff.Blocks))
	for i, b := range m.state.Open.Diff.Blocks {
		all[i] = b.Hash
	}
	m.read.MarkShownIn(m.state.Open.Origin, path, shown, all)
	m.shown.key, m.shown.hashes = "", nil
	return m.saveRead()
}

func (m *Model) cursorPath() string {
	row, ok := state.CursorRow(m.state)
	if !ok {
		return ""
	}
	if row.Kind() == state.RowStash || row.Kind() == state.RowWorktree {
		return ""
	}
	// A file on no side of this index is one a commit, a stash or another
	// worktree holds. The verbs that take this path act on the working tree.
	if row.Kind() == state.RowFile && row.Section() == state.SectionNone {
		return ""
	}
	return row.Path()
}

func (m *Model) nextShownTab() state.Tab {
	for step := 1; step <= int(state.TabCount); step++ {
		candidate := state.Tab((int(m.state.Tab) + step) % int(state.TabCount))
		if m.tabShown(candidate) {
			return candidate
		}
	}
	return m.state.Tab
}

func (m *Model) foldAtCursor() (tea.Model, tea.Cmd) {
	if row, ok := state.CursorRow(m.state); ok && row.Kind() == state.RowDirectory {
		m.state = state.Apply(m.state,
			state.DirectoryFolded{Path: row.DirPath(), Section: row.Section()})
	}
	return m, nil
}

func (m *Model) unfoldAtCursor() (tea.Model, tea.Cmd) {
	if row, ok := state.CursorRow(m.state); ok && row.Kind() == state.RowDirectory &&
		state.FoldedIn(m.state, row.Section(), row.DirPath()) {
		m.state = state.Apply(m.state,
			state.DirectoryFolded{Path: row.DirPath(), Section: row.Section()})
	}
	return m, nil
}

// selectAtCursor is what space does, reached from the command list so the
// mouse can do everything the keyboard can.
func (m *Model) selectAtCursor() (tea.Model, tea.Cmd) {
	if row, ok := state.CursorRow(m.state); ok {
		switch row.Kind() {
		case state.RowSectionHeading:
			m.state = state.Apply(m.state, state.SectionToggled{Section: row.Section()})
			return m, nil
		case state.RowDirectory:
			m.state = state.Apply(m.state, state.DirectorySelectionToggled{
				Path: row.DirPath(), Section: row.Section(),
			})
			return m, nil
		case state.RowFile, state.RowCommit, state.RowStash, state.RowWorktree:
			// A file, commit, stash or worktree row selects the row itself,
			// which is what the path lookup below does.
		}
	}
	if path := m.cursorPath(); path != "" {
		m.state = state.Apply(m.state,
			state.SelectionToggled{Path: path, Section: m.cursorSection()})
	}
	return m, nil
}

// tabShown reports whether the bar draws a tab, which is the same as whether
// the reader can go there: landing on a tab the bar omits leaves the underline
// nowhere and the reader with no way to read their own position.
func (m *Model) tabShown(tab state.Tab) bool {
	for _, t := range layout.TabsOf(m.state, m.read) {
		if t.Logical == tab {
			return true
		}
	}
	return false
}

// peekFor says whether opening path has to take rows from the list. A list
// longer than the pane leaves a diff nothing in the ordinary split, and a diff
// the reader closed reopens the way they left it.
//
// Every path that opens a diff asks here. Answering only where the key is
// pressed dropped the answer on every cursor move, and a pane with more changed
// files than rows showed a diff once and then never again.
func (m *Model) peekFor(path string) bool {
	if m.state.Open.Closed && path == m.state.Open.ClosedFor {
		return true
	}
	return m.diffHasNoRoom()
}

func (m *Model) diffHasNoRoom() bool {
	lines := layout.TabListLineCount(m.state, m.read, m.render)
	return layout.DiffNeedsPeek(lines, m.state.Height-layout.HeaderRowsOf(m.state)-1)
}

// cursorSection is which side of the list the cursor is on. A path staged and
// edited again has a row on both, and they tick apart.
func (m *Model) cursorSection() state.Section {
	row, ok := state.CursorRow(m.state)
	if !ok {
		return state.SectionUnstaged
	}
	return row.Section()
}

func (m *Model) switchTab(tab state.Tab) (tea.Model, tea.Cmd) {
	m.state = state.Apply(m.state, state.TabChanged{Tab: tab})
	// The reads the tab being left asked for answer about rows this pane is no
	// longer drawing. On a repository slow enough for one of them to matter,
	// they were still running while the tab the reader chose waited behind them.
	m.startTabReads()
	return m, m.reloadCurrentTab()
}

func (m *Model) moveCursor(k tea.KeyPressMsg, by int) (tea.Model, tea.Cmd) {
	from := m.state.Cursor
	m.state = state.Apply(m.state, state.CursorMoved{By: by})
	if k.Mod&tea.ModShift != 0 {
		m.state = state.Apply(m.state, state.SelectionRangeExtended{From: from, To: m.state.Cursor})
	}
	if by > 0 && state.Facts[m.state.Tab].HistoryPaging && m.state.Cursor == len(m.state.Rows)-1 {
		return m, m.requestHistoryMore()
	}
	return m, nil
}

func (m *Model) requestHistoryMore() tea.Cmd {
	if !state.Facts[m.state.Tab].HistoryPaging || !m.state.History.HasMore || m.state.History.LoadingMore || len(m.state.History.Commits) == 0 {
		return nil
	}
	offset := len(m.state.History.Commits)
	newestSHA := m.state.History.Commits[0].SHA
	m.state = state.Apply(m.state, state.HistoryMoreRequested{})
	return loadHistoryMore(m.readsForTab(), m.dir, offset, newestSHA)
}

func (m *Model) discardForTab() (tea.Model, tea.Cmd) {
	switch state.Facts[m.state.Tab].DiscardVerb {
	case state.VerbNameRemove:
		return m.requestWorktreeRemove()
	case state.VerbNameDrop:
		return m.requestStashDrop()
	}
	return m.requestDiscard()
}

func (m *Model) diffForTab() (tea.Model, tea.Cmd) {
	row, ok := state.CursorRow(m.state)
	if !ok {
		return m.open(m.cursorPath())
	}
	return m.openRow(m.state.Cursor, layout.Target{
		Path:   row.Path(),
		Commit: m.expandedCommitSHA(),
	})
}

// openRow answers what opening the row at index does. The key path and the
// mouse path both come here, so a tab reached by one is reached by the other.
// t carries what the frame recorded: the mouse has a path from the row it drew,
// which can differ from Rows[index] after a reload shortened the list.
func (m *Model) openRow(index int, t layout.Target) (tea.Model, tea.Cmd) {
	row, ok := state.RowAt(m.state, index)
	if !ok {
		return m, nil
	}
	switch row.Kind() {
	case state.RowCommit:
		if t.Commit == "" {
			return m.requestHistoryDiff()
		}
		return m.toggleCommit(t.Commit)
	case state.RowFile:
		return m.openFileRow(t.Path)
	case state.RowStash, state.RowWorktree:
		return m.toggleExpanded()
	case state.RowSectionHeading, state.RowDirectory:
		// A heading and a directory fold through their own keys.
	}
	return m, nil
}

// toggleExpanded opens the row under the cursor, or closes it if it is the one
// already open.
func (m *Model) toggleExpanded() (tea.Model, tea.Cmd) {
	return m.toggleRow(commandsFor(m.state.Tab).KeyAtCursor(m))
}

// toggleRow opens the named row, or closes it if it is the one already open.
// Closing is a state change and nothing else; history used to ask git for it
// and read the answer to find out it had nothing to read, and the two tabs
// beside it had no way to close a row at all.
func (m *Model) toggleRow(key string) (tea.Model, tea.Cmd) {
	if key == "" {
		return m, nil
	}
	if key == m.expandedKey() {
		m.state = state.Apply(m.state, state.RowCollapsed{})
		m.asked.expandRow = ""
		return m, nil
	}
	return m, commandsFor(m.state.Tab).ExpandRow(m, key)
}

// openFileRow opens one file. Which side it reads from is a fact about the tab,
// not a decision each caller repeats.
func (m *Model) openFileRow(path string) (tea.Model, tea.Cmd) {
	switch state.Facts[m.state.Tab].OpenFile {
	case state.OpenFileFromCommit:
		return m, m.openCommitFile(path, false, true)
	case state.OpenFileFromParent:
		return m.openNestedFile(path)
	case state.OpenFileFromWorkingTree:
	}
	return m.open(path)
}

// openNestedFile opens a file listed under a stash or a worktree row. Both read
// a diff the working tree does not hold, so neither goes through open().
func (m *Model) openNestedFile(path string) (tea.Model, tea.Cmd) {
	if path == "" {
		return m, nil
	}
	dir, cmd := m.nestedFileDiff(path)
	if cmd == nil || dir == "" {
		return m, nil
	}
	// The parent row is what the diff belongs to, so the marks are filed under
	// it: two stashes holding the same path are two files to read.
	parent, _, ok := state.Facts[m.state.Tab].ParentRow(m.state)
	if !ok {
		return m, nil
	}
	save := m.finishReading()
	m.state = state.Apply(m.state, state.DiffOpened{Path: path, Peek: m.peekFor(path),
		Origin: state.FromParent(parent.Path())})
	return m, tea.Batch(save, cmd)
}

// expandedCommitSHA names the commit the cursor row belongs to, or "" when the
// cursor is not on a commit row.
func (m *Model) expandedCommitSHA() string {
	row, ok := state.CursorRow(m.state)
	if !ok {
		return ""
	}
	if row.Kind() != state.RowCommit {
		return ""
	}
	return row.Commit().SHA
}

func (m *Model) cursorToSectionHeading(section int) {
	for i, row := range m.state.Rows {
		if row.Kind() == state.RowSectionHeading && int(row.Section()) == section {
			m.state = state.Apply(m.state, state.CursorMovedTo{Row: i})
			break
		}
	}
}
