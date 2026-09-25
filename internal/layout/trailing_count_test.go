package layout

import "testing"

// The number at the end of a row is dimmed, and this is what finds it. Every
// digit belongs to it: a bound that stops one short of nine reads "19" as the
// 9 alone, and the 1 beside it is left bright — the reader sees two numbers
// where the row has one.
func TestTheTrailingCountIsEveryDigitAtTheEnd(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		line, want string
	}{
		{"changes · 3", "3"},
		{"changes · 19", "19"},
		{"changes · 90", "90"},
		{"changes · 99", "99"},
		{"changes · 1234567890", "1234567890"},
		{"changes · 19   ", "19"},
		{"changes", ""},
		{"", ""},
		{"3", "3"},
		{"9", "9"},
		{"a1b2", "2"},
	} {
		t.Run(c.line, func(t *testing.T) {
			t.Parallel()
			if got := trailingCount(c.line); got != c.want {
				t.Errorf("trailingCount(%q) = %q, want %q", c.line, got, c.want)
			}
		})
	}
}
