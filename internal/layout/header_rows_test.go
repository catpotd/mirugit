package layout

import (
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// HeaderRowsOf is a count the table declares; the drawing emits the lines. The
// tui subtracts the count from a click's row to find which list row was hit
// (pane.go), so the two disagreeing by one puts every click on its neighbor.
// Every other test that touches the count reads it from the table on both
// sides, which stays green however far the two drift apart.
func TestTheCountOfHeaderLinesIsWhatTheDrawingEmits(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	for _, c := range []struct {
		name  string
		state func() state.State
	}{
		{"changes", ninetyFileState},
		{"history", historyPaneState},
		{"stashed", stashPaneStateWithFiles},
		{"worktrees", worktreePaneStateWithFiles},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			for _, notice := range []string{"", "a notice takes a line of its own"} {
				s := c.state()
				s.Notice = notice
				f := Pane(s, nil, w)

				first := -1
				for _, reg := range f.Regions {
					if !isListRow(reg.Target.Kind) {
						continue
					}
					if first < 0 || reg.Row < first {
						first = reg.Row
					}
				}
				if first < 0 {
					t.Fatalf("notice=%q: the pane drew no list row at all", notice)
				}
				if got := HeaderRowsOf(s); got != first {
					t.Errorf("notice=%q: HeaderRowsOf says %d lines sit above the list, "+
						"but the first list row is drawn on line %d", notice, got, first)
				}
			}
		})
	}
}

func historyPaneState() state.State {
	commits := []git.CommitInfo{
		{SHA: "aaa", ShortSHA: "aaa", Subject: "one", Age: "1m"},
		{SHA: "bbb", ShortSHA: "bbb", Subject: "two", Age: "2m"},
	}
	s := state.State{Width: paneWidth, Height: 20, Tab: state.TabHistory}
	return state.Apply(s, state.HistoryLoaded{Commits: commits})
}

// isListRow says whether a region sits on a row of the list rather than in the
// header. Written out in full so a new target kind has to be placed on one side
// or the other rather than silently counting as header.
func isListRow(kind TargetKind) bool {
	switch kind {
	case TargetFile, TargetDirectory, TargetSectionHeading, TargetCheckbox, TargetVerb:
		return true
	case TargetNone, TargetTab, TargetBlock, TargetButton, TargetCommitBox,
		TargetCommit, TargetHelp, TargetHelpLine:
		return false
	}
	return false
}
