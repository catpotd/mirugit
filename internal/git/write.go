package git

import (
	"context"
)

// Outcome separates what git changed from what it accepted and left alone,
// because three operations report success having done nothing.
type Outcome struct {
	Applied []Entry
	Ignored []Entry
}

// Stage runs git add and re-reads status, because add exits 0 without staging
// submodule changes.
func Stage(ctx context.Context, dir string, entries []Entry) (Outcome, error) {
	return stageOrUnstage(ctx, dir, entries, true)
}

// Unstage runs restore --staged or rm --cached when HEAD is missing, then
// re-reads status, because both can exit 0 having changed nothing.
func Unstage(ctx context.Context, dir string, entries []Entry) (Outcome, error) {
	return stageOrUnstage(ctx, dir, entries, false)
}

func stageOrUnstage(ctx context.Context, dir string, entries []Entry, stage bool) (Outcome, error) {
	if len(entries) == 0 {
		return Outcome{}, nil
	}
	before, _, err := Status(ctx, dir)
	if err != nil {
		return Outcome{}, err
	}
	beforeStaged := stagedPaths(before)

	var paths []string
	for _, e := range entries {
		paths = append(paths, e.Pathspecs()...)
	}
	switch {
	case stage:
		err = runWritePaths(ctx, dir, paths, "add")
	case hasHEAD(ctx, dir):
		err = runWritePaths(ctx, dir, paths, "restore", "--staged")
	default:
		// Before the first commit there is nothing to restore from, so
		// unstaging is removing the path from the index.
		err = runWritePaths(ctx, dir, paths, "rm", "--cached", "-r")
	}
	if err != nil {
		return Outcome{}, err
	}

	after, _, err := Status(ctx, dir)
	if err != nil {
		return Outcome{}, err
	}
	afterStaged := stagedPaths(after)

	return stageOutcome(entries, beforeStaged, afterStaged, stage), nil
}

// outcomeByPath records each path at most once in Applied or Ignored, because
// duplicate entries in the input must not inflate Applied.
func outcomeByPath(entries []Entry, classify func(Entry) bool) Outcome {
	var out Outcome
	seen := map[string]bool{}
	for _, e := range entries {
		if seen[e.Path] {
			continue
		}
		seen[e.Path] = true
		if classify(e) {
			out.Applied = append(out.Applied, e)
		} else {
			out.Ignored = append(out.Ignored, e)
		}
	}
	return out
}

func stageOutcome(entries []Entry, beforeStaged, afterStaged map[string]bool, stage bool) Outcome {
	return outcomeByPath(entries, func(e Entry) bool {
		was := beforeStaged[e.Path]
		is := afterStaged[e.Path]
		if stage {
			return is && !was
		}
		return was && !is
	})
}

func hasHEAD(ctx context.Context, dir string) bool {
	_, err := runRead(ctx, dir, "rev-parse", "-q", "--verify", "HEAD")
	return err == nil
}

func stagedPaths(entries []Entry) map[string]bool {
	m := make(map[string]bool, len(entries))
	for _, e := range entries {
		if e.IsStaged() {
			m[e.Path] = true
		}
	}
	return m
}
