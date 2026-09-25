package tui

import (
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
)

// Three tabs expand a row into the files it holds, and every key on those tabs
// acts on the row the cursor is in. The three answered "which row is that"
// differently: stashed and worktrees walked up to the parent, history did not.
// A file inside an expanded commit therefore had no commit, and copying its SHA
// or undoing it did nothing, while the same position on the other two worked.
//
// One table for the three, because written per tab this was right on two of
// them for as long as nobody tried the third.
func TestTheCursorIsInsideTheRowItsFilesBelongTo(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name   string
		open   func(*Model)
		parent func(*Model) (string, bool)
		want   string
	}{
		{
			name: "history",
			open: func(m *Model) {
				m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabHistory})
				m.state = state.Apply(m.state, state.HistoryLoaded{Commits: []git.CommitInfo{
					{SHA: "aaa111", ShortSHA: "aaa111", Subject: "one", Age: "1m"},
					{SHA: "bbb222", ShortSHA: "bbb222", Subject: "two", Age: "2m"},
				}})
				m.state = state.Apply(m.state, state.CommitExpanded{SHA: "aaa111",
					Files: []git.Entry{{Path: "a.txt", Index: git.Modified}}})
			},
			parent: func(m *Model) (string, bool) {
				c, ok := m.currentCommit()
				return c.SHA, ok
			},
			want: "aaa111",
		},
		{
			name: "stashed",
			open: func(m *Model) {
				m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabStashed})
				m.state = state.Apply(m.state, state.StashedLoaded{Stashes: []git.StashRow{
					{Ref: "stash@{0}", SHA: "s0", Message: "one", Status: git.StashApplies},
					{Ref: "stash@{1}", SHA: "s1", Message: "two", Status: git.StashApplies},
				}})
				m.state = state.Apply(m.state, state.StashFilesLoaded{Ref: "stash@{0}",
					Files: []git.Entry{{Path: "a.txt"}}})
			},
			parent: func(m *Model) (string, bool) {
				st, ok := m.currentStash()
				return st.Ref, ok
			},
			want: "stash@{0}",
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
					Files: []git.Entry{{Path: "a.txt"}}})
			},
			parent: func(m *Model) (string, bool) {
				w, ok := m.currentWorktree()
				return w.Path, ok
			},
			want: "/repo",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := newParentModel(t)
			c.open(m)

			fileRow := -1
			for i, r := range m.state.Rows {
				if r.Kind() == state.RowFile {
					fileRow = i
					break
				}
			}
			if fileRow < 1 {
				t.Fatalf("no file row under a parent: %d rows", len(m.state.Rows))
			}

			for _, cursor := range []int{fileRow - 1, fileRow} {
				m.state.Cursor = cursor
				got, ok := c.parent(m)
				if !ok {
					t.Errorf("cursor %d (%s): the reader is in no row at all",
						cursor, whereIs(m, cursor))
					continue
				}
				if got != c.want {
					t.Errorf("cursor %d (%s): the reader is in %q, want %q",
						cursor, whereIs(m, cursor), got, c.want)
				}
			}
		})
	}
}

func whereIs(m *Model, cursor int) string {
	row, ok := state.RowAt(m.state, cursor)
	if !ok {
		return "outside the list"
	}
	if row.Kind() == state.RowFile {
		return "on a file of the row"
	}
	return "on the row itself"
}

func newParentModel(t *testing.T) *Model {
	t.Helper()
	m := &Model{read: emptyRead(), render: layout.Renderer{},
		state: state.State{Width: 90, Height: 30}}
	m.probe.settled = true
	m.state.Changes.Folded = map[string]bool{}
	m.state.Changes.Selected = map[string]bool{}
	m.state.Changes.Stale = map[string]bool{}
	return m
}
