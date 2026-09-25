package state

import (
	"strings"
	"testing"

	"github.com/catpotd/mirugit/internal/git"
)

// The undo of a discard and the discard itself end in the same event, and this
// flag is what tells them apart. Without it the undo reads as a discard: the
// bar offers U to undo what was just undone, and the record of what to undo is
// set to the undo's own snapshot rather than cleared.
func TestTheUndoOfADiscardIsNotReadAsADiscard(t *testing.T) {
	t.Parallel()
	undo := git.Undo{Ref: "refs/mirugit/undo/x", Paths: []string{"a.txt"}}

	discarded := Apply(State{}, DiscardFinished{Applied: 1, Undo: undo, Added: 2, Deleted: 1})
	if discarded.LastUndo == nil {
		t.Error("a discard left nothing to undo")
	}
	if !strings.HasPrefix(discarded.Notice, "discarded 1 file") {
		t.Errorf("a discard says %q", discarded.Notice)
	}

	restored := Apply(discarded, DiscardFinished{Applied: 1, Undo: undo, Restored: true})
	if restored.LastUndo != nil {
		t.Errorf("the undo left something to undo: %+v", restored.LastUndo)
	}
	if !strings.HasPrefix(restored.Notice, "restored 1 file") {
		t.Errorf("the undo says %q, want it to say what it restored", restored.Notice)
	}
	if strings.Contains(restored.Notice, "U undo") {
		t.Errorf("the undo offers U to undo itself: %q", restored.Notice)
	}
}
