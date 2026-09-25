package state

import (
	"testing"

	"github.com/catpotd/mirugit/internal/git"
)

// A row the reader opened stays open until they close it. Moving the cursor
// reads, it does not fold: on the changes tab a folded directory stays folded
// wherever the cursor goes, and on history a commit stays open. The stashed and
// worktrees tabs answered the question a third way — they drew the files under
// whichever row the cursor was on — so arriving at a neighbor closed what was
// open and opened something else.
func TestAnOpenedRowStaysOpenWhenTheCursorLeaves(t *testing.T) {
	t.Parallel()
	for _, tab := range tabsThatExpand() {
		t.Run(tab.name, func(t *testing.T) {
			t.Parallel()
			s := tab.open(tab.state())
			opened := tab.rowsOf(s)

			for _, by := range []int{1, 2, -1, 5} {
				moved := Apply(s, CursorMoved{By: by})
				if got := tab.rowsOf(moved); got != opened {
					t.Errorf("moving the cursor by %d changed what is open:\n  before %s\n  after  %s",
						by, opened, got)
				}
			}
		})
	}
}

// tabsThatExpand is the three tabs whose rows open. The changes tab folds
// directories instead and answers the expansion facts with the state unchanged,
// which is checked by TestEveryTabAnswersEveryFact rather than here.
type expandingTab struct {
	name   string
	state  func() State
	open   func(State) State
	rowsOf func(State) string
	// held is the list the tab keeps for the open row. Closing drops it, and
	// the rows alone do not say so: expandIndex answers -1 once the key is
	// cleared, so a collapse that forgot the list would still draw nothing.
	held func(State) int
}

func tabsThatExpand() []expandingTab {
	return []expandingTab{
		{
			name: "worktrees",
			state: func() State {
				s := State{Width: 80, Height: 20}
				s = Apply(s, WorktreesLoaded{Base: "main", Here: "/a", Worktrees: []git.WorktreeRow{
					{Path: "/a", Name: "a", Branch: "ba"},
					{Path: "/b", Name: "b", Branch: "bb"},
				}})
				return Apply(s, TabChanged{Tab: TabWorktrees})
			},
			open: func(s State) State {
				return Apply(s, WorktreeFilesLoaded{Path: "/a", Files: []git.Entry{{Path: "x.txt"}}})
			},
			rowsOf: shapeOfRows,
			held:   func(s State) int { return len(s.Worktrees.Files) },
		},
		{
			name: "stashed",
			state: func() State {
				s := State{Width: 80, Height: 20}
				s = Apply(s, StashedLoaded{Stashes: []git.StashRow{
					{Ref: "stash@{0}", SHA: "s0", Message: "one"},
					{Ref: "stash@{1}", SHA: "s1", Message: "two"},
				}})
				return Apply(s, TabChanged{Tab: TabStashed})
			},
			open: func(s State) State {
				return Apply(s, StashFilesLoaded{Ref: "stash@{0}", Files: []git.Entry{{Path: "x.txt"}}})
			},
			rowsOf: shapeOfRows,
			held:   func(s State) int { return len(s.Stashed.Files) },
		},
		{
			name: "history",
			state: func() State {
				s := State{Width: 80, Height: 20}
				s = Apply(s, HistoryLoaded{Commits: []git.CommitInfo{
					{SHA: "a", ShortSHA: "a"}, {SHA: "b", ShortSHA: "b"},
				}})
				return Apply(s, TabChanged{Tab: TabHistory})
			},
			open: func(s State) State {
				return Apply(s, CommitExpanded{SHA: "a", Files: []git.Entry{{Path: "x.txt"}}})
			},
			rowsOf: shapeOfRows,
			held:   func(s State) int { return len(s.History.ExpandedFiles) },
		},
	}
}

// The three tabs open a row through three events and close it through one.
// collapse is where that one lands, and no test reached it: measured with
// make cover-zero, collapse, collapseStash and collapseWorktree were all at
// 0.0% while every path that opens a row was covered.
func TestClosingARowDropsTheFilesItShowed(t *testing.T) {
	t.Parallel()
	for _, tab := range tabsThatExpand() {
		t.Run(tab.name, func(t *testing.T) {
			t.Parallel()
			shut := tab.state()
			closed := tab.rowsOf(shut)

			s := tab.open(shut)
			if tab.rowsOf(s) == closed {
				t.Fatal("opening a row changed nothing, so this proves nothing")
			}
			if Facts[s.Tab].ExpandedKey(s) == "" {
				t.Fatal("a row is open and ExpandedKey names none")
			}

			s = Apply(s, RowCollapsed{})
			if got := tab.rowsOf(s); got != closed {
				t.Errorf("closing left rows the opening added:\n  want %s\n  got  %s",
					closed, got)
			}
			if key := Facts[s.Tab].ExpandedKey(s); key != "" {
				t.Errorf("the row is closed and ExpandedKey still names %q", key)
			}
			if n := tab.held(s); n != 0 {
				t.Errorf("the row is closed and %d file(s) are still held", n)
			}
		})
	}
}

// shapeOfRows names each row by kind and key, which is what changes when
// something folds or unfolds. The cursor is left out; it is what moved.
func shapeOfRows(s State) string {
	out := ""
	for _, r := range s.Rows {
		switch r.Kind() {
		case RowWorktree:
			out += " [wt " + r.Worktree().Path + "]"
		case RowStash:
			out += " [stash " + r.Stash().Ref + "]"
		case RowCommit:
			out += " [commit " + r.Commit().SHA + "]"
		case RowFile:
			out += " ->" + r.Path()
		case RowDirectory:
			out += " {" + r.DirPath() + "}"
		case RowSectionHeading:
			out += " #"
		}
	}
	return out
}

// The changes tab has no parent row: a file there sits under a directory and a
// section heading, not under a stash or a commit. Six places ask
// Facts[tab].ParentRow, and every one of them reads the false it answers —
// footer verbs, the origin a diff is opened from, three tab actions. None of
// them was exercised on the changes tab: measured with make cover-zero,
// noParentRow was at 0.0%.
func TestTheChangesTabHasNoParentRow(t *testing.T) {
	t.Parallel()
	s := State{Width: 80, Height: 20}
	s = Apply(s, StatusLoaded{Rows: []git.Entry{
		{Path: "pkg/a.txt", Worktree: git.Modified},
		{Path: "pkg/b.txt", Worktree: git.Modified},
	}, Head: git.Head{Branch: "main"}})

	if s.Tab != TabChanges {
		t.Fatalf("the fixture is on %v, not the changes tab", s.Tab)
	}
	if len(s.Rows) == 0 {
		t.Fatal("no rows, so nothing is under a cursor and this proves nothing")
	}

	for i := range s.Rows {
		at := s
		at.Cursor = i
		if _, _, ok := Facts[TabChanges].ParentRow(at); ok {
			t.Errorf("row %d (%v) answers a parent row on the changes tab",
				i, at.Rows[i].Kind())
		}
	}
	// The other three tabs do have one, which is what makes the answer above a
	// fact about this tab rather than about the fixture.
	for _, tab := range tabsThatExpand() {
		open := tab.open(tab.state())
		if _, _, ok := Facts[open.Tab].ParentRow(open); !ok {
			continue // the cursor may sit on the parent itself
		}
		return
	}
	t.Error("no tab answered a parent row, so the changes tab answering none says nothing")
}
