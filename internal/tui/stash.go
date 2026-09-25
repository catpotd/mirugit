package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

func (m *Model) markStashCursorRead() tea.Cmd {
	if m.state.Tab != state.TabStashed {
		return nil
	}
	stash, ok := m.stashAtCursor()
	if !ok {
		return nil
	}
	m.read.MarkStash(stash.SHA)
	return m.saveRead()
}

// stashStillAt reports whether the list still holds that stash where the read
// was issued for it. A verb the reader has answered but whose list has not
// arrived counts as moved: the answer is on its way, and the list on hand still
// names the stashes as they were.
func (m *Model) stashStillAt(index int, sha string) bool {
	if m.state.Stashed.RefsStale {
		return false
	}
	rows := m.state.Stashed.Stashes
	return index >= 0 && index < len(rows) && rows[index].SHA == sha
}

func (m *Model) currentStash() (git.StashRow, bool) {
	return m.stashAtCursor()
}

func (m *Model) stashAtCursor() (git.StashRow, bool) {
	row, _, ok := state.Facts[state.TabStashed].ParentRow(m.state)
	if !ok {
		return git.StashRow{}, false
	}
	return row.Stash(), true
}

func (m *Model) stashSHAFor(path string) string {
	parent, ok := state.ParentOfFile(m.state.Rows, path, state.RowStash)
	if !ok {
		return ""
	}
	return parent.Stash().SHA
}

func (m *Model) requestStashRestore() (tea.Model, tea.Cmd) {
	stash, ok := m.currentStash()
	if !ok || !state.StashVerbApplies(state.VerbNameRestore, stash.Status) {
		return m, nil
	}
	// The refs move when git runs, not when it answers. A read issued in
	// between — the five-second poll is enough — would ask for a stash that
	// has moved, and the answer applies this again as a safety net.
	m.state = state.Apply(m.state, state.StashRefsMoved{})
	return m, runStashRestore(m.rootContext(), m.dir, stash.Ref, stash.FileCount)
}

// stashBranchName picks a name git will accept. The branch the stash was taken
// from is the one name guaranteed to exist, and this route only matters because
// it is the one that cannot fail.
// stashBranchBase is the name the new branch is asked for. git stash branch
// refuses a name that exists, and the stash's own branch always does, so the
// suffix is what makes the ask answerable at all. Whether that name is free is
// a question for the repository, answered in the command.
func (m *Model) stashBranchBase() string {
	if stash, ok := m.currentStash(); ok && stash.Branch != "" {
		return stash.Branch + "-stash"
	}
	return "stash"
}

func (m *Model) requestStashBranch() (tea.Model, tea.Cmd) {
	stash, ok := m.currentStash()
	if !ok || !state.StashVerbApplies(state.VerbNameBranch, stash.Status) {
		return m, nil
	}
	// See requestStashRestore: the window opens when git is issued.
	m.state = state.Apply(m.state, state.StashRefsMoved{})
	return m, runStashBranch(m.rootContext(), m.dir, stash.Ref, m.stashBranchBase())
}

func (m *Model) requestStashDrop() (tea.Model, tea.Cmd) {
	stash, ok := m.currentStash()
	if !ok || !state.StashVerbApplies(state.VerbNameDrop, stash.Status) {
		return m, nil
	}
	confirm := state.StashDropConfirm{SHA: stash.SHA, Message: stash.Message}
	m.state = state.Apply(m.state, state.StashDropConfirmationShown{Confirm: confirm})
	return m, nil
}

func (m *Model) confirmStashDrop() (tea.Model, tea.Cmd) {
	if m.state.Stashed.DropConfirm == nil {
		return m, nil
	}
	sha := m.state.Stashed.DropConfirm.SHA
	m.state = state.Apply(m.state, state.StashDropConfirmed{})
	return m, runStashDrop(m.rootContext(), m.dir, sha)
}

func (m *Model) cancelStashDrop() (tea.Model, tea.Cmd) {
	m.state = state.Apply(m.state, state.StashDropCancelled{})
	return m, nil
}

// loadCursorStashFiles reads what the stash under the cursor holds. Only that
// one is read, because listing every stash's contents costs a git call each.
// expandStash reads what the stash named by ref holds. The caller says which
// stash; this does not look at the cursor.
func (m *Model) expandStash(ref string) tea.Cmd {
	if m.state.Tab != state.TabStashed || ref == "" {
		return nil
	}
	if m.state.Stashed.DropConfirm != nil || m.state.Stashed.RefsStale {
		return nil
	}
	if ref == m.asked.expandRow {
		return nil
	}
	dir := m.dir
	m.asked.expandRow = ref
	ctx := m.readsForTab()
	return wanted(ctx, func() tea.Msg {
		files, err := git.StashFiles(ctx, dir, ref)
		return stashFilesMsg{ref: ref, files: files, err: err}
	})
}

func (m *Model) stashRefAtCursor() string {
	stash, ok := m.stashAtCursor()
	if !ok {
		return ""
	}
	return stash.Ref
}
