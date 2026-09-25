package layout

import (
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// A verb field is exactly as wide as it was asked for, whichever way the
// terminal draws the characters in it. The keys for fold and unfold are ← and
// →, one rune of two cells wherever ambiguous characters are drawn wide, and
// fmt pads to a count of runes: the field came out a cell short of its width
// and moved the column beside it.
func TestAVerbFieldIsExactlyTheWidthItIsGiven(t *testing.T) {
	t.Parallel()
	for _, eastAsian := range []bool{false, true} {
		w := Renderer{Widths: Widths{EastAsian: eastAsian}}
		for _, verb := range state.AllVerbNames {
			for _, fieldWidth := range []int{verbsWidth, stashCommitWorktreeVerbsWidth} {
				field := w.verbField([]state.VerbName{verb}, fieldWidth)
				if got := w.Of(field); got != fieldWidth {
					t.Errorf("eastAsian=%v: the field for %q is %d cells, want %d: %q",
						eastAsian, verb, got, fieldWidth, field)
				}
			}
		}
	}
}

// The meta a stash row carries is a fixed set of columns, so every row's is the
// same number of cells. It is right-aligned against the pane, which means a row
// whose meta came out wider than another's moved every one of its columns left
// while the row itself stayed exactly as wide as the pane — the width checks see
// nothing, and the list reads as ragged.
//
// A branch name is the only part of it that is not ASCII.
func TestEveryStashRowsMetaIsTheSameWidth(t *testing.T) {
	t.Parallel()
	branches := []string{"main", "a-long-branch-name", "枝の名前", "日本語", "x"}
	for _, eastAsian := range []bool{false, true} {
		w := Renderer{Widths: Widths{EastAsian: eastAsian}}
		want := -1
		for _, branch := range branches {
			meta := w.stashMeta(git.StashRow{Ref: "stash@{0}", Branch: branch,
				FileCount: 2, Age: "2d", Status: git.StashApplies})
			got := w.Of(meta)
			if want < 0 {
				want = got
				continue
			}
			if got != want {
				t.Errorf("eastAsian=%v: the meta for branch %q is %d cells, and another row's is %d: %q",
					eastAsian, branch, got, want, meta)
			}
		}
	}
}

// The counts are a fixed column, so every row's is the same number of cells.
// The minus sign is U+2212, which is two cells wide on some terminals, and fmt
// pads to a count of runes: measured there, a column meant to be 12 came out
// wider and moved every column beside it.
func TestTheFiguresColumnIsAlwaysTheSameWidth(t *testing.T) {
	t.Parallel()
	counts := []git.Count{
		{}, {Added: 1}, {Deleted: 1}, {Added: 1, Deleted: 2},
		{Added: 120, Deleted: 3}, {Added: 99999, Deleted: 99999}, {Binary: true},
	}
	for _, eastAsian := range []bool{false, true} {
		w := Renderer{Widths: Widths{EastAsian: eastAsian}}
		for _, c := range counts {
			got := w.Of(w.figures(Row{Count: c}))
			if got != figuresWidth {
				t.Errorf("eastAsian=%v %+v: %d cells, want %d: %q",
					eastAsian, c, got, figuresWidth, w.figures(Row{Count: c}))
			}
		}
	}
}
