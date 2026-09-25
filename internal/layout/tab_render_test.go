package layout

import (
	"testing"

	"github.com/catpotd/mirugit/internal/state"
)

// A nil field here is a tab that draws nothing with no error. renderFor answers
// for every tab, so the check is that every answer is complete.
func TestEveryTabHasItsRenderers(t *testing.T) {
	t.Parallel()
	for i := range int(state.TabCount) {
		tab := state.Tab(i)
		r := renderFor(tab)
		name := state.Facts[tab].Name
		if r.Pane == nil {
			t.Errorf("%s: Pane is nil", name)
		}
		if r.List == nil {
			t.Errorf("%s: List is nil", name)
		}
		if r.FooterVerbs == nil {
			t.Errorf("%s: FooterVerbs is nil", name)
		}
		if r.Bar == nil {
			t.Errorf("%s: Bar is nil", name)
		}
	}
}
