package layout

import (
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// nestedFileRow is the row the stashed and worktrees tabs draw for a file held
// by the row above it. The pane builds it from a state row; this builds it from
// the entry alone, which is what the drawing tests measure. It lived in the
// pane as two identical functions that only the tests called.
func nestedFileRow(e git.Entry) Row {
	count := e.WorktreeCount
	if e.IsStaged() && !e.IsUnstaged() {
		count = e.IndexCount
	}
	return Row{
		Entry: e,
		Count: count,
		State: rowState{
			Indent:   1,
			Read:     state.Read,
			NoSelect: true,
			FullPath: true,
		},
	}
}
