package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
)

// The probe asks the terminal where the cursor is, draws one ambiguous-width
// glyph, then asks again: the difference is how many columns that terminal
// gives the glyph. Every row drawn afterwards is measured with the answer.
//
// The second question is asked by the command returned for
// widthProbeAfterDrawMsg, and a mutation that dropped that command survived the
// suite. Dropping it is silent: the deadline settles the probe half a second
// later with the guess, and the reader sees a pane measured for the wrong
// terminal. So this runs the commands the way the program does rather than
// handing the model the second answer itself.
func TestTheWidthProbeAsksTheTerminalTwice(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name      string
		second    int
		eastAsian bool
	}{
		{"a terminal that gives the glyph two columns", 12, true},
		{"a terminal that gives it one", 11, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := &Model{render: layout.Renderer{Widths: layout.Widths{IsWidthAssumed: true}},
				read: emptyRead(), state: state.State{Width: 80, Height: 24}}

			answers := []int{10, c.second}
			queue := []tea.Msg{tea.CursorPositionMsg{X: answers[0]}}
			answers = answers[1:]
			for range 10 {
				if len(queue) == 0 || m.probe.settled {
					break
				}
				msg := queue[0]
				queue = queue[1:]
				next, cmd := m.Update(msg)
				m = next.(*Model)
				if cmd == nil {
					continue
				}
				out := cmd()
				if out == tea.RequestCursorPosition() {
					if len(answers) == 0 {
						t.Fatal("the terminal was asked for the cursor a third time")
					}
					queue = append(queue, tea.CursorPositionMsg{X: answers[0]})
					answers = answers[1:]
					continue
				}
				queue = append(queue, out)
			}

			if !m.probe.settled {
				t.Fatal("the probe never finished: the second question was never asked")
			}
			if m.render.IsWidthAssumed {
				t.Error("the width is still the guess after the terminal answered")
			}
			if m.render.EastAsian != c.eastAsian {
				t.Errorf("EastAsian is %v, want %v", m.render.EastAsian, c.eastAsian)
			}
		})
	}
}

// The probe waits for the terminal to say where the cursor landed, and the
// timeout is what ends the wait when no answer comes. It has to reach the
// handler through Update: reported as unhandled, the message falls through to
// updateResult, which knows nothing about it, and the pane draws the probe
// glyph for the rest of the session with no rows on it.
func TestTheWidthProbeTimeoutSettlesTheProbeThroughUpdate(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	m.probe.settled = false
	m.render.EastAsian = true
	m.render.IsWidthAssumed = false

	next, _ := m.Update(widthProbeTimeoutMsg{})
	m = next.(*Model)

	if !m.probe.settled {
		t.Error("the timeout did not settle the probe")
	}
	if !m.render.IsWidthAssumed {
		t.Error("the width was not marked as assumed after the timeout")
	}
	if m.render.EastAsian {
		t.Error("the timeout left the wide-character assumption on")
	}
}
