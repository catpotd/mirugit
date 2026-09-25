package state

import (
	"strings"
	"testing"

	"github.com/catpotd/mirugit/internal/git"
)

// The bar says what a stash did. "stashed 0 files" is a sentence about nothing
// happening, and the case above this one already covers the reason nothing did:
// the files it was given had no changes. A bound that lets zero through says
// both, and the second overwrites the first.
func TestTheBarSaysHowManyFilesWereStashedOnlyWhenSomeWere(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name    string
		applied int
		ignored []string
		want    string
	}{
		{"two files stashed", 2, nil, "stashed 2 files"},
		{"one file stashed", 1, nil, "stashed 1 file"},
		{"nothing stashed and nothing ignored", 0, nil, ""},
		{"nothing stashed because the files had no changes", 0, []string{"a.txt"},
			"stashed nothing"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := Apply(State{}, VerbFinished{
				Verb: VerbStash, Applied: c.applied, Ignored: c.ignored})
			if c.want == "" {
				if s.Notice != "" {
					t.Errorf("the bar says %q with nothing stashed", s.Notice)
				}
				return
			}
			if !strings.HasPrefix(s.Notice, c.want) {
				t.Errorf("the bar says %q, want it to start with %q", s.Notice, c.want)
			}
		})
	}
}

// A verb that applied to nothing leaves the selection where it was. The ticks
// are taken away as the verb consumes them, and a verb that consumed none has
// none to take: clearing them anyway loses the reader's selection to a verb
// that did not act on it, and there is nothing on screen to say it went.
func TestAVerbThatAppliedToNothingKeepsTheSelection(t *testing.T) {
	t.Parallel()
	build := func(t *testing.T) State {
		t.Helper()
		s := State{Width: 90, Height: 30}
		s = Apply(s, StatusLoaded{
			Head: git.Head{Branch: "main"},
			Rows: []git.Entry{{Path: "a.txt", Worktree: git.Modified}},
		})
		for _, row := range s.Rows {
			if row.Kind() == RowFile {
				s = Apply(s, SelectionToggled{Path: row.Path(), Section: row.Section()})
			}
		}
		if len(s.Changes.Selected) != 1 {
			t.Fatalf("%d rows are ticked, want one", len(s.Changes.Selected))
		}
		return Apply(s, VerbRequested{Verb: VerbStash, Targets: []string{"a.txt"}, Block: -1})
	}

	none := Apply(build(t), VerbFinished{Verb: VerbStash, Applied: 0})
	if len(none.Changes.Selected) != 1 {
		t.Errorf("a verb that applied to nothing left %d ticks, want the one it started with",
			len(none.Changes.Selected))
	}

	one := Apply(build(t), VerbFinished{Verb: VerbStash, Applied: 1})
	if len(one.Changes.Selected) != 0 {
		t.Errorf("a verb that applied to the row left %d ticks, want none",
			len(one.Changes.Selected))
	}
}

// A discard that applied to nothing leaves the selection where it was, for the
// reason a verb that applied to nothing does: the ticks are taken away as the
// discard consumes them, and one that consumed none has none to take.
func TestADiscardThatAppliedToNothingKeepsTheSelection(t *testing.T) {
	t.Parallel()
	build := func(t *testing.T) State {
		t.Helper()
		s := State{Width: 90, Height: 30}
		s = Apply(s, StatusLoaded{
			Head: git.Head{Branch: "main"},
			Rows: []git.Entry{{Path: "a.txt", Worktree: git.Modified}},
		})
		for _, row := range s.Rows {
			if row.Kind() == RowFile {
				s = Apply(s, SelectionToggled{Path: row.Path(), Section: row.Section()})
			}
		}
		if len(s.Changes.Selected) != 1 {
			t.Fatalf("%d rows are ticked, want one", len(s.Changes.Selected))
		}
		return Apply(s, VerbRequested{Verb: VerbDiscard, Targets: []string{"a.txt"}, Block: -1})
	}

	none := Apply(build(t), DiscardFinished{Applied: 0, Added: 1, Deleted: 1})
	if len(none.Changes.Selected) != 1 {
		t.Errorf("a discard that applied to nothing left %d ticks, want the one it started with",
			len(none.Changes.Selected))
	}

	one := Apply(build(t), DiscardFinished{Applied: 1, Added: 1, Deleted: 1})
	if len(one.Changes.Selected) != 0 {
		t.Errorf("a discard that applied to the row left %d ticks, want none",
			len(one.Changes.Selected))
	}
}
