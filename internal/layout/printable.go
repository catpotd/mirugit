package layout

import (
	"strings"
	"unicode/utf8"
)

// tabStop is narrower than git's eight because the pane is 77 columns and the
// diff body starts after a line-number gutter. Eight columns of indent leaves
// little of a nested line visible.
const tabStop = 4

// controlReplacement is one cell wide, so a line's width does not change with
// the bytes a file happens to hold.
const controlReplacement = '·'

// Printable makes a string from the repository safe to draw. Every string this
// package renders — a path, a branch, a commit subject, a diff line — is
// content someone else wrote, and the terminal acts on the control characters
// in it: ESC [ 2 J clears the screen and ESC ] 0 ; rewrites the window title.
// Tabs are expanded rather than replaced because they carry the indentation of
// the file, and because a tab measures zero cells while the terminal advances
// on it, which put every column after it in the wrong place.
func Printable(s string) string {
	if !needsRewrite(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	col := 0
	for _, r := range s {
		switch {
		case r == '\t':
			n := tabStop - col%tabStop
			b.WriteString(strings.Repeat(" ", n))
			col += n
		case r < 0x20 || r == 0x7f:
			b.WriteRune(controlReplacement)
			col++
		default:
			b.WriteRune(r)
			col += (Widths{}).Of(string(r))
		}
	}
	return b.String()
}

func needsRewrite(s string) bool {
	// Bytes that are not UTF-8 are rewritten too. A path can hold them —
	// core.quotePath=false hands them over raw — and the width they measure is
	// not the width the terminal draws, so the row comes out short.
	if !utf8.ValidString(s) {
		return true
	}
	for i := range len(s) {
		if s[i] < 0x20 || s[i] == 0x7f {
			return true
		}
	}
	return false
}
