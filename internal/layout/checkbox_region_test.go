package layout

import (
	"strings"
	"testing"

	"github.com/catpotd/mirugit/internal/state"
)

// A checkbox is three cells wide and the click has to land on all three. The
// section heading's region started one cell to the right of the box it names,
// so the leftmost cell of the box did nothing and the cell after it toggled a
// section the reader was not pointing at.
//
// The three kinds of checkbox are checked together because they are drawn in
// the same column and were given three different regions.
func TestEveryCheckboxRegionCoversTheBoxItDraws(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := ninetyFileState()
	// A box is drawn only once something is selected, which is also when a
	// reader would reach for one.
	for _, row := range s.Rows {
		if row.Kind() == state.RowFile {
			s = state.Apply(s, state.SelectionToggled{Path: row.Path(), Section: row.Section()})
			break
		}
	}
	f := Pane(s, nil, w)

	boxes := 0
	for _, reg := range f.Regions {
		if reg.Target.Kind != TargetCheckbox {
			continue
		}
		boxes++
		line := stripSGR(f.Lines[reg.Row])
		// Columns are cells, and the cursor mark is one cell of three bytes.
		at := strings.Index(line, "[")
		drawn := -1
		if at >= 0 {
			drawn = w.Of(line[:at])
		}
		if drawn < 0 {
			t.Errorf("row %d has a checkbox region and no box: %q", reg.Row, line)
			continue
		}
		// The box is "[ ]" or "[✓]" or "[-]": three cells from the bracket.
		if reg.ColStart != drawn || reg.ColEnd != drawn+3 {
			t.Errorf("row %d draws its box at [%d,%d) and takes clicks at [%d,%d): %q",
				reg.Row, drawn, drawn+3, reg.ColStart, reg.ColEnd, strings.TrimRight(line, " "))
		}
	}
	if boxes == 0 {
		t.Fatal("no checkbox was drawn, so this proves nothing")
	}
}
