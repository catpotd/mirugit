package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FetchHeadModTime returns when this repository last heard from a remote. A
// zero time with no error means fetch has never run.
//
// Two files answer it. A linked worktree keeps a FETCH_HEAD of its own and a
// fetch run from there writes that one; the refs a fetch brings back are shared
// with every worktree, so a fetch run from any of them is news for all of them.
// Reading the worktree's own file alone said "never fetched" in a worktree
// whose remote refs were minutes old. Both are asked for in one call, because
// this runs on every poll.
func FetchHeadModTime(ctx context.Context, dir string) (time.Time, error) {
	out, err := runRead(ctx, dir, "rev-parse", "--git-path", "FETCH_HEAD", "--git-common-dir")
	if err != nil {
		return time.Time{}, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 2 {
		return time.Time{}, fmt.Errorf("git rev-parse answered %d lines, want 2", len(lines))
	}
	var newest time.Time
	for _, path := range []string{lines[0], filepath.Join(lines[1], "FETCH_HEAD")} {
		if !filepath.IsAbs(path) {
			path = filepath.Join(dir, path)
		}
		stat, err := os.Stat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return time.Time{}, err
		}
		if stat.ModTime().After(newest) {
			newest = stat.ModTime()
		}
	}
	return newest, nil
}
