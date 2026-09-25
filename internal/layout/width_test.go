package layout

import (
	"strings"
	"testing"
)

func TestAmbiguousGlyphsFollowTheTerminalSetting(t *testing.T) {
	t.Parallel()
	// Eight of the glyphs in this interface are East Asian Ambiguous. A
	// terminal configured for CJK draws them two cells wide, and every row
	// overflows if the layout assumes one.
	narrow := Renderer{}
	wide := Renderer{Widths: Widths{EastAsian: true}}
	for _, g := range []string{"·", "↑", "─", "━", "…", "█", "▌", "●"} {
		if got := narrow.Of(g); got != 1 {
			t.Errorf("narrow %q = %d, want 1", g, got)
		}
		if got := wide.Of(g); got != 2 {
			t.Errorf("wide %q = %d, want 2", g, got)
		}
	}
}

func TestJapaneseIsTwoCellsEitherWay(t *testing.T) {
	t.Parallel()
	for _, w := range []Renderer{{}, {Widths: Widths{EastAsian: true}}} {
		if got := w.Of("あ"); got != 2 {
			t.Errorf("%+v: あ = %d, want 2", w, got)
		}
	}
}

func TestTruncateKeepsTheFrontAndMarksTheCut(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	if got, want := w.Truncate("abcdefgh", 5), "abcd…"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := w.Truncate("abc", 5), "abc"; got != want {
		t.Errorf("short strings are returned whole: got %q, want %q", got, want)
	}
}

func TestTruncateFrontKeepsTheTailBecauseItNamesTheFile(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	// The budget is filled, not undershot: eight cells hold the ellipsis and
	// the seven cells nearest the end.
	if got, want := w.TruncateFront("a/b/c/d.txt", 8), "…c/d.txt"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := w.Of(w.TruncateFront("a/b/c/d.txt", 8)); got != 8 {
		t.Errorf("result is %d cells, want 8", got)
	}
}

func TestTruncateNeverExceedsTheBudgetWithWideCharacters(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	got := w.Truncate("ユースケース層へ移動", 7)
	if n := w.Of(got); n > 7 {
		t.Errorf("%q is %d cells, want at most 7", got, n)
	}
}

func TestTruncateCommitSubjectFillsTheBudgetFromTheEnd(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	got := w.Truncate("refactor: 同期所有権の判定を移動", 20)
	if n := w.Of(got); n != 20 {
		t.Fatalf("%q is %d cells, want 20", got, n)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("got %q, want an ellipsis at the end", got)
	}
}

func TestTruncateStopsBeforeAHalfWideCharacter(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	got := w.Truncate("ababあextra", 5)
	if n := w.Of(got); n != 5 {
		t.Fatalf("%q is %d cells, want 5", got, n)
	}
	if got != "abab…" {
		t.Errorf("got %q, want abab…", got)
	}
}

func TestPadPlacesTheRightSideAgainstTheRightEdge(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	got := w.Pad("left", "right", 20)
	if n := w.Of(got); n != 20 {
		t.Fatalf("%q is %d cells, want 20", got, n)
	}
	if got[len(got)-5:] != "right" {
		t.Errorf("got %q, want it to end in \"right\"", got)
	}
}

// Truncate keeps the front and stops at the first cluster that does not fit.
// Going on instead of stopping picks up whatever narrow cluster comes after the
// wide one that did not fit, so the text reads with a character missing from
// the middle: "あa" cut to one cell comes out as the a.
//
// Every case the suite had cuts a run of clusters that are all the same width,
// where stopping and skipping give the same answer.
func TestTruncateStopsAtTheFirstClusterThatDoesNotFit(t *testing.T) {
	t.Parallel()
	w := Widths{}
	const ellipsisW = 1

	for _, c := range []struct {
		name string
		s    string
		max  int
		want string
	}{
		// The room left over is padded, so what is kept is a blank rather than
		// the a that comes after the wide cluster.
		{"a wide cluster that does not fit, then a narrow one that would",
			"あa", 1 + ellipsisW, " …"},
		{"a narrow cluster, then a wide one that does not fit, then a narrow one",
			"aあb", 1 + ellipsisW, "a…"},
		{"every cluster the same width", "abcdef", 4 + ellipsisW, "abcd…"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := w.Truncate(c.s, c.max)
			if got != c.want {
				t.Errorf("Truncate(%q, %d) = %q, want %q", c.s, c.max, got, c.want)
			}
			if n := w.Of(got); n > c.max {
				t.Errorf("%q is %d cells, want at most %d", got, n, c.max)
			}
		})
	}
}

// A field exactly as wide as the mark holds the mark and nothing else. That is
// the narrowest cut that still says text was dropped; one cell narrower has no
// room even for that and comes back empty, because a mark in a field of zero
// makes the row one cell too wide and every row under it shifts.
func TestTruncateIntoAFieldTheWidthOfTheMarkKeepsTheMark(t *testing.T) {
	t.Parallel()
	w := Widths{}
	markW := w.Of(ellipsis)
	if markW != 1 {
		t.Fatalf("the mark is %d cells; this test is written for one", markW)
	}

	for _, c := range []struct {
		name string
		max  int
		want string
	}{
		{"a cell wider than the mark", markW + 1, "a" + ellipsis},
		{"exactly the width of the mark", markW, ellipsis},
		{"a cell narrower than the mark", markW - 1, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := w.Truncate("abcdef", c.max)
			if got != c.want {
				t.Errorf("Truncate into %d cells = %q, want %q", c.max, got, c.want)
			}
			if n := w.Of(got); n > c.max {
				t.Errorf("%q is %d cells, wider than the %d it had", got, n, c.max)
			}
		})
	}
}

// A name that fills the field exactly is whole, and a whole name is drawn as it
// is. Cutting it there spends a cell on the mark that says text was dropped and
// drops a character to pay for it, so the pane says a file is not what it is.
func TestTruncateFrontLeavesANameThatFillsTheFieldAlone(t *testing.T) {
	t.Parallel()
	for _, eastAsian := range []bool{false, true} {
		w := Renderer{Widths: Widths{EastAsian: eastAsian}}
		for _, s := range []string{"a/b/c.txt", "あいう", "", "x"} {
			if got := w.TruncateFront(s, w.Of(s)); got != s {
				t.Errorf("east asian %v: a field of %d cells drew %q as %q",
					eastAsian, w.Of(s), s, got)
			}
		}
	}
}

// A field one cell wide holds the mark that says text was dropped, and nothing
// else. Answering with an empty string there draws a name as no name at all,
// where a single mark says the row has one and the pane is too narrow for it.
func TestTruncateFrontFillsAFieldWideEnoughForTheMarkAlone(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	const long = "a/b/c/d.txt"
	if got, want := w.TruncateFront(long, w.Of(ellipsis)), ellipsis; got != want {
		t.Errorf("a field of %d cells drew %q as %q, want %q",
			w.Of(ellipsis), long, got, want)
	}
	// One cell narrower there is no room even for the mark.
	if got := w.TruncateFront(long, w.Of(ellipsis)-1); got != "" {
		t.Errorf("a field of %d cells drew %q, want nothing", w.Of(ellipsis)-1, got)
	}
}
