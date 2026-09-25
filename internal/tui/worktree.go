package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

func (m *Model) markWorktreeCursorRead() tea.Cmd {
	if m.state.Tab != state.TabWorktrees {
		return nil
	}
	row, ok := m.worktreeAtCursor()
	if !ok || row.SHA == "" {
		return nil
	}
	m.read.MarkWorktree(row.Path, row.SHA)
	return m.saveRead()
}

func (m *Model) currentWorktree() (git.WorktreeRow, bool) {
	return m.worktreeAtCursor()
}

func (m *Model) worktreeAtCursor() (git.WorktreeRow, bool) {
	row, _, ok := state.Facts[state.TabWorktrees].ParentRow(m.state)
	if !ok {
		return git.WorktreeRow{}, false
	}
	return row.Worktree(), true
}

func (m *Model) worktreeDirFor(path string) string {
	parent, ok := state.ParentOfFile(m.state.Rows, path, state.RowWorktree)
	if !ok {
		return ""
	}
	return parent.Worktree().Path
}

func worktreeFileStaged(row state.Row) bool {
	e := row.Entry()
	return e.Worktree == git.Unchanged && e.Index != git.Unchanged && e.Index != git.Unmerged
}

func (m *Model) requestWorktreeGo() (tea.Model, tea.Cmd) {
	row, ok := m.currentWorktree()
	if !ok || !state.WorktreeVerbApplies(state.VerbNameGo, row, row.Path == m.state.Worktrees.Here) {
		return m, nil
	}
	// Where the pane is now is said once the rebind arrived: saying it here
	// would name a worktree the pane failed to move to.
	save := m.markWorktreeCursorRead()
	return m, tea.Batch(save, m.rebindTo(row.Path))
}

func (m *Model) requestWorktreeRemove() (tea.Model, tea.Cmd) {
	row, ok := m.currentWorktree()
	if !ok || !state.WorktreeVerbApplies(state.VerbNameRemove, row, row.Path == m.state.Worktrees.Here) {
		return m, nil
	}
	m.state = state.Apply(m.state, state.WorktreeRemoveConfirmationShown{
		Confirm: state.WorktreeRemoveConfirm{Path: row.Path, Name: row.Name}})
	return m, nil
}

func (m *Model) confirmWorktreeRemove() (tea.Model, tea.Cmd) {
	if m.state.Worktrees.RemoveConfirm == nil {
		return m, nil
	}
	path := m.state.Worktrees.RemoveConfirm.Path
	m.state = state.Apply(m.state, state.WorktreeRemoveConfirmed{})
	return m, runWorktreeRemove(m.rootContext(), m.startDir, path)
}

func (m *Model) cancelWorktreeRemove() (tea.Model, tea.Cmd) {
	m.state = state.Apply(m.state, state.WorktreeRemoveCancelled{})
	return m, nil
}

// expandWorktree reads what the tree at path has changed. The caller says which
// tree; this does not look at the cursor.
func (m *Model) expandWorktree(path string) tea.Cmd {
	if m.state.Tab != state.TabWorktrees || path == "" {
		return nil
	}
	if path == m.asked.expandRow {
		return nil
	}
	m.asked.expandRow = path
	ctx := m.readsForTab()
	return wanted(ctx, func() tea.Msg {
		files, err := git.WorktreeFiles(ctx, path)
		return worktreeFilesMsg{path: path, files: files, err: err}
	})
}

func (m *Model) worktreePathAtCursor() string {
	row, ok := m.worktreeAtCursor()
	if !ok {
		return ""
	}
	return row.Path
}
