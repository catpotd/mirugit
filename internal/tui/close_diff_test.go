package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
)

// The diff header prints "esc close". Closing records which path was closed so
// that hover, which opens whatever the cursor names, does not open it again on
// the same frame. Recording the wrong name makes the key look ignored.
//
// The four tabs run from one table because the name the closing used came from
// cursorPath, which is empty for a file a commit, a stash or another worktree
// holds: the three tabs that hold files of another tree all had it.
func TestEscClosesTheDiffOnEveryTab(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		open func(*Model)
		path string
	}{
		{
			name: "changes",
			open: func(m *Model) {
				m.state = state.Apply(m.state, state.StatusLoaded{
					Rows: []git.Entry{{Path: "a.txt", Worktree: git.Modified}},
					Head: git.Head{Branch: "main"}})
				m.state = state.Apply(m.state, state.CursorMoved{By: 2})
			},
			path: "a.txt",
		},
		{
			name: "history",
			open: func(m *Model) {
				m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabHistory})
				m.state = state.Apply(m.state, state.HistoryLoaded{Commits: []git.CommitInfo{
					{SHA: "aaa", ShortSHA: "aaa", Subject: "one", Age: "1m"}}})
				m.state = state.Apply(m.state, state.CommitExpanded{SHA: "aaa",
					Files: []git.Entry{{Path: "note.txt", Index: git.Modified}}})
				m.state = state.Apply(m.state, state.CursorMoved{By: 1})
			},
			path: "note.txt",
		},
		{
			name: "stashed",
			open: func(m *Model) {
				m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabStashed})
				m.state = state.Apply(m.state, state.StashedLoaded{Stashes: []git.StashRow{
					{Ref: "stash@{0}", SHA: "s0", Message: "one", Status: git.StashApplies}}})
				m.state = state.Apply(m.state, state.StashFilesLoaded{Ref: "stash@{0}",
					Files: []git.Entry{{Path: "held.txt"}}})
				m.state = state.Apply(m.state, state.CursorMoved{By: 1})
			},
			path: "held.txt",
		},
		{
			name: "worktrees",
			open: func(m *Model) {
				m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabWorktrees})
				m.state = state.Apply(m.state, state.WorktreesLoaded{Base: "main", Here: "/repo",
					Worktrees: []git.WorktreeRow{
						{Path: "/repo", Name: "repo", Main: true, SHA: "aaa"},
						{Path: "/wt", Name: "wt", SHA: "bbb"}}})
				m.state = state.Apply(m.state, state.WorktreeFilesLoaded{Path: "/repo",
					Files: []git.Entry{{Path: "side.txt"}}})
				m.state = state.Apply(m.state, state.CursorMoved{By: 1})
			},
			path: "side.txt",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := &Model{read: emptyRead(), render: layout.Renderer{},
				state: state.State{Width: 90, Height: 30}, dir: t.TempDir()}
			m.probe.settled = true
			m.state.Changes.Folded = map[string]bool{}
			m.state.Changes.Selected = map[string]bool{}
			m.state.Changes.Stale = map[string]bool{}
			c.open(m)

			row, ok := state.CursorRow(m.state)
			if !ok || row.Kind() != state.RowFile || row.Path() != c.path {
				t.Fatalf("the cursor is not on %s, so this proves nothing", c.path)
			}
			origin, ok := state.OriginOf(m.state, m.state.Cursor)
			if !ok {
				t.Fatalf("%s has no origin, so this proves nothing", c.path)
			}
			m.state = state.Apply(m.state, state.DiffOpened{Path: c.path, Origin: origin})
			m.state = state.Apply(m.state, state.DiffLoaded{Diff: git.FileDiff{Path: c.path,
				Blocks: []git.Block{{Header: "@@ -1 +1 @@", Lines: []string{"+x"}, Hash: "h1"}}}})

			next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
			m = next.(*Model)

			if m.state.Open.Path != "" {
				t.Errorf("esc left the diff open on %q", m.state.Open.Path)
			}
			if m.state.Open.ClosedFor != c.path {
				t.Errorf("the closing recorded %q, want %q; hover opens whatever the "+
					"cursor names unless the two agree", m.state.Open.ClosedFor, c.path)
			}
		})
	}
}
