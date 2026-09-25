package tui

import (
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// A read mark belongs to the diff that was on screen. Closing one is what
// finishes it, and the cursor has already left by then: moving off a row is the
// most common way to close a diff. Filing the mark under the row the cursor
// landed on marks a file the reader never opened, and the same path staged and
// unstaged is where that lands on a row the reader can see.
func TestTheReadMarkGoesToTheSideTheDiffCameFrom(t *testing.T) {
	t.Parallel()
	m := newParentModel(t)
	m.state = state.Apply(m.state, state.StatusLoaded{
		Rows: []git.Entry{{Path: "a.txt", Index: git.Modified, Worktree: git.Modified}},
		Head: git.Head{Branch: "main"}})

	staged, unstaged := -1, -1
	for i, row := range m.state.Rows {
		if row.Kind() != state.RowFile || row.Path() != "a.txt" {
			continue
		}
		if row.Section() == state.SectionStaged {
			staged = i
		} else {
			unstaged = i
		}
	}
	if staged < 0 || unstaged < 0 {
		t.Fatalf("a.txt is not on both sides: staged=%d unstaged=%d", staged, unstaged)
	}

	m.state = state.Apply(m.state, state.CursorMoved{By: staged - m.state.Cursor})
	m.state = state.Apply(m.state, state.DiffOpened{Path: "a.txt",
		Origin: state.WorkingTree(state.SectionStaged)})
	m.state = state.Apply(m.state, state.DiffLoaded{Diff: git.FileDiff{Path: "a.txt",
		Blocks: []git.Block{{Header: "@@ -1 +1 @@", Lines: []string{"+x"}, Hash: "h1"}}}})
	m.recordShownBlocks()

	// Apply runs before the update finishes reading, which is the order the
	// cursor keys take.
	m.state = state.Apply(m.state, state.CursorMoved{By: unstaged - staged})
	_ = m.finishReading()

	if got := m.read.MarkIn(state.WorkingTree(state.SectionStaged), "a.txt", []string{"h1"}); got != state.Read {
		t.Errorf("the staged side, whose diff was read, is %v, want Read", got)
	}
	if got := m.read.MarkIn(state.WorkingTree(state.SectionUnstaged), "a.txt", []string{"h2"}); got != state.Unread {
		t.Errorf("the unstaged side, never opened, is %v, want Unread", got)
	}
}
