package state

import (
	"testing"

	"github.com/catpotd/mirugit/internal/git"
)

// A diff is read for a file, so a row that is not one has no origin to read it
// from. Answering "yes, here is one" hands back the zero Origin, whose key is
// "none:" — every row that is not a file then shares one key, and the read
// marks of all of them run together.
//
// The list moves under a click too: an index past the end names no row at all.
func TestOnlyAFileRowHasAnOriginToReadFrom(t *testing.T) {
	t.Parallel()
	s := State{Width: 90, Height: 30}
	s = Apply(s, StatusLoaded{
		Head: git.Head{Branch: "main"},
		Rows: []git.Entry{{Path: "app/a.txt", Worktree: git.Modified}},
	})

	file, heading, directory := -1, -1, -1
	for i, row := range s.Rows {
		if row.Kind() == RowFile {
			file = i
		}
		if row.Kind() == RowSectionHeading {
			heading = i
		}
		if row.Kind() == RowDirectory {
			directory = i
		}
	}
	if file < 0 || heading < 0 || directory < 0 {
		t.Fatalf("the list holds no file, heading or directory: %d rows", len(s.Rows))
	}

	for _, c := range []struct {
		name string
		at   int
		want bool
	}{
		{"a file row", file, true},
		{"a section heading", heading, false},
		{"a directory", directory, false},
		{"one past the end of the list", len(s.Rows), false},
		{"below the list", -1, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			origin, ok := OriginOf(s, c.at)
			if ok != c.want {
				t.Errorf("an origin = %v, want %v", ok, c.want)
			}
			if !c.want && origin != (Origin{}) {
				t.Errorf("a row with no origin answered with %+v", origin)
			}
		})
	}
}

// On the history tab a file row reads its diff from the commit printed above
// it. A reload can drop that commit between the frame a click was drawn on and
// the click arriving, and the file row is then on the list with nothing above
// it. Answering with an origin hands back the zero one, whose key is shared by
// every row that has none, and the read marks of all of them run together.
func TestAFileRowWithNoCommitAboveItHasNoOrigin(t *testing.T) {
	t.Parallel()
	s := State{Width: 90, Height: 30}
	s = Apply(s, HistoryLoaded{Commits: []git.CommitInfo{
		{SHA: "aaa", ShortSHA: "aaa", Subject: "tip"}}})
	s = Apply(s, TabChanged{Tab: TabHistory})
	s = Apply(s, CommitExpanded{SHA: "aaa", Files: []git.Entry{{Path: "a.txt"}}})

	var files []Row
	for _, row := range s.Rows {
		if row.Kind() == RowFile {
			files = append(files, row)
		}
	}
	if len(files) != 1 {
		t.Fatalf("the list holds %d file rows, want one: %d rows", len(files), len(s.Rows))
	}
	if _, ok := OriginOf(s, len(s.Rows)-1); !ok {
		t.Fatal("the file row under its commit has no origin, so this proves nothing")
	}

	// The commit above it is gone; the file row is not.
	s.Rows = files
	origin, ok := OriginOf(s, 0)
	if ok {
		t.Errorf("a file row with no commit above it was answered with %+v", origin)
	}
	if origin != (Origin{}) {
		t.Errorf("the answer carries %+v rather than nothing", origin)
	}
}

// On the stashed and worktrees tabs a file row reads its diff from the stash or
// tree it is listed under, and that parent can go the same way a commit can: a
// reload between the frame and the key drops it, leaving the file row with
// nothing above it. Answering with an origin hands back the zero one, which
// names the working tree of the changes tab and reads a different file.
func TestAFileRowWithNoParentAboveItHasNoOrigin(t *testing.T) {
	t.Parallel()
	s := State{Width: 90, Height: 30}
	s = Apply(s, StashedLoaded{Stashes: []git.StashRow{
		{Ref: "stash@{0}", SHA: "s0", Message: "one"}}})
	s = Apply(s, TabChanged{Tab: TabStashed})
	s = Apply(s, StashFilesLoaded{Ref: "stash@{0}",
		Files: []git.Entry{{Path: "a.txt", Worktree: git.Modified}}})

	var files []Row
	for _, row := range s.Rows {
		if row.Kind() == RowFile {
			files = append(files, row)
		}
	}
	if len(files) != 1 {
		t.Fatalf("the list holds %d file rows, want one: %d rows", len(files), len(s.Rows))
	}
	s.Cursor = len(s.Rows) - 1
	if _, ok := OriginOf(s, s.Cursor); !ok {
		t.Fatal("the file row under its stash has no origin, so this proves nothing")
	}

	// The stash above it is gone; the file row is not.
	s.Rows = files
	s.Cursor = 0
	origin, ok := OriginOf(s, 0)
	if ok {
		t.Errorf("a file row with no stash above it was answered with %+v", origin)
	}
	if origin != (Origin{}) {
		t.Errorf("the answer carries %+v rather than nothing", origin)
	}
}
