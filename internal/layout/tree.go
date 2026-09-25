package layout

import (
	"fmt"
	"strings"

	"github.com/catpotd/mirugit/internal/state"
)

// Tab carries its count and unread mark because a tab hides detail; without
// them it would hide existence, which is the elision this program refuses
// everywhere else.
type Tab struct {
	Logical state.Tab
	Name    string
	Count   int
	Unread  bool
	Number  int
}

func (t Tab) label() string {
	s := fmt.Sprintf("%s %d", t.Name, t.Count)
	if t.Unread {
		s += " ·"
	}
	return s
}

func (w Renderer) TabBar(tabs []Tab, active int, right string, width int) (string, string, []Region) {
	var b strings.Builder
	regions := make([]Region, 0, len(tabs))
	b.WriteString(" ")
	col := 1
	for i, t := range tabs {
		if i > 0 {
			b.WriteString("   ")
			col += 3
		}
		label := t.label()
		regions = append(regions, Region{
			Target:   Target{Kind: TargetTab, Tab: t.Logical, Name: t.Name},
			ColStart: col,
			ColEnd:   col + w.Of(label),
		})
		b.WriteString(label)
		col += w.Of(label)
	}
	// Pad overflows rather than truncating, so a long branch name pushed the bar
	// past the pane and ran into the last tab with no gap. Cut the right side to
	// what is left, keeping one cell between it and the tabs, and cut the tabs
	// themselves when even they do not fit.
	left := b.String()
	if room := width - col - 1; w.Of(right) > room {
		right = w.Truncate(right, max(room, 0))
	}
	if room := width - w.Of(right); w.Of(left) > room {
		left = w.Truncate(left, max(room, 0))
		// The regions were recorded before the cut, so the ones past it name
		// columns the bar no longer draws. A click cannot land there, but a
		// region that runs off the pane is a claim the drawing does not keep.
		regions = clipRegions(regions, w.Of(left))
	}
	bar := w.Pad(left, right, width)

	heavyStart, heavyEnd := -1, -1
	if active >= 0 && active < len(regions) {
		heavyStart, heavyEnd = regions[active].ColStart, regions[active].ColEnd
	}
	// The span is in columns, and so is the rule. Marking it by rune index put
	// the heavier stretch under the wrong tab on a terminal that draws the mark
	// in two cells.
	ruleStr := w.ruleWithSpan(width, heavyStart, heavyEnd)
	bar = w.colorTabBar(bar, tabs, regions)
	return bar, w.colorTabRule(ruleStr), regions
}

func (w Renderer) colorTabBar(bar string, tabs []Tab, regions []Region) string {
	p := w.Palette
	if !p.Enabled {
		return bar
	}
	line := bar
	for i := len(tabs) - 1; i >= 0; i-- {
		if i >= len(regions) {
			continue
		}
		plain := tabs[i].label()
		// A tab the cut left only part of keeps its plain text: the swap below
		// puts the whole label back where a piece of it is drawn, and the bar
		// then reaches past the pane and wraps, which moves every row under it.
		if regions[i].ColEnd-regions[i].ColStart < w.Of(plain) {
			continue
		}
		colored := plain
		if tabs[i].Unread {
			colored = strings.Replace(plain, " ·", p.Mark(" ·"), 1)
		}
		if colored == plain {
			continue
		}
		start := cellByteAt(line, regions[i].ColStart, w)
		end := cellByteAt(line, regions[i].ColEnd, w)
		line = line[:start] + colored + line[end:]
	}
	return line
}

func cellByteAt(line string, col int, w Renderer) int {
	cells := 0
	for i, r := range line {
		if cells >= col {
			return i
		}
		cells += w.Of(string(r))
	}
	return len(line)
}

// clipRegions drops the regions past the last column drawn and shortens the one
// the cut runs through.
func clipRegions(regions []Region, drawn int) []Region {
	kept := regions[:0]
	for _, r := range regions {
		if r.ColStart >= drawn {
			continue
		}
		r.ColEnd = min(r.ColEnd, drawn)
		kept = append(kept, r)
	}
	return kept
}
