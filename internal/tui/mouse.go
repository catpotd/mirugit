package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
)

// A click arms the row it lands on. Nothing else does: the pointer moving over
// the pane leaves the armed row where the reader put it, so the row a click
// lands on is the row the click was aimed at.
func (m *Model) click(t layout.Target, mod tea.KeyMod) (tea.Model, tea.Cmd) {
	// The commit box takes every key while it has the keyboard, so a verb the
	// mouse starts there opens a confirmation the footer does not draw — it
	// names the box first — and the y that would answer it goes into the
	// message. A click away from the box is the reader leaving it, and leaving
	// is all it does: running the verb as well would spend a click the reader
	// meant for the text.
	if m.state.Changes.MessageFocused && t.Kind != layout.TargetCommitBox {
		m.state = state.Apply(m.state, state.MessageBlurred{})
		return m, nil
	}
	// Row verbs still show the pre-confirmation list; acting on them would hit a
	// target the reader never confirmed. Match the key path: cancel only.
	if _, pending := state.PendingConfirmation(m.state); pending {
		return m.cancelConfirmation()
	}
	switch t.Kind {
	case layout.TargetHelp:
		m.state = state.Apply(m.state, state.HelpOpened{})
		return m, nil
	case layout.TargetHelpLine:
		return m.helpLineClick(t)
	case layout.TargetCheckbox:
		return m.checkboxClick(t, mod)
	case layout.TargetSectionHeading:
		m.cursorToSectionHeading(int(t.Section))
		return m, nil
	case layout.TargetFile:
		return m.fileClick(t, mod)
	case layout.TargetBlock:
		m.state = state.Apply(m.state, state.BlockCursorSet{Block: t.Block})
	case layout.TargetDirectory:
		return m.directoryClick(t)
	case layout.TargetTab:
		if t.Tab == m.state.Tab {
			return m, nil
		}
		return m.switchTab(t.Tab)
	case layout.TargetCommitBox:
		m.state = state.Apply(m.state, state.MessageFocused{})
	case layout.TargetCommit:
		return m.requestCommit()
	case layout.TargetButton:
		if t.Verb == state.VerbNameSync {
			return m.requestSync()
		}
	case layout.TargetVerb:
		return m.verbClick(t)
	case layout.TargetNone:
		// A click on a cell no region covers reaches here only off a row;
		// handleMouseClick hands a click on a row to that row's own target.
	}
	return m, nil
}

func (m *Model) helpLineClick(t layout.Target) (tea.Model, tea.Cmd) {
	m.state = state.Apply(m.state, state.HelpClosed{})
	return m.runVerb(t.Verb)
}

func (m *Model) checkboxClick(t layout.Target, mod tea.KeyMod) (tea.Model, tea.Cmd) {
	if t.Section != state.SectionNone {
		m.state = state.Apply(m.state, state.SectionToggled{Section: t.Section})
		return m, nil
	}
	// t.Row names a row of the frame drawn last. A reload from the file watcher
	// can shorten Rows before the click arrives.
	row, ok := state.RowAt(m.state, t.Row)
	if !ok {
		return m, nil
	}
	if row.Kind() == state.RowDirectory {
		m.state = state.Apply(m.state, state.DirectorySelectionToggled{
			Path: t.Path, Section: row.Section()})
		return m, nil
	}
	if mod&tea.ModShift != 0 {
		m.state = state.Apply(m.state, state.SelectionRangeExtended{From: m.state.Cursor, To: t.Row})
	} else {
		m.state = state.Apply(m.state, state.SelectionToggled{
			Path: t.Path, Section: row.Section()})
	}
	return m, nil
}

func (m *Model) fileClick(t layout.Target, mod tea.KeyMod) (tea.Model, tea.Cmd) {
	row, ok := state.RowAt(m.state, t.Row)
	if !ok {
		return m, nil
	}
	if mod&tea.ModShift != 0 {
		m.state = state.Apply(m.state, state.SelectionRangeExtended{From: m.state.Cursor, To: t.Row})
		m.state = state.Apply(m.state, state.CursorMovedTo{Row: t.Row})
		if row.Kind() != state.RowCommit {
			return m, nil
		}
	} else {
		m.state = state.Apply(m.state, state.CursorMovedTo{Row: t.Row})
	}
	// A click on a commit row with no SHA in the frame does nothing; the key
	// path reaches requestHistoryDiff instead, which reads the cursor row.
	if row.Kind() == state.RowCommit && t.Commit == "" {
		return m, nil
	}
	return m.openRow(t.Row, t)
}

func (m *Model) directoryClick(t layout.Target) (tea.Model, tea.Cmd) {
	row, ok := state.RowAt(m.state, t.Row)
	if !ok {
		return m, nil
	}
	// The section comes from the row rather than the target: one path shows
	// under both headings, and each copy folds on its own.
	sec := row.Section()
	m.state = state.Apply(m.state, state.CursorMovedTo{Row: t.Row})
	m.state = state.Apply(m.state,
		state.DirectoryFolded{Path: t.Path, Section: sec})
	return m, nil
}

func (m *Model) verbClick(t layout.Target) (tea.Model, tea.Cmd) {
	if t.Verb == state.VerbNameStage && t.Block >= 0 {
		return m.requestBlockStage(t.Block)
	}
	return m.runVerb(t.Verb)
}
