package git

import (
	"context"
	"strings"
)

// aheadBehind counts how many commits the branch is behind and ahead of its
// upstream. A repository with no upstream is not an error; both counts are zero.
// Anything else is, because returning 0/0 made a broken repository look synced.
// The follow-up questions run only after a failure, so the common path stays at
// one git call.
func aheadBehind(ctx context.Context, dir string) (behind, ahead int, err error) {
	out, err := runRead(ctx, dir, "rev-list", "--left-right", "--count", "@{upstream}...HEAD")
	if err != nil {
		if isRepo(ctx, dir) && !hasUpstream(ctx, dir) {
			return 0, 0, nil
		}
		return 0, 0, err
	}
	// rev-list --left-right prints the upstream side first.
	behind, ahead = parseTwoCounts(string(out))
	return behind, ahead, nil
}

// isRepo separates "this branch has no upstream" from "this directory answers
// nothing". Both make rev-parse @{upstream} fail, and only the first one is a
// normal state to draw.
func isRepo(ctx context.Context, dir string) bool {
	_, err := runRead(ctx, dir, "rev-parse", "--git-dir")
	return err == nil
}

// parseTwoCounts reads a line holding two numbers. Which one is ahead and
// which is behind depends on the git command that printed it, so the caller
// names them: rev-list --left-right prints behind first, for-each-ref prints
// ahead first. Two copies of this parser existed with the names swapped.
func parseTwoCounts(s string) (first, second int) {
	parts := strings.Fields(strings.TrimSpace(s))
	if len(parts) != 2 {
		return 0, 0
	}
	// A count of commits cannot be negative. The worktree row draws these as
	// "↓2 ↑1", and a negative one would print a sign the reader cannot act on.
	return atLeastZero(parts[0]), atLeastZero(parts[1])
}
