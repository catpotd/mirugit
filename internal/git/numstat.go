package git

import (
	"cmp"
	"context"
	"maps"
	"os"
	"strconv"
	"strings"
)

// Count separates a binary file, which git reports as dashes, from one that
// changed zero lines.
type Count struct {
	Added   int
	Deleted int
	Binary  bool
}

// Counts returns what it could read and, alongside it, why the rest is absent.
// A caller that stopped at the error drew every file as +0 −0, which is what an
// unchanged file looks like.
func Counts(ctx context.Context, dir string, staged bool) (map[string]Count, error) {
	out, err := runRead(ctx, dir, numstatArgs(staged)...)
	var counts map[string]Count
	var missing error
	if err == nil {
		counts = parseNumstat(out)
	} else {
		counts, missing = countsPerPath(ctx, dir, staged)
		missing = cmp.Or(missing, err)
	}
	if staged {
		return counts, missing
	}
	// git says nothing about a file it has never seen, so an untracked file
	// would draw +0 −0 and no bar. Its whole content is what it adds.
	return counts, cmp.Or(addUntrackedCounts(ctx, dir, counts), missing)
}

func numstatArgs(staged bool) []string {
	args := []string{"diff", "--numstat", "-z"}
	if staged {
		args = append(args, "--cached")
	}
	return args
}

// countsPerPath asks one path at a time, because git refuses the whole
// repository when a single path will not open and the others still have
// counts. It costs a git per changed file, which is why it runs only after the
// one call has already failed.
func countsPerPath(ctx context.Context, dir string, staged bool) (map[string]Count, error) {
	names, err := diffNames(ctx, dir, staged)
	if err != nil {
		return map[string]Count{}, err
	}
	counts := make(map[string]Count, len(names))
	var missing error
	for _, name := range names {
		out, err := runRead(ctx, dir, append(numstatArgs(staged), "--", pathspec(name))...)
		if err != nil {
			missing = cmp.Or(missing, err)
			continue
		}
		maps.Copy(counts, parseNumstat(out))
	}
	return counts, missing
}

func addUntrackedCounts(ctx context.Context, dir string, counts map[string]Count) error {
	out, err := runRead(ctx, dir, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return err
	}
	var missing error
	for _, path := range splitNULPaths(out) {
		c, err := countAgainstNothing(ctx, dir, path)
		if err != nil {
			missing = cmp.Or(missing, err)
			continue
		}
		counts[path] = c
	}
	return missing
}

// countAgainstNothing asks git rather than counting newlines here, so that what
// git calls binary and what this calls binary stay the same answer.
func countAgainstNothing(ctx context.Context, dir, path string) (Count, error) {
	// runRead hands back stdout alongside the error, and --no-index reports a
	// difference with exit 1, which is the normal answer for a file that did
	// not exist before.
	// --no-index works on the filesystem rather than the index, so it takes a
	// real name; a pathspec is read as a file called ":(literal)...".
	out, err := runRead(ctx, dir, "diff", "--no-index", "--numstat", "-z",
		os.DevNull, "--", path)
	if err != nil && !noIndexExitOne(err) {
		return Count{}, err
	}
	for _, field := range strings.Split(string(out), "\x00") {
		parts := strings.SplitN(field, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		if parts[0] == "-" {
			return Count{Binary: true}, nil
		}
		added, convErr := strconv.Atoi(parts[0])
		if convErr != nil {
			continue
		}
		return Count{Added: added}, nil
	}
	return Count{}, nil
}

// atLeastZero reads a count git printed. Anything that is not a count reads as
// none, because the summary line adds these up and a negative one would take
// the total below what the reader can see.
func atLeastZero(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func parseNumstat(out []byte) map[string]Count {
	fields := splitNUL(out)
	counts := map[string]Count{}

	for i := 0; i < len(fields); i++ {
		parts := strings.SplitN(fields[i], "\t", 3)
		if len(parts) != 3 {
			continue
		}
		c := Count{Binary: parts[0] == "-"}
		if !c.Binary {
			// A non-number from git means the count is unknown, and zero is
			// what unknown draws. A negative one is drawn as a bar of negative
			// length, so it is unknown too.
			c.Added = atLeastZero(parts[0])
			c.Deleted = atLeastZero(parts[1])
		}
		path := parts[2]
		if path == "" && i+2 < len(fields) {
			path = fields[i+2] // old is i+1, new is i+2
			i += 2
		}
		if path == "" {
			continue
		}
		counts[path] = c
	}
	return counts
}
