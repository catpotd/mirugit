package layout

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode"

	"github.com/charmbracelet/x/ansi"
)

// The terminal is asked at startup whether it draws an ambiguous-width glyph in
// one column or two, and Widths pads with the answer. The library that puts
// those lines on the screen measures them with ansi, which reads
// RUNEWIDTH_EASTASIAN once at init and cannot be told anything after that:
// bubbletea v2.0.9 keeps setWidthMethod unexported.
//
// So the two agree only when the environment variable matches what the probe
// found. This test records which glyphs the agreement covers, so that a glyph
// added later is a decision rather than an accident, and it runs the same under
// either setting.
func TestTheGlyphsWhoseWidthTheTerminalDecides(t *testing.T) {
	t.Parallel()
	// Written out rather than derived from the source, so adding a glyph to the
	// drawing does not quietly extend the list it belongs to.
	want := map[rune]bool{
		'·': true, '—': true, '…': true,
		'←': true, '↑': true, '→': true, '↓': true,
		'─': true, '━': true, '┈': true,
		'█': true, '▉': true, '▊': true, '▋': true,
		'▌': true, '▍': true, '▎': true, '▏': true,
		'●': true,
	}
	// ansi answers 2 for these when RUNEWIDTH_EASTASIAN is set, which is the
	// setting that makes it agree with a terminal that draws them wide.
	ansiCountsWide := ansi.StringWidth("·") == 2
	for r := range want {
		s := string(r)
		narrowOf := Widths{EastAsian: false}
		wideOf := Widths{EastAsian: true}
		narrow, wide := narrowOf.Of(s), wideOf.Of(s)
		if narrow != 1 || wide != 2 {
			t.Errorf("%q is listed as ambiguous but measures %d narrow and %d wide",
				s, narrow, wide)
		}
		wantAnsi := 1
		if ansiCountsWide {
			wantAnsi = 2
		}
		if got := ansi.StringWidth(s); got != wantAnsi {
			t.Errorf("%q measures %d under ansi, want %d with RUNEWIDTH_EASTASIAN %v",
				s, got, wantAnsi, ansiCountsWide)
		}
	}

	for _, r := range glyphsInDrawing(t) {
		if want[r] {
			continue
		}
		s := string(r)
		narrow, wide := Widths{EastAsian: false}, Widths{EastAsian: true}
		if narrow.Of(s) != wide.Of(s) {
			t.Errorf("%q U+%04X is drawn and its width depends on the terminal, "+
				"but it is not in the list above", s, r)
		}
	}
}

// The one setting a reader can change is RUNEWIDTH_EASTASIAN, so the README has
// to name it: without it, a terminal that draws ambiguous glyphs wide gets a
// pane whose padding and whose cell model disagree.
func TestTheReadmeNamesTheWidthEnvironmentVariable(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "RUNEWIDTH_EASTASIAN") {
		t.Error("the README does not name the variable that makes the drawing " +
			"library agree with a terminal that draws ambiguous glyphs wide")
	}
}

// glyphsInDrawing collects the non-ASCII runes in the string and rune literals
// of this package. A glyph reaches the screen from here, so the source is where
// the list has to be checked against.
func glyphsInDrawing(t *testing.T) []rune {
	t.Helper()
	literal := regexp.MustCompile(`"([^"\\]*)"|'(.)'`)
	seen := map[rune]bool{}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") ||
			strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		raw, err := os.ReadFile(entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		body := string(raw)
		for _, m := range literal.FindAllStringSubmatch(body, -1) {
			for _, group := range m[1:] {
				for _, r := range group {
					if r > unicode.MaxASCII {
						seen[r] = true
					}
				}
			}
		}
	}
	out := make([]rune, 0, len(seen))
	for r := range seen {
		out = append(out, r)
	}
	return out
}
