package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

func TestTabBarSaysNeverFetchedWithoutFetchHead(t *testing.T) {
	t.Parallel()
	dir := testRepo(t)
	m, err := New(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 53})
	m.probe.settled = true
	m.View()
	if !strings.Contains(m.frame.Lines[0], "never fetched") {
		t.Fatalf("tab bar = %q, want never fetched on the right", m.frame.Lines[0])
	}
}

func TestPressingTheCurrentTabNumberKeepsTheCursor(t *testing.T) {
	t.Parallel()
	dir := repoWithUnstagedChange(t)
	m, err := New(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 53})
	m.probe.settled = true
	repo, err := git.Load(context.Background(), dir, mustGitDir(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	m.state = state.Apply(m.state, state.StatusLoaded{Rows: repo.Entries, Head: repo.Head})
	next, _ := m.Update(tea.KeyPressMsg{Code: 'j'})
	m = next.(*Model)
	before := m.state.Cursor
	next, _ = m.Update(tea.KeyPressMsg{Code: '1'})
	m = next.(*Model)
	if m.state.Cursor != before || m.state.Tab != state.TabChanges {
		t.Errorf("cursor=%d want %d, tab=%d", m.state.Cursor, before, m.state.Tab)
	}
}

func TestATabRoundTripRestoresTheCursor(t *testing.T) {
	t.Parallel()
	dir := repoWithUnstagedChange(t)
	m, err := New(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 53})
	m.probe.settled = true
	repo, err := git.Load(context.Background(), dir, mustGitDir(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	m.state = state.Apply(m.state, state.StatusLoaded{Rows: repo.Entries, Head: repo.Head})
	next, _ := m.Update(tea.KeyPressMsg{Code: 'j'})
	m = next.(*Model)
	before := m.state.Cursor
	next, _ = m.Update(tea.KeyPressMsg{Code: '2'})
	m = next.(*Model)
	next, _ = m.Update(tea.KeyPressMsg{Code: '1'})
	m = next.(*Model)
	if m.state.Cursor != before {
		t.Errorf("got %d, want %d", m.state.Cursor, before)
	}
}
