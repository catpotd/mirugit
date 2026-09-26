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

func TestUnrelatedStashOffersBranchAndDropButNotRestore(t *testing.T) {
	t.Parallel()
	verbs := StashVerbs(git.StashUnrelated)
	if len(verbs) != 2 || !StashVerbApplies(VerbNameBranch, git.StashUnrelated) ||
		!StashVerbApplies(VerbNameDrop, git.StashUnrelated) {
		t.Fatalf("unrelated stash verbs = %v, want branch and drop", verbs)
	}
	if StashVerbApplies(VerbNameRestore, git.StashUnrelated) {
		t.Fatal("unrelated stash should not offer restore")
	}
}
