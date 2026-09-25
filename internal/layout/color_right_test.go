package layout

import (
	"strings"
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// colorRight paints the right half of a row. A caller that colors only its last
// column has to say both which column that is and how to paint it: with one and
// not the other there is nothing to paint, and painting anyway hands paintTail
// a different number of cells than it took away, which leaves the row short.
func TestColoringTheLastColumnNeedsBothThePainterAndTheColumns(t *testing.T) {
	t.Parallel()
	paint := func() string { return "<painted>" }

	for _, c := range []struct {
		name    string
		in      subjectRowInput
		right   string
		painted bool
	}{
		{"a painter and the columns", subjectRowInput{
			ColorLastColumn: paint, RightColumns: []string{"a", "main"}}, "a main", true},
		{"a painter and no columns", subjectRowInput{
			ColorLastColumn: paint}, "a main", false},
		{"the columns and no painter", subjectRowInput{
			RightColumns: []string{"a", "main"}}, "a main", false},
		{"neither", subjectRowInput{}, "a main", false},
		{"a painter, columns, and a right half that does not end with the last one",
			subjectRowInput{ColorLastColumn: paint, RightColumns: []string{"a", "main"}},
			"a other", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := c.in.colorRight(c.right)
			if strings.Contains(got, "<painted>") != c.painted {
				t.Errorf("colorRight(%q) = %q, want it painted = %v", c.right, got, c.painted)
			}
			if !c.painted && got != c.right {
				t.Errorf("colorRight(%q) changed it to %q", c.right, got)
			}
		})
	}
}

// The verbs and the stale mark are drawn beside the row the cursor is on, and
// their regions are what a click aims at. A row the cursor is elsewhere from
// draws neither, so it carries no region for them: putting one there gives the
// reader a target on a row that shows nothing.
func TestOnlyTheCursorRowCarriesRegionsForItsVerbs(t *testing.T) {
	t.Parallel()
	var w Renderer
	verbs := []state.VerbName{state.VerbNameSelect}

	for _, c := range []struct {
		name   string
		cursor bool
		stale  bool
		want   bool
	}{
		{"the cursor row with verbs", true, false, true},
		{"the cursor row that is stale", true, true, true},
		{"another row with verbs", false, false, false},
		{"another row that is stale", false, true, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r := Row{Entry: git.Entry{Path: "a.txt", Worktree: git.Modified}, Index: 2}
			r.State.Cursor = c.cursor
			rr := rowRight{text: "s stage", verbs: verbs, fieldWidth: 7, stale: c.stale}

			verbRegion := false
			for _, reg := range w.fileRowRegions(r, rr, 4, 20, 60) {
				if reg.Target.Kind == TargetVerb {
					verbRegion = true
				}
			}
			if verbRegion != c.want {
				t.Errorf("a region for the verbs = %v, want %v", verbRegion, c.want)
			}
		})
	}
}
