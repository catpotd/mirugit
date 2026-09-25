package tui

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// Every verb that writes to the repository answers with a message, and each
// answer has to do two things: say so when it failed, and reload when it did
// not. A handler that gets the test backwards drops both — the reader sees no
// error and the list keeps showing what the verb removed until the next poll.
//
// The handlers are one shape written six times, so they are driven from one
// table. A mutation of the test in handleStashDropMsg survived the suite.
func TestEveryWriteAnswersWithAnErrorOrAReload(t *testing.T) {
	t.Parallel()
	failed := errors.New("git said no")

	for _, c := range []struct {
		name string
		ok   tea.Msg
		bad  tea.Msg
	}{
		{"discard", discardMsg{finished: state.DiscardFinished{Applied: 1}},
			discardMsg{err: failed}},
		{"undo of a discard", undiscardMsg{finished: state.DiscardFinished{Applied: 1, Restored: true}},
			undiscardMsg{err: failed}},
		{"dropping a stash", stashDropMsg{}, stashDropMsg{err: failed}},
		{"restoring a stash", stashVerbMsg{}, stashVerbMsg{err: failed}},
		{"removing a worktree", worktreeRemoveMsg{}, worktreeRemoveMsg{err: failed}},
		{"committing", commitMsg{finished: state.CommitFinished{SHA: "abc", Files: 1}},
			commitMsg{err: failed}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			done := newParentModel(t)
			next, cmd := done.Update(c.ok)
			done = next.(*Model)
			if cmd == nil {
				t.Errorf("nothing was reloaded after %s finished, so the list keeps "+
					"showing what it changed", c.name)
			}
			if done.state.NoticeFailed {
				t.Errorf("%s finished and the pane says it failed: %q",
					c.name, done.state.Notice)
			}

			broken := newParentModel(t)
			next, _ = broken.Update(c.bad)
			broken = next.(*Model)
			if !broken.state.NoticeFailed {
				t.Errorf("%s failed and the pane says nothing: %q",
					c.name, broken.state.Notice)
			}
		})
	}
}

// cursorPath answers the path of a file the working tree holds, and "" for
// everything else. A stash row's path is its ref and a worktree row's is a
// directory; handing either to a command that expects a file asks git about a
// path that is not one.
//
// No reader can reach that today, because the two tabs whose rows are stashes
// and worktrees do not follow the cursor. The guard is what keeps that true, so
// this states it rather than leaving it to the tabs' other settings.
func TestCursorPathNamesOnlyAFileTheWorkingTreeHolds(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		row  state.Row
		want string
	}{
		{"a file on the unstaged side",
			state.FileRow(git.Entry{Path: "a.txt", Worktree: git.Modified}, state.SectionUnstaged),
			"a.txt"},
		{"a file on the staged side",
			state.FileRow(git.Entry{Path: "a.txt", Index: git.Modified}, state.SectionStaged),
			"a.txt"},
		{"a file listed under a stash or a worktree",
			state.FileRow(git.Entry{Path: "a.txt"}, state.SectionNone),
			""},
		{"a stash", state.StashRowOf(git.StashRow{Ref: "stash@{0}", SHA: "s0"}), ""},
		{"a worktree", state.WorktreeRowOf(git.WorktreeRow{Path: "/wt", Name: "wt"}), ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := newParentModel(t)
			m.state.Rows = []state.Row{c.row}
			m.state.Cursor = 0
			if got := m.cursorPath(); got != c.want {
				t.Errorf("cursorPath is %q, want %q", got, c.want)
			}
		})
	}
}
