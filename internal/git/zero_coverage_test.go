package git

import (
	"strings"
	"testing"
)

// Both errors travel to the pane as a Notice. A message that says nothing about
// what failed is the same to the reader as no message at all.
func TestErrorMessagesNameWhatFailed(t *testing.T) {
	t.Parallel()
	if got := errMissingUndo.Error(); !strings.Contains(got, "undo") {
		t.Errorf("missingUndoError = %q, want the word undo", got)
	}
	oversized := &OversizedPathspecError{Entries: 9000, Bytes: 600_000}
	got := oversized.Error()
	for _, want := range []string{"9000", "600000", "512 KB"} {
		if !strings.Contains(got, want) {
			t.Errorf("OversizedPathspecError = %q, want it to name %q", got, want)
		}
	}
}
