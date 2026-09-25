package layout

import (
	"strings"
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

func TestTabBarUnderlinesTheActiveTabWithACharacter(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	tabs := []Tab{{Name: "changes", Count: 5, Unread: true}, {Name: "history", Count: 3}}
	bar, rule, _ := w.TabBar(tabs, 0, "fetched 4m ago", paneWidth)

	if w.Of(bar) != paneWidth || w.Of(rule) != paneWidth {
		t.Fatalf("bar %d, rule %d, want %d", w.Of(bar), w.Of(rule), paneWidth)
	}
	// The distinction must survive a terminal without color.
	if !strings.Contains(rule, "━") {
		t.Error("the active tab has no heavy underline")
	}
	start := strings.Index(bar, "changes")
	if []rune(rule)[start] != '━' {
		t.Error("the underline does not sit under the active tab")
	}
}

func TestTabLabelCarriesItsCountAndUnreadMark(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	bar, _, _ := w.TabBar([]Tab{{Name: "worktrees", Count: 2, Unread: true}}, 0, "", paneWidth)
	if !strings.Contains(bar, "worktrees 2 ·") {
		t.Errorf("got %q, want it to carry the count and the mark", bar)
	}
}

func TestTabBarRegionsNameTheirTab(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	tabs := []Tab{{Name: "changes", Count: 5}, {Name: "history", Count: 3}}
	bar, _, regions := w.TabBar(tabs, 0, "", paneWidth)

	var hit int
	for _, r := range regions {
		if r.Target.Kind == TargetTab {
			hit++
			if !strings.Contains(cellSlice(bar, r.ColStart, r.ColEnd), r.Target.Name) {
				t.Errorf("region for %q does not cover its label", r.Target.Name)
			}
		}
	}
	if hit != 2 {
		t.Errorf("want a region per tab, got %d", hit)
	}
}

func TestTabBarRegionCarriesLogicalTabWhenStashedIsHidden(t *testing.T) {
	t.Parallel()
	s := state.State{History: state.History{Commits: []git.CommitInfo{{Subject: "a"}}}, Worktrees: state.Worktrees{List: []git.WorktreeRow{{Path: "/main"}, {Path: "/wt"}}}}
	tabs := TabsOf(s, nil)
	if len(tabs) != 3 {
		t.Fatalf("got %d tabs, want 3: %+v", len(tabs), tabs)
	}
	w := Renderer{}
	_, _, regions := w.TabBar(tabs, activeTab(state.TabWorktrees, tabs), "", paneWidth)
	for _, r := range regions {
		if r.Target.Kind != TargetTab || r.Target.Name != "worktrees" {
			continue
		}
		if r.Target.Tab != state.TabWorktrees {
			t.Fatalf("worktrees region Tab = %v, want TabWorktrees", r.Target.Tab)
		}
		return
	}
	t.Fatal("worktrees tab region not found")
}
