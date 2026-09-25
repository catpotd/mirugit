package state

import (
	"testing"

	"github.com/catpotd/mirugit/internal/git"
)

// A stash list is re-read often, and what each stash holds is read separately
// and more slowly. The counts already read are carried into the new list so the
// rows do not go blank between the two reads — but only into rows that have not
// been read themselves. A count of -1 is a row waiting for its read; a count of
// zero is a row that was read and found nothing, and carrying the old numbers
// over it shows a stash that is no longer there.
func TestOnlyAStashWaitingForItsReadInheritsTheOldOne(t *testing.T) {
	t.Parallel()
	had := []git.StashRow{
		{Ref: "stash@{0}", SHA: "s0", FileCount: 3, Status: git.StashApplies},
		{Ref: "stash@{1}", SHA: "s1", FileCount: 2, Status: git.StashConflicts},
	}
	now := []git.StashRow{
		{Ref: "stash@{0}", SHA: "s0", FileCount: -1},
		{Ref: "stash@{1}", SHA: "s1", FileCount: 0},
	}

	got := carryStashReads(had, now)
	if len(got) != 2 {
		t.Fatalf("the list holds %d rows, want 2", len(got))
	}
	if got[0].FileCount != 3 || got[0].Status != git.StashApplies {
		t.Errorf("the row waiting for its read is %+v, want the counts it had", got[0])
	}
	if got[1].FileCount != 0 {
		t.Errorf("a stash read as holding nothing says %d files", got[1].FileCount)
	}
	if got[1].Status != git.StashStatus(0) {
		t.Errorf("a stash read as holding nothing carries the verdict %v", got[1].Status)
	}
}
