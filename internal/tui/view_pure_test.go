package tui

import (
	"strings"
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
)

// View draws and nothing else. It used to call syncStale, which shells out to
// git, so one git process ran on every message the pane was sent.
func TestViewDoesNotReachTheRepository(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	entry := git.Entry{Path: "a.txt", Worktree: git.Modified}
	m := &Model{
		state: state.State{
			Tab: state.TabChanges, Width: 77, Height: 24,
			Changes: state.Changes{Entries: []git.Entry{entry}, Selected: map[string]bool{}},
			Rows:    []state.Row{state.FileRow(entry, state.SectionUnstaged)},
			Open: state.OpenDiff{
				Path:   "a.txt",
				Origin: state.WorkingTree(state.SectionUnstaged),
				Diff:   git.FileDiff{Path: "a.txt", Blocks: []git.Block{{Hash: "h"}}},
			},
		},
		render: layout.Renderer{}, read: emptyRead(),
		// An empty dir makes every git call fail loudly rather than run here.
		dir: "",
	}
	m.probe.settled = true
	before := m.state
	v := m.View()
	if !strings.Contains(v.Content, "a.txt") {
		t.Fatalf("描画が空: %q", v.Content)
	}
	if m.state.Changes.Stale[before.Open.Path] != before.Changes.Stale[before.Open.Path] {
		t.Error("View が Stale を書き換えた")
	}
	if m.state.Fetched != before.Fetched {
		t.Error("View が Fetched を書き換えた")
	}
	_ = dir
}
