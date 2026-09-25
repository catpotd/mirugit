package git

import (
	"context"
	"fmt"
	"strings"
)

const maxStashPathspecBytes = 512 * 1024

// OversizedPathspecError is returned before git runs, because stash push can
// write the entry and still leave the worktree dirty when the path list is too
// large.
type OversizedPathspecError struct {
	Entries int
	Bytes   int
}

func (e *OversizedPathspecError) Error() string {
	return fmt.Sprintf("stash pathspecs for %d files exceed the 512 KB limit (%d bytes)",
		e.Entries, e.Bytes)
}

// Stash confirms by the length of git stash list, because three inputs make
// the call report success having stashed nothing.
func Stash(ctx context.Context, dir string, entries []Entry) (Outcome, error) {
	if len(entries) == 0 {
		return Outcome{}, nil
	}

	paths, total, n := stashPathspecs(entries)
	if total > maxStashPathspecBytes {
		return Outcome{}, &OversizedPathspecError{Entries: n, Bytes: total}
	}

	before, _, err := Status(ctx, dir)
	if err != nil {
		return Outcome{}, err
	}
	beforeByPath := indexByPath(before)

	countBefore, err := stashCount(ctx, dir)
	if err != nil {
		return Outcome{}, err
	}

	args := []string{"stash", "push"}
	if stashNeedsUntracked(entries) {
		args = append(args, "-u")
	}
	if err := runWritePaths(ctx, dir, paths, args...); err != nil {
		return Outcome{}, err
	}

	countAfter, err := stashCount(ctx, dir)
	if err != nil {
		return Outcome{}, err
	}
	if countAfter <= countBefore {
		return Outcome{Ignored: append([]Entry(nil), entries...)}, nil
	}

	after, _, err := Status(ctx, dir)
	if err != nil {
		return Outcome{}, err
	}
	return diffOutcome(entries, beforeByPath, indexByPath(after)), nil
}

func stashNeedsUntracked(entries []Entry) bool {
	for _, e := range entries {
		if e.Worktree == Untracked {
			return true
		}
	}
	return false
}

func stashPathspecs(entries []Entry) (paths []string, totalBytes, entryCount int) {
	for _, e := range entries {
		entryCount++
		for _, ps := range e.Pathspecs() {
			paths = append(paths, ps)
			totalBytes += len(ps)
		}
	}
	return paths, totalBytes, entryCount
}

func stashCount(ctx context.Context, dir string) (int, error) {
	out, err := runRead(ctx, dir, "stash", "list")
	if err != nil {
		return 0, err
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return 0, nil
	}
	return strings.Count(trimmed, "\n") + 1, nil
}
