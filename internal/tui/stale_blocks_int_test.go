package tui

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/catpotd/mirugit/internal/git"
)

// The open diff is compared with the repository before a stage runs against it.
// A file that has gained or lost a block since it was read is not the file on
// the screen, and answering "fresh" staged blocks the reader never saw.
func TestADiffWithADifferentNumberOfBlocksIsStale(t *testing.T) {
	t.Parallel()
	dir := repoWithFiveDistantBlocks(t)
	shown, err := git.Diff(context.Background(), dir, "long.txt", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(shown.Blocks) != 5 {
		t.Fatalf("the repository has %d blocks, want 5", len(shown.Blocks))
	}

	stale, err := diffStale(context.Background(), dir, "long.txt", false, shown)
	if err != nil {
		t.Fatal(err)
	}
	if stale {
		t.Error("a diff read from the file it was read from answered stale")
	}

	// One change put back leaves four blocks where five were read.
	content, err := os.ReadFile(dir + "/long.txt")
	if err != nil {
		t.Fatal(err)
	}
	rows := strings.Split(string(content), "\n")
	rows[389] = strconv.Itoa(390)
	if err := os.WriteFile(dir+"/long.txt", []byte(strings.Join(rows, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	current, err := git.Diff(context.Background(), dir, "long.txt", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(current.Blocks) != 4 {
		t.Fatalf("the file now has %d blocks, want 4", len(current.Blocks))
	}

	stale, err = diffStale(context.Background(), dir, "long.txt", false, shown)
	if err != nil {
		t.Fatal(err)
	}
	if !stale {
		t.Error("a diff that lost a block since it was read answered fresh")
	}
}
