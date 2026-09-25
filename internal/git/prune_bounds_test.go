package git

import "testing"

// The snapshots undo restores from are dropped past either bound: the hundred
// that are kept, and the age. A ref exactly as old as the bound is not yet past
// it, and dropping it takes away an undo the reader was told they have.
func TestAnUndoRefGoesOnlyWhenItIsPastABound(t *testing.T) {
	t.Parallel()
	const cutoff int64 = 1_700_000_000
	for _, c := range []struct {
		name string
		at   int
		ts   int64
		want bool
	}{
		{"the newest, made now", 0, cutoff + 1, false},
		{"as old as the bound", 0, cutoff, false},
		{"one second older than the bound", 0, cutoff - 1, true},
		{"the last one kept", undoKeepCount - 1, cutoff + 1, false},
		{"one past the number kept", undoKeepCount, cutoff + 1, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := staleUndoRef(c.at, c.ts, cutoff); got != c.want {
				t.Errorf("staleUndoRef(%d, %d, %d) = %v, want %v",
					c.at, c.ts, cutoff, got, c.want)
			}
		})
	}
}
