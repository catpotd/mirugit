package layout

import (
	"strings"
	"testing"

	"github.com/catpotd/mirugit/internal/state"
)

// The unread dot on a tab is painted by replacing it in a line that already
// holds the tab names. The replacement is placed by counting cells, and a tab
// name of two-cell characters would otherwise cut the line mid-character.
//
// This was the one path in the drawing that no test reached, so the cell count
// it depends on was never run.
func TestTheUnreadDotIsPaintedWithoutCuttingTheLine(t *testing.T) {
	t.Parallel()
	w := Renderer{Palette: Palette{Enabled: true, Theme: slate}}
	p := w.Palette
	tabs := []Tab{
		{Logical: state.TabChanges, Name: "changes", Count: 3, Unread: true, Number: 1},
		{Logical: state.TabHistory, Name: "history", Count: 7, Number: 2},
		{Logical: state.TabStashed, Name: "stashed", Count: 1, Unread: true, Number: 3},
	}
	bar, _, regions := w.TabBar(tabs, 0, "", paneWidth)

	if len(regions) != len(tabs) {
		t.Fatalf("%d regions for %d tabs", len(regions), len(tabs))
	}
	// The line is as wide as the pane whether or not anything was painted:
	// a replacement placed by byte offset rather than by cell would shift it.
	// Widths.Of counts escape bytes as cells, so the colors come off first.
	plain := stripSGR(bar)
	if got := w.Of(plain); got != paneWidth {
		t.Errorf("the bar is %d cells, want %d: %q", got, paneWidth, plain)
	}
	for _, tab := range tabs {
		if !strings.Contains(plain, tab.Name) {
			t.Errorf("the bar does not name %q: %q", tab.Name, plain)
		}
	}
	// Two dots were asked for and two have to be there.
	if got := strings.Count(plain, "·"); got != 2 {
		t.Errorf("the bar draws %d unread dots, want 2: %q", got, plain)
	}
	// A tab the bar draws in full is painted: the dot is the mark the color
	// carries, and a bar that says "unread" in plain text on a wide pane has
	// lost it. The pane here is wide enough for every tab, so every dot the
	// bar draws is one the paint has to reach.
	if bar == plain {
		t.Errorf("nothing was painted on a bar with room for every tab: %q", bar)
	}
	if want := strings.Count(plain, "·"); strings.Count(bar, p.Mark(" ·")) != want {
		t.Errorf("the bar paints %d of its %d dots: %q",
			strings.Count(bar, p.Mark(" ·")), want, bar)
	}

	// Painting happens only with color on; without it the same line comes out.
	plainRenderer := Renderer{}
	bare, _, _ := plainRenderer.TabBar(tabs, 0, "", paneWidth)
	if stripSGR(bare) != plain {
		t.Errorf("color changed what the bar says:\n with: %q\n without: %q", plain, stripSGR(bare))
	}
}

// A pane too narrow for the whole bar keeps only the tabs that fit, and the
// regions of the ones cut off are dropped with them. The painting walks the
// tabs, not the regions, so every tab past the cut has no region to read: the
// bar is drawn from the same list either way, and a tab whose columns are gone
// is one the reader cannot see or click.
func TestTheUnreadDotIsNotPaintedOnATabTheBarNoLongerDraws(t *testing.T) {
	t.Parallel()
	w := Renderer{Palette: Palette{Enabled: true, Theme: slate}}
	tabs := []Tab{
		{Logical: state.TabChanges, Name: "changes", Count: 3, Unread: true, Number: 1},
		{Logical: state.TabHistory, Name: "history", Count: 7, Unread: true, Number: 2},
		{Logical: state.TabStashed, Name: "stashed", Count: 1, Unread: true, Number: 3},
		{Logical: state.TabWorktrees, Name: "worktrees", Count: 2, Unread: true, Number: 4},
	}
	clipped := 0
	for width := 1; width <= 70; width++ {
		bar, _, regions := w.TabBar(tabs, 0, "main", width)
		if len(regions) < len(tabs) {
			clipped++
		}
		// The bar is as wide as the pane at every width, painted or not.
		if got := w.Of(stripSGR(bar)); got != width {
			t.Errorf("at %d columns the bar is %d cells: %q", width, got, stripSGR(bar))
		}
		// A dot is drawn only for a tab the bar still holds.
		if dots := strings.Count(stripSGR(bar), "·"); dots > len(regions) {
			t.Errorf("at %d columns the bar draws %d dots for %d regions",
				width, dots, len(regions))
		}
	}
	if clipped == 0 {
		t.Fatal("no width dropped a region, so this proves nothing")
	}
}
