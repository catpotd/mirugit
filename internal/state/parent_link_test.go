package state

import (
	"testing"
	"time"

	"github.com/catpotd/mirugit/internal/git"
)

// The builder knows which row it is placing a file under and records it. The
// walk that used to find the parent again scanned backwards for the nearest row
// of that kind, which is the same answer only while the list has one level of
// nesting and every row of that kind above is a parent.
func TestAFileRowKnowsWhatItIsPrintedUnder(t *testing.T) {
	t.Parallel()
	s := State{Width: 90, Height: 30}
	s.Changes.Folded = map[string]bool{}
	s.Changes.Selected = map[string]bool{}
	s.Changes.Stale = map[string]bool{}
	s = Apply(s, TabChanged{Tab: TabHistory})
	s = Apply(s, HistoryLoaded{Commits: []git.CommitInfo{
		{SHA: "aaa", ShortSHA: "aaa", Subject: "tip"},
		{SHA: "bbb", ShortSHA: "bbb", Subject: "old"},
	}})
	s = Apply(s, CommitExpanded{SHA: "bbb", Files: []git.Entry{
		{Path: "a.txt", Index: git.Modified},
	}})

	fileRow := -1
	for i, r := range s.Rows {
		if r.Kind() == RowFile {
			fileRow = i
		}
	}
	if fileRow < 0 {
		t.Fatal("the commit was not expanded, so this proves nothing")
	}
	parent, at, ok := ParentAbove(s.Rows, fileRow, RowCommit)
	if !ok {
		t.Fatal("the file is printed under no commit")
	}
	if parent.Commit().SHA != "bbb" || at != fileRow-1 {
		t.Errorf("the file belongs to %q at %d, want bbb at %d",
			parent.Commit().SHA, at, fileRow-1)
	}
}

// Rows are dropped from the front when a commit leaves the log, and a link that
// then names the row itself walks in a circle. The scan this replaced could not
// loop, so the guard is what keeps that true.
func TestAParentLinkThatDoesNotPointUpwardsStops(t *testing.T) {
	t.Parallel()
	s := State{Width: 90, Height: 30}
	s.Changes.Folded = map[string]bool{}
	s.Changes.Selected = map[string]bool{}
	s.Changes.Stale = map[string]bool{}
	s = Apply(s, TabChanged{Tab: TabHistory})
	s = Apply(s, HistoryLoaded{Commits: []git.CommitInfo{{SHA: "aaa", ShortSHA: "aaa"}}})
	s = Apply(s, CommitExpanded{SHA: "aaa", Files: []git.Entry{{Path: "a.txt"}}})

	orphaned := s.Rows[1:]

	done := make(chan struct{})
	go func() {
		defer close(done)
		if _, _, ok := ParentAbove(orphaned, 0, RowCommit); ok {
			t.Error("a row whose link names itself answered with a parent")
		}
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("ParentAbove is walking in a circle")
	}
}

// A click carries the path the row had when it was drawn, and the list moves
// under it: a reload from the watcher can drop the file the click names. The
// answer then has to be "no parent", not the parent of whichever file is
// printed first — that one belongs to a different commit than the click did.
//
// The three callers (stash.go, worktree.go, history.go) act on what comes back,
// so a wrong parent is a verb run against the wrong row.
func TestAPathThatIsNotOnTheListHasNoParent(t *testing.T) {
	t.Parallel()
	s := State{Width: 90, Height: 30}
	s.Changes.Folded = map[string]bool{}
	s.Changes.Selected = map[string]bool{}
	s.Changes.Stale = map[string]bool{}
	s = Apply(s, TabChanged{Tab: TabHistory})
	s = Apply(s, HistoryLoaded{Commits: []git.CommitInfo{
		{SHA: "aaa", ShortSHA: "aaa", Subject: "tip"},
		{SHA: "bbb", ShortSHA: "bbb", Subject: "old"},
	}})
	s = Apply(s, CommitExpanded{SHA: "bbb", Files: []git.Entry{
		{Path: "a.txt", Index: git.Modified},
	}})

	if _, ok := ParentOfFile(s.Rows, "a.txt", RowCommit); !ok {
		t.Fatal("the file that is on the list has no parent, so this proves nothing")
	}
	parent, ok := ParentOfFile(s.Rows, "gone.txt", RowCommit)
	if ok {
		t.Errorf("a path nothing on the list carries was answered with %q",
			parent.Commit().SHA)
	}
}
