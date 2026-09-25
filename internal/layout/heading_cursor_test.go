package layout

import (
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// A section heading's region names the row a click on it should move the cursor
// to. That is the heading itself while the cursor is already on it, and the
// caller's fallback otherwise. Reading the cursor the other way round names the
// heading whenever the cursor is anywhere else, so a click on the staged
// heading while the cursor sits on a file moves it to the wrong row.
func TestTheHeadingNamesItselfOnlyWhileTheCursorIsOnIt(t *testing.T) {
	t.Parallel()
	s := state.State{Width: 90, Height: 30}
	s = state.Apply(s, state.StatusLoaded{
		Head: git.Head{Branch: "main"},
		Rows: []git.Entry{
			{Path: "staged.txt", Index: git.Modified},
			{Path: "unstaged.txt", Worktree: git.Modified},
		},
	})

	heading, file := -1, -1
	for i, row := range s.Rows {
		if row.Kind() == state.RowSectionHeading && row.Section() == state.SectionStaged {
			heading = i
		}
		if row.Kind() == state.RowFile && row.Section() == state.SectionStaged {
			file = i
		}
	}
	if heading < 0 || file < 0 {
		t.Fatalf("no staged heading or file in %d rows, so this proves nothing", len(s.Rows))
	}

	const fallback = 99
	s.Cursor = heading
	if got := headingCursorIndex(s, state.SectionStaged, fallback); got != heading {
		t.Errorf("with the cursor on the heading the row is %d, want the heading at %d", got, heading)
	}
	s.Cursor = file
	if got := headingCursorIndex(s, state.SectionStaged, fallback); got != fallback {
		t.Errorf("with the cursor on a file the row is %d, want the fallback %d", got, fallback)
	}
}
