package layout

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// The verbs belong to the row the cursor is on: they are what the next keypress
// would do, and only one row can be that row. Every other row keeps the column
// that says what it is — for a stash, which branch it came from, how many files
// it holds, how old it is and whether it still applies.
//
// Drawing them everywhere costs more than a wrong-looking word. The verb field
// is 24 cells and the meta is 28, and the tail is replaced by cells, so a row
// that took the verbs also came out four columns short.
func TestOnlyTheCursorRowShowsVerbs(t *testing.T) {
	t.Parallel()
	const width = 77
	s := state.State{Width: width, Height: 20}
	s = state.Apply(s, state.StashedLoaded{Stashes: []git.StashRow{
		{Ref: "stash@{0}", SHA: "a", Branch: "main", Message: "first", FileCount: 2,
			Age: "2d", Status: git.StashApplies},
		{Ref: "stash@{1}", SHA: "b", Branch: "side", Message: "second", FileCount: 5,
			Age: "3d", Status: git.StashApplies},
	}})
	s = state.Apply(s, state.TabChanged{Tab: state.TabStashed})
	if s.Cursor != 0 {
		t.Fatalf("cursor starts at row %d; this test reads row 1 as the one without it", s.Cursor)
	}

	for _, w := range []Renderer{{}, {Palette: Palette{Enabled: true}}} {
		f := Pane(s, nil, w)
		rows := stashRowsOf(t, f.Lines)

		if !strings.Contains(rows[0], "restore") {
			t.Errorf("color=%v: the cursor row does not offer its verbs: %q", w.Enabled, rows[0])
		}
		if strings.Contains(rows[1], "restore") {
			t.Errorf("color=%v: a row without the cursor offers verbs: %q", w.Enabled, rows[1])
		}
		if !strings.Contains(rows[1], "side") || !strings.Contains(rows[1], "applies") {
			t.Errorf("color=%v: a row without the cursor lost its branch and status: %q",
				w.Enabled, rows[1])
		}
		for i, row := range rows {
			if got := w.Of(row); got != width {
				t.Errorf("color=%v: stash row %d is %d columns, want %d: %q",
					w.Enabled, i, got, width, row)
			}
		}
	}
}

// stashRowsOf picks the two stash rows out of a frame by the message each was
// built with, so a change to the rows above them does not silently shift what
// this reads.
func stashRowsOf(t *testing.T, lines []string) []string {
	t.Helper()
	var rows []string
	for _, line := range lines {
		plain := ansi.Strip(line)
		if strings.Contains(plain, "first") || strings.Contains(plain, "second") {
			rows = append(rows, plain)
		}
	}
	if len(rows) != 2 {
		t.Fatalf("found %d stash rows in the frame, want 2", len(rows))
	}
	return rows
}

// The history tab settles the same question. It used to answer it twice — once
// where the verbs are chosen and once where the row is drawn — while the stash
// and worktree tabs answered it only in the drawing. The caller's copy is gone,
// so this is what holds the tab to the same rule.
func TestOnlyTheCursorCommitShowsVerbs(t *testing.T) {
	t.Parallel()
	const width = 77
	s := state.State{Width: width, Height: 20}
	s = state.Apply(s, state.HistoryLoaded{Commits: []git.CommitInfo{
		{SHA: "aaaaaaa", ShortSHA: "aaaaaaa", Subject: "the first subject", Age: "2d"},
		{SHA: "bbbbbbb", ShortSHA: "bbbbbbb", Subject: "the second subject", Age: "3d"},
	}})
	s = state.Apply(s, state.TabChanged{Tab: state.TabHistory})
	if s.Cursor != 0 {
		t.Fatalf("cursor starts at row %d; this test reads row 1 as the one without it", s.Cursor)
	}

	for _, w := range []Renderer{{}, {Palette: Palette{Enabled: true}}} {
		rows := rowsHolding(t, Pane(s, nil, w).Lines, "the first subject", "the second subject")
		if !strings.Contains(rows[0], "d diff") {
			t.Errorf("color=%v: the cursor row does not offer its verbs: %q", w.Enabled, rows[0])
		}
		if strings.Contains(rows[1], "d diff") {
			t.Errorf("color=%v: a row without the cursor offers verbs: %q", w.Enabled, rows[1])
		}
		if !strings.Contains(rows[1], "3d") {
			t.Errorf("color=%v: a row without the cursor lost its age: %q", w.Enabled, rows[1])
		}
		for i, row := range rows {
			if got := w.Of(row); got != width {
				t.Errorf("color=%v: commit row %d is %d columns, want %d: %q",
					w.Enabled, i, got, width, row)
			}
		}
	}
}

// rowsHolding picks rows out of a frame by text each was built with, so a change
// to the rows above them does not silently shift what a test reads.
func rowsHolding(t *testing.T, lines []string, wanted ...string) []string {
	t.Helper()
	rows := make([]string, len(wanted))
	for i, want := range wanted {
		for _, line := range lines {
			if plain := ansi.Strip(line); strings.Contains(plain, want) {
				rows[i] = plain
				break
			}
		}
		if rows[i] == "" {
			t.Fatalf("no row holding %q", want)
		}
	}
	return rows
}
