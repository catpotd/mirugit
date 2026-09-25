package state

import "testing"

// The notice is the only thing that tells the reader what a verb did, and the
// four arms that pick its wording are ordered: the first one that matches wins.
// Two tests drove VerbStage through this switch and neither read the notice, so
// a mutation that let the stash wording answer for every verb survived the
// suite: staging one file said "stashed 1 file".
//
// This drives every arm, and the combinations that must stay silent.
func TestVerbFinishedPicksItsWording(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		event  VerbFinished
		notice string
	}{
		{"a notice from the verb wins over the wording here",
			VerbFinished{Verb: VerbStash, Notice: "nothing to stash", Applied: 2},
			"nothing to stash"},
		{"stash that applied nothing names the files it skipped",
			VerbFinished{Verb: VerbStash, Ignored: []string{"a", "b"}},
			"stashed nothing · 2 files had no changes"},
		{"another verb counts the files it skipped",
			VerbFinished{Verb: VerbStage, Ignored: []string{"a"}},
			"1 unchanged"},
		{"discard says nothing about what it skipped",
			VerbFinished{Verb: VerbDiscard, Ignored: []string{"a"}},
			""},
		{"stash counts what it took",
			VerbFinished{Verb: VerbStash, Applied: 3},
			"stashed 3 files"},
		{"stage says nothing about what it took",
			VerbFinished{Verb: VerbStage, Applied: 3},
			""},
		{"discard says nothing about what it took",
			VerbFinished{Verb: VerbDiscard, Applied: 3},
			""},
		{"unstage says nothing about what it took",
			VerbFinished{Verb: VerbUnstage, Applied: 1},
			""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := State{Changes: Changes{Selected: map[string]bool{}}}
			s = Apply(s, c.event)
			if s.Notice != c.notice {
				t.Errorf("notice is %q, want %q", s.Notice, c.notice)
			}
		})
	}
}
