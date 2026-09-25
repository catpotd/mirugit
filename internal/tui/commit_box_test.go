package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// A focused field takes every key it can type. Focusing one on a tab that draws
// no field left j, k and the tab numbers going into a box the reader cannot
// see: after c on the stashed tab, j j 1 answered with the message "jj1" and
// the cursor never moved. Nothing on screen said why the pane had stopped.
//
// Every tab is driven from one table, because c is one key and the field is one
// field, and the answer differed per tab with nothing comparing them.
func TestOnlyTheTabDrawingTheFieldTakesTheCommitKey(t *testing.T) {
	t.Parallel()
	for _, tab := range []state.Tab{
		state.TabChanges, state.TabHistory, state.TabStashed, state.TabWorktrees,
	} {
		t.Run(state.Facts[tab].Name, func(t *testing.T) {
			t.Parallel()
			m := everyTabModel(t)
			m.state = state.Apply(m.state, state.TabChanged{Tab: tab})
			m.View()

			next, _ := m.Update(tea.KeyPressMsg{Code: 'c'})
			m = next.(*Model)
			m.View()

			drawn := commitBoxDrawn(m)
			if m.state.Changes.MessageFocused != drawn {
				t.Fatalf("c focused the field %v while the pane draws it %v",
					m.state.Changes.MessageFocused, drawn)
			}
			if drawn {
				return
			}

			// The keys have to keep working where no field took them.
			before := m.state.Cursor
			next, _ = m.Update(tea.KeyPressMsg{Code: 'j'})
			m = next.(*Model)
			if m.state.Changes.Message != "" {
				t.Errorf("j went into the message field: %q", m.state.Changes.Message)
			}
			if len(m.state.Rows) > 1 && m.state.Cursor == before {
				t.Errorf("j did not move the cursor from %d", before)
			}
		})
	}
}

func commitBoxDrawn(m *Model) bool {
	for _, line := range m.frame.Lines {
		if strings.HasPrefix(line, "[ ") && strings.HasSuffix(strings.TrimRight(line, " "), " ]") {
			return true
		}
	}
	return false
}

func everyTabModel(t *testing.T) *Model {
	t.Helper()
	m := newParentModel(t)
	m.state = state.Apply(m.state, state.StatusLoaded{
		Rows: []git.Entry{
			{Path: "a.txt", Worktree: git.Modified},
			{Path: "b.txt", Worktree: git.Modified},
		},
		Head: git.Head{Branch: "main"},
	})
	m.state = state.Apply(m.state, state.HistoryLoaded{Commits: []git.CommitInfo{
		{SHA: "aaa", ShortSHA: "aaa", Subject: "one", Age: "1m"},
		{SHA: "bbb", ShortSHA: "bbb", Subject: "two", Age: "2m"},
	}})
	m.state = state.Apply(m.state, state.StashedLoaded{Stashes: []git.StashRow{
		{Ref: "stash@{0}", SHA: "s0", Message: "one", Status: git.StashApplies},
		{Ref: "stash@{1}", SHA: "s1", Message: "two", Status: git.StashApplies},
	}})
	m.state = state.Apply(m.state, state.WorktreesLoaded{Base: "main", Here: "/repo",
		Worktrees: []git.WorktreeRow{
			{Path: "/repo", Name: "repo", Main: true, SHA: "aaa"},
			{Path: "/wt", Name: "wt", SHA: "bbb"},
		}})
	return m
}
