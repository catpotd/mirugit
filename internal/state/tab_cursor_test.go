package state

import (
	"testing"

	"github.com/catpotd/mirugit/internal/git"
)

// A tab remembers where the cursor was, so coming back lands on the row the
// reader left. Two places changed the tab and only one did the remembering:
// a tab that emptied under the reader sent them to changes through the other,
// which lost both the cursor of the tab they left and the one on changes.
func TestEveryWayOfChangingTabsRemembersTheCursor(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name  string
		leave func(State) State
	}{
		{"the reader switches tabs", func(s State) State {
			return Apply(s, TabChanged{Tab: TabChanges})
		}},
		{"the last stash goes and the tab with it", func(s State) State {
			return Apply(s, StashedLoaded{Stashes: nil})
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := State{Width: 90, Height: 30}
			s.Changes.Folded = map[string]bool{}
			s.Changes.Selected = map[string]bool{}
			s.Changes.Stale = map[string]bool{}
			s = Apply(s, StatusLoaded{Rows: []git.Entry{
				{Path: "a.txt", Worktree: git.Modified},
				{Path: "b.txt", Worktree: git.Modified},
			}, Head: git.Head{Branch: "main"}})
			s.Cursor = len(s.Rows) - 1
			onChanges := s.Cursor
			if onChanges == 0 {
				t.Fatal("the changes cursor is already at the top, so this proves nothing")
			}

			stashes := []git.StashRow{
				{Ref: "stash@{0}", SHA: "s0", Status: git.StashApplies},
				{Ref: "stash@{1}", SHA: "s1", Status: git.StashApplies},
			}
			s = Apply(s, StashedLoaded{Stashes: stashes})
			s = Apply(s, TabChanged{Tab: TabStashed})
			s.Cursor = 1
			onStashed := s.Cursor

			s = c.leave(s)
			if s.Tab != TabChanges {
				t.Fatalf("the reader is on %s, want changes", Facts[s.Tab].Name)
			}
			if s.Cursor != onChanges {
				t.Errorf("changes starts at %d, want the row the reader left at %d",
					s.Cursor, onChanges)
			}

			s = Apply(s, StashedLoaded{Stashes: stashes})
			s = Apply(s, TabChanged{Tab: TabStashed})
			if s.Cursor != onStashed {
				t.Errorf("stashed starts at %d, want the row the reader left at %d",
					s.Cursor, onStashed)
			}
		})
	}
}
