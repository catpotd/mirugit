package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// A tracked path and an untracked path can name the same place with different
// roles: `f` is a file in the index and a directory on disk, holding an
// untracked `f/new.txt`. Discard has to see that file to know that restoring
// `f` would take it, and it sees it because the listing names untracked files
// one by one. Asking for the default instead reports `f/` and the file inside
// is never named.
//
// TestDiscardKeepsAnUntrackedFileUnderAPathThatWasAFile is what breaks if the
// protection goes; this is what breaks if the listing stops answering the
// question the protection asks.
func TestStatusListsUntrackedFilesRatherThanTheirDirectory(t *testing.T) {
	t.Parallel()
	dir := newRepo(t)
	if err := os.MkdirAll(filepath.Join(dir, "new/deeper"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"new/a.txt", "new/deeper/b.txt"} {
		if err := os.WriteFile(filepath.Join(dir, path), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	entries, _, err := Status(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, e := range entries {
		got[e.Path] = true
	}
	for _, want := range []string{"new/a.txt", "new/deeper/b.txt"} {
		if !got[want] {
			t.Errorf("%q is missing from %v", want, got)
		}
	}
	for _, unwanted := range []string{"new", "new/", "new/deeper", "new/deeper/"} {
		if got[unwanted] {
			t.Errorf("%q is a directory and was listed as an entry", unwanted)
		}
	}
}
