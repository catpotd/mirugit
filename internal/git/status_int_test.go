package git

import (
	"context"
	"os"
	"testing"
)

func TestStatusExpandsAnUntrackedDirectory(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.MkdirAll(dir+"/newdir", 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"a.txt", "b.txt", "c.txt"} {
		if err := os.WriteFile(dir+"/newdir/"+n, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	entries, _, err := Status(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	// Without --untracked-files=all this is one entry, "newdir/", and the three
	// files are invisible.
	if len(entries) != 3 {
		t.Fatalf("want 3 entries, got %d: %+v", len(entries), entries)
	}
}

func TestStatusReadsAPathThatBeginsWithADash(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/-dash.txt", []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, _, err := Status(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Path != "-dash.txt" {
		t.Fatalf("got %+v", entries)
	}
}
