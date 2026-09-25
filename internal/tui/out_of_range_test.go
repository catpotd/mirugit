package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
)

// Twenty-three places check that a row index is inside the list before reading
// it, and a mutation run showed four of them could be broken without a test
// noticing. The guards exist because the list moves: a reload from the watcher
// shortens Rows between the frame a click was drawn on and the click arriving,
// and Issue #288 was a click on a row that had gone.
//
// These two sweeps drive every verb and every clickable kind at an index the
// list does not have. Nothing may panic, and nothing may act.

func hostileCursors(rows int) []int {
	return []int{-1, -7, rows, rows + 1, rows + 99}
}

func modelForRange(t *testing.T, rows []state.Row, cursor int) *Model {
	t.Helper()
	s := state.State{Width: 80, Height: 24, Tab: state.TabChanges, Rows: rows, Cursor: cursor}
	s.Changes.Folded = map[string]bool{}
	s.Changes.Selected = map[string]bool{}
	s.Changes.Stale = map[string]bool{}
	m := &Model{state: s, render: layout.Renderer{Clipboard: true, Browser: true},
		read: emptyRead(), dir: t.TempDir()}
	m.probe.settled = true
	return m
}

func rangeTestRows() []state.Row {
	return []state.Row{
		state.SectionHeadingRow(state.SectionUnstaged),
		state.DirectoryRow("dir", state.SectionUnstaged),
		state.FileRow(git.Entry{Path: "dir/a.txt", Worktree: git.Modified}, state.SectionUnstaged),
	}
}

func TestEveryVerbSurvivesACursorOutsideTheList(t *testing.T) {
	t.Parallel()
	for _, tab := range []state.Tab{
		state.TabChanges, state.TabHistory, state.TabStashed, state.TabWorktrees,
	} {
		for _, rows := range [][]state.Row{nil, rangeTestRows()} {
			for _, cursor := range hostileCursors(len(rows)) {
				for _, name := range state.AllVerbNames {
					m := modelForRange(t, rows, cursor)
					m.state = state.Apply(m.state, state.TabChanged{Tab: tab})
					m.state.Cursor = cursor
					m.state.Rows = rows
					// A panic here fails the test by unwinding it, which is the
					// assertion: a verb may do nothing, but it may not crash.
					_, _ = m.runVerb(name)
				}
			}
		}
	}
}

func TestEveryClickableKindSurvivesARowOutsideTheList(t *testing.T) {
	t.Parallel()
	kinds := []layout.TargetKind{
		layout.TargetNone, layout.TargetFile, layout.TargetDirectory,
		layout.TargetSectionHeading, layout.TargetTab, layout.TargetBlock,
		layout.TargetButton, layout.TargetCommitBox, layout.TargetCommit,
		layout.TargetCheckbox, layout.TargetVerb, layout.TargetHelp,
		layout.TargetHelpLine,
	}
	for _, rows := range [][]state.Row{nil, rangeTestRows()} {
		for _, row := range hostileCursors(len(rows)) {
			for _, kind := range kinds {
				m := modelForRange(t, rows, 0)
				t.Run("", func(t *testing.T) {
					_, _ = m.click(layout.Target{
						Kind: kind, Row: row, Path: "dir/a.txt",
						Section: state.SectionUnstaged, Verb: state.VerbNameStage,
						Block: row,
					}, 0)
				})
			}
		}
	}
}

// The cursor is what a key acts on, so the same sweep has to reach the key
// handler rather than only the verb behind it.
func TestEveryKeySurvivesACursorOutsideTheList(t *testing.T) {
	t.Parallel()
	keys := []tea.KeyPressMsg{
		{Code: 'j'}, {Code: 'k'}, {Code: ' '}, {Code: 's'}, {Code: 'u'},
		{Code: 'x'}, {Code: 'z'}, {Code: 'd'}, {Code: 'r'}, {Code: 'n'},
		{Code: 'y'}, {Code: 'o'}, {Code: 'p'}, {Code: 'b'}, {Code: 'g'},
		{Code: 'c'}, {Code: 'a'}, {Code: 'R'}, {Code: 'f'}, {Code: 'S'},
		{Code: tea.KeyEnter}, {Code: tea.KeyEsc}, {Code: tea.KeyLeft},
		{Code: tea.KeyRight}, {Code: '?'},
	}
	for _, rows := range [][]state.Row{nil, rangeTestRows()} {
		for _, cursor := range hostileCursors(len(rows)) {
			for _, k := range keys {
				m := modelForRange(t, rows, cursor)
				_, _ = m.key(k)
			}
		}
	}
}
