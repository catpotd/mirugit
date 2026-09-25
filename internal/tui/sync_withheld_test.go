package tui

import (
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
)

// S pulls and pushes, and both need a branch with an upstream. A detached HEAD
// has neither: git would refuse, and the refusal would reach the reader as a
// line of git's own words about a key they were offered.
//
// README says nothing is offered that cannot be pressed. The header does not
// draw the sync button here, so the key does nothing — and that pairing is what
// this checks, because a key that is drawn and does nothing is the same defect
// read from the other side.
func TestSyncIsWithheldWhereItCannotRun(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name    string
		head    git.Head
		offered bool
	}{
		{"a detached HEAD has no branch to push",
			git.Head{Detached: true}, false},
		{"a branch with no upstream has nowhere to push",
			git.Head{Branch: "main", Ahead: 2}, false},
		{"a branch level with its upstream has nothing to do",
			git.Head{Branch: "main", HasUpstream: true}, false},
		{"a branch ahead of its upstream has something to push",
			git.Head{Branch: "main", HasUpstream: true, Ahead: 1}, true},
		{"a branch behind its upstream has something to pull",
			git.Head{Branch: "main", HasUpstream: true, Behind: 1}, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := fixed(t)
			m.dir = t.TempDir()
			m.state = state.Apply(m.state, state.StatusLoaded{
				Head: c.head,
				Rows: []git.Entry{{Path: "a.txt", Worktree: git.Modified}},
			})

			// requestSync answers with no command at all when the button is
			// not drawn, so the command's presence is the key's answer. Running
			// it would reach the network.
			_, cmd := m.requestSync()
			if ran := cmd != nil; ran != c.offered {
				t.Errorf("S ran git = %v, and the header offers the button = %v",
					ran, c.offered)
			}
			if got := layout.SyncOffered(c.head); got != c.offered {
				t.Errorf("the header offers the button = %v, want %v", got, c.offered)
			}
		})
	}
}
