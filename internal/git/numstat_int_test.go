package git

import (
	"context"
	"os"
	"testing"
)

// git diff --numstat says nothing about a file it has never seen, so an
// untracked file drew +0 −0 and no bar. Its whole content counts as added, so a
// pane holding one reports its lines in the total.
func TestCountsIncludeUntrackedFiles(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/tracked.txt", []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "base")
	if err := os.WriteFile(dir+"/tracked.txt", []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	body := make([]byte, 0, 238)
	for range 119 {
		body = append(body, []byte("s\n")...)
	}
	if err := os.WriteFile(dir+"/fresh.txt", body, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/logo.bin", []byte{0, 1, 2, 0}, 0o644); err != nil {
		t.Fatal(err)
	}

	counts, err := Counts(context.Background(), dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := counts["fresh.txt"].Added; got != 119 {
		t.Errorf("fresh.txt added = %d, want 119", got)
	}
	if got := counts["tracked.txt"].Added; got != 1 {
		t.Errorf("tracked.txt added = %d, want 1", got)
	}
	if !counts["logo.bin"].Binary {
		t.Errorf("logo.bin = %+v, want binary", counts["logo.bin"])
	}
}
