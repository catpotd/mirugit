package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
)

// currentCommit reports the commit the cursor is in — the commit row itself, or
// the one a file row belongs to. The second result says whether there is one,
// because a Row carries a value rather than a pointer and a zero CommitInfo is
// not distinguishable from a missing one.
func (m *Model) currentCommit() (git.CommitInfo, bool) {
	commit, _, ok := m.currentCommitAt()
	return commit, ok
}

// currentCommitAt also answers where the commit is in the history, which is
// what decides whether undo is offered. Counting commit rows above the cursor
// instead answers one for a file row of the tip commit, and undo is only
// offered at zero.
func (m *Model) currentCommitAt() (git.CommitInfo, int, bool) {
	if m.state.Tab != state.TabHistory {
		return git.CommitInfo{}, -1, false
	}
	row, at, ok := state.Facts[m.state.Tab].ParentRow(m.state)
	if !ok {
		return git.CommitInfo{}, -1, false
	}
	return row.Commit(), state.CommitIndexAt(m.state, at), true
}

func (m *Model) requestCopySHA() (tea.Model, tea.Cmd) {
	commit, index, ok := m.currentCommitAt()
	if !ok {
		return m, nil
	}
	if !state.CommitVerbApplies(state.VerbNameSHA, commit, index,
		state.HostTools{Clipboard: m.render.Clipboard}) {
		return m, nil
	}
	return m, runCopySHA(m.clipboardCommand(), commit.ShortSHA)
}

func (m *Model) requestOpenCommit() (tea.Model, tea.Cmd) {
	commit, index, ok := m.currentCommitAt()
	if !ok {
		return m, nil
	}
	_, openable := state.HTTPSRemoteBase(m.state.History.RemoteURL)
	if !state.CommitVerbApplies(state.VerbNameOpen, commit, index,
		state.HostTools{Browser: m.render.Browser && openable}) {
		return m, nil
	}
	url, ok := layout.RemoteCommitURL(m.state.History.RemoteURL, commit.SHA)
	if !ok {
		return m, nil
	}
	return m, runOpenCommit(m.browserCommand(), url)
}

func (m *Model) followHistoryCursor() tea.Cmd {
	// The cursor row's own path, not cursorPath: that one answers "" for a file
	// on no side of this index, which is every file a commit holds.
	row, ok := state.CursorRow(m.state)
	if !ok || row.Kind() != state.RowFile {
		return nil
	}
	path := row.Path()
	if path == "" {
		return nil
	}
	if m.state.Open.Closed && path == m.state.Open.ClosedFor {
		return nil
	}
	if path == m.state.Open.Path {
		return nil
	}
	return m.openCommitFile(path, m.state.Open.Peek, m.state.Open.PrioritizeDiff)
}

// openCommitFile opens a file as the commit that holds it changed it. Three
// callers wrote these five lines: the key, the mouse, and the cursor following
// into an expanded commit. The three had to keep agreeing about which origin
// the diff carries, and a diff opened with the wrong one reads the working tree.
func (m *Model) openCommitFile(path string, peek, prioritizeDiff bool) tea.Cmd {
	sha := m.commitSHAFor(path)
	if sha == "" {
		return nil
	}
	m.state = state.Apply(m.state, state.DiffOpened{
		Path: path, Peek: peek, PrioritizeDiff: prioritizeDiff, Origin: state.FromCommit(sha)})
	return loadCommitDiff(m.startDiffRead(), m.dir, sha, path)
}

func (m *Model) commitSHAFor(path string) string {
	parent, ok := state.ParentOfFile(m.state.Rows, path, state.RowCommit)
	if !ok {
		return ""
	}
	return parent.Commit().SHA
}

func (m *Model) toggleCommit(sha string) (tea.Model, tea.Cmd) {
	return m.toggleRow(sha)
}

func (m *Model) requestHistoryUndo() (tea.Model, tea.Cmd) {
	if m.state.Tab != state.TabHistory {
		return m, nil
	}
	commit, index, ok := m.currentCommitAt()
	if !ok {
		return m, nil
	}
	if !state.CommitVerbApplies(state.VerbNameUncommit, commit, index, state.HostTools{}) {
		return m, nil
	}
	return m, runUndoCommit(m.rootContext(), m.dir)
}

func (m *Model) requestHistoryDiff() (tea.Model, tea.Cmd) {
	row, ok := state.CursorRow(m.state)
	if !ok {
		return m, nil
	}
	switch row.Kind() {
	case state.RowCommit:
		return m.toggleCommit(row.Commit().SHA)
	case state.RowFile:
		return m, m.openCommitFile(row.Path(), false, true)
	case state.RowStash, state.RowWorktree, state.RowSectionHeading, state.RowDirectory:
	}
	// The history tab holds commits and the files each one touched; no other
	// row kind is built for it.
	return m, nil
}

// expandCommit reads what the commit named by sha holds. The caller says which
// commit; this does not look at the cursor.
func (m *Model) expandCommit(sha string) tea.Cmd {
	if m.state.Tab != state.TabHistory || sha == "" {
		return nil
	}
	return loadCommitFiles(m.readsForTab(), m.dir, sha)
}

func (m *Model) commitSHAAtCursor() string {
	commit, _, ok := m.currentCommitAt()
	if !ok {
		return ""
	}
	return commit.SHA
}
