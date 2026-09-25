package layout

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/catpotd/mirugit/internal/state"
)

// A row keeps a blank cell between what it is called and what sits to its right.
// Without one, a name cut short ran straight into the next column and read as a
// single word: "…u uncommit", where the mark that says text was dropped looks
// like part of the key. Measured on the history tab at 60 and 80 columns.
//
// It reads the drawn rows rather than the code that lays them out, because the
// two fields are built in different places and only meet on the screen.
func TestARowKeepsACellBetweenItsNameAndWhatFollows(t *testing.T) {
	t.Parallel()
	for _, w := range []Renderer{{}, {Clipboard: true, Browser: true}} {
		for _, tab := range []state.Tab{
			state.TabChanges, state.TabHistory, state.TabStashed, state.TabWorktrees,
		} {
			for _, width := range sweptWidths() {
				for _, cursorAt := range []int{0, 3} {
					s := fittingState(tab, width, 24)
					s = state.Apply(s, state.CursorMoved{By: cursorAt})
					f := Pane(s, nil, w)
					for _, reg := range f.Regions {
						if reg.Target.Kind != TargetFile || reg.Row >= len(f.Lines) {
							continue
						}
						if reg.ColEnd >= width {
							continue
						}
						// Either the last cell the name was given is blank, or the
						// one after it is. Which of the two depends on the tab:
						// a worktree's columns keep their own trailing blank, a
						// file row's name ends where its text ends.
						plain := ansi.Strip(f.Lines[reg.Row])
						pair := drawnCells(w, plain, reg.ColEnd-1, reg.ColEnd+1)
						if !strings.Contains(pair, " ") {
							t.Errorf("tab %s at width %d: the name runs into the column beside it at %d: %q",
								state.Facts[tab].Name, width, reg.ColEnd, plain)
						}
					}
				}
			}
		}
	}
}

// A row does not move sideways as the cursor arrives at it. The box appears
// only on the row the cursor is on, and a row that closed up the box's columns
// when there was no box drew its own text one column further left: measured,
// the fold arrow sat at column 6 until the cursor reached the row and at 7
// after, wherever the cursor mark is drawn in two cells.
func TestArrivingAtARowDoesNotMoveIt(t *testing.T) {
	t.Parallel()
	for _, w := range []Renderer{{}, {Widths: Widths{EastAsian: true}}} {
		for _, width := range sweptWidths() {
			columns := map[string]int{}
			for _, at := range cursorRowKinds(fittingState(state.TabChanges, width, 24)) {
				s := state.Apply(fittingState(state.TabChanges, width, 24),
					state.CursorMoved{By: at.by})
				for _, line := range Pane(s, nil, w).Lines {
					plain := ansi.Strip(line)
					if col := columnOf(w, plain, "▾ "); col >= 0 {
						name := strings.TrimSpace(plain)
						if prior, seen := columns[name]; seen && col != prior {
							t.Errorf("width %d color=%v: %q sits at %d and %d",
								width, w.Enabled, name, prior, col)
						}
						columns[name] = col
					}
				}
			}
		}
	}
}

// columnOf answers which cell text starts at, or -1. It counts cells rather
// than bytes: a mark before it may be three bytes and one cell.
func columnOf(w Renderer, line, text string) int {
	at := strings.Index(line, text)
	if at < 0 {
		return -1
	}
	return w.Of(line[:at])
}
