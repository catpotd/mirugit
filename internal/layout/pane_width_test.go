package layout

import (
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// A row wider than the pane wraps in the terminal and shifts every row under it.
// Below minPaneWidth the bar, the commit box and the file rows each overflow by
// a cell or two, so the pane says so instead of drawing them.
func TestEveryPaneRowFitsTheWidth(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	for _, tc := range []struct {
		name  string
		state state.State
	}{
		{"changes", artifactState()},
		{"long branch", longBranchState()},
		{"worktrees", worktreesPaneState()},
	} {
		s := tc.state
		for width := minPaneWidth; width <= 200; width++ {
			s.Width = width
			for i, line := range Pane(s, artifactMarks{}, w).Lines {
				if got := w.Of(line); got != width {
					t.Errorf("%s width %d, line %d: %d cells: %q",
						tc.name, width, i, got, line)
				}
			}
		}
	}
}

// A branch name long enough to push the bar past the pane on its own.
func longBranchState() state.State {
	s := artifactState()
	s.Head = git.Head{Branch: "feature/add-the-new-payment-flow-to-checkout"}
	s.Fetched = "fetched 3m ago"
	return s
}

// The worktrees tab lays its rows out in fixed columns, which ignored the pane
// width until the row was padded against it.
func worktreesPaneState() state.State {
	s := state.State{
		Height: 53, Tab: state.TabWorktrees,
		Head: git.Head{Branch: "main"},
		Worktrees: state.Worktrees{
			List: []git.WorktreeRow{
				{Path: "/repo", Name: "repo", Branch: "main", Main: true},
				{Path: "/repo/wt", Name: "a-long-worktree-name", Branch: "feature/x", SHA: "abc", Dirty: 2},
			},
			Base: "main",
			Here: "/repo",
		},
	}
	s.Rows = state.WorktreeRowsFor(s)
	return s
}

func TestANarrowPaneSaysSoInsteadOfWrapping(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := artifactState()
	for width := 1; width < minPaneWidth; width++ {
		s.Width = width
		lines := Pane(s, artifactMarks{}, w).Lines
		for i, line := range lines {
			if got := w.Of(line); got != width {
				t.Fatalf("width %d, line %d: %d cells: %q", width, i, got, line)
			}
		}
		if len(lines) != s.Height {
			t.Fatalf("width %d: %d lines, want %d", width, len(lines), s.Height)
		}
	}
}
