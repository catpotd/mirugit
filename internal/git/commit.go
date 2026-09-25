package git

import (
	"context"
	"errors"
	"strings"
)

var (
	// ErrUnmerged is returned while a u record remains, because staging every
	// conflicted path clears the conflict and the commit then succeeds with
	// the markers inside it.
	ErrUnmerged = errors.New("unmerged entries block commit")
	// ErrEmptyMessage is returned because git would open an editor otherwise.
	ErrEmptyMessage = errors.New("commit message is empty")
	// ErrNothingStaged is returned because an empty index cannot make a commit.
	ErrNothingStaged = errors.New("nothing staged")
)

// Commit refuses while an unmerged entry exists, because staging every
// conflicted path clears the conflict as far as git is concerned and the
// commit then succeeds with the markers inside it.
func Commit(ctx context.Context, dir, message string) (string, error) {
	entries, _, err := Status(ctx, dir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.IsConflicted() {
			return "", ErrUnmerged
		}
	}
	if strings.TrimSpace(message) == "" {
		return "", ErrEmptyMessage
	}
	staged := false
	for _, e := range entries {
		if e.IsStaged() {
			staged = true
			break
		}
	}
	if !staged {
		return "", ErrNothingStaged
	}
	if err := runWritePaths(ctx, dir, nil, "commit", "-m", message); err != nil {
		return "", err
	}
	out, err := runRead(ctx, dir, "rev-parse", "--short", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
