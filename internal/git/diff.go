package git

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Block is what git calls a hunk. The name follows the interface, where "hunk"
// names nothing the reader can see.
type Block struct {
	Header           string
	Lines            []string
	OldStart         int // the left side of the @@ header
	NewStart         int // its right side
	LinePrefixLength int
	Added            int
	Deleted          int
	Hash             string
}

type FileDiff struct {
	Path     string
	Blocks   []Block
	Binary   bool
	ModeOnly bool
	Size     int64
	Mode     string
	Header   []string
	// TooLong says the body was dropped rather than held. Blocks is empty, and
	// Lines is what git printed.
	TooLong bool
	// Lines counts the body lines git printed for this file, whether or not
	// they were kept.
	Lines int
}

// diffLineLimit bounds what one file's diff keeps. The pane draws a screen of
// lines at a time, and a generated file, a lockfile or a minified bundle
// arrives with hundreds of thousands: measured, a 400,000-line rewrite made git
// print 33 MB and this hold 211 MB of it until the file was closed. The figure
// is well past any diff a person reads and still small enough that holding one
// costs about a megabyte.
const diffLineLimit = 20000

func Diff(ctx context.Context, dir, path string, staged bool) (FileDiff, error) {
	args := []string{"diff"}
	if staged {
		args = append(args, "--cached")
	}
	args = append(args, "--", pathspec(path))
	out, err := runRead(ctx, dir, args...)
	if err != nil {
		return FileDiff{}, err
	}
	files := parseDiff(out, path)
	if len(files) == 0 && !staged {
		mode, err := fileMode(ctx, dir, path)
		if err != nil {
			return FileDiff{}, err
		}
		if mode == "" {
			// git says nothing about a path it has never seen, which would leave
			// the diff area blank for the file an agent just wrote.
			return diffAgainstNothing(ctx, dir, path)
		}
	}
	if len(files) == 0 {
		return FileDiff{Path: path}, nil
	}
	d := files[0]
	if d.Binary {
		if err := fillBinaryMeta(ctx, dir, path, staged, &d); err != nil {
			return FileDiff{}, err
		}
	}
	return d, nil
}

// Diffs returns every changed file in one git invocation, because hashing
// each path separately costs twenty milliseconds per file.
func Diffs(ctx context.Context, dir string, staged bool) (map[string]FileDiff, error) {
	args := []string{"diff"}
	if staged {
		args = append(args, "--cached")
	}
	names, err := diffNames(ctx, dir, staged)
	if err != nil {
		return nil, err
	}
	out, err := runRead(ctx, dir, args...)
	if err != nil {
		return nil, err
	}
	files := parseDiff(out, "")
	// The header cannot say which path it belongs to: git quotes a non-ASCII
	// name, and a name containing " b/" splits the header in the wrong place.
	// Pairing with the -z list carries the bytes git actually holds, but only
	// while the two line up. A conflicted path staged for commit is listed
	// with no diff body behind it, and then position N means two different
	// files; asking per path is slower but cannot mis-key.
	if len(files) != len(names) {
		return diffsPerPath(ctx, dir, names, staged)
	}
	m := make(map[string]FileDiff, len(files))
	for i, f := range files {
		f.Path = names[i]
		if f.Binary {
			if err := fillBinaryMeta(ctx, dir, f.Path, staged, &f); err != nil {
				return nil, err
			}
		}
		m[f.Path] = f
	}
	return m, nil
}

func diffsPerPath(ctx context.Context, dir string, names []string, staged bool) (map[string]FileDiff, error) {
	m := make(map[string]FileDiff, len(names))
	for _, name := range names {
		d, err := Diff(ctx, dir, name, staged)
		if err != nil {
			return nil, err
		}
		m[name] = d
	}
	return m, nil
}

// noIndexExitOne reports whether git diff --no-index failed only because a
// difference exists, because exit 1 is the normal answer for that case.
func noIndexExitOne(err error) bool {
	var ee *ExitError
	return errors.As(err, &ee) && ee.Code == 1
}

// diffAgainstNothing reads a file that has no version behind it, which is what
// --no-index answers. It works on the filesystem rather than the index, so it
// takes a real name; a pathspec is read as a file called ":(literal)...".
func diffAgainstNothing(ctx context.Context, dir, path string) (FileDiff, error) {
	out, err := runRead(ctx, dir, "diff", "--no-index", os.DevNull, "--", path)
	if err != nil && !noIndexExitOne(err) {
		return FileDiff{}, err
	}
	files := parseDiff(out, path)
	if len(files) == 0 {
		return FileDiff{Path: path}, nil
	}
	d := files[0]
	if d.Binary {
		if err := fillBinaryMeta(ctx, dir, path, false, &d); err != nil {
			return FileDiff{}, err
		}
	}
	return d, nil
}

func diffNames(ctx context.Context, dir string, staged bool) ([]string, error) {
	args := []string{"diff", "-z", "--name-only"}
	if staged {
		args = append(args, "--cached")
	}
	out, err := runRead(ctx, dir, args...)
	if err != nil {
		return nil, err
	}
	trimmed := strings.TrimSuffix(string(out), "\x00")
	if trimmed == "" {
		return nil, nil
	}
	// A conflicted path is listed once per merge stage, while the diff body
	// holds one combined entry for it. Without this the two counts disagree
	// and every path is dropped.
	seen := map[string]bool{}
	var names []string
	for _, n := range strings.Split(trimmed, "\x00") {
		if seen[n] {
			continue
		}
		seen[n] = true
		names = append(names, n)
	}
	return names, nil
}

// KnownPaths returns paths still in the working tree or the last hundred commits.
// Read marks for paths that left both would otherwise grow without bound.
func KnownPaths(ctx context.Context, dir string) (map[string]bool, error) {
	alive := map[string]bool{}
	// -z on every one of these three. Without it core.quotePath escapes any
	// non-ASCII name, and the caller compares the escaped form against a real
	// path: Prune then drops the read marks of every such file on startup.
	add := func(out []byte) {
		for _, path := range splitNULPaths(out) {
			alive[path] = true
		}
	}
	out, err := runRead(ctx, dir, "ls-files", "-z")
	if err != nil {
		return nil, err
	}
	add(out)
	// A repository with no commits has no log to read, and git exits 128 rather
	// than printing nothing.
	if !hasHEAD(ctx, dir) {
		return alive, nil
	}
	logOut, err := runRead(ctx, dir, "log", "-100", "--name-only", "--format=", "-z")
	if err != nil {
		return nil, err
	}
	add(logOut)
	other, err := runRead(ctx, dir, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	add(other)
	return alive, nil
}

// The path is given rather than read from the header. git quotes a header that
// holds a non-ASCII name, and a path containing " b/" splits the header in the
// wrong place; both produce the wrong path with no error.
var (
	newline           = []byte("\n")
	diffGitPrefix     = []byte("diff --git ")
	diffCCPrefix      = []byte("diff --cc ")
	binaryFilesPrefix = []byte("Binary files ")
	blockPrefix       = []byte("@@")
	headerPrefixes    = [][]byte{[]byte("--- "), []byte("+++ "), []byte("index "),
		[]byte("new file mode "), []byte("deleted file mode ")}
	renamePrefixes = [][]byte{[]byte("rename from "), []byte("similarity index ")}
)

func hasAnyPrefix(line []byte, prefixes [][]byte) bool {
	for _, p := range prefixes {
		if bytes.HasPrefix(line, p) {
			return true
		}
	}
	return false
}

func parseDiff(out []byte, path string) []FileDiff {
	var files []FileDiff
	var cur *FileDiff
	var block *Block

	flush := func() {
		if cur != nil && block != nil && !cur.TooLong {
			block.Hash = hashBlock(*block)
			cur.Blocks = append(cur.Blocks, *block)
			block = nil
		}
	}
	// Walking the bytes rather than splitting a string of them: the split built
	// one string header per line up front, and turning the output into a string
	// first copied it. On the 800,000-line diff that started this, that was
	// 12.8 MB of headers on top of a 33 MB copy.
	for raw := range bytes.Lines(out) {
		raw = bytes.TrimSuffix(raw, newline)
		switch {
		case bytes.HasPrefix(raw, diffGitPrefix), bytes.HasPrefix(raw, diffCCPrefix):
			flush()
			if cur != nil {
				files = append(files, *cur)
			}
			cur = &FileDiff{Path: path, ModeOnly: true, Header: []string{string(raw)}}
		case cur == nil:
		case hasAnyPrefix(raw, headerPrefixes):
			cur.Header = append(cur.Header, string(raw))
		case bytes.HasPrefix(raw, binaryFilesPrefix):
			cur.Binary, cur.ModeOnly = true, false
		case hasAnyPrefix(raw, renamePrefixes):
			cur.ModeOnly = false
		case bytes.HasPrefix(raw, blockPrefix):
			flush()
			cur.ModeOnly = false
			if cur.TooLong {
				continue
			}
			line := string(raw)
			oldStart, newStart := parseBlockHeader(line)
			block = &Block{Header: line, OldStart: oldStart, NewStart: newStart,
				LinePrefixLength: linePrefixLength(line)}
		// Counting past the limit rather than stopping: the reader is told how
		// long the diff is, and a figure that stops at the limit would say every
		// long diff is the same length.
		case cur.TooLong:
			cur.Lines++
		case block != nil:
			cur.Lines++
			if cur.Lines > diffLineLimit {
				// The blocks already read go too. Holding the first twenty
				// thousand lines of a file nobody can read to the end costs what
				// holding all of them costs, minus the tail.
				cur.TooLong = true
				cur.Blocks = nil
				block = nil
			} else {
				appendBlockLine(block, string(raw))
			}
		}
	}
	flush()
	if cur != nil {
		files = append(files, *cur)
	}
	return files
}

func appendBlockLine(block *Block, line string) {
	// The split on "\n" yields one empty string past the final newline,
	// and letting it into the last block would change that block's hash
	// whenever another block is appended after it.
	if line == "" {
		return
	}
	block.Lines = append(block.Lines, line)
	// Count only the first column so block totals match git diff --numstat
	// (ours side); combined diffs can mark the second column alone.
	switch line[0] {
	case '+':
		block.Added++
	case '-':
		block.Deleted++
	}
}

func fillBinaryMeta(ctx context.Context, dir, path string, staged bool, d *FileDiff) error {
	mode, err := fileMode(ctx, dir, path)
	if err != nil {
		return err
	}
	size, err := blobSize(ctx, dir, path, staged)
	if err != nil {
		return err
	}
	d.Mode = mode
	d.Size = size
	return nil
}

func fileMode(ctx context.Context, dir, path string) (string, error) {
	out, err := runRead(ctx, dir, "ls-files", "-s", "--", pathspec(path))
	if err != nil {
		return "", err
	}
	line := strings.TrimSpace(string(out))
	if line == "" {
		return "", nil
	}
	return strings.Fields(line)[0], nil
}

func blobSize(ctx context.Context, dir, path string, staged bool) (int64, error) {
	if !staged {
		full := filepath.Join(dir, path)
		if stat, err := os.Stat(full); err == nil {
			return stat.Size(), nil
		}
	}
	out, err := runRead(ctx, dir, "cat-file", "-s", ":"+path)
	if err != nil {
		return 0, err
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("cat-file -s %q: %w", path, err)
	}
	return n, nil
}

// parseBlockHeader reads where a hunk starts on each side. Git omits a count of
// one, so both @@ -40,3 +42,5 @@ and @@ -40 +42 @@ must be accepted. Combined
// diffs use @@@ with two old sides; the last field is the new side.
func parseBlockHeader(header string) (oldStart, newStart int) {
	trimmed := strings.TrimSpace(header)
	if strings.HasPrefix(trimmed, "@@@") {
		inner := strings.TrimSpace(strings.TrimPrefix(trimmed, "@@@"))
		inner = strings.TrimSuffix(strings.TrimSpace(inner), "@@@")
		parts := strings.Fields(strings.TrimSpace(inner))
		if len(parts) < 1 {
			return 0, 0
		}
		return parseHunkStart(parts[0]), parseHunkStart(parts[len(parts)-1])
	}
	inner := strings.TrimSpace(strings.TrimPrefix(trimmed, "@@"))
	inner = strings.TrimSuffix(strings.TrimSpace(inner), "@@")
	parts := strings.Fields(strings.TrimSpace(inner))
	if len(parts) < 2 {
		return 0, 0
	}
	oldStart = parseHunkStart(parts[0])
	newStart = parseHunkStart(parts[1])
	return oldStart, newStart
}

func (b Block) LineAdded(line string) bool {
	return b.hasLineMarker(line, '+')
}

func (b Block) LineDeleted(line string) bool {
	return b.hasLineMarker(line, '-')
}

func (b Block) hasLineMarker(line string, marker byte) bool {
	length := b.LinePrefixLength
	if length < 1 {
		length = 1
	}
	for i := 0; i < len(line) && i < length; i++ {
		if line[i] == marker {
			return true
		}
	}
	return false
}

func linePrefixLength(header string) int {
	length := 0
	for length < len(header) && header[length] == '@' {
		length++
	}
	return max(length-1, 1)
}

func parseHunkStart(side string) int {
	if len(side) == 0 {
		return 0
	}
	if side[0] == '-' || side[0] == '+' {
		side = side[1:]
	}
	if side == "" {
		return 0
	}
	if i := strings.IndexByte(side, ','); i >= 0 {
		side = side[:i]
	}
	n, err := strconv.Atoi(side)
	if err != nil || n < 0 {
		// A hunk starts at a line the file has. The gutter draws this number in
		// a fixed field, and a negative one is wider than the field allows.
		return 0
	}
	return n
}

// The @@ header is excluded so that an edit elsewhere in the file, which shifts
// every line number below it, does not make a block the reader already read
// look new.
func hashBlock(b Block) string {
	h := sha256.New()
	for _, l := range b.Lines {
		h.Write([]byte(l))
		h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// IgnoredDirs names the directories git ignores, relative to dir. The watcher
// asks so it does not spend its descriptor budget on node_modules.
func IgnoredDirs(ctx context.Context, dir string) ([]string, error) {
	out, err := runRead(ctx, dir, "ls-files", "-z", "--others", "--directory", "--ignored",
		"--exclude-standard")
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, p := range splitNULPaths(out) {
		dirs = append(dirs, strings.TrimSuffix(p, "/"))
	}
	return dirs, nil
}
