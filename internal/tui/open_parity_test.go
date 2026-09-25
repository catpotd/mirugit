package tui

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
)

// enter とファイル行のクリックは同じ行を開く。二つの実装が別々に書かれている
// ので、片方を直したときにもう片方が置き去りになる。この test はそのずれを
// 見つけるためにあり、統一する前から通る。
func TestEnterAndClickOpenTheSameRow(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	for _, tc := range []struct {
		tab   state.Tab
		build func(*testing.T) *Model
	}{
		{state.TabChanges, changesModelWithFileRow},
		{state.TabHistory, historyModelWithFileRow},
		{state.TabStashed, stashedModelWithFileRow},
		{state.TabWorktrees, worktreesModelWithFileRow},
	} {
		name := state.Facts[tc.tab].Name
		byKey := tc.build(t)
		row, col := fileRowAt(t, byKey)
		byKey.state.Cursor = byKey.frame.Regions[row].Target.Row
		keyNext, keyCmd := byKey.diffForTab()

		byClick := tc.build(t)
		clickNext, clickCmd, _ := byClick.handleMouseClick(
			tea.MouseClickMsg{X: col, Y: byClick.frame.Regions[row].Row, Button: tea.MouseLeft})

		kp := keyNext.(*Model).state.Open.Path
		cp := clickNext.(*Model).state.Open.Path
		if kp != cp {
			t.Errorf("%s: enter は %q を開き、クリックは %q を開いた", name, kp, cp)
		}
		if (keyCmd == nil) != (clickCmd == nil) {
			t.Errorf("%s: enter cmd=%v, クリック cmd=%v", name, keyCmd != nil, clickCmd != nil)
		}
	}
}

// fileRowAt names the first region pointing at a RowFile. Stash and worktree
// rows also carry TargetFile, so the region kind alone picks the parent row.
func fileRowAt(t *testing.T, m *Model) (int, int) {
	t.Helper()
	for i, r := range m.frame.Regions {
		if r.Target.Kind != layout.TargetFile {
			continue
		}
		if r.Target.Row < 0 || r.Target.Row >= len(m.state.Rows) {
			continue
		}
		if m.state.Rows[r.Target.Row].Kind() == state.RowFile {
			return i, r.ColStart
		}
	}
	t.Fatalf("file row region not found in %d regions", len(m.frame.Regions))
	return 0, 0
}

func modelWith(t *testing.T, s state.State) *Model {
	t.Helper()
	m, err := New(context.Background(), testRepo(t), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.probe.settled = true
	m.state = s
	m.frame = layout.Pane(m.state, m.read, m.render)
	return m
}

func changesModelWithFileRow(t *testing.T) *Model {
	t.Helper()
	s := state.State{Width: 77, Height: 24, Tab: state.TabChanges}
	s = state.Apply(s, state.StatusLoaded{
		Rows: []git.Entry{{Path: "a.txt", Worktree: git.Modified}}})
	return modelWith(t, s)
}

func historyModelWithFileRow(t *testing.T) *Model {
	t.Helper()
	commit := git.CommitInfo{SHA: "abc", Subject: "one"}
	s := state.State{Width: 77, Height: 24, Tab: state.TabChanges}
	s = state.Apply(s, state.HistoryLoaded{Commits: []git.CommitInfo{commit}})
	s = state.Apply(s, state.TabChanged{Tab: state.TabHistory})
	s = state.Apply(s, state.CommitExpanded{SHA: "abc", Files: []git.Entry{{Path: "a.txt"}}})
	return modelWith(t, s)
}

func stashedModelWithFileRow(t *testing.T) *Model {
	t.Helper()
	stash := git.StashRow{Ref: "stash@{0}", SHA: "s1", Message: "m", Status: git.StashApplies}
	s := state.State{Width: 77, Height: 24, Tab: state.TabChanges}
	s = state.Apply(s, state.StashedLoaded{Stashes: []git.StashRow{stash}})
	s = state.Apply(s, state.TabChanged{Tab: state.TabStashed})
	s = state.Apply(s, state.StashFilesLoaded{Ref: "stash@{0}", Files: []git.Entry{{Path: "a.txt"}}})
	return modelWith(t, s)
}

func worktreesModelWithFileRow(t *testing.T) *Model {
	t.Helper()
	s := state.State{Width: 77, Height: 24, Tab: state.TabChanges}
	s = state.Apply(s, state.WorktreesLoaded{
		Worktrees: []git.WorktreeRow{
			{Path: "/repo", Name: "repo", Main: true},
			{Path: "/wt", Name: "wt", SHA: "abc"},
		}, Base: "main"})
	s = state.Apply(s, state.TabChanged{Tab: state.TabWorktrees})
	s.Cursor = 1
	s = state.Apply(s, state.WorktreeFilesLoaded{Path: "/wt", Files: []git.Entry{{Path: "a.txt"}}})
	return modelWith(t, s)
}
