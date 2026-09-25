package git

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// -u tells git to take untracked files as well, and it is passed only when
// there are some: a stash of tracked changes that also carries -u picks up
// whatever the reader left lying about — build output, editor files — and the
// pop later puts them all back.
func TestUntrackedIsAskedForOnlyWhenThereIsOne(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name    string
		entries []Entry
		want    bool
	}{
		{"nothing at all", nil, false},
		{"one tracked change", []Entry{{Path: "a.txt", Worktree: Modified}}, false},
		{"one staged change", []Entry{{Path: "a.txt", Index: Modified}}, false},
		{"one untracked file", []Entry{{Path: "a.txt", Worktree: Untracked}}, true},
		{"tracked and untracked together", []Entry{
			{Path: "a.txt", Worktree: Modified},
			{Path: "b.txt", Worktree: Untracked}}, true},
		{"a deleted file", []Entry{{Path: "a.txt", Worktree: Deleted}}, false},
		{"a conflicted file", []Entry{
			{Path: "a.txt", Index: Unmerged, Worktree: Unmerged}}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := stashNeedsUntracked(c.entries); got != c.want {
				t.Errorf("stashNeedsUntracked = %v, want %v", got, c.want)
			}
		})
	}
}

// The bound is on how long the command line may be, and it is the largest
// request that is still sent: a stash whose pathspecs come to exactly the
// limit is one git can take, and refusing it tells the reader their selection
// is too big when it is not. One byte more is refused, which is what the bound
// is for.
func TestTheStashPathspecBoundIsTheLargestRequestStillSent(t *testing.T) {
	t.Parallel()
	entriesOf := func(total int) []Entry {
		// One entry whose pathspec is exactly total bytes long.
		spec := pathspec("")
		name := strings.Repeat("a", total-len(spec))
		e := Entry{Path: name, Worktree: Modified}
		if got := len(e.Pathspecs()[0]); got != total {
			t.Fatalf("the fixture is %d bytes, want %d", got, total)
		}
		return []Entry{e}
	}

	var tooLong *OversizedPathspecError
	// The directory is empty, so the guard is all that runs: every git call
	// refuses an empty directory before it starts.
	_, err := Stash(context.Background(), "", entriesOf(maxStashPathspecBytes))
	if errors.As(err, &tooLong) {
		t.Errorf("a request of exactly the limit was refused: %v", err)
	}
	_, err = Stash(context.Background(), "", entriesOf(maxStashPathspecBytes+1))
	if !errors.As(err, &tooLong) {
		t.Errorf("a request one byte past the limit answered %v", err)
	}
}
