package state

import (
	"reflect"
	"testing"
)

// Apply asks five groups in turn and stops at the first one that reports it
// handled the event. A case that reports false instead falls past every other
// group and reaches the end, where Apply answers the state it was given: the
// event does nothing, silently. Folding a directory could be turned into that
// with the whole suite still green.
//
// The groups are driven directly rather than through Apply, because the report
// is what breaks and Apply does not return it. Two groups claiming the same
// event is the other way this goes wrong: the second one never runs, and which
// one wins depends on the order they are asked in.
func TestExactlyOneGroupHandlesEachEvent(t *testing.T) {
	t.Parallel()
	groups := []struct {
		name string
		fn   func(State, Event) (State, bool)
	}{
		{"applyNavigation", applyNavigation},
		{"applyLoaded", applyLoaded},
		{"applySelection", applySelection},
		{"applyVerb", applyVerb},
		{"applyCommitMessage", applyCommitMessage},
	}
	for _, e := range everyEvent() {
		name := reflect.TypeOf(e).Name()
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var took []string
			for _, g := range groups {
				s := State{Changes: Changes{
					Selected: map[string]bool{},
					Folded:   map[string]bool{},
					Stale:    map[string]bool{},
				}}
				if _, ok := g.fn(s, e); ok {
					took = append(took, g.name)
				}
			}
			if len(took) != 1 {
				t.Errorf("%d groups report handling %s: %v", len(took), name, took)
			}
		})
	}
}
