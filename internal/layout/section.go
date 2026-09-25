package layout

import (
	"fmt"
	"strings"

	"github.com/catpotd/mirugit/internal/state"
)

// Heading counts what a tab groups, because a heading that hides the size of what
// it groups is the elision this program refuses everywhere else. All four tabs pass
// through this one formatter so name, count, and column stay aligned.
func (w Renderer) Heading(
	name string,
	count int,
	added, deleted int,
	sec state.Section,
	checkbox string,
	right string,
	width int,
) (string, []Region) {
	var body string
	if sec == state.SectionStaged || sec == state.SectionUnstaged {
		body = fmt.Sprintf("%s · %d %s", name, count, state.FileWord(count))
		if added != 0 || deleted != 0 {
			body = fmt.Sprintf("%s · %s %s", body, state.Plus(added), state.Minus(deleted))
		}
	} else {
		body = fmt.Sprintf("%s · %d", name, count)
	}

	var left string
	if checkbox == "" {
		left = "      " + body
	} else {
		// The box sits in the same column as the file boxes below it: after the
		// column the cursor owns. That column is as wide as the cursor mark,
		// which is one cell on most terminals and two where ambiguous
		// characters are drawn wide — and the mark is put in afterwards, so the
		// room for it has to be here.
		left = strings.Repeat(" ", w.cursorMarkWidth()) + checkbox + "  " + body
	}

	// Pad overflows rather than truncating, so a narrow pane drew a heading wider
	// than the pane and pushed every row below out of place.
	if room := width - w.Of(right); w.Of(left) > room {
		left = w.Truncate(left, room)
	}
	line := w.Pad(left, right, width)
	if sec == state.SectionStaged || sec == state.SectionUnstaged {
		line = replaceOnce(line, checkbox, colorCheckbox(w.Palette, checkbox))
		line = w.colorCounts(line)
	}
	line = w.colorDots(line)

	if sec != state.SectionStaged && sec != state.SectionUnstaged {
		return line, nil
	}

	regions := []Region{{
		Target:   Target{Kind: TargetSectionHeading, Section: sec},
		ColStart: 0,
		ColEnd:   width,
	}}
	if checkbox != "" {
		boxStart, boxEnd := w.checkboxColumns()
		regions = append([]Region{{
			Target:   Target{Kind: TargetCheckbox, Section: sec},
			ColStart: boxStart,
			ColEnd:   boxEnd,
		}}, regions...)
	}
	return line, regions
}

// cursorMark is what marks the row the cursor is on, in the column to the left
// of everything else.
const cursorMark = "▌"

// cursorMarkWidth is how many columns that leaves for it.
func (w Renderer) cursorMarkWidth() int {
	return max(w.Of(cursorMark), 1)
}
