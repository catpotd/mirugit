package layout

import (
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// A pane one line tall is what a reader gets while dragging a split, and the
// frame still has to be a rectangle: as many lines as the height, each exactly
// as wide as the width. The width has a floor below which the pane says it is
// too narrow; the height has none, so every height has to draw.
//
// Height and width are swept together because the split between the list and
// the diff is arithmetic on both, and neither is safe to hold still. The widths
// step one column at a time at the narrow end: measured, three defects lived in
// bands that a list of round numbers stepped over.
//
// The heights run past where the diff area opens. Stopping at 13 meant the
// diff was asked for at every height and drawn at none: the area closes below
// minDiffRows, so none of diff.go was reached by any sweep.
func TestEveryHeightDrawsARectangle(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	// Long enough to need cutting at the narrow end of the sweep: a header and a
	// line that both fit every width tell nothing about the widths where they do
	// not. Measured, dropping the block header's truncation left this test green
	// while the header was eleven cells.
	// Lines long enough to need cutting at the narrow end of the sweep. A diff
	// whose lines fit every width says nothing about the widths where they do
	// not.
	//
	// Two branches stay out of reach here, and inflating this fixture does not
	// buy them: blockHeadLine's truncation needs "block 100/1000" to pass 20
	// cells, so about a thousand blocks, and DiffHeader's needs a path long
	// enough to crowd out "esc close", which means a differently named row.
	// Both were measured: 120 blocks left the truncation unreached and lowered
	// the coverage of padToHeight.
	diff := git.FileDiff{Path: "f00.txt", Blocks: []git.Block{
		{Header: "@@ -1,40 +1,42 @@ func aFunctionWithARatherLongName(argument int) error {",
			Lines: []string{
				"+a line of added code long enough to need cutting in a narrow pane",
				"-a line of removed code long enough to need cutting in a narrow pane",
				" a context line long enough to need cutting in a narrow pane",
			}, Hash: "h1"},
		{Header: "@@ -90,3 +92,4 @@ func another(argument string) (string, error) {",
			Lines: []string{"+three"}, Hash: "h2"},
	}}
	for tab := range state.TabCount {
		t.Run(state.Facts[tab].Name, func(t *testing.T) {
			t.Parallel()
			// peek is its own shape: it holds the diff to minDiffRows and gives
			// the rest to the list, by arithmetic the full-open path does not
			// run. Without it, the chrome rows that path subtracts were counted
			// by no test at all.
			for _, opened := range []struct{ open, peek bool }{
				{false, false}, {true, false}, {true, true},
			} {
				open, peek := opened.open, opened.peek
				for height := range 30 {
					for _, width := range sweptWidths() {
						s := paneStateFor(t, tab)
						s.Height = height
						s.Width = width
						if open {
							s = state.Apply(s, state.DiffOpened{Path: "f00.txt", Peek: peek,
								Origin: state.WorkingTree(state.SectionUnstaged)})
							s = state.Apply(s, state.DiffLoaded{Diff: diff})
						}
						f := Pane(s, nil, w)

						if len(f.Lines) != height {
							t.Errorf("height %d open=%v peek=%v: the frame has %d lines",
								height, open, peek, len(f.Lines))
						}
						for i, line := range f.Lines {
							if got := w.Of(line); got != s.Width {
								t.Errorf("height %d open=%v peek=%v width %d line %d: %d cells, want %d",
									height, open, peek, s.Width, i, got, s.Width)
							}
						}
						// The footer is appended last and the frame is then cut
						// to the height, so a body that drew one line too many
						// pushes the footer off the bottom while the line count
						// still comes out right. The last line being the footer
						// is the part of an overflow that cannot hide.
						//
						// A pane too narrow to draw at all says so instead and
						// has no footer to check.
						if width >= minPaneWidth && height >= HeaderRowsOf(s)+2 && len(f.Lines) > 0 {
							last := ansi.Strip(f.Lines[len(f.Lines)-1])
							if want := w.footerPlain(s, s.Width); last != want {
								t.Errorf("height %d open=%v peek=%v width %d: the last line is not the footer: %q",
									height, open, peek, width, last)
							}
						}
						// A region that names a line the frame does not have sends
						// a click into nothing.
						for _, reg := range f.Regions {
							if reg.Row < 0 || reg.Row >= len(f.Lines) {
								t.Errorf("height %d open=%v peek=%v width %d: a region points at line %d of %d",
									height, open, peek, width, reg.Row, len(f.Lines))
							}
						}
					}
				}
			}
		})
	}
}

// The help overlay replaces the pane, so it answers for the height on its own.
func TestEveryHeightDrawsTheHelpAsARectangle(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	for tab := range state.TabCount {
		t.Run(state.Facts[tab].Name, func(t *testing.T) {
			t.Parallel()
			for height := range 14 {
				s := paneStateFor(t, tab)
				s.Height = height
				s = state.Apply(s, state.HelpOpened{})
				f := Pane(s, nil, w)

				if len(f.Lines) != height {
					t.Errorf("height %d: the help has %d lines", height, len(f.Lines))
				}
				for i, line := range f.Lines {
					if got := w.Of(line); got != s.Width {
						t.Errorf("height %d line %d: %d cells, want %d",
							height, i, got, s.Width)
					}
				}
			}
		})
	}
}
