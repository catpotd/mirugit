package git

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
)

// bigDiff is one file rewritten line for line, which is what a generated file, a
// lockfile or a minified bundle looks like to git.
func bigDiff(lines int) []byte {
	var b strings.Builder
	b.WriteString("diff --git a/big.txt b/big.txt\n")
	b.WriteString("index 1111111..2222222 100644\n")
	b.WriteString("--- a/big.txt\n+++ b/big.txt\n")
	fmt.Fprintf(&b, "@@ -1,%d +1,%d @@\n", lines, lines)
	for i := range lines {
		fmt.Fprintf(&b, "-original line %08d padding-text-here\n", i)
		fmt.Fprintf(&b, "+CHANGED line %08d padding-text-here\n", i)
	}
	return []byte(b.String())
}

// The diff of a file the pane cannot draw was held whole until the file was
// closed: 400,000 rewritten lines made git print 33 MB and this hold 211 MB of
// it. The pane shows a screen of lines at a time, so what it keeps must follow
// the screen rather than the file.
func TestAHugeDiffIsNotHeldWhole(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a diff of eight hundred thousand lines")
	}
	if raceDetector {
		t.Skip("the detector allocates too, and the budget is this program's")
	}
	out := bigDiff(400_000)

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	files := parseDiff(out, "big.txt")
	runtime.ReadMemStats(&after)
	held := after.HeapAlloc - min(after.HeapAlloc, before.HeapAlloc)

	if len(files) != 1 {
		t.Fatalf("parsed %d files, want one", len(files))
	}
	d := files[0]
	if !d.TooLong {
		t.Error("a diff of 800,000 lines is not marked too long")
	}
	if len(d.Blocks) != 0 {
		t.Errorf("%d blocks kept for a diff that will not be drawn", len(d.Blocks))
	}
	if d.Lines != 800_000 {
		t.Errorf("Lines = %d, want the 800,000 git printed", d.Lines)
	}
	// The input itself is 33 MB and the reader is told the count, so the figure
	// that matters is what survives the call.
	const budget = 4 << 20
	if held > budget {
		t.Errorf("held %d bytes after parsing, budget %d", held, budget)
	}
}

// A diff small enough to read is still read. The cap must not take the ordinary
// case with it.
func TestADiffUnderTheLimitKeepsItsBlocks(t *testing.T) {
	t.Parallel()
	files := parseDiff(bigDiff(10), "big.txt")
	if len(files) != 1 {
		t.Fatalf("parsed %d files, want one", len(files))
	}
	d := files[0]
	if d.TooLong {
		t.Error("a 20-line diff is marked too long")
	}
	if len(d.Blocks) != 1 {
		t.Fatalf("%d blocks, want one", len(d.Blocks))
	}
	if got := len(d.Blocks[0].Lines); got != 20 {
		t.Errorf("%d lines kept, want 20", got)
	}
	if d.Lines != 20 {
		t.Errorf("Lines = %d, want 20", d.Lines)
	}
}

// The limit is the largest diff that is still kept, not the smallest that is
// dropped. A file of exactly that many lines is one the pane can draw, and
// moving the line by one throws away a diff the reader was about to read.
func TestTheLineCountLimitIsTheLargestDiffThatIsKept(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("builds two diffs of twenty thousand lines")
	}
	atLimit := parseDiff(bigDiff(diffLineLimit/2), "big.txt")
	if len(atLimit) != 1 {
		t.Fatalf("parsed %d files, want one", len(atLimit))
	}
	d := atLimit[0]
	if d.Lines != diffLineLimit {
		t.Fatalf("the fixture is %d lines, want exactly the limit of %d",
			d.Lines, diffLineLimit)
	}
	if d.TooLong {
		t.Error("a diff of exactly the limit is marked too long")
	}
	if len(d.Blocks) != 1 {
		t.Errorf("%d blocks kept for a diff of exactly the limit", len(d.Blocks))
	}

	oneOver := parseDiff(append(bigDiff(diffLineLimit/2),
		[]byte("+one line past the limit\n")...), "big.txt")
	if len(oneOver) != 1 {
		t.Fatalf("parsed %d files, want one", len(oneOver))
	}
	d = oneOver[0]
	if d.Lines != diffLineLimit+1 {
		t.Fatalf("the fixture is %d lines, want one past the limit", d.Lines)
	}
	if !d.TooLong {
		t.Error("a diff one line past the limit is not marked too long")
	}
	if len(d.Blocks) != 0 {
		t.Errorf("%d blocks kept for a diff that will not be drawn", len(d.Blocks))
	}
}
