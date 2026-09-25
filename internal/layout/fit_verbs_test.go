package layout

import (
	"strings"
	"testing"

	"github.com/catpotd/mirugit/internal/state"
)

// fitVerbs drops from the middle while there are more than two: the first verb
// is what the reader came for and the last is the way out of the selection,
// which a reader on the mouse has no other route to. Down to two, there is no
// middle left and the rest goes from the end.
//
// Letting the first loop run at two takes the one before the last, so the first
// verb — the one the row is about — is the one that goes, and the exit stays.
func TestFitVerbsDropsFromTheMiddleAndKeepsTheEnds(t *testing.T) {
	t.Parallel()
	var w Renderer
	all := []state.VerbName{
		state.VerbNameStage, state.VerbNameStash, state.VerbNameDiscard, state.VerbNameSelect,
	}
	width := func(v []state.VerbName) int {
		return w.Of(strings.Join(keyedVerbs(v), "   "))
	}

	for _, c := range []struct {
		name string
		in   []state.VerbName
		room int
		want []state.VerbName
	}{
		{"room for all of them", all, width(all), all},
		{"one short of all", all, width(all) - 1,
			[]state.VerbName{state.VerbNameStage, state.VerbNameStash, state.VerbNameSelect}},
		{"room for two", all, width([]state.VerbName{state.VerbNameStage, state.VerbNameSelect}),
			[]state.VerbName{state.VerbNameStage, state.VerbNameSelect}},
		{"room for one", all, width(all[:1]),
			[]state.VerbName{state.VerbNameStage}},
		{"no room at all", all, 0, nil},
		{"two that fit", all[:2], width(all[:2]), all[:2]},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := fitVerbs(w, append([]state.VerbName(nil), c.in...), c.room)
			if len(got) != len(c.want) {
				t.Fatalf("kept %v, want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("verb %d is %q, want %q: %v", i, got[i], c.want[i], got)
				}
			}
			if len(got) > 0 && width(got) > c.room {
				t.Errorf("what was kept is %d cells, wider than the %d it had", width(got), c.room)
			}
		})
	}
}
