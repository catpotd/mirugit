package state

import (
	"testing"

	"github.com/catpotd/mirugit/internal/git"
)

// The list is rebuilt from scratch, so the cursor is put back by the row it was
// on rather than by its number. The row that moves to the top of the list is
// the case the number gets right by accident everywhere else: left where it
// was, the cursor lands on the stash below the one the reader was reading.
func TestTheCursorFollowsItsRowToTheTopOfTheList(t *testing.T) {
	t.Parallel()
	s := State{Tab: TabStashed}
	s = Apply(s, StashedLoaded{Stashes: []git.StashRow{
		{Ref: "stash@{0}", SHA: "s0", Message: "newest"},
		{Ref: "stash@{1}", SHA: "s1", Message: "middle"},
		{Ref: "stash@{2}", SHA: "s2", Message: "oldest"},
	}})
	s = Apply(s, CursorMoved{By: 1})
	if got := s.Rows[s.Cursor].Path(); got != "stash@{1}" {
		t.Fatalf("the cursor is on %q, want the middle stash", got)
	}

	// The newest stash is dropped, so the one under the cursor is now first.
	s = Apply(s, StashedLoaded{Stashes: []git.StashRow{
		{Ref: "stash@{1}", SHA: "s1", Message: "middle"},
		{Ref: "stash@{2}", SHA: "s2", Message: "oldest"},
	}})
	if got := s.Rows[s.Cursor].Path(); got != "stash@{1}" {
		t.Errorf("the cursor moved to %q when the stash above it was dropped", got)
	}
	if s.Cursor != 0 {
		t.Errorf("the cursor is at %d, want the first row", s.Cursor)
	}
}

// The row the cursor is on is remembered by its key, and the first row has one
// like every other. Reading nothing for it leaves the cursor on the number it
// had, so a tree that appears above the one the reader was on moves them down
// the list without a key being pressed.
func TestTheCursorOnTheFirstRowFollowsItDown(t *testing.T) {
	t.Parallel()
	s := State{Tab: TabWorktrees}
	s = Apply(s, WorktreesLoaded{Base: "main", Worktrees: []git.WorktreeRow{
		{Path: "/repo", Name: "repo", Main: true, Branch: "main"},
		{Path: "/wt-b", Name: "wt-b", Branch: "b"},
	}})
	if got := s.Rows[s.Cursor].Path(); got != "/repo" {
		t.Fatalf("the cursor is on %q, want the first tree", got)
	}

	// A tree checked out while the pane was open goes above the one it is on.
	s = Apply(s, WorktreesLoaded{Base: "main", Worktrees: []git.WorktreeRow{
		{Path: "/wt-a", Name: "wt-a", Branch: "a"},
		{Path: "/repo", Name: "repo", Main: true, Branch: "main"},
		{Path: "/wt-b", Name: "wt-b", Branch: "b"},
	}})
	if got := s.Rows[s.Cursor].Path(); got != "/repo" {
		t.Errorf("the cursor moved to %q when a tree appeared above it", got)
	}
}
