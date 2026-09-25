package state

import (
	"testing"

	"github.com/catpotd/mirugit/internal/git"
)

// The tab bar counts what every tab holds, not just the one on screen, so a tab
// switch must not clear the payload of the tab being left. Only the open diff,
// the scroll offset and the selection go.
func TestSwitchingTabsKeepsTheOtherTabsCounts(t *testing.T) {
	t.Parallel()
	s := State{Width: 77, Height: 24, Tab: TabChanges}
	s = Apply(s, StatusLoaded{Rows: []git.Entry{{Path: "a.txt", Worktree: git.Modified}}})
	s = Apply(s, HistoryLoaded{Commits: []git.CommitInfo{{SHA: "a", Subject: "one"}}})
	s = Apply(s, StashedLoaded{Stashes: []git.StashRow{{Ref: "stash@{0}", SHA: "b", Message: "m"}}})
	s = Apply(s, WorktreesLoaded{
		Worktrees: []git.WorktreeRow{
			{Path: "/repo", Name: "repo", Main: true},
			{Path: "/wt", Name: "wt"},
		},
		Base: "main",
	})
	s = Apply(s, DiffOpened{Path: "a.txt", Origin: WorkingTree(SectionUnstaged)})
	s.ScrollTop = 3

	for _, tab := range []Tab{TabHistory, TabStashed, TabWorktrees, TabChanges} {
		s = Apply(s, TabChanged{Tab: tab})
		if got := len(s.Changes.Entries); got != 1 {
			t.Errorf("%s へ移った後の changes = %d, want 1", Facts[tab].Name, got)
		}
		if got := len(s.History.Commits); got != 1 {
			t.Errorf("%s へ移った後の history = %d, want 1", Facts[tab].Name, got)
		}
		if got := len(s.Stashed.Stashes); got != 1 {
			t.Errorf("%s へ移った後の stashed = %d, want 1", Facts[tab].Name, got)
		}
		if got := len(s.Worktrees.List); got != 2 {
			t.Errorf("%s へ移った後の worktrees = %d, want 2", Facts[tab].Name, got)
		}
	}
	if s.Open.Path != "" {
		t.Errorf("開いていた diff が残っている: %q", s.Open.Path)
	}
	if s.ScrollTop != 0 {
		t.Errorf("ScrollTop = %d, want 0", s.ScrollTop)
	}
}
