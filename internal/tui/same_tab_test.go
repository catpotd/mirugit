package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
)

// Switching tabs asks git for the new tab's rows. Asking for the tab already on
// screen spends a git process on an answer the pane is holding, and the digit
// keys and the tab bar are two ways to ask for it.
func TestAskingForTheTabAlreadyOnScreenReadsNothing(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		ask  func(*Model) tea.Cmd
		read bool
	}{
		{"the digit of the tab on screen", func(m *Model) tea.Cmd {
			_, cmd := m.navKey(tea.KeyPressMsg{Code: '1'})
			return cmd
		}, false},
		{"the digit of another tab", func(m *Model) tea.Cmd {
			_, cmd := m.navKey(tea.KeyPressMsg{Code: '2'})
			return cmd
		}, true},
		{"a click on the tab on screen", func(m *Model) tea.Cmd {
			_, cmd := m.click(layout.Target{Kind: layout.TargetTab, Tab: state.TabChanges}, 0)
			return cmd
		}, false},
		{"a click on another tab", func(m *Model) tea.Cmd {
			_, cmd := m.click(layout.Target{Kind: layout.TargetTab, Tab: state.TabHistory}, 0)
			return cmd
		}, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := &Model{state: state.State{Width: 90, Height: 30},
				render: layout.Renderer{}, dir: t.TempDir(), read: emptyRead()}
			m.probe.settled = true
			m.state.Changes.Folded = map[string]bool{}
			m.state.Changes.Selected = map[string]bool{}
			m.state.Changes.Stale = map[string]bool{}
			m.state = state.Apply(m.state, state.StatusLoaded{
				Rows: []git.Entry{{Path: "a.txt", Worktree: git.Modified}},
				Head: git.Head{Branch: "main"}})
			// The digit keys refuse a tab the bar does not draw, and the bar
			// draws history only once there is a commit.
			m.state = state.Apply(m.state, state.HistoryLoaded{Commits: []git.CommitInfo{
				{SHA: "aaa", ShortSHA: "aaa", Subject: "one", Age: "1m"}}})
			m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabChanges})
			if !m.tabShown(state.TabHistory) {
				t.Fatal("the history tab is not on the bar, so the digit proves nothing")
			}
			if m.state.Tab != state.TabChanges {
				t.Fatalf("the pane starts on %v, so this proves nothing", m.state.Tab)
			}

			if got := c.ask(m) != nil; got != c.read {
				t.Errorf("git was asked for rows %v, want %v", got, c.read)
			}
		})
	}
}
