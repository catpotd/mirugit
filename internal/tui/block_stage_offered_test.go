package tui

import (
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// `s` on a diff block stages that block, and there are four things that have to
// be true for it to mean anything: a diff is open, the cursor is on the file it
// belongs to, that file is not one a merge left unmerged, and its changes are
// on the unstaged side. A block already staged has nothing for s to do — git
// has it — and README says nothing is offered that cannot be pressed.
//
// Each of the four had no test. They are checked one at a time, so a condition
// dropped from the guard names itself.
func TestStagingOneBlockIsOfferedOnlyWhereItCanRun(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name    string
		section state.Section
		// entry is the row the cursor sits on.
		entry   git.Entry
		open    bool
		blocks  int
		offered bool
	}{
		{"an unstaged file with a diff open", state.SectionUnstaged,
			git.Entry{Path: "a.txt", Worktree: git.Modified}, true, 1, true},
		{"a file whose changes are already staged", state.SectionStaged,
			git.Entry{Path: "a.txt", Index: git.Modified}, true, 1, false},
		{"a file a merge left unmerged", state.SectionUnstaged,
			git.Entry{Path: "a.txt", Index: git.Unmerged, Worktree: git.Unmerged}, true, 1, false},
		{"no diff open", state.SectionUnstaged,
			git.Entry{Path: "a.txt", Worktree: git.Modified}, false, 0, false},
		{"a diff open with no blocks in it", state.SectionUnstaged,
			git.Entry{Path: "a.txt", Worktree: git.Modified}, true, 0, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := fixed(t)
			m.state = state.Apply(m.state, state.StatusLoaded{
				Head: git.Head{Branch: "main"},
				Rows: []git.Entry{c.entry},
			})
			m.state.Cursor = state.FirstFileRow(m.state.Rows)
			if c.open {
				blocks := make([]git.Block, c.blocks)
				for i := range blocks {
					blocks[i] = git.Block{Header: "@@ -1 +1 @@", Lines: []string{"+x"}}
				}
				m.state = state.Apply(m.state, state.DiffOpened{
					Path: c.entry.Path, Origin: state.WorkingTree(c.section)})
				m.state = state.Apply(m.state, state.DiffLoaded{
					Diff: git.FileDiff{Path: c.entry.Path, Blocks: blocks}})
			}

			if got := m.blockStageReady(); got != c.offered {
				t.Errorf("staging one block is offered = %v, want %v", got, c.offered)
			}
		})
	}
}

// The cursor can sit outside the list: a reload from the watcher shortens Rows
// between the frame a key was drawn on and the key arriving. There is no row to
// read a section off then, and the offer has nothing to be about. Every case in
// the sweep above has a row under the cursor.
func TestStagingOneBlockIsNotOfferedWithNoRowUnderTheCursor(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	m.state = state.Apply(m.state, state.StatusLoaded{
		Head: git.Head{Branch: "main"},
		Rows: []git.Entry{{Path: "a.txt", Worktree: git.Modified}},
	})
	m.state.Cursor = state.FirstFileRow(m.state.Rows)
	m.state = state.Apply(m.state, state.DiffOpened{
		Path: "a.txt", Origin: state.WorkingTree(state.SectionUnstaged)})
	m.state = state.Apply(m.state, state.DiffLoaded{
		Diff: git.FileDiff{Path: "a.txt", Blocks: []git.Block{
			{Header: "@@ -1 +1 @@", Lines: []string{"+x"}}}}})
	if !m.blockStageReady() {
		t.Fatal("the offer is not there to begin with, so this proves nothing")
	}

	m.state.Cursor = len(m.state.Rows)
	if _, ok := state.CursorRow(m.state); ok {
		t.Fatalf("the cursor still names a row at %d of %d", m.state.Cursor, len(m.state.Rows))
	}
	if m.blockStageReady() {
		t.Error("staging one block is offered with no row under the cursor")
	}
}
