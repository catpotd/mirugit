package layout

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// drawState is a repository large enough for the cost to show: 200 changed
// files across ten directories, drawn into a pane taller than one screen.
func drawState() state.State {
	entries := make([]git.Entry, 0, 200)
	for i := range 200 {
		entries = append(entries, git.Entry{
			Path:          fmt.Sprintf("internal/pkg%02d/file%03d.go", i%10, i),
			Worktree:      git.Modified,
			WorktreeCount: git.Count{Added: i % 40, Deleted: i % 7},
		})
	}
	s := state.State{Width: 100, Height: 50, Tab: state.TabChanges}
	return state.Apply(s, state.StatusLoaded{Rows: entries, Head: git.Head{Branch: "main"}})
}

func BenchmarkPane(b *testing.B) {
	s := drawState()
	w := Renderer{}
	b.ReportAllocs()
	for b.Loop() {
		_ = Pane(s, nil, w)
	}
}

func BenchmarkPad(b *testing.B) {
	w := Widths{}
	b.ReportAllocs()
	for b.Loop() {
		_ = w.Pad("internal/pkg03/file042.go", "+12 −3", 100)
	}
}

// A frame is drawn on every keypress, so its cost is the cost of moving the
// cursor. Bytes are the bound rather than time: the same machine gives a
// different time under load and the same byte count always.
//
// Measured 287 KB for this state. The bound is a tenth above it rather than the
// forty percent it used to leave: a change that built a verb list for every row
// instead of the cursor's added 140 KB and passed, and only the race detector's
// own overhead pushed the total far enough to fail. A budget that only fails
// under -race is one that reports a defect in whichever job runs -race.
// Not parallel: ReadMemStats counts the whole process, so a test allocating
// beside this one lands in the figure. A test without t.Parallel runs while the
// parallel ones are paused, and other packages are other processes.
func TestDrawingAFrameStaysWithinItsBudget(t *testing.T) {
	if raceDetector {
		t.Skip("the detector allocates too, and the budget is this program's")
	}
	const (
		budgetBytes = 315 << 10
		frames      = 50
	)
	s := drawState()
	w := Renderer{}

	// A fixed number of frames rather than testing.Benchmark: the benchmark
	// picks its own iteration count from the clock, so under load it runs a
	// handful and the setup shows up in the per-frame figure. This measures
	// the same work every time.
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	for range frames {
		sink = Pane(s, nil, w)
	}
	runtime.ReadMemStats(&after)

	perFrame := (after.TotalAlloc - before.TotalAlloc) / frames
	if perFrame > budgetBytes {
		t.Errorf("one frame allocates %d bytes, budget %d", perFrame, budgetBytes)
	}
}

// sink keeps the compiler from dropping the frame it was asked to build.
var sink Frame

// scaleState is a repository of n changed files, drawn into the same pane. The
// pane shows fifty rows whatever n is.
func scaleState(n int) state.State {
	entries := make([]git.Entry, 0, n)
	for i := range n {
		entries = append(entries, git.Entry{
			Path:          fmt.Sprintf("internal/pkg%02d/file%05d.go", i%10, i),
			Worktree:      git.Modified,
			WorktreeCount: git.Count{Added: i % 40, Deleted: i % 7},
		})
	}
	s := state.State{Width: 100, Height: 50, Tab: state.TabChanges}
	return state.Apply(s, state.StatusLoaded{Rows: entries, Head: git.Head{Branch: "main"}})
}

func BenchmarkPaneScale(b *testing.B) {
	for _, n := range []int{200, 2000, 20000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			s := scaleState(n)
			w := Renderer{}
			b.ReportAllocs()
			for b.Loop() {
				_ = Pane(s, nil, w)
			}
		})
	}
}

// The pane draws fifty rows whether the repository has two hundred changed
// files or twenty thousand, so what it allocates should not follow the
// repository. It did: the list was laid out whole and then sliced, which cost
// 3,963 allocations at two hundred files and 379,082 at twenty thousand.
//
// Counting allocations rather than time, because time on a shared machine is
// noise and an allocation is a decision the code made.
func TestDrawingDoesNotFollowTheSizeOfTheRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the drawing a hundred times")
	}
	if raceDetector {
		t.Skip("the detector allocates too, and the budget is this program's")
	}
	small := allocsPerPane(t, 200)
	large := allocsPerPane(t, 20000)

	// A hundred times the files, and the pane may allocate half again as much.
	// The slack is for the one slice per line that holds the list's shape,
	// which is what says where the window falls.
	if large > small*3/2 {
		t.Errorf("two hundred files allocate %d and twenty thousand allocate %d; "+
			"the drawing follows the repository rather than the pane", small, large)
	}
}

func allocsPerPane(t *testing.T, files int) uint64 {
	t.Helper()
	allocs, _ := paneCost(t, files)
	return allocs
}

func paneCost(t *testing.T, files int) (allocs, bytes uint64) {
	t.Helper()
	s := scaleState(files)
	w := Renderer{}
	// One frame first, so what a first call sets up is not counted.
	_ = Pane(s, nil, w)

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	for range 20 {
		_ = Pane(s, nil, w)
	}
	runtime.ReadMemStats(&after)
	return (after.Mallocs - before.Mallocs) / 20, (after.TotalAlloc - before.TotalAlloc) / 20
}

// The count of allocations went flat before the bytes did: the list still built
// one line and one click target for every row behind the window, and a target
// is a large struct. Measured at twenty thousand files, one frame allocated
// 30.4 MB, which is 1,520 bytes for each row the reader cannot see.
//
// Bytes per row rather than a ratio between two sizes, because the walk over
// the rows is still there and its cost is still linear — what this holds is the
// price of a row the pane does not draw. Counting bytes rather than time, for
// the reason the test above gives.
func TestARowOutsideTheWindowIsCheap(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the drawing a hundred times")
	}
	if raceDetector {
		t.Skip("the detector allocates too, and the budget is this program's")
	}
	const files = 20000
	// Measured at 161 bytes a row after the targets stopped being built, which
	// is the line holding each row's place. The ceiling is set close to it on
	// purpose: at 400 it still passed with the heading laying every row of its
	// section out again, which is 175 bytes a row and one of the two things
	// this test exists to keep out.
	const budget = 250

	_, bytes := paneCost(t, files)
	if perRow := bytes / files; perRow > budget {
		t.Errorf("one frame allocates %d bytes for %d files, %d a row; budget %d a row",
			bytes, files, perRow, budget)
	}
}
