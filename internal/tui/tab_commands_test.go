package tui

import (
	"testing"

	"github.com/catpotd/mirugit/internal/state"
)

// A nil field here is a tab that silently never reloads, never opens a nested
// file, or never follows its cursor. commandsFor answers for every tab, so the
// check is that every answer is complete.
func TestEveryTabHasItsCommands(t *testing.T) {
	t.Parallel()
	for i := range int(state.TabCount) {
		tab := state.Tab(i)
		c := commandsFor(tab)
		name := state.Facts[tab].Name
		if c.Reload == nil {
			t.Errorf("%s: Reload is nil", name)
		}
		if c.NestedDiff == nil {
			t.Errorf("%s: NestedDiff is nil", name)
		}
		if c.ExpandRow == nil {
			t.Errorf("%s: ExpandRow is nil", name)
		}
		if c.KeyAtCursor == nil {
			t.Errorf("%s: KeyAtCursor is nil", name)
		}
		if c.MarkCursorRead == nil {
			t.Errorf("%s: MarkCursorRead is nil", name)
		}
	}
}
