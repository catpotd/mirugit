package state

import "testing"

// A read mark says the reader finished one file. Section named where a file
// came from but not which one, so "history:" stood for every commit and "none:"
// for every stash: reading a file in one commit marked it read in every other
// commit that touched the same path.
func TestAReadMarkBelongsToTheOneItWasMadeIn(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		read Origin
		ask  Origin
	}{
		{"another commit", FromCommit("aaa"), FromCommit("bbb")},
		{"another stash", FromParent("stash@{0}"), FromParent("stash@{1}")},
		{"another worktree", FromParent("/wt-a"), FromParent("/wt-b")},
		{"a commit and a stash that happen to share a name",
			FromCommit("abc"), FromParent("abc")},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r := empty("/tmp/read.json", "/repo")
			r.MarkFileIn(c.read, "a.txt", nil)

			if got := r.MarkIn(c.read, "a.txt", nil); got != Read {
				t.Fatalf("the one that was read answers %v, so this proves nothing", got)
			}
			if got := r.MarkIn(c.ask, "a.txt", nil); got != Unread {
				t.Errorf("%s answers %v for a file never opened there, want Unread",
					c.name, got)
			}
		})
	}
}

// The two sides of the index hold the same lines when one is staged, so reading
// through on one side counts on the other. That is the one crossing that has to
// stay.
func TestStagingWhatWasReadKeepsItReadOnBothSides(t *testing.T) {
	t.Parallel()
	r := empty("/tmp/read.json", "/repo")
	r.MarkFileIn(WorkingTree(SectionUnstaged), "a.txt", []string{"h1"})

	if got := r.MarkIn(WorkingTree(SectionStaged), "a.txt", []string{"h1"}); got != Read {
		t.Errorf("staging what was read answers %v, want Read", got)
	}
	// The crossing is by content, so it needs the hashes. Without them the row
	// answers for itself, which is what every row on the staged side does after
	// a reload replaces the cached hashes.
	if got := r.MarkIn(FromCommit("aaa"), "a.txt", nil); got != Unread {
		t.Errorf("a commit answers %v for a file never opened there, want Unread", got)
	}
}
