package tui

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// R rereads what the pane draws, and the tab bar draws a count per tab. Only
// three of the four followed the repository: the worktree list was read once at
// startup, so a reader who added a worktree in another terminal and pressed R
// saw the old number and had to restart to reach the tab.
//
// Measured: git worktree list --porcelain takes 10 ms, which is what a key the
// reader pressed on purpose can spend.
func TestReloadRereadsTheWorktrees(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to git")
	}
	t.Parallel()
	dir := testRepo(t)
	m, err := New(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.probe.settled = true
	m.state = state.Apply(m.state, state.Resized{Width: 90, Height: 53})
	m = pumpInit(t, m)

	m.View()
	if strings.Contains(m.frame.Lines[0], "worktrees") {
		t.Fatalf("this test needs a repository with one worktree: %q", m.frame.Lines[0])
	}

	second := filepath.Join(t.TempDir(), "second")
	runGitIn(t, dir, "worktree", "add", "-q", second, "-b", "other")

	next, reload := m.Update(tea.KeyPressMsg{Code: 'r', Mod: tea.ModShift})
	m = pumpCmd(t, next.(*Model), reload)

	m.View()
	if !strings.Contains(m.frame.Lines[0], "worktrees") {
		t.Errorf("R did not find the worktree added beside it: %q", m.frame.Lines[0])
	}
}

// pumpCmd runs a command and everything it asks for, the way the runtime would.
func pumpCmd(t *testing.T, m *Model, cmd tea.Cmd) *Model {
	t.Helper()
	queue := []tea.Cmd{cmd}
	for i := 0; i < 40 && len(queue) > 0; i++ {
		next := queue[0]
		queue = queue[1:]
		if next == nil {
			continue
		}
		msg := next()
		if msg == nil {
			continue
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			queue = append(queue, batch...)
			continue
		}
		switch msg.(type) {
		case watchEventMsg, watchPollTickMsg, watchReadyMsg, watchDebounceDoneMsg:
			continue
		}
		model, out := m.Update(msg)
		m = model.(*Model)
		queue = append(queue, out)
	}
	return m
}

// What each tree holds — its file count, how far its branch is from the base —
// costs a git per tree, and only the worktrees tab draws it. Every reload now
// reads the worktree list so the bar's fourth count follows the repository, and
// a reload from the changes tab has to stop at the list.
func TestAReloadOffTheWorktreesTabDoesNotAskEachTree(t *testing.T) {
	t.Parallel()
	trees := []git.WorktreeRow{
		{Path: "/repo", Name: "repo", Main: true, SHA: "aaa"},
		{Path: "/wt", Name: "wt", SHA: "bbb"},
	}
	for _, c := range []struct {
		tab   state.Tab
		asks  bool
		named string
	}{
		{state.TabChanges, false, "changes"},
		{state.TabWorktrees, true, "worktrees"},
	} {
		t.Run(c.named, func(t *testing.T) {
			t.Parallel()
			m := fixed(t)
			m.state = state.Apply(m.state, state.TabChanged{Tab: c.tab})

			_, cmd := m.Update(worktreesMsg{trees: trees, base: "main"})

			asked := 0
			walkCmds(cmd, func(msg tea.Msg) {
				if _, ok := msg.(worktreeStatusMsg); ok {
					asked++
				}
			})
			if c.asks && asked == 0 {
				t.Error("the worktrees tab drew rows it never read")
			}
			if !c.asks && asked > 0 {
				t.Errorf("a reload on the %s tab spent %d gits on rows nobody draws",
					c.named, asked)
			}
		})
	}
}

// The worktree list is read once per reload. reloadCurrentTab reads it for the
// bar's count, and the worktrees tab's own Reload read it again: the tab whose
// rows cost a git each was the one that paid for them twice.
func TestAReloadReadsTheWorktreeListOnce(t *testing.T) {
	t.Parallel()
	for _, tab := range []state.Tab{state.TabChanges, state.TabHistory,
		state.TabStashed, state.TabWorktrees} {
		t.Run(state.Facts[tab].Name, func(t *testing.T) {
			t.Parallel()
			m := fixed(t)
			m.dir = t.TempDir()
			m.state = state.Apply(m.state, state.TabChanged{Tab: tab})

			read := 0
			walkCmds(m.reloadCurrentTab(), func(msg tea.Msg) {
				if _, ok := msg.(worktreesMsg); ok {
					read++
				}
			})
			if read != 1 {
				t.Errorf("a reload on the %s tab read the worktree list %d times",
					state.Facts[tab].Name, read)
			}
		})
	}
}

// A worktree's status is read one tree at a time, and a tree removed while its
// read is in flight is gone by the time git is asked. Reporting that put git's
// complaint on top of a remove that worked — and at a reader whose only part in
// it was having the tab open while another terminal removed one.
func TestAWorktreeReadTheListMovedPastIsNotReported(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabWorktrees})
	m.state = state.Apply(m.state, state.WorktreesLoaded{Base: "main", Here: "/repo",
		Worktrees: []git.WorktreeRow{
			{Path: "/repo", Name: "repo", Main: true, SHA: "aaa"},
			{Path: "/wt", Name: "wt", SHA: "bbb"},
		}})

	gone := errors.New("'/removed' is not a working tree")
	next, _ := m.Update(worktreeStatusMsg{index: 1, path: "/removed", err: gone})
	m = next.(*Model)

	if m.state.Notice != "" {
		t.Errorf("a read the list had moved past was reported: %q", m.state.Notice)
	}
}

// A read the list still holds failed on its own and is the reader's to see.
func TestAWorktreeReadThatFailsOnItsOwnIsReported(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabWorktrees})
	m.state = state.Apply(m.state, state.WorktreesLoaded{Base: "main", Here: "/repo",
		Worktrees: []git.WorktreeRow{
			{Path: "/repo", Name: "repo", Main: true, SHA: "aaa"},
			{Path: "/wt", Name: "wt", SHA: "bbb"},
		}})

	next, _ := m.Update(worktreeStatusMsg{index: 1, path: "/wt",
		err: errors.New("permission denied")})
	m = next.(*Model)

	if m.state.Notice != "permission denied" {
		t.Errorf("a read that failed on its own said %q", m.state.Notice)
	}
}

// Removing the tree under the cursor leaves the cursor on a row for a directory
// that is gone, and reading its files there asks git about a tree it no longer
// has. The new list is what moves the cursor off it, so nothing is read until
// that list arrives — the same shape a stash drop needed.
func TestRemovingAWorktreeDoesNotReadTheOneItRemoved(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabWorktrees})
	m.state = state.Apply(m.state, state.WorktreesLoaded{Base: "main", Here: "/repo",
		Worktrees: []git.WorktreeRow{
			{Path: "/repo", Name: "repo", Main: true, SHA: "aaa"},
			{Path: "/wt", Name: "wt", SHA: "bbb"},
		}})
	for i, row := range m.state.Rows {
		if row.Kind() == state.RowWorktree && row.Worktree().Path == "/wt" {
			m.state.Cursor = i
		}
	}

	_, cmd := m.Update(worktreeRemoveMsg{})

	read := 0
	walkCmds(cmd, func(msg tea.Msg) {
		if f, ok := msg.(worktreeFilesMsg); ok && f.path == "/wt" {
			read++
		}
	})
	if read > 0 {
		t.Error("the pane asked git about the worktree it had just removed")
	}
}

// A remove that could not run says so and asks for the list again. The shared
// failure path applies nothing and returns, which would leave the pane with the
// tree still drawn and no word about why.
func TestARemoveThatFailsSaysSoAndAsksForTheListAgain(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	m.dir = t.TempDir()
	m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabWorktrees})

	next, cmd := m.Update(worktreeRemoveMsg{err: errors.New("git refused")})
	m = next.(*Model)

	if m.state.Notice != "git refused" {
		t.Errorf("a remove that could not run said %q", m.state.Notice)
	}
	asked := false
	walkCmds(cmd, func(msg tea.Msg) {
		if _, ok := msg.(worktreesMsg); ok {
			asked = true
		}
	})
	if !asked {
		t.Error("a remove that failed did not ask for the list again")
	}
}
