package layout

import (
	"strings"
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// A merge whose conflicts are all resolved has an empty file list, and an empty
// list reads as nothing left to do. git is waiting for a commit and says so
// only in the porcelain-free status this pane never runs.
func TestTheUnfinishedOperationIsOnScreen(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	for _, c := range []struct {
		state git.InProgress
		says  []string
	}{
		{git.NothingInProgress, nil},
		{git.MergeInProgress, []string{"merge in progress", "git commit", "git merge --abort"}},
		{git.RebaseInProgress, []string{"rebase in progress", "git rebase --continue"}},
		{git.CherryPickInProgress, []string{"cherry-pick in progress", "git cherry-pick --continue"}},
		{git.RevertInProgress, []string{"revert in progress", "git revert --continue"}},
		// bisect ends with a reset rather than an abort, so the line has one
		// command instead of two.
		{git.BisectInProgress, []string{"bisect in progress", "git bisect reset"}},
	} {
		t.Run(c.state.Verb(), func(t *testing.T) {
			t.Parallel()
			s := ninetyFileState()
			s.Unfinished = c.state
			body := strings.Join(Pane(s, nil, w).Lines, "\n")

			for _, want := range c.says {
				if !strings.Contains(body, want) {
					t.Errorf("the pane does not say %q:\n%s", want, firstLines(body, 4))
				}
			}
			if c.says == nil && strings.Contains(body, "in progress") {
				t.Errorf("a clean tree says an operation is unfinished:\n%s", firstLines(body, 4))
			}
		})
	}
}

// Every click is placed by subtracting the header row count from the row that
// was clicked. A line drawn above the list that the count does not include puts
// every click on its neighbor.
func TestTheUnfinishedLineIsCountedAsAHeaderRow(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	for tab := range state.TabCount {
		t.Run(state.Facts[tab].Name, func(t *testing.T) {
			t.Parallel()
			for _, unfinished := range []git.InProgress{git.NothingInProgress, git.MergeInProgress} {
				s := paneStateFor(t, tab)
				s.Unfinished = unfinished
				f := Pane(s, nil, w)

				first := -1
				for _, reg := range f.Regions {
					if !isListRow(reg.Target.Kind) {
						continue
					}
					if first < 0 || reg.Row < first {
						first = reg.Row
					}
				}
				if first < 0 {
					t.Fatalf("%v: the pane drew no list row", unfinished)
				}
				if got := HeaderRowsOf(s); got != first {
					t.Errorf("%v: HeaderRowsOf says %d rows sit above the list, "+
						"but the first list row is on line %d", unfinished, got, first)
				}
			}
		})
	}
}

func paneStateFor(t *testing.T, tab state.Tab) state.State {
	t.Helper()
	switch tab {
	case state.TabHistory:
		return historyPaneState()
	case state.TabStashed:
		return stashPaneStateWithFiles()
	case state.TabWorktrees:
		return worktreePaneStateWithFiles()
	case state.TabChanges:
	}
	return ninetyFileState()
}

func firstLines(body string, n int) string {
	lines := strings.Split(body, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}
