package layout

import (
	"fmt"
	"testing"

	"github.com/catpotd/mirugit/internal/git"

	"github.com/catpotd/mirugit/internal/state"
)

// A pane is drawn while the reader is in the middle of something: rows picked
// out, a question waiting for an answer, a commit message being typed. Each
// puts different text in the header and different marks in the gutters, and the
// sweep that runs the tabs draws none of them.
//
// It runs one height and two renderers rather than the full cross of the other
// sweep: what this one varies is the mode, and the width is the axis the modes
// interact with.
func TestEveryModeDrawsARectangle(t *testing.T) {
	t.Parallel()
	for _, w := range []Renderer{{}, {Widths: Widths{EastAsian: true},
		Palette: Palette{Enabled: true}, Clipboard: true, Browser: true}} {
		for _, tab := range []state.Tab{
			state.TabChanges, state.TabHistory, state.TabStashed, state.TabWorktrees,
		} {
			for _, mode := range paneModes() {
				for _, width := range sweptWidths() {
					s := mode.reach(fittingState(tab, width, 24))
					checkFrameFits(t, w, s, fmt.Sprintf("%s on %s at width %d color=%v",
						mode.name, state.Facts[tab].Name, width, w.Enabled))
				}
			}
		}
	}
}

type paneMode struct {
	name  string
	reach func(state.State) state.State
}

// paneModes are the states a keypress puts the pane into and leaves it in.
func paneModes() []paneMode {
	return []paneMode{
		{"rows picked out", func(s state.State) state.State {
			return state.Apply(s, state.TabSelectionToggled{})
		}},
		{"a discard waiting for an answer", func(s state.State) state.State {
			return state.Apply(s, state.DiscardConfirmationShown{
				Confirm: state.DiscardConfirm{Files: 3, Added: 4, Deleted: 5}})
		}},
		{"a binary file open", func(s state.State) state.State {
			return openDiffOnCursorWith(s, func(d git.FileDiff) git.FileDiff {
				d.Blocks = nil
				d.Binary = true
				d.Size = 1234567
				d.Mode = "100644"
				return d
			})
		}},
		{"a file whose only change is its mode", func(s state.State) state.State {
			return openDiffOnCursorWith(s, func(d git.FileDiff) git.FileDiff {
				d.Blocks = nil
				d.ModeOnly = true
				return d
			})
		}},
		{"the help open", func(s state.State) state.State {
			return state.Apply(s, state.HelpOpened{})
		}},
		{"the help scrolled", func(s state.State) state.State {
			s = state.Apply(s, state.HelpOpened{})
			return state.Apply(s, state.HelpScrolled{By: 4})
		}},
		{"a message being typed", func(s state.State) state.State {
			s = state.Apply(s, state.MessageFocused{})
			return state.Apply(s, state.MessageEdited{
				Text: "日本語のコミットメッセージを書いている途中"})
		}},
	}
}
