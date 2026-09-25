package state

import (
	"testing"

	"github.com/catpotd/mirugit/internal/git"
)

// CommitIndexAt counts the commits above a row, and undo is offered only at
// zero — on the tip. Counting the rows that are not commits instead answers a
// number that grows with the file rows an expanded commit puts on the list, so
// the tip stops being the tip as soon as the reader opens it.
func TestTheCommitIndexCountsCommitRowsAndNoOthers(t *testing.T) {
	t.Parallel()
	s := State{Width: 90, Height: 30}
	s = Apply(s, HistoryLoaded{Commits: []git.CommitInfo{
		{SHA: "aaa", ShortSHA: "aaa", Subject: "tip"},
		{SHA: "bbb", ShortSHA: "bbb", Subject: "old"},
	}})
	s = Apply(s, TabChanged{Tab: TabHistory})
	s = Apply(s, CommitExpanded{SHA: "aaa", Files: []git.Entry{
		{Path: "a.txt"}, {Path: "b.txt"},
	}})

	commits, files := []int{}, []int{}
	for i, row := range s.Rows {
		if row.Kind() == RowCommit {
			commits = append(commits, i)
		}
		if row.Kind() == RowFile {
			files = append(files, i)
		}
	}
	if len(commits) != 2 || len(files) != 2 {
		t.Fatalf("the list holds %d commits and %d files, want two of each: %d rows",
			len(commits), len(files), len(s.Rows))
	}

	if got := CommitIndexAt(s, commits[0]); got != 0 {
		t.Errorf("the tip is at index %d, want 0", got)
	}
	// The files of the tip sit between it and the next commit. They are not
	// commits, so the index does not move across them.
	for _, at := range files {
		if got := CommitIndexAt(s, at); got != 1 {
			t.Errorf("a file row of the tip is at index %d, want 1", got)
		}
	}
	if got := CommitIndexAt(s, commits[1]); got != 1 {
		t.Errorf("the second commit is at index %d, want 1", got)
	}
}
