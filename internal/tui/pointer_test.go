package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
)

// The pointer names no row. It used to arm whatever it passed over, which took
// the verbs away from the row the reader armed and, while a diff was open,
// replaced that diff with the diff of the row the pointer crossed. Crossing the
// pane to reach something is not a choice of row.
//
// Every region the pane draws is checked, because the old rule had two of them:
// a row armed the row, and a section's checkbox armed that section's heading.
func TestPointingAtAnythingLeavesTheCursorWhereItWas(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	m.View()
	if len(m.frame.Regions) == 0 {
		t.Fatal("nothing was drawn, so this proves nothing")
	}
	want := m.state.Cursor

	for _, r := range m.frame.Regions {
		next, _ := m.Update(tea.MouseMotionMsg{X: r.ColStart, Y: r.Row})
		m = next.(*Model)
		if m.state.Cursor != want {
			t.Fatalf("pointing at a %v at row %d moved the cursor from %d to %d",
				r.Target.Kind, r.Row, want, m.state.Cursor)
		}
	}
}

// The reader opens a diff and then moves the pointer. The open diff is theirs
// until they open another one: the pointer passing over the list must not swap
// it for whatever it crossed.
func TestPointingAtAnotherFileKeepsTheOpenDiff(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	m.View()
	open := m.state.Open.Path
	if open == "" {
		t.Fatal("no diff is open, so this proves nothing")
	}

	other := ""
	for _, r := range m.frame.Regions {
		if r.Target.Kind == layout.TargetFile && r.Target.Path != "" && r.Target.Path != open {
			other = r.Target.Path
			next, _ := m.Update(tea.MouseMotionMsg{X: r.ColStart, Y: r.Row})
			m = next.(*Model)
			break
		}
	}
	if other == "" {
		t.Fatal("only one file row was drawn, so this proves nothing")
	}
	if m.state.Open.Path != open {
		t.Errorf("pointing at %q changed the open diff to %q, want %q",
			other, m.state.Open.Path, open)
	}
}

// The stashed and worktrees tabs read a row's files from git when the cursor
// lands on it. While the pointer armed rows, sweeping it down the list put one
// git process on every row it crossed, and each answer that arrived replaced
// the files already on screen. The pointer starts nothing now.
func TestPointingStartsNoCommandAndKeepsTheOpenRow(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name     string
		open     func(*Model)
		expanded func(state.State) string
	}{
		{
			name: "stashed",
			open: func(m *Model) {
				m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabStashed})
				m.state = state.Apply(m.state, state.StashedLoaded{Stashes: []git.StashRow{
					{Ref: "stash@{0}", SHA: "s0", Message: "one", Status: git.StashApplies},
					{Ref: "stash@{1}", SHA: "s1", Message: "two", Status: git.StashApplies},
				}})
				m.state = state.Apply(m.state, state.StashFilesLoaded{Ref: "stash@{0}",
					Files: []git.Entry{{Path: "only-in-stash.go"}}})
			},
			expanded: func(s state.State) string { return s.Stashed.Expanded },
		},
		{
			name: "worktrees",
			open: func(m *Model) {
				m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabWorktrees})
				m.state = state.Apply(m.state, state.WorktreesLoaded{Base: "main",
					Worktrees: []git.WorktreeRow{
						{Path: "/repo", Name: "repo", Main: true, SHA: "aaa"},
						{Path: "/wt", Name: "wt", SHA: "bbb"},
					}})
				m.state = state.Apply(m.state, state.WorktreeFilesLoaded{Path: "/repo",
					Files: []git.Entry{{Path: "only-in-wt.go"}}})
			},
			expanded: func(s state.State) string { return s.Worktrees.Expanded },
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := &Model{read: emptyRead(), render: layout.Renderer{},
				state: state.State{Width: 90, Height: 30}}
			m.probe.settled = true
			m.state.Changes.Folded = map[string]bool{}
			m.state.Changes.Selected = map[string]bool{}
			m.state.Changes.Stale = map[string]bool{}
			c.open(m)
			m.state.Cursor = 0
			m.state = state.Apply(m.state, state.CursorMoved{By: 0})
			m.View()

			wantExpanded := c.expanded(m.state)
			if wantExpanded == "" {
				t.Fatal("no row is open, so this proves nothing")
			}
			for _, r := range m.frame.Regions {
				next, cmd := m.Update(tea.MouseMotionMsg{X: r.ColStart, Y: r.Row})
				m = next.(*Model)
				if cmd != nil {
					t.Fatalf("pointing at a %v at row %d started %d commands",
						r.Target.Kind, r.Row, len(messagesFrom(cmd)))
				}
				if got := c.expanded(m.state); got != wantExpanded {
					t.Fatalf("pointing at a %v at row %d changed the open row to %q, want %q",
						r.Target.Kind, r.Row, got, wantExpanded)
				}
			}
		})
	}
}

// The stashed and worktrees tabs draw the files of the row under the cursor as
// rows of their own. Pointing at one of those files must leave the list that
// holds it open.
//
// Both tabs are driven here from one table. Written per tab, this was fixed on
// one of them and left on the other for as long as nobody pointed at the other.
func TestPointingAtAnExpandedFileKeepsTheListOpen(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name  string
		open  func(*Model)
		files func(state.State) []git.Entry
	}{
		{
			name: "stashed",
			open: func(m *Model) {
				m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabStashed})
				m.state = state.Apply(m.state, state.StashedLoaded{Stashes: []git.StashRow{
					{Ref: "stash@{0}", SHA: "s0", Message: "one", Status: git.StashApplies},
					{Ref: "stash@{1}", SHA: "s1", Message: "two", Status: git.StashApplies},
				}})
				m.state = state.Apply(m.state, state.StashFilesLoaded{Ref: "stash@{0}", Files: []git.Entry{
					{Path: "a.txt"}, {Path: "b.txt"},
				}})
			},
			files: func(s state.State) []git.Entry { return s.Stashed.Files },
		},
		{
			name: "worktrees",
			open: func(m *Model) {
				m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabWorktrees})
				m.state = state.Apply(m.state, state.WorktreesLoaded{Base: "main", Worktrees: []git.WorktreeRow{
					{Path: "/repo", Name: "repo", Main: true, SHA: "aaa"},
					{Path: "/wt", Name: "wt", SHA: "bbb"},
				}})
				m.state = state.Apply(m.state, state.WorktreeFilesLoaded{Path: "/repo", Files: []git.Entry{
					{Path: "a.txt"}, {Path: "b.txt"},
				}})
			},
			files: func(s state.State) []git.Entry { return s.Worktrees.Files },
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := &Model{read: emptyRead(), render: layout.Renderer{},
				state: state.State{Width: 90, Height: 30}}
			m.probe.settled = true
			m.state.Changes.Folded = map[string]bool{}
			m.state.Changes.Selected = map[string]bool{}
			m.state.Changes.Stale = map[string]bool{}
			c.open(m)
			m.state.Cursor = 0
			m.state = state.Apply(m.state, state.CursorMoved{By: 0})

			if len(c.files(m.state)) == 0 {
				t.Fatal("no files are shown, so this proves nothing")
			}
			m.View()

			row := fileRowScreenRow(t, m, "b.txt")
			next, _ := m.Update(tea.MouseMotionMsg{X: 20, Y: row})
			m = next.(*Model)

			if len(c.files(m.state)) == 0 {
				t.Error("pointing at one of the files closed the list holding it")
			}
		})
	}
}

// The row that is open is the one the reader opened, and the pointer passing
// over the list is not them closing it.
func TestPointingAtAnotherRowLeavesTheOpenOneOpen(t *testing.T) {
	t.Parallel()
	m := &Model{read: emptyRead(), render: layout.Renderer{},
		state: state.State{Width: 90, Height: 30}}
	m.probe.settled = true
	m.state.Changes.Folded = map[string]bool{}
	m.state.Changes.Selected = map[string]bool{}
	m.state.Changes.Stale = map[string]bool{}
	m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabStashed})
	m.state = state.Apply(m.state, state.StashedLoaded{Stashes: []git.StashRow{
		{Ref: "stash@{0}", SHA: "s0", Message: "one", Status: git.StashApplies},
		{Ref: "stash@{1}", SHA: "s1", Message: "two", Status: git.StashApplies},
	}})
	m.state = state.Apply(m.state, state.StashFilesLoaded{Ref: "stash@{0}", Files: []git.Entry{
		{Path: "a.txt"},
	}})
	m.state.Cursor = 0
	m.state = state.Apply(m.state, state.CursorMoved{By: 0})
	m.View()

	row := -1
	for _, r := range m.frame.Regions {
		if i := r.Target.Row; i >= 0 && i < len(m.state.Rows) &&
			m.state.Rows[i].Kind() == state.RowStash &&
			m.state.Rows[i].Stash().Ref == "stash@{1}" {
			row = r.Row
			break
		}
	}
	if row < 0 {
		t.Fatal("no region for the second stash")
	}
	next, _ := m.Update(tea.MouseMotionMsg{X: 20, Y: row})
	m = next.(*Model)

	if len(m.state.Stashed.Files) != 1 {
		t.Errorf("pointing at the second stash closed the first: %v", m.state.Stashed.Files)
	}
	if m.state.Stashed.Expanded != "s0" {
		t.Errorf("the open stash is %q, want s0", m.state.Stashed.Expanded)
	}
}

func fileRowScreenRow(t *testing.T, m *Model, path string) int {
	t.Helper()
	for _, r := range m.frame.Regions {
		if r.Target.Kind == layout.TargetFile && r.Target.Path == path {
			return r.Row
		}
	}
	t.Fatalf("no region for the file row %q", path)
	return -1
}
