package layout

import (
	"fmt"
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// everythingUnread answers Unread for any file, so a tab's dot is lit by the
// files that tab counts rather than by which ones happen to be marked.
type everythingUnread struct{}

func (everythingUnread) MarkIn(state.Origin, string, []string) state.Mark { return state.Unread }
func (everythingUnread) StashMark(string) state.Mark                      { return state.Read }
func (everythingUnread) WorktreeMark(string, string) state.Mark           { return state.Read }
func (everythingUnread) BlockRead(string, string) bool                    { return true }
func (everythingUnread) HashesIn(state.Origin, string) []string           { return []string{"h"} }

// A tab's dot says whether that tab holds something unread. It has to answer
// the same whichever tab is being drawn, because the bar shows all of them at
// once. The changes dot used to read the rows of whatever tab was open: opening
// a stash lit it for files that are not in the working tree.
func TestATabsDotDoesNotDependOnWhichTabIsOpen(t *testing.T) {
	t.Parallel()
	base := func() state.State {
		s := state.State{Width: 90, Height: 30, Tab: state.TabChanges}
		s.Changes.Folded = map[string]bool{}
		s.Changes.Selected = map[string]bool{}
		s.Changes.Stale = map[string]bool{}
		s = state.Apply(s, state.StatusLoaded{Rows: nil, Head: git.Head{Branch: "main"}})
		s = state.Apply(s, state.StashedLoaded{Stashes: []git.StashRow{
			{Ref: "stash@{0}", SHA: "s0", Message: "one", Status: git.StashApplies},
		}})
		s = state.Apply(s, state.HistoryLoaded{Commits: []git.CommitInfo{
			{SHA: "aaa", ShortSHA: "aaa", Subject: "one", Age: "1m"},
		}})
		s = state.Apply(s, state.WorktreesLoaded{Base: "main", Worktrees: []git.WorktreeRow{
			{Path: "/repo", Name: "repo", Main: true, SHA: "aaa"},
		}})
		return s
	}

	// The working tree is clean, so the changes dot is off wherever it is read.
	want := changesTabUnread(base(), everythingUnread{})
	if want {
		t.Fatal("a clean working tree already lights the changes dot, so this proves nothing")
	}

	for _, c := range []struct {
		name string
		open func(state.State) state.State
	}{
		{"the stashed tab with its files showing", func(s state.State) state.State {
			s = state.Apply(s, state.TabChanged{Tab: state.TabStashed})
			return state.Apply(s, state.StashFilesLoaded{Ref: "stash@{0}",
				Files: []git.Entry{{Path: "in-stash.txt"}}})
		}},
		{"the worktrees tab with its files showing", func(s state.State) state.State {
			s = state.Apply(s, state.TabChanged{Tab: state.TabWorktrees})
			return state.Apply(s, state.WorktreeFilesLoaded{Path: "/repo",
				Files: []git.Entry{{Path: "in-worktree.txt"}}})
		}},
		{"the history tab with a commit expanded", func(s state.State) state.State {
			s = state.Apply(s, state.TabChanged{Tab: state.TabHistory})
			return state.Apply(s, state.CommitExpanded{SHA: "aaa",
				Files: []git.Entry{{Path: "in-commit.txt", Index: git.Modified}}})
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := c.open(base())
			if len(s.Rows) < 2 {
				t.Fatalf("nothing is expanded, so this proves nothing: %d rows", len(s.Rows))
			}
			if got := changesTabUnread(s, everythingUnread{}); got != want {
				t.Errorf("the changes dot is %v while %s, and %v on the changes tab",
					got, c.name, want)
			}
		})
	}
}

// oneUnread answers Unread for the stash and the tree it names, and Read for
// every other, so a dot that lights is lit by the row the test asked about.
type oneUnread struct {
	stashSHA     string
	worktreePath string
}

func (oneUnread) MarkIn(state.Origin, string, []string) state.Mark { return state.Read }
func (o oneUnread) StashMark(sha string) state.Mark {
	if sha == o.stashSHA {
		return state.Unread
	}
	return state.Read
}
func (o oneUnread) WorktreeMark(path, _ string) state.Mark {
	if path == o.worktreePath {
		return state.Unread
	}
	return state.Read
}
func (oneUnread) BlockRead(string, string) bool          { return true }
func (oneUnread) HashesIn(state.Origin, string) []string { return []string{"h"} }

// The stashed and worktrees dots light when one of their rows is unread and go
// out when none is. Nothing checked the lit half: a mutation that returned
// false where these return true survived the suite, which means the dot could
// have stopped lighting and no test would say so.
//
// A worktree with no SHA is skipped rather than counted: the list arrives
// before the read that fills it in, and a row that has not been read yet is not
// a row with something new in it.
func TestTheStashedAndWorktreesDotsFollowTheirOwnRows(t *testing.T) {
	t.Parallel()
	s := state.State{Width: 90, Height: 30}
	s = state.Apply(s, state.StashedLoaded{Stashes: []git.StashRow{
		{Ref: "stash@{0}", SHA: "s0", Message: "one"},
		{Ref: "stash@{1}", SHA: "s1", Message: "two"},
	}})
	s = state.Apply(s, state.WorktreesLoaded{Base: "main", Worktrees: []git.WorktreeRow{
		{Path: "/repo", Name: "repo", Main: true, SHA: "aaa"},
		{Path: "/side", Name: "side", SHA: "bbb"},
	}})

	for _, c := range []struct {
		name  string
		marks ReadMarks
		stash bool
		tree  bool
	}{
		{"nothing unread", oneUnread{}, false, false},
		{"one stash unread", oneUnread{stashSHA: "s1"}, true, false},
		{"one tree unread", oneUnread{worktreePath: "/side"}, false, true},
		{"both unread", oneUnread{stashSHA: "s0", worktreePath: "/repo"}, true, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := stashTabUnread(s, c.marks); got != c.stash {
				t.Errorf("the stashed dot is %v, want %v", got, c.stash)
			}
			if got := worktreeTabUnread(s, c.marks); got != c.tree {
				t.Errorf("the worktrees dot is %v, want %v", got, c.tree)
			}
		})
	}
}

// A tree the read has not reached carries no SHA, and marking it unread would
// light the dot for every repository on the first frame.
func TestAWorktreeWithNoShaDoesNotLightTheDot(t *testing.T) {
	t.Parallel()
	s := state.State{Width: 90, Height: 30}
	s = state.Apply(s, state.WorktreesLoaded{Base: "main", Worktrees: []git.WorktreeRow{
		git.WorktreeRowSkeleton("/side", "side", "b"),
	}})
	if s.Worktrees.List[0].SHA != "" {
		t.Fatal("the skeleton carries a SHA, so this proves nothing")
	}
	if worktreeTabUnread(s, oneUnread{worktreePath: "/side"}) {
		t.Error("a tree whose SHA has not arrived lights the dot")
	}
}

// unreadIn answers Unread for one section and Read for the other, so a test can
// tell which section's mark lit the dot.
type unreadIn struct{ origin state.Origin }

func (u unreadIn) MarkIn(o state.Origin, _ string, _ []string) state.Mark {
	if o == u.origin {
		return state.Unread
	}
	return state.Read
}
func (unreadIn) StashMark(string) state.Mark            { return state.Read }
func (unreadIn) WorktreeMark(string, string) state.Mark { return state.Read }
func (unreadIn) BlockRead(string, string) bool          { return true }
func (unreadIn) HashesIn(state.Origin, string) []string { return []string{"h"} }

// A file is staged and unstaged separately, and each side is marked read on its
// own. The dot is lit by an unread side that holds a change; the mark left on
// the side that holds none says nothing. A file staged, read, then changed
// again in the working tree keeps its staged mark, and counting that mark for a
// file with nothing staged lights the dot for a change nobody made.
func TestTheChangesDotReadsTheSectionThatHoldsTheChange(t *testing.T) {
	t.Parallel()
	staged := state.WorkingTree(state.SectionStaged)
	unstaged := state.WorkingTree(state.SectionUnstaged)
	for _, c := range []struct {
		name  string
		entry git.Entry
		marks state.Origin
		want  bool
	}{
		{"unstaged change, unread on the staged side",
			git.Entry{Path: "a.txt", Worktree: git.Modified}, staged, false},
		{"staged change, unread on the unstaged side",
			git.Entry{Path: "a.txt", Index: git.Modified}, unstaged, false},
		{"unstaged change, unread on the unstaged side",
			git.Entry{Path: "a.txt", Worktree: git.Modified}, unstaged, true},
		{"staged change, unread on the staged side",
			git.Entry{Path: "a.txt", Index: git.Modified}, staged, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := state.State{Width: 90, Height: 30, Tab: state.TabChanges}
			s.Changes.Folded = map[string]bool{}
			s.Changes.Selected = map[string]bool{}
			s.Changes.Stale = map[string]bool{}
			s = state.Apply(s, state.StatusLoaded{Rows: []git.Entry{c.entry},
				Head: git.Head{Branch: "main"}})
			if got := changesTabUnread(s, unreadIn{c.marks}); got != c.want {
				t.Errorf("the changes dot is %v, want %v", got, c.want)
			}
		})
	}
}

// unreadEverywhere answers unread for a file, a stash and a worktree alike,
// which everythingUnread does not: it answers for files only.
type unreadEverywhere struct{}

func (unreadEverywhere) MarkIn(state.Origin, string, []string) state.Mark { return state.Unread }
func (unreadEverywhere) StashMark(string) state.Mark                      { return state.Unread }
func (unreadEverywhere) WorktreeMark(string, string) state.Mark           { return state.Unread }
func (unreadEverywhere) BlockRead(string, string) bool                    { return false }
func (unreadEverywhere) HashesIn(state.Origin, string) []string           { return []string{"h"} }

// A pane with no read marks at all — the file could not be read, or the reader
// asked for none — knows of nothing unread. Answering "unread" there lights
// every tab's dot on every draw, and a dot that is always on says nothing.
func TestNoReadMarksMeansNothingIsUnread(t *testing.T) {
	t.Parallel()
	s := state.State{Width: 90, Height: 30, Tab: state.TabChanges}
	s.Changes.Folded = map[string]bool{}
	s.Changes.Selected = map[string]bool{}
	s.Changes.Stale = map[string]bool{}
	s = state.Apply(s, state.StatusLoaded{Rows: []git.Entry{
		{Path: "a.txt", Worktree: git.Modified}}, Head: git.Head{Branch: "main"}})
	s = state.Apply(s, state.StashedLoaded{Stashes: []git.StashRow{
		{Ref: "stash@{0}", SHA: "s0", Message: "one", Status: git.StashApplies}}})
	s = state.Apply(s, state.WorktreesLoaded{Base: "main", Here: "/repo",
		Worktrees: []git.WorktreeRow{
			{Path: "/repo", Name: "repo", Main: true, SHA: "aaa"},
			{Path: "/wt", Name: "wt", SHA: "bbb"}}})

	for _, c := range []struct {
		name string
		lit  func(state.State, ReadMarks) bool
	}{
		{"changes", changesTabUnread},
		{"stashed", stashTabUnread},
		{"worktrees", worktreeTabUnread},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if c.lit(s, nil) {
				t.Error("the dot is lit for a pane that holds no read marks")
			}
			// The same rows with marks that answer unread light it, so the
			// answer above is not "this tab never lights".
			if !c.lit(s, unreadEverywhere{}) {
				t.Error("the dot is not lit for rows every mark calls unread")
			}
		})
	}
}

// Every repository has a main working tree, so a count of one says the reader
// added none: the tab would hold a single row naming the directory they are
// already in. It is left out of the bar until there is a second tree, and the
// count is what says so.
func TestTheWorktreesTabIsCountedOnlyOnceThereIsASecondTree(t *testing.T) {
	t.Parallel()
	trees := func(n int) state.State {
		s := state.State{Width: 90, Height: 30}
		list := make([]git.WorktreeRow, n)
		for i := range list {
			list[i] = git.WorktreeRow{Path: fmt.Sprintf("/w%d", i),
				Name: fmt.Sprintf("w%d", i), SHA: "aaa"}
		}
		if n > 0 {
			list[0].Main = true
		}
		return state.Apply(s, state.WorktreesLoaded{Worktrees: list, Base: "main", Here: "/w0"})
	}
	for _, c := range []struct{ trees, want int }{
		{0, 0}, {1, 0}, {2, 2}, {5, 5},
	} {
		if got := worktreeTabCount(trees(c.trees)); got != c.want {
			t.Errorf("%d trees are counted as %d, want %d", c.trees, got, c.want)
		}
		if got := state.TabVisible(state.TabWorktrees, worktreeTabCount(trees(c.trees))); got != (c.want > 0) {
			t.Errorf("%d trees put the tab in the bar = %v, want %v",
				c.trees, got, c.want > 0)
		}
	}
}
