package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
)

// y confirms a discard and also copies a commit SHA. A pending dialog has to win,
// or a reader answering "yes" would copy a SHA instead.
func TestYConfirmsBeforeItCopies(t *testing.T) {
	t.Parallel()
	commit := git.CommitInfo{SHA: "abc", Subject: "one"}
	m := &Model{
		state: state.State{
			Tab: state.TabHistory, Width: 77, Height: 24,
			History: state.History{Commits: []git.CommitInfo{commit}},
			Rows:    []state.Row{state.CommitRow(commit)},
			Changes: state.Changes{
				DiscardConfirm: &state.DiscardConfirm{Targets: []string{"a.txt"}, Files: 1},
			},
		},
		render: layout.Renderer{Clipboard: true},
		read:   emptyRead(),
		dir:    t.TempDir(),
	}
	next, _ := m.Update(tea.KeyPressMsg{Code: 'y'})
	if next.(*Model).state.Changes.DiscardConfirm != nil {
		t.Error("確認中の y が discard を確定していない")
	}
}
