package tui

import (
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
)

// Committing runs git, and git rejects an empty message and an empty index with
// errors of its own. Letting either through turns "the key did nothing" into a
// git failure printed at the reader, which reads as a broken repository rather
// than as nothing to commit.
func TestCommitAsksGitOnlyWhenThereIsSomethingToCommit(t *testing.T) {
	t.Parallel()
	staged := []git.Entry{{Path: "a.txt", Index: git.Modified}}
	unstaged := []git.Entry{{Path: "a.txt", Worktree: git.Modified}}
	for _, c := range []struct {
		name    string
		message string
		rows    []git.Entry
		runs    bool
		notice  bool
	}{
		{"no message and nothing staged", "", unstaged, false, false},
		{"no message with something staged", "", staged, false, false},
		{"a message with nothing staged", "a message", unstaged, false, true},
		{"a message with something staged", "a message", staged, true, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := &Model{state: state.State{Width: 90, Height: 30},
				render: layout.Renderer{}, dir: t.TempDir(), read: emptyRead()}
			m.probe.settled = true
			m.state.Changes.Folded = map[string]bool{}
			m.state.Changes.Selected = map[string]bool{}
			m.state.Changes.Stale = map[string]bool{}
			m.state = state.Apply(m.state, state.StatusLoaded{Rows: c.rows,
				Head: git.Head{Branch: "main"}})
			m.state = state.Apply(m.state, state.MessageEdited{Text: c.message})
			if m.state.Changes.Message != c.message {
				t.Fatalf("the message did not take: %q", m.state.Changes.Message)
			}

			_, cmd := m.requestCommit()
			if (cmd != nil) != c.runs {
				t.Errorf("git was asked to commit %v, want %v", cmd != nil, c.runs)
			}
			if (m.state.Notice != "") != c.notice {
				t.Errorf("the pane said %q, want a notice: %v", m.state.Notice, c.notice)
			}
		})
	}
}
