package git

import "testing"

// Prune keeps the first undoKeepCount of this order and drops the rest, so the
// order decides which snapshot the reader can still undo to. Snapshots are
// timestamped to the second and two can share one: which of those is dropped
// has to be the same answer every run, or the reader loses a different one each
// time they start the program.
func TestTheUndoRefsAreOrderedNewestFirstAndTiesByName(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		in   []undoRef
		want []string
	}{
		{"nothing", nil, nil},
		{"one", []undoRef{{"a", 1}}, []string{"a"}},
		{"newest first", []undoRef{{"a", 1}, {"b", 3}, {"c", 2}}, []string{"b", "c", "a"}},
		{"one timestamp, ordered by name",
			[]undoRef{{"c", 1}, {"a", 1}, {"b", 1}}, []string{"a", "b", "c"}},
		{"ties inside a run",
			[]undoRef{{"d", 2}, {"b", 3}, {"c", 2}, {"a", 3}},
			[]string{"a", "b", "c", "d"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			refs := append([]undoRef(nil), c.in...)
			newestFirst(refs)
			if len(refs) != len(c.want) {
				t.Fatalf("ordered %d refs, want %d: %v", len(refs), len(c.want), refs)
			}
			for i := range refs {
				if refs[i].ref != c.want[i] {
					t.Errorf("ref %d is %q, want %q: %v", i, refs[i].ref, c.want[i], refs)
				}
			}
		})
	}
}

// The order must not depend on the order they arrived in: two runs that read
// the same refs in a different order keep the same ones.
func TestTheUndoOrderDoesNotDependOnHowTheyArrived(t *testing.T) {
	t.Parallel()
	forwards := []undoRef{{"a", 1}, {"b", 1}, {"c", 1}, {"d", 2}}
	backwards := []undoRef{{"d", 2}, {"c", 1}, {"b", 1}, {"a", 1}}
	newestFirst(forwards)
	newestFirst(backwards)
	for i := range forwards {
		if forwards[i] != backwards[i] {
			t.Fatalf("the two orders differ at %d: %v and %v", i, forwards, backwards)
		}
	}
}
