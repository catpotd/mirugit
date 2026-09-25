package layout

import (
	"regexp"
	"testing"
	"unicode/utf8"
)

// Everything drawn here starts as bytes from a repository someone else wrote.
// The three functions below are what stands between those bytes and the
// terminal, so their promises have to hold for any input, not for the ones a
// test author thought of.

var control = regexp.MustCompile(`[\x00-\x1f\x7f]`)

func FuzzPrintable(f *testing.F) {
	f.Add("internal/layout/diff.go")
	f.Add("a\x1b[2Jb")
	f.Add("\ttabbed")
	f.Add("日本語のファイル名.txt")
	f.Add("")
	f.Fuzz(func(t *testing.T, s string) {
		got := Printable(s)
		if control.MatchString(got) {
			t.Fatalf("Printable(%q) = %q, which still acts on the terminal", s, got)
		}
		// A width is only worth measuring if the string is text. Bytes that are
		// not UTF-8 measure one width and draw another.
		if !utf8.ValidString(got) {
			t.Fatalf("Printable(%q) = %q, which is not UTF-8", s, got)
		}
		// Running it twice must answer the same as running it once. Otherwise a
		// caller that has already made a string safe cannot tell.
		if again := Printable(got); again != got {
			t.Fatalf("Printable is not idempotent: %q → %q → %q", s, got, again)
		}
	})
}

func FuzzTruncateFits(f *testing.F) {
	f.Add("internal/layout/diff.go", 10)
	f.Add("日本語のファイル名.txt", 7)
	f.Add("emoji-👨‍👩‍👧‍👦.txt", 5)
	// A wide cluster that does not fit followed by a narrow one that would: the
	// shape that tells stopping at the first cluster apart from skipping it.
	f.Add("あa", 2)
	f.Add("", 0)
	f.Fuzz(func(t *testing.T, s string, max int) {
		if max < 0 || max > 500 {
			t.Skip("a pane is never this wide")
		}
		w := Widths{}
		got := w.Truncate(Printable(s), max)
		// A row wider than the field wraps, and a wrapped row shifts every row
		// under it. This is the promise the whole pane rests on.
		if n := w.Of(got); n > max {
			t.Fatalf("Truncate(%q, %d) is %d cells: %q", s, max, n, got)
		}
		front := w.TruncateFront(Printable(s), max)
		if n := w.Of(front); n > max {
			t.Fatalf("TruncateFront(%q, %d) is %d cells: %q", s, max, n, front)
		}
	})
}

func FuzzPadIsExactlyTheWidth(f *testing.F) {
	f.Add("left", "right", 20)
	f.Add("日本語", "…", 8)
	f.Add("", "", 0)
	f.Fuzz(func(t *testing.T, left, right string, total int) {
		if total < 0 || total > 500 {
			t.Skip("a pane is never this wide")
		}
		w := Widths{}
		l, r := Printable(left), Printable(right)
		if w.Of(l)+w.Of(r) > total {
			t.Skip("the caller cuts both halves before padding")
		}
		if n := w.Of(w.Pad(l, r, total)); n != total {
			t.Fatalf("Pad(%q, %q, %d) is %d cells", l, r, total, n)
		}
	})
}
