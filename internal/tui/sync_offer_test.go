package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
)

// The sync button and the S key are one offer. They were two predicates in two
// layers, and the only thing tying them together was a comment on one saying it
// matched the other.
//
// Every combination of upstream and distance is driven through both, because
// the two agreed on the cases anyone had thought to try.
func TestTheSyncKeyRunsExactlyWhereTheButtonIs(t *testing.T) {
	t.Parallel()
	for _, head := range []git.Head{
		{Branch: "main"},
		{Branch: "main", HasUpstream: true},
		{Branch: "main", HasUpstream: true, Ahead: 1},
		{Branch: "main", HasUpstream: true, Behind: 1},
		{Branch: "main", HasUpstream: true, Behind: 2, Ahead: 3},
		{Branch: "main", Ahead: 1},
		{Branch: "main", Behind: 1},
	} {
		t.Run(headName(head), func(t *testing.T) {
			t.Parallel()
			m := newParentModel(t)
			m.state = state.Apply(m.state, state.StatusLoaded{
				Rows: []git.Entry{{Path: "a.txt", Worktree: git.Modified}},
				Head: head,
			})
			m.View()

			drawn := false
			for _, line := range m.frame.Lines {
				if strings.Contains(line, "[sync ") || strings.Contains(line, "[push ") ||
					strings.Contains(line, "[pull ") {
					drawn = true
					break
				}
			}

			// requestSync is where the guard sits and where the S key, the
			// help line and the click on the button all arrive. Update wraps
			// its answer with the follow commands, so the cmd it returns says
			// nothing about whether sync ran.
			_, cmd := m.requestSync()
			if ran := cmd != nil; ran != drawn {
				t.Errorf("sync runs %v while the button is drawn %v", ran, drawn)
			}

			// The key has to reach it rather than answering on its own.
			next, _ := m.Update(tea.KeyPressMsg{Code: 's', Text: "S", Mod: tea.ModShift})
			if _, ok := next.(*Model); !ok {
				t.Fatal("S did not come back with the model")
			}

			// The button is the third way in, and the one a reader on the mouse
			// has. A click that reaches nothing leaves the button drawn and
			// dead.
			if !drawn {
				return
			}
			region, found := syncButtonRegion(m)
			if !found {
				t.Fatal("the button is drawn and no region covers it")
			}
			if _, cmd := m.click(region, 0); cmd == nil {
				t.Error("clicking the sync button ran nothing")
			}
		})
	}
}

func headName(h git.Head) string {
	name := "no upstream"
	if h.HasUpstream {
		name = "upstream"
	}
	return name + ", " + string(rune('0'+h.Behind)) + " behind, " +
		string(rune('0'+h.Ahead)) + " ahead"
}

func syncButtonRegion(m *Model) (layout.Target, bool) {
	for _, r := range m.frame.Regions {
		if r.Target.Kind == layout.TargetButton {
			return r.Target, true
		}
	}
	return layout.Target{}, false
}
