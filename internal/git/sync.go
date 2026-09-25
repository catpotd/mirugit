package git

import (
	"context"
	"errors"
	"strings"
)

// ErrWouldMerge means pull cannot fast-forward and merge is outside v1.
var ErrWouldMerge = errors.New("pull would merge")

// Fetch updates remote-tracking refs. A repository with no remotes is not an
// error; there is nothing to retrieve.
func Fetch(ctx context.Context, dir string) error {
	has, err := hasRemotes(ctx, dir)
	if err != nil {
		return err
	}
	if !has {
		return nil
	}
	return runNetwork(ctx, dir, "fetch", "-q")
}

// pull fast-forwards the current branch from its upstream. A pull that would
// merge is refused rather than leaving a merge commit the pane cannot finish.
func pull(ctx context.Context, dir string) error {
	if !hasUpstream(ctx, dir) {
		return nil
	}
	ff, err := canFastForward(ctx, dir)
	if err != nil {
		return err
	}
	if !ff {
		return ErrWouldMerge
	}
	return runNetwork(ctx, dir, "pull", "--ff-only", "-q")
}

// push sends local commits to the upstream branch. A branch with no upstream is
// not an error; there is nowhere to send them.
func push(ctx context.Context, dir string) error {
	if !hasUpstream(ctx, dir) {
		return nil
	}
	return runNetwork(ctx, dir, "push", "-q")
}

// Sync runs fetch, then a fast-forward pull when behind, then push when ahead.
func Sync(ctx context.Context, dir string) error {
	if err := Fetch(ctx, dir); err != nil {
		return err
	}
	if !hasUpstream(ctx, dir) {
		return nil
	}
	behind, ahead, err := aheadBehind(ctx, dir)
	if err != nil {
		return err
	}
	if behind > 0 {
		// pull refuses a merge on its own. Asking canFastForward here as well
		// ran the same git command twice and put the decision in two places.
		if err := pull(ctx, dir); err != nil {
			return err
		}
		// A branch that is behind and ahead at once has diverged, and pull
		// refuses that: reaching here means the fast-forward left the branch
		// holding exactly what the upstream holds. There is nothing to send,
		// and asking git again to hear so was a third subprocess per sync.
		return nil
	}
	if ahead > 0 {
		return push(ctx, dir)
	}
	return nil
}

// RemoteURL returns origin's URL when configured; no origin is not an error.
// Any other failure is, because an empty string reads as "no remote" and the
// history tab then hides the key that opens a commit on the web.
func RemoteURL(ctx context.Context, dir string) (string, error) {
	out, err := runRead(ctx, dir, "remote", "get-url", "origin")
	if err == nil {
		return strings.TrimSpace(string(out)), nil
	}
	// Listing remotes answers in any working repository, so its own failure is
	// what separates "no origin" from "this directory answers nothing".
	remotes, listErr := runRead(ctx, dir, "remote")
	if listErr != nil {
		return "", err
	}
	if namesRemote(string(remotes), "origin") {
		return "", err
	}
	return "", nil
}

func namesRemote(list, name string) bool {
	for _, line := range strings.Split(list, "\n") {
		if strings.TrimSpace(line) == name {
			return true
		}
	}
	return false
}

// hasRemotes answers false only for a repository that has none. A directory
// that answers nothing at all is a different case: reading it as "no remote"
// turned a broken repository into one where f and S did nothing and said
// nothing. aheadBehind separates the two the same way.
func hasRemotes(ctx context.Context, dir string) (bool, error) {
	out, err := runRead(ctx, dir, "remote")
	if err != nil {
		return false, err
	}
	return len(out) > 0, nil
}

func hasUpstream(ctx context.Context, dir string) bool {
	_, err := runRead(ctx, dir, "rev-parse", "--verify", "@{upstream}")
	return err == nil
}

// canFastForward reports whether @{upstream} can be merged into HEAD by moving
// HEAD forward only. Diverged histories exit 1 and are not an error. Any other
// exit is: reporting false made a broken repository say "pull would merge".
func canFastForward(ctx context.Context, dir string) (bool, error) {
	_, err := runRead(ctx, dir, "merge-base", "--is-ancestor", "HEAD", "@{upstream}")
	if err == nil {
		return true, nil
	}
	var ee *ExitError
	if errors.As(err, &ee) && ee.Code == 1 {
		return false, nil
	}
	return false, err
}
