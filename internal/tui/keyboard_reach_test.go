package tui

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
)

// A terminal that sends no mouse events, or a reader who does not use one, had
// j and k and nothing else: a hundred rows took a hundred presses while the
// wheel took a few notches.
func TestTheKeyboardReachesEveryRowOfALongList(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		key  tea.KeyPressMsg
		from int
		want func(m *Model, listRows int) int
	}{
		{"down arrow", tea.KeyPressMsg{Code: tea.KeyDown}, 5,
			func(*Model, int) int { return 6 }},
		{"up arrow", tea.KeyPressMsg{Code: tea.KeyUp}, 5,
			func(*Model, int) int { return 4 }},
		{"page down", tea.KeyPressMsg{Code: tea.KeyPgDown}, 0,
			func(_ *Model, rows int) int { return rows }},
		{"page up from the bottom", tea.KeyPressMsg{Code: tea.KeyPgUp}, 40,
			func(_ *Model, rows int) int { return 40 - rows }},
		{"home", tea.KeyPressMsg{Code: tea.KeyHome}, 40,
			func(*Model, int) int { return 0 }},
		{"end", tea.KeyPressMsg{Code: tea.KeyEnd}, 0,
			func(m *Model, _ int) int { return len(m.state.Rows) - 1 }},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := longListModel(t)
			m.state = state.Apply(m.state, state.CursorMoved{By: c.from - m.state.Cursor})
			if m.state.Cursor != c.from {
				t.Fatalf("the cursor did not start at %d: %d", c.from, m.state.Cursor)
			}
			listRows := m.listRowsNow()
			if listRows < 2 {
				t.Fatalf("the list shows %d rows; a page move needs more", listRows)
			}

			next, _ := m.Update(c.key)
			m = next.(*Model)
			if got := m.state.Cursor; got != c.want(m, listRows) {
				t.Errorf("the cursor is at %d, want %d", got, c.want(m, listRows))
			}
		})
	}
}

// Every key that moves the cursor has to leave it on a row, not on a heading or
// past the end. Apply clamps, and the page keys pass a distance rather than a
// destination, so the clamp is what keeps them inside.
func TestThePageKeysLeaveTheCursorInsideTheList(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		key  rune
	}{
		{"page up", tea.KeyPgUp},
		{"page down", tea.KeyPgDown},
		{"home", tea.KeyHome},
		{"end", tea.KeyEnd},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := longListModel(t)
			for range 4 {
				next, _ := m.Update(tea.KeyPressMsg{Code: c.key})
				m = next.(*Model)
				if m.state.Cursor < 0 || m.state.Cursor >= len(m.state.Rows) {
					t.Fatalf("the cursor left the list: %d of %d",
						m.state.Cursor, len(m.state.Rows))
				}
			}
		})
	}
}

func longListModel(t *testing.T) *Model {
	t.Helper()
	rows := make([]git.Entry, 60)
	for i := range rows {
		rows[i] = git.Entry{Path: fmt.Sprintf("f%02d.txt", i), Worktree: git.Modified}
	}
	m := &Model{read: emptyRead(), render: layout.Renderer{}, dir: t.TempDir(),
		state: state.State{Width: 90, Height: 30}}
	m.probe.settled = true
	m.state.Changes.Folded = map[string]bool{}
	m.state.Changes.Selected = map[string]bool{}
	m.state.Changes.Stale = map[string]bool{}
	m.state = state.Apply(m.state, state.StatusLoaded{Rows: rows,
		Head: git.Head{Branch: "main"}})
	return m
}

// A pane too short to show a list row leaves the page distance at zero, and a
// page key that moves nothing reads as a key that does not work. One row is the
// smallest move that is still a move.
func TestAPageKeyStillMovesWhenThePaneIsTooShortForAList(t *testing.T) {
	t.Parallel()
	m := longListModel(t)
	m.state = state.Apply(m.state, state.Resized{Width: 90, Height: 6})
	if rows, _, _ := m.windowRows(); rows > 1 {
		t.Fatalf("the list still shows %d rows; this needs a pane with none", rows)
	}

	before := m.state.Cursor
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	if got := next.(*Model).state.Cursor; got == before {
		t.Errorf("page down left the cursor at %d", got)
	}
}
