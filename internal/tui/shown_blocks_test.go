package tui

import (
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// Hovering a file in the history tab opens it before git answers, so the pane
// holds an open path with no blocks for as long as the read takes. The blocks
// recorded for the file before it are not written to the marks until the diff
// closes, and starting a fresh record in that window drops them: the reader
// scrolled through them and the pane forgot.
func TestTheBlockRecordSurvivesADiffThatHasNotArrived(t *testing.T) {
	t.Parallel()
	commit := git.CommitInfo{SHA: "aaa", ShortSHA: "aaa", Subject: "one"}
	files := []git.Entry{
		{Path: "f1.txt", Index: git.Modified},
		{Path: "f2.txt", Index: git.Modified},
	}
	m := historyModel(t, []git.CommitInfo{commit}, "aaa", files, 1)

	m.state = state.Apply(m.state, state.DiffOpened{Path: "f1.txt", Origin: state.FromCommit("aaa")})
	rows := m.diffRowsNow()
	if rows < 2 {
		t.Fatalf("the diff area is %d rows; this needs at least 2", rows)
	}
	// One block fills the area, so the second is reached only by scrolling:
	// the area opens with a blank row and the block takes a head row.
	lines := make([]string, rows-2)
	for i := range lines {
		lines[i] = "+x"
	}
	diff := git.FileDiff{Path: "f1.txt", Blocks: []git.Block{
		{Header: "@@ -1 +1 @@", Lines: lines, Hash: "hash-a"},
		{Header: "@@ -9 +9 @@", Lines: lines, Hash: "hash-b"},
	}}
	_, _ = m.handleDiffMsg(diffMsg{Diff: diff})
	_, _ = m.moveBlockCursor(1)
	if len(m.shown.hashes) != 2 {
		t.Fatalf("the reader has not seen both blocks yet: %v", m.shown.hashes)
	}

	m.state = state.Apply(m.state, state.CursorMoved{By: 1})
	if cmd := m.followHistoryCursor(); cmd == nil {
		t.Fatal("moving to f2.txt read no diff, so the window this needs never opens")
	}
	if m.state.Open.Path != "f2.txt" || len(m.state.Open.Diff.Blocks) != 0 {
		t.Fatalf("Open is %+v, want f2.txt with no blocks yet", m.state.Open)
	}
	_, _ = m.moveBlockCursor(1)

	m.state = state.Apply(m.state, state.CursorMoved{By: -1})
	_ = m.followHistoryCursor()
	_, _ = m.handleDiffMsg(diffMsg{Diff: diff})
	_ = m.finishReading()

	if !m.read.BlockRead("f1.txt", "hash-b") {
		t.Error("the block the reader scrolled to lost its mark while f2.txt was loading")
	}
}
