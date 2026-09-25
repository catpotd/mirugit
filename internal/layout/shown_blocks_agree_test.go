package layout

import (
	"fmt"
	"strings"
	"testing"

	"github.com/catpotd/mirugit/internal/git"
)

// The read marks are written from ShownBlockHashes, and what the reader saw is
// what DiffArea drew. The two counted the rows apart: the area opens with a
// blank row and puts one between blocks, and a count without them marked a
// block read while its last line was still below the fold.
func TestShownBlockHashesNamesTheBlocksDiffAreaDraws(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	for _, sizes := range [][]int{{1, 1, 1}, {3, 1, 4}, {2}, {5, 2}} {
		blocks := make([]git.Block, len(sizes))
		for i, size := range sizes {
			lines := make([]string, size)
			for j := range lines {
				lines[j] = fmt.Sprintf("+b%dl%d", i, j)
			}
			blocks[i] = git.Block{Header: "@@ -1 +1 @@", Lines: lines,
				Hash: fmt.Sprintf("h%d", i)}
		}
		d := git.FileDiff{Path: "a.txt", Blocks: blocks}

		for startBlock := range len(blocks) {
			for height := 1; height <= 24; height++ {
				drawn, _ := w.DiffArea(d, 0, startBlock, 0, false, 60, height)
				var want []string
				for i := startBlock; i < len(blocks); i++ {
					if !allLinesDrawn(drawn, blocks[i]) {
						break
					}
					want = append(want, blocks[i].Hash)
				}
				got := ShownBlockHashes(d, startBlock, 0, height)
				if strings.Join(got, ",") != strings.Join(want, ",") {
					t.Errorf("sizes %v start %d height %d: marked %v read, drew %v",
						sizes, startBlock, height, got, want)
				}
			}
		}
	}
}

func allLinesDrawn(drawn []string, b git.Block) bool {
	for _, line := range b.Lines {
		found := false
		for _, row := range drawn {
			if strings.Contains(row, line) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
