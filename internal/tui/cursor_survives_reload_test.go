package tui

import (
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// A reload arrives on a timer and from the file watcher, so it lands while the
// reader is looking at a row. The list is rebuilt from scratch, and holding the
// index instead of the row moves the cursor onto whatever took that line.
//
// The four tabs are driven from one table because the rebuild is one function
// and each tab used to settle the cursor its own way.
func TestAReloadLeavesTheCursorOnTheSameRow(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name  string
		open  func(*Model)
		grow  func(*Model)
		onRow func(*Model) string
	}{
		{
			name: "changes",
			open: func(m *Model) {
				m.state = state.Apply(m.state, state.StatusLoaded{Rows: []git.Entry{
					{Path: "b/mid.txt", Worktree: git.Modified},
					{Path: "c/last.txt", Worktree: git.Modified},
				}, Head: git.Head{Branch: "main"}})
			},
			// A path that sorts first pushes every row below it down one line.
			grow: func(m *Model) {
				m.state = state.Apply(m.state, state.StatusLoaded{Rows: []git.Entry{
					{Path: "a/first.txt", Worktree: git.Modified},
					{Path: "b/mid.txt", Worktree: git.Modified},
					{Path: "c/last.txt", Worktree: git.Modified},
				}, Head: git.Head{Branch: "main"}})
			},
		},
		{
			name: "stashed",
			open: func(m *Model) {
				m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabStashed})
				m.state = state.Apply(m.state, state.StashedLoaded{Stashes: []git.StashRow{
					{Ref: "stash@{0}", SHA: "s0", Message: "mid", Status: git.StashApplies},
					{Ref: "stash@{1}", SHA: "s1", Message: "last", Status: git.StashApplies},
				}})
			},
			grow: func(m *Model) {
				m.state = state.Apply(m.state, state.StashedLoaded{Stashes: []git.StashRow{
					{Ref: "stash@{0}", SHA: "s2", Message: "new", Status: git.StashApplies},
					{Ref: "stash@{1}", SHA: "s0", Message: "mid", Status: git.StashApplies},
					{Ref: "stash@{2}", SHA: "s1", Message: "last", Status: git.StashApplies},
				}})
			},
		},
		{
			name: "worktrees",
			open: func(m *Model) {
				m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabWorktrees})
				m.state = state.Apply(m.state, state.WorktreesLoaded{Base: "main", Here: "/repo",
					Worktrees: []git.WorktreeRow{
						{Path: "/repo", Name: "repo", Main: true},
						{Path: "/mid", Name: "mid"},
					}})
			},
			grow: func(m *Model) {
				m.state = state.Apply(m.state, state.WorktreesLoaded{Base: "main", Here: "/repo",
					Worktrees: []git.WorktreeRow{
						{Path: "/repo", Name: "repo", Main: true},
						{Path: "/added", Name: "added"},
						{Path: "/mid", Name: "mid"},
					}})
			},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := newParentModel(t)
			c.open(m)
			m.state.Cursor = len(m.state.Rows) - 1
			before := rowPath(m)
			if before == "" {
				t.Fatal("the cursor is on no row, so this proves nothing")
			}

			c.grow(m)

			if after := rowPath(m); after != before {
				t.Errorf("the reload moved the cursor from %q to %q", before, after)
			}
		})
	}
}

func rowPath(m *Model) string {
	row, ok := state.CursorRow(m.state)
	if !ok {
		return ""
	}
	return row.Path()
}
