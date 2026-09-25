package state

import (
	"testing"

	"github.com/catpotd/mirugit/internal/git"
)

// Holding j asks for one diff per row and the answers race. A late answer for a
// row the reader has left must not be drawn as the row they are on now.
func TestALateDiffAnswerIsDropped(t *testing.T) {
	t.Parallel()
	s := State{Width: 77, Height: 24, Tab: TabChanges}
	s = Apply(s, StatusLoaded{Rows: []git.Entry{
		{Path: "a.txt", Worktree: git.Modified},
		{Path: "b.txt", Worktree: git.Modified},
	}})
	s = Apply(s, DiffOpened{Path: "b.txt", Origin: WorkingTree(SectionUnstaged)})
	s = Apply(s, DiffLoaded{Diff: git.FileDiff{
		Path: "a.txt", Blocks: []git.Block{{Hash: "h-of-a"}}}})
	if got := len(s.Open.Diff.Blocks); got != 0 {
		t.Errorf("a.txt の応答を b.txt の diff として採った: Path=%q Blocks=%d",
			s.Open.Diff.Path, got)
	}
	s = Apply(s, DiffLoaded{Diff: git.FileDiff{
		Path: "b.txt", Blocks: []git.Block{{Hash: "h-of-b"}}}})
	if got := len(s.Open.Diff.Blocks); got != 1 {
		t.Errorf("b.txt の応答を落とした: Blocks=%d", got)
	}
}
