package layout

import (
	"strings"
	"testing"

	"github.com/catpotd/mirugit/internal/state"
)

// A staged or unstaged heading prints the lines added and removed after the
// file count, and prints neither when there are none: "+0 −0" on a section that
// changed no line is a number the reader has to work out means nothing.
//
// Both halves of the condition were open. Inverting the deleted half leaves
// every heading test green, because they all pass counts that are not zero.
func TestAHeadingPrintsLineCountsOnlyWhenThereAreSome(t *testing.T) {
	t.Parallel()
	var w Renderer
	const width = 60

	for _, c := range []struct {
		name           string
		added, deleted int
		wantCounts     bool
	}{
		{"lines added and removed", 3, 4, true},
		{"only added", 3, 0, true},
		{"only removed", 0, 4, true},
		{"neither", 0, 0, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			line, _ := w.Heading("staged", 2, c.added, c.deleted,
				state.SectionStaged, "", "", width)
			got := strings.Contains(line, state.Plus(c.added)) &&
				strings.Contains(line, state.Minus(c.deleted))
			if got != c.wantCounts {
				t.Errorf("counts on the heading = %v, want %v: %q", got, c.wantCounts, line)
			}
		})
	}
}
