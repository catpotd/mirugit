package tui

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
)

func modelWithOpenDiff(t *testing.T) *Model {
	t.Helper()
	blocks := make([]git.Block, 6)
	for i := range blocks {
		blocks[i] = git.Block{
			Header: "@@ -1 +1 @@",
			Lines:  []string{"+line"},
			Added:  1,
			Hash:   fmt.Sprintf("h%d", i+1),
		}
	}
	s := state.State{Width: 77, Height: 30, Tab: state.TabChanges}
	s = state.Apply(s, state.StatusLoaded{
		Rows: []git.Entry{{Path: "a.txt", Worktree: git.Modified}},
		Head: git.Head{Branch: "main"},
	})
	s = state.Apply(s, state.DiffOpened{Path: "a.txt", Origin: state.WorkingTree(state.SectionUnstaged)})
	s = state.Apply(s, state.DiffLoaded{
		Diff: git.FileDiff{Path: "a.txt", Blocks: blocks}})
	m := &Model{state: s, render: layout.Renderer{}, read: emptyRead(), dir: t.TempDir()}
	m.probe.settled = true
	return m
}

// Clicking a block is the only way to reach a block without stepping through
// every one before it, and nothing asserted that the click landed.
func TestClickingABlockMovesTheBlockCursor(t *testing.T) {
	t.Parallel()
	m := modelWithOpenDiff(t)
	m.View()

	want := 3
	hit := layout.Region{Row: -1}
	for _, r := range m.frame.Regions {
		if r.Target.Kind == layout.TargetBlock && r.Target.Block == want {
			hit = r
			break
		}
	}
	if hit.Row < 0 {
		t.Fatalf("ブロック %d の領域が描画されていない", want)
	}

	next, _ := m.Update(tea.MouseClickMsg{
		X: hit.ColStart, Y: hit.Row, Button: tea.MouseLeft})
	m = next.(*Model)
	if m.state.Open.BlockCursor != want {
		t.Errorf("ブロック %d をクリックしたが BlockCursor は %d",
			want, m.state.Open.BlockCursor)
	}
}

// The stage verb drawn beside a block stages that block; the same verb on the
// row stages the file. verbClick tells them apart by the block the region
// carries, and the first block is numbered 0 — a bound that excludes it sends
// the click to the row's verb, which stages the whole file. That is the wrong
// change made, not a change refused.
func TestClickingStageOnAnyBlockStagesThatBlock(t *testing.T) {
	t.Parallel()
	for _, block := range []int{0, 1, 5} {
		t.Run(fmt.Sprintf("block %d", block), func(t *testing.T) {
			t.Parallel()
			m := modelWithOpenDiff(t)
			m.state = state.Apply(m.state, state.BlockCursorSet{Block: 3})

			next, _ := m.verbClick(layout.Target{Verb: state.VerbNameStage, Block: block})
			m = next.(*Model)

			if m.state.Open.BlockCursor != block {
				t.Errorf("clicked stage on block %d and the block cursor is %d; "+
					"the click went to the row's verb, which stages the file",
					block, m.state.Open.BlockCursor)
			}
		})
	}
}
