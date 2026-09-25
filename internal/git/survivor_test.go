package git

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// The records below all reach git's own output, so a short one is a truncated
// read rather than something to reason about. Reading past its end is a panic
// in a pane that was drawing a list.
func TestIndexInfoDropsARecordItCannotRead(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name         string
		in           string
		wantLines    []string
		wantStaged   []string
		wantUnmerged []string
	}{
		{"a staged entry",
			"100644 aaa 0\tapp/a.txt\x00",
			[]string{"100644 aaa 0\tapp/a.txt"}, []string{"app/a.txt"}, nil},
		{"a record with two fields is dropped, not read",
			"100644 aaa\tapp/a.txt\x00",
			nil, nil, nil},
		{"a stage other than zero is the entry a merge left behind",
			"100644 aaa 2\tapp/a.txt\x00",
			nil, nil, []string{"100644 aaa 2\tapp/a.txt"}},
		{"a record with no tab holds no path",
			"100644 aaa 0\x00",
			nil, nil, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			lines, staged, unmerged := parseIndexInfo([]byte(c.in))
			if !reflect.DeepEqual(lines, c.wantLines) {
				t.Errorf("lines = %q, want %q", lines, c.wantLines)
			}
			if !reflect.DeepEqual(staged, c.wantStaged) {
				t.Errorf("staged = %q, want %q", staged, c.wantStaged)
			}
			if !reflect.DeepEqual(unmerged, c.wantUnmerged) {
				t.Errorf("unmerged = %q, want %q", unmerged, c.wantUnmerged)
			}
		})
	}
}

// A verb reports how many files it changed, and the number comes from comparing
// the status before with the status after. A file staged and then discarded
// changes on the index side while the working tree stays as it was, so testing
// both sides together counts it as untouched and the notice says nothing
// happened.
func TestAFileCountsAsChangedWhenEitherSideMoved(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name          string
		before, after Entry
		want          bool
	}{
		{"the index side moved",
			Entry{Path: "a", Index: Modified}, Entry{Path: "a"}, true},
		{"the working tree side moved",
			Entry{Path: "a", Worktree: Modified}, Entry{Path: "a"}, true},
		{"both moved",
			Entry{Path: "a", Index: Modified, Worktree: Modified}, Entry{Path: "a"}, true},
		{"neither moved",
			Entry{Path: "a", Index: Modified}, Entry{Path: "a", Index: Modified}, false},
		{"the file appeared",
			Entry{}, Entry{Path: "a", Worktree: Added}, true},
		{"the file went",
			Entry{Path: "a", Worktree: Modified}, Entry{}, true},
		// Both sides are read out of a map keyed by path, and a name in
		// neither status comes back as two entries with nothing in them. The
		// verb touched no such file, so the notice must not count it.
		{"the file is in neither status", Entry{}, Entry{}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := entryChanged(c.before, c.after); got != c.want {
				t.Errorf("entryChanged = %v, want %v", got, c.want)
			}
		})
	}
}

// The figures beside a file count the lines it adds and removes. A diff body is
// split on newlines and the split leaves an empty string past the last one, so
// reading it as a line would add one to every count.
func TestAnEmptyLineIsNeitherAddedNorDeleted(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		line           string
		prefixLength   int
		added, deleted bool
	}{
		{"", 0, false, false},
		{"+x", 0, true, false},
		{"-x", 0, false, true},
		{" x", 0, false, false},
		// A combined diff prefixes with two characters and either may carry it.
		{" +x", 2, true, false},
		{" -x", 2, false, true},
		{"+---", 1, true, false},
		{"-+++", 1, false, true},
		// A line of one character has no second one to read. git writes them:
		// a context line of an empty line is a single space, and the last
		// line of a body with no trailing newline arrives alone.
		{" ", 1, false, false},
		{"+", 1, true, false},
		{"-", 1, false, true},
		{"x", 1, false, false},
	} {
		t.Run("line "+c.line, func(t *testing.T) {
			t.Parallel()
			block := Block{LinePrefixLength: c.prefixLength}
			if got := block.LineAdded(c.line); got != c.added {
				t.Errorf("LineAdded(%q) = %v, want %v", c.line, got, c.added)
			}
			if got := block.LineDeleted(c.line); got != c.deleted {
				t.Errorf("LineDeleted(%q) = %v, want %v", c.line, got, c.deleted)
			}
		})
	}
}

// A worktree row draws how far it is from the base. A count of zero is not a
// direction to draw: "↑0" reads as work to push that is not there.
func TestAWorktreeRowNamesOnlyTheDistanceItHas(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		ahead, behind int
		want          string
	}{
		{0, 0, ""},
		{2, 0, "↑2"},
		{0, 3, "↓3"},
		{2, 3, "↑2 ↓3"},
	} {
		if got := FormatWorktreeAheadBehind(c.ahead, c.behind); got != c.want {
			t.Errorf("ahead=%d behind=%d is %q, want %q", c.ahead, c.behind, got, c.want)
		}
	}
}

// A rename is one entry naming two paths, and discard has to see both: the path
// it took and the path it left. Missing the old one made discard treat the
// entry as colliding with the snapshot and unstage it instead.
func TestBothSidesOfARenameAreSelected(t *testing.T) {
	t.Parallel()
	set := entryPathSet([]Entry{
		{Path: "after.txt", OldPath: "before.txt"},
		{Path: "plain.txt"},
	})
	for _, want := range []string{"after.txt", "before.txt", "plain.txt"} {
		if !set[want] {
			t.Errorf("%q is missing from %v", want, set)
		}
	}
	if set[""] {
		t.Errorf("the empty path is in the set: %v", set)
	}
}

// Diffs reads every changed file in one call, and fills in the size and mode of
// each binary one. A read that fails there leaves the row saying a binary file
// is zero bytes, which reads as an empty file rather than as a failure.
func TestDiffsReportsAFailedBinaryRead(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	env := gitTestEnv(dir)
	if err := os.WriteFile(filepath.Join(dir, "b.bin"), []byte{0, 1, 2, 0}, 0o644); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, env, dir, "add", "b.bin")
	runGitTest(t, env, dir, "commit", "-q", "-m", "add")
	if err := os.WriteFile(filepath.Join(dir, "b.bin"), []byte{0, 9, 9, 0, 9}, 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := Diffs(context.Background(), dir, false)
	if err != nil {
		t.Fatal(err)
	}
	d, ok := files["b.bin"]
	if !ok {
		t.Fatalf("b.bin is missing from %v", files)
	}
	if !d.Binary {
		t.Fatal("git did not call it binary, so this proves nothing")
	}
	// The size is the file's own, not a stand-in: a row that says five bytes
	// for a file of five bytes is the only answer that reads as true.
	if d.Size != 5 {
		t.Errorf("the binary row says %d bytes, want 5", d.Size)
	}
	if d.Mode != "100644" {
		t.Errorf("the binary row says mode %q, want 100644", d.Mode)
	}
}

// Discard on a repository with no commits has nothing to restore from, so it
// removes the files instead. A removal that fails and says nothing leaves the
// pane reporting a discard that did not happen.
func TestDiscardReportsAFailedRemoval(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := t.TempDir()
	env := gitTestEnv(dir)
	runGitTest(t, env, dir, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, env, dir, "add", "a.txt")

	entries, _, err := Status(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	undo, err := Snapshot(context.Background(), dir, entries, "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Discard(context.Background(), dir, entries, undo); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "a.txt")); err == nil {
		t.Error("the file survived a discard that reported success")
	}
}
