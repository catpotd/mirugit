package git

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// MinVersion is the oldest git whose output this program can read. Two commands
// set it and neither degrades: for-each-ref's ahead-behind field, which the
// worktrees tab needs, and merge-tree --write-tree, which decides whether a
// stash still applies. An older git answers "unknown field name" and "usage:",
// and the pane can only print the command line it ran.
var MinVersion = Version{2, 41}

// Version is the major and minor of a git build. The patch level is dropped
// because no feature this program uses turns on one.
type Version struct {
	Major int
	Minor int
}

func (v Version) String() string { return fmt.Sprintf("%d.%d", v.Major, v.Minor) }

// OlderThan orders two versions.
func (v Version) OlderThan(other Version) bool {
	if v.Major != other.Major {
		return v.Major < other.Major
	}
	return v.Minor < other.Minor
}

// CheckVersion reports a git this program cannot read, by name and by number.
// The check runs once at startup rather than at the first failing command,
// because the commands that need a new git are spread across three tabs and a
// reader who never opens the worktrees tab would meet the failure at a moment
// that says nothing about its cause.
func CheckVersion(ctx context.Context, dir string) error {
	out, err := runRead(ctx, dir, "--version")
	if err != nil {
		return err
	}
	return VersionRefusal(string(out))
}

// VersionRefusal is the judgement CheckVersion makes on what git printed, kept
// apart from running git so it can be checked against the strings real builds
// print without one of them on the machine.
func VersionRefusal(line string) error {
	got, ok := ParseVersion(line)
	if !ok {
		// A build that names itself in some other way is not evidence of an old
		// git, and refusing to start on an unreadable version string would turn
		// a cosmetic difference into a failure to run.
		return nil
	}
	if got.OlderThan(MinVersion) {
		return fmt.Errorf("git %s is too old: mirugit needs git %s or newer",
			got, MinVersion)
	}
	return nil
}

// ParseVersion reads the major and minor out of what git --version prints. The
// forms seen in the wild are "git version 2.41.0" and Apple's
// "git version 2.50.1 (Apple Git-155)".
func ParseVersion(line string) (Version, bool) {
	fields := strings.Fields(line)
	for i, f := range fields {
		if f != "version" || i+1 >= len(fields) {
			continue
		}
		parts := strings.SplitN(fields[i+1], ".", 3)
		if len(parts) < 2 {
			return Version{}, false
		}
		major, err := strconv.Atoi(parts[0])
		if err != nil {
			return Version{}, false
		}
		minor, err := strconv.Atoi(parts[1])
		if err != nil {
			return Version{}, false
		}
		return Version{Major: major, Minor: minor}, true
	}
	return Version{}, false
}
