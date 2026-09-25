package layout

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// The row between two blocks is blank unless the theme marks it and color is
// on. Both halves matter and neither was drawn by a test: the mark is only in
// one of the two themes, and joining the halves the wrong way divides by the
// width of an empty mark on the theme that has none.
func TestTheGapBetweenBlocksIsMarkedOnlyWhenTheThemeSaysSoInColor(t *testing.T) {
	t.Parallel()
	const width = 8
	blank := strings.Repeat(" ", width)

	for _, c := range []struct {
		name  string
		w     Renderer
		blank bool
	}{
		{"a theme with no mark, in color", Renderer{Palette: Palette{Enabled: true}}, true},
		{"a theme with no mark, plain", Renderer{}, true},
		{"a theme with a mark, plain", Renderer{Palette: Palette{Theme: ThemeByName("cozmic")}}, true},
		{"a theme with a mark, in color",
			Renderer{Palette: Palette{Enabled: true, Theme: ThemeByName("cozmic")}}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := c.w.blockGap(width)
			if c.blank {
				if got != blank {
					t.Errorf("the gap is %q, want %d blanks", got, width)
				}
				return
			}
			if got == blank {
				t.Errorf("the theme marks the gap and it was drawn blank: %q", got)
			}
			if w := c.w.Of(ansi.Strip(got)); w != width {
				t.Errorf("the marked gap is %d cells, want %d: %q", w, width, got)
			}
		})
	}
}
