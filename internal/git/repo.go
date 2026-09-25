package git

import (
	"cmp"
	"context"
)

// Repo joins the status with both sets of counts in one place, so that no
// caller repeats the join and disagrees about it.
type Repo struct {
	Entries   []Entry
	Head      Head
	RemoteURL string
	// Unfinished is the multi-step operation git is part way through. status
	// does not mention it, so a merge whose conflicts are all resolved reads as
	// a clean tree.
	Unfinished InProgress
	// CountsErr says why the added and deleted figures are absent. They come
	// from a second git that refuses the whole repository when one path will
	// not open, and Entries is complete either way: a figure drawn beside a row
	// must not decide whether the row is drawn.
	CountsErr error
}

// Load reads everything the changes tab draws. gitDir is where this worktree's
// state lives, resolved once by the caller: it takes a git call to find and
// does not change.
func Load(ctx context.Context, dir, gitDir string) (Repo, error) {
	entries, head, err := Status(ctx, dir)
	if err != nil {
		return Repo{}, err
	}
	staged, stagedErr := Counts(ctx, dir, true)
	working, workingErr := Counts(ctx, dir, false)
	for i, e := range entries {
		entries[i].IndexCount = staged[e.Path]
		entries[i].WorktreeCount = working[e.Path]
	}
	head.Behind, head.Ahead, err = aheadBehind(ctx, dir)
	if err != nil {
		return Repo{}, err
	}
	url, err := RemoteURL(ctx, dir)
	if err != nil {
		return Repo{}, err
	}
	unfinished, err := OperationInProgress(gitDir)
	if err != nil {
		return Repo{}, err
	}
	return Repo{Entries: entries, Head: head, RemoteURL: url, Unfinished: unfinished,
		CountsErr: cmp.Or(stagedErr, workingErr)}, nil
}
