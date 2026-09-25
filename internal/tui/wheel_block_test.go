package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
)

// wheelModel opens a diff whose blocks each fill the diff area, so one notch of
// the wheel moves the view by exactly one block and no two blocks are on screen
// together.
func wheelModel(t *testing.T) *Model {
	t.Helper()
	m := newParentModel(t)
	m.state = state.Apply(m.state, state.StatusLoaded{
		Rows: []git.Entry{{Path: "a.txt", Worktree: git.Modified}},
		Head: git.Head{Branch: "main"}})
	m.state = state.Apply(m.state, state.DiffOpened{Path: "a.txt",
		Origin: state.WorkingTree(state.SectionUnstaged)})
	rows := m.diffRowsNow()
	if rows < 3 {
		t.Fatalf("the diff area is %d rows; this needs at least 3", rows)
	}
	// The area opens with a blank row and each block takes a head row, so a
	// block of this many lines is exactly what one screenful holds.
	lines := make([]string, rows-2)
	for i := range lines {
		lines[i] = "+x"
	}
	blocks := make([]git.Block, 4)
	for i := range blocks {
		blocks[i] = git.Block{Header: fmt.Sprintf("@@ -%d +%d @@", i*20+1, i*20+1),
			Lines: lines, Hash: fmt.Sprintf("h%d", i)}
	}
	_, _ = m.handleDiffMsg(diffMsg{Diff: git.FileDiff{Path: "a.txt", Blocks: blocks}})
	return m
}

func wheelOverDiff(t *testing.T, m *Model, notches int) {
	t.Helper()
	listRows, _, _ := m.windowRows()
	y := layout.HeaderRowsOf(m.state) + listRows
	for range notches {
		_, _, _ = m.handleMouseWheel(tea.MouseWheelMsg{Button: tea.MouseWheelDown, Y: y})
	}
}

// Only the block under the block cursor carries the "s stage" label, and only it
// is what s stages. Scrolling the view without the cursor left the footer
// offering a key that acted on a block the reader could not see.
func TestTheWheelMovesTheBlockThatStageActsOn(t *testing.T) {
	t.Parallel()
	m := wheelModel(t)
	wheelOverDiff(t, m, 2)

	if got, want := m.state.Open.BlockCursor, m.state.Open.Scroll; got != want {
		t.Errorf("s stages block %d while the pane shows block %d", got, want)
	}
	if m.state.Open.Scroll == 0 {
		t.Error("the wheel moved nothing, so this proves nothing")
	}
}

func TestWheelReachesTheEndOfAnOversizedBlock(t *testing.T) {
	t.Parallel()
	m := newParentModel(t)
	m.state = state.Apply(m.state, state.StatusLoaded{
		Rows: []git.Entry{{Path: "a.txt", Worktree: git.Modified}},
		Head: git.Head{Branch: "main"},
	})
	m.state = state.Apply(m.state, state.DiffOpened{Path: "a.txt",
		Origin: state.WorkingTree(state.SectionUnstaged)})
	lines := make([]string, m.diffRowsNow()+10)
	for i := range lines {
		lines[i] = fmt.Sprintf("+line-%03d", i)
	}
	_, _ = m.handleDiffMsg(diffMsg{Diff: git.FileDiff{Path: "a.txt", Blocks: []git.Block{{
		Header: "@@ -0,0 +1,40 @@", OldStart: 0, NewStart: 1, Lines: lines, Hash: "long",
	}}}})

	wheelOverDiff(t, m, len(lines))
	if m.state.Open.BlockLine == 0 {
		t.Fatal("the wheel did not move inside the oversized block")
	}
	lastLine := fmt.Sprintf("+line-%03d", len(lines)-1)
	if !strings.Contains(m.View().Content, lastLine) {
		t.Fatal("the last line of an oversized block is not reachable with the wheel")
	}
	_ = m.finishReading()
	if !m.read.BlockRead("a.txt", "long") {
		t.Fatal("the oversized block is still unread after its last line was shown")
	}
}

// Read marks record the blocks the reader could see. A notch that moved the
// view without recording left every block but the last one unread, however far
// the reader scrolled.
func TestTheWheelRecordsTheBlocksItScrolledPast(t *testing.T) {
	t.Parallel()
	m := wheelModel(t)
	wheelOverDiff(t, m, 3)
	_ = m.finishReading()

	for i := range 4 {
		hash := fmt.Sprintf("h%d", i)
		if !m.read.BlockRead("a.txt", hash) {
			t.Errorf("%s was scrolled through and is still unread", hash)
		}
	}
}
