package git

import (
	"context"
	"strconv"
	"strings"
)

type Head struct {
	Branch      string
	Detached    bool
	Initial     bool
	HasUpstream bool
	Ahead       int
	Behind      int
}

func Status(ctx context.Context, dir string) ([]Entry, Head, error) {
	out, err := runRead(ctx, dir, "status", "--porcelain=v2", "-z",
		"--branch", "--untracked-files=all")
	if err != nil {
		return nil, Head{}, err
	}
	return parseStatus(out)
}

func parseStatus(out []byte) ([]Entry, Head, error) {
	fields := splitNUL(out)
	var entries []Entry
	var head Head

	for i := 0; i < len(fields); i++ {
		f := fields[i]
		switch {
		case f == "":
		case strings.HasPrefix(f, "# "):
			readHeader(f, &head)
		case strings.HasPrefix(f, "1 "):
			entries = appendWithPath(entries, ordinaryEntry(f, ordinaryFields))
		case strings.HasPrefix(f, "2 "):
			e := ordinaryEntry(f, renameFields)
			if i+1 < len(fields) {
				e.OldPath = fields[i+1]
				i++
			}
			entries = appendWithPath(entries, e)
		case strings.HasPrefix(f, "u "):
			entries = appendWithPath(entries, Entry{
				Path: fieldAfter(f, unmergedFields), Index: Unmerged, Worktree: Unmerged})
		case strings.HasPrefix(f, "? "):
			entries = appendWithPath(entries, Entry{Path: f[2:], Worktree: Untracked})
		}
	}
	return entries, head, nil
}

func readHeader(f string, head *Head) {
	switch {
	case strings.HasPrefix(f, "# branch.head "):
		head.Branch = f[len("# branch.head "):]
		head.Detached = head.Branch == "(detached)"
	case f == "# branch.oid (initial)":
		head.Initial = true
	case strings.HasPrefix(f, "# branch.upstream "):
		head.HasUpstream = true
	case strings.HasPrefix(f, "# branch.ab "):
		for _, part := range strings.Fields(f[len("# branch.ab "):]) {
			n, err := strconv.Atoi(part[1:])
			if err != nil {
				continue
			}
			if part[0] == '+' {
				head.Ahead = n
			} else {
				head.Behind = n
			}
		}
	}
}

// Each record has a fixed number of space-separated fields before the path.
// Taking the text after the last space instead would return "ace.txt" for a
// file named "sp ace.txt", and that string as a pathspec can match a different
// file that exists.
const (
	ordinaryFields = 8  // 1 <XY> <sub> <mH> <mI> <mW> <hH> <hI> <path>
	renameFields   = 9  // 2 ... <score> <path>
	unmergedFields = 10 // u <XY> <sub> <m1> <m2> <m3> <mW> <h1> <h2> <h3> <path>
)

func ordinaryEntry(f string, before int) Entry {
	parts := strings.SplitN(f, " ", 4)
	// git writes the whole record, so a short one is a truncated read. Reading
	// the status letters off it indexed past the end and took the pane down.
	if len(parts) < 2 || len(parts[1]) < 2 {
		return Entry{}
	}
	xy := parts[1]
	return Entry{
		Path:      fieldAfter(f, before),
		Index:     codeOf(xy[0]),
		Worktree:  codeOf(xy[1]),
		Submodule: len(parts) > 2 && strings.HasPrefix(parts[2], "S"),
	}
}

// appendWithPath drops a record too short to carry a path. git always writes
// the whole record, so a short one is a truncated read rather than a file, and
// an Entry with no path becomes a pathspec that names every path.
func appendWithPath(entries []Entry, e Entry) []Entry {
	if e.Path == "" {
		return entries
	}
	return append(entries, e)
}

// fieldAfter returns everything past the first n spaces, spaces included.
func fieldAfter(f string, n int) string {
	parts := strings.SplitN(f, " ", n+1)
	if len(parts) <= n {
		return ""
	}
	return parts[n]
}

func codeOf(b byte) Kind {
	if b == '.' {
		return Unchanged
	}
	return Kind(b)
}
