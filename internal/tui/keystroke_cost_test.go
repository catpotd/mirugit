package tui

import (
	"fmt"
	"runtime"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
)

// Every key and every pointer move runs the update loop, and the update loop
// asks where the cursor sits and how far the list scrolls. Both answers are
// positions, and both were being answered by laying the whole list out: a
// keystroke in a repository of twenty thousand changed files cost 209 ms and
// 280,158 allocations, and the pointer moves on its own.
//
// The pane shows fifty rows whatever the repository holds, so neither answer
// should follow the repository.
func TestAKeystrokeDoesNotFollowTheSizeOfTheRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a repository of twenty thousand files and runs the update loop over it")
	}
	if raceDetector {
		t.Skip("the detector allocates too, and the budget is this program's")
	}
	small := allocsPerKeystroke(t, 200, 'j')
	large := allocsPerKeystroke(t, 20000, 'j')

	// The two slices that hold the list's shape still grow with the repository,
	// and a growing slice reallocates as it doubles: measured 28, 37, 54 and 74
	// allocations at 200, 2,000, 20,000 and 200,000 files — a hundredfold in
	// input for twice the count. Laying the rows out instead cost 2,906 and
	// 280,158 at the first and third of those.
	//
	// Three times the count is the room that leaves: it holds against doubling
	// but not against following the repository.
	if large > small*3 {
		t.Errorf("two hundred files cost %d allocations per keystroke and twenty "+
			"thousand cost %d; the update loop follows the repository", small, large)
	}
}

// Opening a diff asks a second question of the list — whether the pane has room
// for one — and that question was answered by laying every row out. j moves the
// cursor and never reaches it, so the budget above passed while d cost the whole
// repository: measured 2,922 allocations at 200 files and 280,201 at 20,000,
// against 44 and 94 after the count replaced the layout.
func TestOpeningADiffDoesNotFollowTheSizeOfTheRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a repository of twenty thousand files and runs the update loop over it")
	}
	if raceDetector {
		t.Skip("the detector allocates too, and the budget is this program's")
	}
	small := allocsPerKeystroke(t, 200, 'd')
	large := allocsPerKeystroke(t, 20000, 'd')

	if large > small*3 {
		t.Errorf("two hundred files cost %d allocations per d and twenty thousand "+
			"cost %d; opening a diff follows the repository", small, large)
	}
}

func allocsPerKeystroke(t *testing.T, files int, key rune) uint64 {
	t.Helper()
	rows := make([]git.Entry, files)
	for i := range rows {
		rows[i] = git.Entry{
			Path:     fmt.Sprintf("pkg%02d/f%05d.go", i%10, i),
			Worktree: git.Modified,
		}
	}
	m := &Model{read: emptyRead(), render: layout.Renderer{},
		state: state.State{Width: 100, Height: 50}, dir: t.TempDir()}
	m.probe.settled = true
	m.state.Changes.Folded = map[string]bool{}
	m.state.Changes.Selected = map[string]bool{}
	m.state.Changes.Stale = map[string]bool{}
	m.state = state.Apply(m.state, state.StatusLoaded{Rows: rows,
		Head: git.Head{Branch: "main"}})

	next, _ := m.Update(tea.KeyPressMsg{Code: key})
	m = next.(*Model)

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	for range 20 {
		n, _ := m.Update(tea.KeyPressMsg{Code: key})
		m = n.(*Model)
	}
	runtime.ReadMemStats(&after)
	return (after.Mallocs - before.Mallocs) / 20
}
