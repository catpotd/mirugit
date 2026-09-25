package tui

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
)

func longListState(t *testing.T) state.State {
	t.Helper()
	entries := make([]git.Entry, 0, 40)
	commits := make([]git.CommitInfo, 0, 40)
	stashes := make([]git.StashRow, 0, 40)
	trees := make([]git.WorktreeRow, 0, 40)
	for i := range 40 {
		entries = append(entries, git.Entry{
			Path: fmt.Sprintf("f%02d.txt", i), Worktree: git.Modified})
		commits = append(commits, git.CommitInfo{SHA: fmt.Sprintf("c%02d", i), Subject: "s"})
		stashes = append(stashes, git.StashRow{
			Ref: fmt.Sprintf("stash@{%d}", i), SHA: fmt.Sprintf("s%02d", i), Message: "m"})
		trees = append(trees, git.WorktreeRow{
			Path: fmt.Sprintf("/w%02d", i), Name: fmt.Sprintf("w%02d", i), Main: i == 0})
	}
	s := state.State{Width: 77, Height: 20, Tab: state.TabChanges}
	s = state.Apply(s, state.StatusLoaded{Rows: entries})
	s = state.Apply(s, state.HistoryLoaded{Commits: commits})
	s = state.Apply(s, state.StashedLoaded{Stashes: stashes})
	s = state.Apply(s, state.WorktreesLoaded{Worktrees: trees, Base: "main"})
	return s
}

// The wheel moves the window without moving the cursor, on every tab. It was
// not handled at all: Update had no MouseWheelMsg case, so every notch was
// dropped.
func TestWheelScrollsEveryTab(t *testing.T) {
	t.Parallel()
	base := longListState(t)
	for _, tab := range []state.Tab{
		state.TabChanges, state.TabHistory, state.TabStashed, state.TabWorktrees,
	} {
		name := state.Facts[tab].Name
		m := &Model{
			state:  state.Apply(base, state.TabChanged{Tab: tab}),
			render: layout.Renderer{}, read: emptyRead(), dir: t.TempDir(),
		}
		cursor := m.state.Cursor
		next, _ := m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: 12, Y: 10})
		m = next.(*Model)
		if m.state.ScrollTop == 0 {
			t.Errorf("%s: ホイールを下へ回しても ScrollTop が 0 のまま", name)
		}
		if m.state.Cursor != cursor {
			t.Errorf("%s: ホイールがカーソルを %d から %d へ動かした",
				name, cursor, m.state.Cursor)
		}
		for range 60 {
			n, _ := m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: 12, Y: 10})
			m = n.(*Model)
		}
		bottom := m.state.ScrollTop
		n, _ := m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: 12, Y: 10})
		if got := n.(*Model).state.ScrollTop; got != bottom {
			t.Errorf("%s: 下端を越えて %d から %d へ動いた", name, bottom, got)
		}
		for range 60 {
			n, _ := m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp, X: 12, Y: 10})
			m = n.(*Model)
		}
		if m.state.ScrollTop != 0 {
			t.Errorf("%s: 上端まで戻して ScrollTop = %d, want 0", name, m.state.ScrollTop)
		}
	}
}
