package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
)

func historyModelWithMore(t *testing.T) *Model {
	t.Helper()
	s := state.State{Width: 77, Height: 24, Tab: state.TabHistory}
	s.Changes.Selected = map[string]bool{}
	s = state.Apply(s, state.HistoryLoaded{Commits: []git.CommitInfo{
		{SHA: "c", Subject: "third"},
		{SHA: "b", Subject: "second"},
	}, HasMore: true})
	s.Cursor = len(s.Rows) - 1
	return &Model{state: s, render: layout.Renderer{}, read: emptyRead(), dir: t.TempDir()}
}

func TestHistoryBottomRequestsTheNextPage(t *testing.T) {
	t.Parallel()
	m := historyModelWithMore(t)
	next, cmd := m.moveCursor(tea.KeyPressMsg{}, 1)
	m = next.(*Model)
	if !m.state.History.LoadingMore {
		t.Error("the bottom history row did not start the next page read")
	}
	if cmd == nil {
		t.Error("the bottom history row did not issue a page read")
	}
}

func TestHistoryMoreMessageAppendsTheMatchingPage(t *testing.T) {
	t.Parallel()
	m := historyModelWithMore(t)
	m.state = state.Apply(m.state, state.HistoryMoreRequested{})
	next, _ := m.Update(historyMoreMsg{
		offset:    2,
		newestSHA: "c",
		page: git.HistoryPage{Commits: []git.CommitInfo{
			{SHA: "a", Subject: "first"},
		}},
	})
	m = next.(*Model)
	if got := len(m.state.History.Commits); got != 3 {
		t.Fatalf("commits = %d, want 3", got)
	}
	if m.state.History.Commits[2].SHA != "a" {
		t.Errorf("last commit = %q, want a", m.state.History.Commits[2].SHA)
	}
	if m.state.History.LoadingMore {
		t.Error("a completed page read left history loading")
	}
}

func TestHistoryMoreMessageDropsAPageAfterHistoryReloaded(t *testing.T) {
	t.Parallel()
	m := historyModelWithMore(t)
	m.state = state.Apply(m.state, state.HistoryMoreRequested{})
	m.state = state.Apply(m.state, state.HistoryLoaded{Commits: []git.CommitInfo{
		{SHA: "d", Subject: "new tip"},
	}, HasMore: true})
	next, _ := m.Update(historyMoreMsg{
		offset:    2,
		newestSHA: "c",
		page:      git.HistoryPage{Commits: []git.CommitInfo{{SHA: "a", Subject: "first"}}},
	})
	m = next.(*Model)
	if got := len(m.state.History.Commits); got != 1 {
		t.Fatalf("commits = %d, want 1 after dropping stale page", got)
	}
	if m.state.History.LoadingMore {
		t.Error("a stale page left history loading")
	}
}
