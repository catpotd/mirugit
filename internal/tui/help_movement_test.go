package tui

import (
	"strconv"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/state"
)

// The help's own first line reads "j k ↑ ↓ move the cursor", and the four are
// one answer everywhere else. Inside the help they were two: j and k scrolled
// it, and the arrows closed it and moved the list behind — so the key the
// reader had just read about did something else in the pane they read it in.
//
// The page and edge keys are the same question. A list long enough to scroll is
// what the help is; if pgdn moves a page in the list, it moves a page here.
func TestTheHelpMovesWithEveryMovementKey(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		key  tea.KeyPressMsg
		down bool
	}{
		{"j", tea.KeyPressMsg{Code: 'j'}, true},
		{"down arrow", tea.KeyPressMsg{Code: tea.KeyDown}, true},
		{"page down", tea.KeyPressMsg{Code: tea.KeyPgDown}, true},
		{"end", tea.KeyPressMsg{Code: tea.KeyEnd}, true},
		{"k", tea.KeyPressMsg{Code: 'k'}, false},
		{"up arrow", tea.KeyPressMsg{Code: tea.KeyUp}, false},
		{"page up", tea.KeyPressMsg{Code: tea.KeyPgUp}, false},
		{"home", tea.KeyPressMsg{Code: tea.KeyHome}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := fixed(t)
			// Short enough that the list of keys does not fit, which is the
			// only pane where moving means anything.
			m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 14})
			m.state = state.Apply(m.state, state.HelpOpened{})
			if !c.down {
				// Scrolling up from the top moves nothing, so the keys that go
				// up are asked from somewhere they can leave.
				m.state = state.Apply(m.state, state.HelpScrolled{By: 3})
			}
			before := m.state.HelpScroll

			next, _ := m.Update(c.key)
			m = next.(*Model)

			if !m.state.HelpOpen {
				t.Fatalf("%s closed the help; the help lists it as a way to move", c.name)
			}
			if c.down && m.state.HelpScroll <= before {
				t.Errorf("%s left the help at line %d", c.name, m.state.HelpScroll)
			}
			if !c.down && m.state.HelpScroll >= before {
				t.Errorf("%s left the help at line %d", c.name, m.state.HelpScroll)
			}
		})
	}
}

// end lands on the last line, and a distance near the top of the integer range
// would wrap to negative and land on the first: HelpScrolled adds to the offset
// without bounds and the clamp only holds the far end.
//
// A pane one row tall is the other edge. Height - 2 is negative there, and a
// page of a negative number of rows runs backwards.
func TestTheHelpReachesItsEndsFromAnyPaneSize(t *testing.T) {
	t.Parallel()
	for _, height := range []int{1, 2, 3, 14, 53} {
		t.Run(paneHeightName(height), func(t *testing.T) {
			t.Parallel()
			m := fixed(t)
			m.state = state.Apply(m.state, state.Resized{Width: 77, Height: height})
			m.state = state.Apply(m.state, state.HelpOpened{})
			// From a line other than the first: the offset is added to, so a
			// distance that would wrap only wraps once there is something to
			// add it to. Scrolling and then jumping to the end is the order a
			// reader arrives in anyway.
			next, _ := m.Update(tea.KeyPressMsg{Code: 'j'})
			m = next.(*Model)

			next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
			m = next.(*Model)
			end := m.state.HelpScroll
			if want := m.render.HelpScrollClamp(helpLinesForTest, m.state.Tab, m.helpRows()); end != want {
				t.Errorf("end left the help at line %d, and its last line is %d", end, want)
			}

			next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
			m = next.(*Model)
			if m.state.HelpScroll != end {
				t.Errorf("a page past the end moved to line %d", m.state.HelpScroll)
			}

			next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyHome})
			m = next.(*Model)
			if m.state.HelpScroll != 0 {
				t.Errorf("home left the help at line %d", m.state.HelpScroll)
			}
		})
	}
}

// helpLinesForTest asks the clamp for its far end, which is what end has to
// reach. The clamp holds it whatever it is given.
const helpLinesForTest = 1 << 20

func paneHeightName(height int) string {
	return "a pane " + strconv.Itoa(height) + " rows tall"
}
