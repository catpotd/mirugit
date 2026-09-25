package tui

import (
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// The cursor row is marked read once, when the cursor arrives on it. Every
// keypress comes through here, and marking on each one wrote the record again
// for a cursor that had not moved.
//
// Both halves of the guard matter. Inverting the first turns it into "always
// stop", and the row the reader is looking at is never marked: its dot stays
// out for the whole session, and the record never learns they saw it.
func TestTheCursorRowIsMarkedOnceOnArrival(t *testing.T) {
	t.Parallel()
	// The worktrees tab is one of the two that mark a row read; the changes and
	// history tabs record nothing, so the guard is never reached from them.
	m := fixed(t)
	m.state = state.Apply(m.state, state.WorktreesLoaded{Base: "main", Here: "/a",
		Worktrees: []git.WorktreeRow{
			{Path: "/a", Name: "a", Branch: "ba", SHA: "s1"},
			{Path: "/b", Name: "b", Branch: "bb", SHA: "s2"},
		}})
	m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabWorktrees})
	m.asked.readRow = ""

	if cmd := m.markCursorRead(); cmd == nil {
		t.Fatal("arriving on a row asked for no mark")
	}
	first := m.asked.readRow
	if first == "" {
		t.Fatal("the row that was marked was not recorded")
	}
	if cmd := m.markCursorRead(); cmd != nil {
		t.Error("the same row was marked twice without the cursor moving")
	}

	m.state.Cursor++
	if cmd := m.markCursorRead(); cmd == nil {
		t.Error("moving to the next row asked for no mark")
	}
	if m.asked.readRow == first {
		t.Error("the cursor moved and the recorded row did not")
	}
}
