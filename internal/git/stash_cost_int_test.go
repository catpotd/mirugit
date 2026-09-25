package git

import (
	"context"
	"testing"
	"time"
)

// The list a reload reads costs one git. What each stash holds — how many files
// it has, whether popping it would conflict — costs three or four more per
// stash, and only the stashed tab draws any of it.
//
// Measured with ten stashes before the split: the list took 2.5 s against 0.34 s
// for the whole of Load, on a read that runs on every watcher event and every
// five-second poll.
func TestReadingTheStashListDoesNotFollowWhatEachStashHolds(t *testing.T) {
	if testing.Short() {
		t.Skip("builds ten stashes")
	}
	t.Parallel()
	dir := newRepo(t)
	env := gitTestEnv(dir)
	write(t, dir, "a.txt", "base\n")
	runGitTest(t, env, dir, "add", ".")
	runGitTest(t, env, dir, "commit", "-m", "base")
	for i := range 10 {
		write(t, dir, "a.txt", "edit\n"+string(rune('a'+i))+"\n")
		runGitTest(t, env, dir, "stash", "push", "-m", "held")
	}

	start := time.Now()
	rows, err := StashList(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	listing := time.Since(start)
	if len(rows) != 10 {
		t.Fatalf("got %d stashes, want 10", len(rows))
	}

	start = time.Now()
	if _, err := Load(context.Background(), dir, dir+"/.git"); err != nil {
		t.Fatal(err)
	}
	loading := time.Since(start)

	// Load is five gits and is what a reload costs when nothing is stashed. A
	// list that reads one git per stash grew past it by a factor of seven at ten
	// stashes; the bound is generous because these are real processes on a
	// machine running other tests.
	if listing > loading*2 {
		t.Errorf("listing ten stashes took %v against %v for the whole repository; "+
			"the list is reading what each stash holds", listing, loading)
	}
}

// What each stash holds is read when the tab that draws it asks.
func TestFillingAStashSaysWhatItHolds(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to git")
	}
	t.Parallel()
	dir := newRepo(t)
	env := gitTestEnv(dir)
	write(t, dir, "a.txt", "base\n")
	write(t, dir, "b.txt", "base\n")
	runGitTest(t, env, dir, "add", ".")
	runGitTest(t, env, dir, "commit", "-m", "base")
	write(t, dir, "a.txt", "edit\n")
	write(t, dir, "b.txt", "edit\n")
	runGitTest(t, env, dir, "stash", "push", "-m", "held")

	_, head, err := Status(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := StashList(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d stashes, want 1", len(rows))
	}
	if rows[0].Status != StashUnknown {
		t.Errorf("the list decided the status before anyone asked: %v", rows[0].Status)
	}

	if err := FillStashStatus(context.Background(), dir, &rows[0], head); err != nil {
		t.Fatal(err)
	}
	if rows[0].FileCount != 2 {
		t.Errorf("the stash holds two files and says %d", rows[0].FileCount)
	}
	if rows[0].Status != StashApplies {
		t.Errorf("nothing stands in the way of this stash and it says %v", rows[0].Status)
	}
}

// stashListFilled is the pair the stashed tab reads: the list, then what each
// stash holds. Tests that check FileCount or Status want both.
func stashListFilled(t *testing.T, dir string, head Head) []StashRow {
	t.Helper()
	rows, err := StashList(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	for i := range rows {
		if err := FillStashStatus(context.Background(), dir, &rows[i], head); err != nil {
			t.Fatal(err)
		}
	}
	return rows
}
