package git

import (
	"context"
	"fmt"
	"strings"
)

// StageBlock applies one hunk to the index, because whole-file add cannot
// leave the other hunks unstaged.
func StageBlock(ctx context.Context, dir string, diff FileDiff, blockIndex int) error {
	if blockIndex < 0 || blockIndex >= len(diff.Blocks) {
		return fmt.Errorf("block %d out of range", blockIndex)
	}
	patch := buildBlockPatch(diff, blockIndex)
	_, err := execGit(ctx, dir, nil, patch, []string{"apply", "--cached"})
	return err
}

func buildBlockPatch(d FileDiff, blockIndex int) []byte {
	var b strings.Builder
	for _, l := range d.Header {
		b.WriteString(l)
		b.WriteString("\n")
	}
	block := d.Blocks[blockIndex]
	b.WriteString(block.Header)
	b.WriteString("\n")
	for _, l := range block.Lines {
		b.WriteString(l)
		b.WriteString("\n")
	}
	return []byte(b.String())
}
