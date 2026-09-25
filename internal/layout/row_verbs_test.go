package layout

import (
	"strings"
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// A row offers its verbs on the line the cursor is on and nowhere else. Every
// line offering them would make the pane a wall of words with no way to tell
// which one a key would act on.
//
// The four tabs each write this guard, and a mutation that inverted one of them
// survived the suite: on the stashed tab every file row but the cursor's would
// have shown "d diff".
func TestOnlyTheCursorLineOffersItsVerbs(t *testing.T) {
	t.Parallel()
	w := Renderer{Clipboard: true, Browser: true}

	for _, c := range []struct {
		name  string
		state func() state.State
		verb  string
	}{
		{"stashed", stashPaneStateWithFiles, "d diff"},
		{"worktrees", worktreePaneStateWithFiles, "d diff"},
		{"changes", changesRowsState, "s stage"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := c.state()
			if len(s.Rows) < 2 {
				t.Fatalf("one row only, so this proves nothing: %d", len(s.Rows))
			}
			for cursor := range s.Rows {
				s.Cursor = cursor
				lines, _ := TabList(s, emptyReadMarks{}, w)
				offering := 0
				for _, line := range lines {
					if strings.Contains(line, c.verb) {
						offering++
					}
				}
				if offering > 1 {
					t.Errorf("cursor %d: %d lines offer %q\n%s",
						cursor, offering, c.verb, strings.Join(lines, "\n"))
				}
			}
		})
	}
}

// The history tab draws its own file rows, and its commits carry their own
// verbs, so it is built here rather than shared with the two above.
func TestOnlyTheCursorLineOffersItsVerbsOnHistory(t *testing.T) {
	t.Parallel()
	w := Renderer{Clipboard: true, Browser: true}
	s := state.State{Width: paneWidth, Height: 24, Tab: state.TabHistory}
	s.Changes.Folded = map[string]bool{}
	s.Changes.Selected = map[string]bool{}
	s = state.Apply(s, state.HistoryLoaded{Commits: []git.CommitInfo{
		{SHA: "aaa", ShortSHA: "aaa", Subject: "one", Age: "1m"},
		{SHA: "bbb", ShortSHA: "bbb", Subject: "two", Age: "2m"},
	}})
	s = state.Apply(s, state.CommitExpanded{SHA: "aaa", Files: []git.Entry{
		{Path: "a.txt", Index: git.Modified},
		{Path: "b.txt", Index: git.Modified},
	}})

	for cursor := range s.Rows {
		s.Cursor = cursor
		lines, _ := TabList(s, emptyReadMarks{}, w)
		offering := 0
		for _, line := range lines {
			if strings.Contains(line, "d diff") {
				offering++
			}
		}
		if offering > 1 {
			t.Errorf("cursor %d: %d lines offer \"d diff\"\n%s",
				cursor, offering, strings.Join(lines, "\n"))
		}
	}
}

func changesRowsState() state.State {
	s := state.State{Width: paneWidth, Height: 24, Tab: state.TabChanges}
	s.Changes.Folded = map[string]bool{}
	s.Changes.Selected = map[string]bool{}
	s.Changes.Stale = map[string]bool{}
	return state.Apply(s, state.StatusLoaded{Rows: []git.Entry{
		{Path: "app/a.txt", Worktree: git.Modified},
		{Path: "app/b.txt", Worktree: git.Modified},
	}, Head: git.Head{Branch: "main"}})
}
