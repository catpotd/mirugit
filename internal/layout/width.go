// Package layout turns a state and a viewport into the lines to draw and the
// regions a click can land in. It reaches no terminal and no repository, so
// that the two can be asserted to agree.
package layout

import (
	"github.com/rivo/uniseg"
	"strings"

	"github.com/clipperhouse/displaywidth"
)

// Widths carries everything a line needs that is not state: how wide a
// character draws, what color it takes, and which host tools exist. Every
// drawing method hangs off it, so a new one that needs any of those goes here
// rather than taking another parameter.
//
// Color is not split out into Palette alone. Of the 24 color methods, 19 need
// Of or paintTail to place the escape at the right column, so a coloring method
// that could not measure would have to be handed the widths anyway.
//
// EastAsian comes from what the terminal answered when asked, not from the
// locale: the two disagree, and the terminal is the one drawing.
// Widths measures a string the way the terminal draws it. It is the part of
// Renderer that is actually about width, and the five methods below are the
// only ones that need nothing else.
type Widths struct {
	EastAsian bool
	// IsWidthAssumed records that startup presumed one column for ambiguous
	// characters because the terminal did not answer the width probe. The help
	// overlay says so, because every column on screen rests on that guess.
	IsWidthAssumed bool
}

// Renderer turns state into lines and click regions. Every drawing function in
// this package hangs off it, which is why it carries what they all need: how
// wide a string is drawn, which colors to paint, and which host tools exist so
// that a verb no tool can run is never offered.
//
// It used to be called Widths, and 74 methods hung off a type whose name said
// it was a measurement. Two of its five fields were about width; the rest were
// there because the drawing functions needed them.
type Renderer struct {
	Widths
	// Clipboard and Browser record which host tools exist; layout does not probe
	// for them.
	Clipboard bool
	Browser   bool
	Palette
}

func (w Widths) Of(s string) int {
	return displaywidth.Options{EastAsianWidth: w.EastAsian}.String(s)
}

func printableASCII(s string) bool {
	for i := range len(s) {
		if s[i] < 0x20 || s[i] > 0x7e {
			return false
		}
	}
	return true
}

const ellipsis = "…"

// eachCluster walks s one grapheme cluster at a time and stops when f says so.
// A rune loop measures each code point on its own, so a family emoji joined by
// zero-width joiners counts as four characters rather than one two-cell glyph
// and the caller cuts the line short.
func (w Widths) eachCluster(s string, f func(cluster string, cells, offset int) bool) {
	// Printable ASCII is one byte, one cluster, one cell. Handing it to uniseg
	// instead ran the grapheme, word and line-break state machines over every
	// byte, and a profile of one frame put 43% of it there. Paths and verbs are
	// nearly all ASCII, and this is on the path a keypress redraws.
	if printableASCII(s) {
		for i := range len(s) {
			if !f(s[i:i+1], 1, i) {
				return
			}
		}
		return
	}
	g := uniseg.NewGraphemes(s)
	offset := 0
	for g.Next() {
		cluster := g.Str()
		if !f(cluster, w.Of(cluster), offset) {
			return
		}
		offset += len(cluster)
	}
}

// Truncate drops the end, because prose carries its sense at the front.
func (w Widths) Truncate(s string, max int) string {
	if w.Of(s) <= max {
		return s
	}
	ellipsisW := w.Of(ellipsis)
	if max < ellipsisW {
		// There is not even room for the mark that says text was dropped.
		// Returning it anyway put one cell into a field of zero, and a row one
		// cell too wide wraps and shifts every row under it.
		return ""
	}
	room := max - ellipsisW
	var b strings.Builder
	used := 0
	w.eachCluster(s, func(cluster string, n, _ int) bool {
		if used+n > room {
			return false
		}
		b.WriteString(cluster)
		used += n
		return true
	})
	if used < room {
		b.WriteString(strings.Repeat(" ", room-used))
	}
	return b.String() + ellipsis
}

// TruncateFront drops the beginning, because the tail of a path names the file.
func (w Widths) TruncateFront(s string, max int) string {
	if w.Of(s) <= max {
		return s
	}
	if max < w.Of(ellipsis) {
		// Same reason as Truncate: a field this narrow cannot hold even the
		// mark that says text was dropped.
		return ""
	}
	room := max - w.Of(ellipsis)
	type piece struct {
		cells, offset int
	}
	var pieces []piece
	w.eachCluster(s, func(_ string, cells, offset int) bool {
		pieces = append(pieces, piece{cells, offset})
		return true
	})
	used, start := 0, len(s)
	for i := len(pieces) - 1; i >= 0; i-- {
		if used+pieces[i].cells > room {
			break
		}
		used += pieces[i].cells
		start = pieces[i].offset
	}
	return ellipsis + s[start:]
}

// Pad lets an overlong pair overflow rather than squeezing it, so the caller
// sees the bug instead of a quietly wrong line.
// Pad measures what it built rather than trusting the two halves to add up. Of
// is not additive across a join: a prepended concatenation mark measures zero
// cells on its own and forms one cluster with whatever follows, so a left half
// ending in one swallows the first pad space and the row comes out a cell
// short. Every click region on the row is recorded by column, so a row a cell
// short puts every one of them in the wrong place.
func (w Widths) Pad(left, right string, total int) string {
	gap := total - w.Of(left) - w.Of(right)
	if gap < 0 {
		gap = 0
	}
	line := left + strings.Repeat(" ", gap) + right
	// One more space per swallowed cell. The bound is the width itself: a row
	// cannot need more padding than the row is wide.
	for range total {
		if w.Of(line) >= total {
			break
		}
		gap++
		line = left + strings.Repeat(" ", gap) + right
	}
	return line
}
