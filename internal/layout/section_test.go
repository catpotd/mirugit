package layout

import (
	"strings"
	"testing"

	"github.com/catpotd/mirugit/internal/state"
)

func TestSectionHeadingIsExactly77Cells(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	line, _ := w.Heading("staged", 1, 7, 1, state.SectionStaged, "", "", paneWidth)
	if got := w.Of(line); got != paneWidth {
		t.Errorf("got %d cells, want %d: %q", got, paneWidth, line)
	}
}

func TestSectionHeadingWithNoSelectionStartsAtSixSpaces(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	line, _ := w.Heading("staged", 1, 7, 1, state.SectionStaged, "", "", paneWidth)
	if !strings.HasPrefix(line, "      staged") {
		t.Errorf("got %q, want six spaces before the name", line)
	}
}

func TestSectionHeadingWithPartialSelectionShowsDashCheckbox(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	line, _ := w.Heading("changes", 5, 259, 3, state.SectionUnstaged, "[-]", "", paneWidth)
	if !strings.HasPrefix(line, " [-]  changes") {
		t.Errorf("got %q, want partial-selection checkbox", line)
	}
}

func TestSectionHeadingWithFullSelectionShowsTickCheckbox(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	line, _ := w.Heading("staged", 2, 10, 1, state.SectionStaged, "[✓]", "", paneWidth)
	if !strings.HasPrefix(line, " [✓]  staged") {
		t.Errorf("got %q, want full-selection checkbox", line)
	}
}

// The mark sits in column zero on a heading exactly as it does on a file row.
// It used to land behind the checkbox, which the heading grows only once the
// cursor arrives, so the column changed with the cursor.
//
// Both answers the terminal can give about the mark's width are swept. A
// heading built with room for one cell and marked with two came out a cell
// wider than the pane; a version that refused to mark it unless the room
// matched drew no mark at all, and the reader could not see which row the
// cursor was on.
func TestCursorOnASectionHeadingSitsInColumnZero(t *testing.T) {
	t.Parallel()
	for _, eastAsian := range []bool{false, true} {
		w := Renderer{Widths: Widths{EastAsian: eastAsian}}
		for _, checkbox := range []string{"", "[ ]", "[-]", "[✓]"} {
			line, _ := w.Heading("changes", 2, 7, 1, state.SectionUnstaged, checkbox, "", paneWidth)
			got := w.cursorSectionHeading(line)
			if !strings.HasPrefix(got, cursorMark) {
				t.Errorf("eastAsian=%v checkbox %q: got %q, want the mark first",
					eastAsian, checkbox, got)
			}
			if w.Of(got) != paneWidth {
				t.Errorf("eastAsian=%v checkbox %q: %d cells, want %d",
					eastAsian, checkbox, w.Of(got), paneWidth)
			}
		}
	}
}
