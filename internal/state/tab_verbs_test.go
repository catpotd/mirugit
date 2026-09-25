package state

import (
	"testing"

	"github.com/catpotd/mirugit/internal/git"
)

func TestStashVerbAppliesWithholdsDropWhileUnknown(t *testing.T) {
	t.Parallel()
	if StashVerbApplies(VerbNameDrop, git.StashUnknown) {
		t.Fatal("drop should not apply while stash status is unknown")
	}
}

func TestStashVerbsWithholdAllVerbsWhileUnknown(t *testing.T) {
	t.Parallel()
	if verbs := StashVerbs(git.StashUnknown); len(verbs) != 0 {
		t.Fatalf("unknown stash should offer no verbs: %v", verbs)
	}
}
