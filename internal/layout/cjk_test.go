package layout

import (
	"strings"
	"testing"

	"github.com/catpotd/mirugit/internal/git"
)

func TestEveryCellOfAJapaneseNameIsClickable(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	name := "新しいファイル名.swift"
	line, regions := w.FileRow(Row{Entry: git.Entry{Path: name, Worktree: git.Modified}}, paneWidth)

	// Where the name actually sits, measured from the line rather than from the
	// region under test.
	byteAt := strings.Index(line, name)
	if byteAt < 0 {
		t.Fatalf("the name is not in %q", line)
	}
	first := w.Of(line[:byteAt])
	last := first + w.Of(name) - 1

	f := Frame{Lines: []string{line}, Regions: []Region{{
		Target: regions[0].Target, Row: 0,
		ColStart: regions[0].ColStart, ColEnd: regions[0].ColEnd,
	}}}
	for _, col := range []int{first, (first + last) / 2, last} {
		region, _, _ := f.Hit(0, col)
		if region.Path != name {
			t.Errorf("cell %d (name spans %d..%d) resolved to %q", col, first, last, region.Path)
		}
	}
	region, _, _ := f.Hit(0, last+1)
	if region.Kind != TargetNone {
		t.Errorf("the cell after the name resolved to %v", region.Kind)
	}
}
