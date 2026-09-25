package layout

import (
	"strings"
	"testing"

	"github.com/catpotd/mirugit/internal/state"
)

// The box says whether a whole directory or section is selected, and it is the
// one thing on those two rows that changes as the reader ticks files. Nothing
// checked that it is drawn in the mark color: a mutation that colored the box
// only on the rows that have none survived the suite.
func TestTheSelectionBoxIsDrawnInTheMarkColor(t *testing.T) {
	t.Parallel()
	const width = 80
	w := Renderer{Palette: Palette{Enabled: true}}

	for _, box := range []string{"[✓]", "[-]"} {
		colored := colorCheckbox(w.Palette, box)
		if colored == box {
			t.Fatalf("the palette leaves %q unchanged, so this test proves nothing", box)
		}

		t.Run("directory "+box, func(t *testing.T) {
			t.Parallel()
			line, _ := w.directoryRow("src", 3, box, false, false, 0, width)
			if !strings.Contains(line, colored) {
				t.Errorf("the box is drawn plain: %q", line)
			}
		})

		t.Run("section "+box, func(t *testing.T) {
			t.Parallel()
			line, _ := w.Heading("unstaged", 3, 1, 1, state.SectionUnstaged, box, "", width)
			if !strings.Contains(line, colored) {
				t.Errorf("the box is drawn plain: %q", line)
			}
		})
	}
}

// The box and the counts are colored on the two headings that have files under
// them and on no other. The unstaged heading is the one the test above draws,
// so the staged half of that test and the section that has neither were both
// open: joining them the wrong way leaves the staged heading plain and paints a
// heading that has no box.
func TestOnlyTheHeadingsWithFilesUnderThemAreColored(t *testing.T) {
	t.Parallel()
	const width = 80
	const box = "[✓]"
	w := Renderer{Palette: Palette{Enabled: true}}
	colored := colorCheckbox(w.Palette, box)
	if colored == box {
		t.Fatal("the palette leaves the box unchanged, so this proves nothing")
	}

	for _, c := range []struct {
		name string
		sec  state.Section
		want bool
	}{
		{"staged", state.SectionStaged, true},
		{"unstaged", state.SectionUnstaged, true},
		{"a heading with no section of its own", state.SectionNone, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			line, _ := w.Heading(c.name, 3, 1, 1, c.sec, box, "", width)
			if got := strings.Contains(line, colored); got != c.want {
				t.Errorf("the box is colored = %v, want %v: %q", got, c.want, line)
			}
		})
	}
}
