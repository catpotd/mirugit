package tui

import (
	"errors"
	"fmt"
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
)

// A worktree row is marked read when the cursor rests on it, and the mark is
// keyed by path and SHA. A row whose SHA has not arrived cannot be marked:
// the key would name a tip that is not the one the reader saw, and the dot
// beside the row would go out for a state nobody looked at.
//
// The two halves have to stay apart. Inverting the SHA test leaves the mark
// off every row that has one — every real row — and puts it on the rows that
// have none, which is both halves wrong at once.
func TestOnlyAWorktreeWhoseTipIsKnownIsMarkedRead(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		sha  string
		want state.Mark
	}{
		{"a tree whose tip has arrived", "abc123", state.Read},
		{"a tree still waiting for its tip", "", state.Unread},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := state.State{Width: 90, Height: 30}
			s = state.Apply(s, state.WorktreesLoaded{Base: "main", Here: "/a",
				Worktrees: []git.WorktreeRow{
					{Path: "/a", Name: "a", Branch: "ba", SHA: c.sha},
				}})
			s = state.Apply(s, state.TabChanged{Tab: state.TabWorktrees})
			m := &Model{state: s, read: emptyRead(), render: layout.Renderer{}, dir: t.TempDir()}
			m.probe.settled = true

			row, ok := m.worktreeAtCursor()
			if !ok || row.Path != "/a" {
				t.Fatalf("the cursor is not on the tree, so this proves nothing: %+v", row)
			}

			m.markWorktreeCursorRead()

			if got := m.read.WorktreeMark("/a", c.sha); got != c.want {
				t.Errorf("the mark is %v, want %v", got, c.want)
			}
		})
	}
}

// A verb's answer names the row it was issued for by index, and the answer is
// reported only while the list still holds that row there. The first row is
// index 0: a bound that excludes it drops the answer for the topmost tree, so a
// remove that failed there says nothing at all and the reader sees the row stay
// with no reason given.
func TestTheFirstWorktreeCountsAsStillThere(t *testing.T) {
	t.Parallel()
	s := state.State{Width: 90, Height: 30}
	s = state.Apply(s, state.WorktreesLoaded{Base: "main", Here: "/a",
		Worktrees: []git.WorktreeRow{
			{Path: "/a", Name: "a", Branch: "ba", SHA: "s1"},
			{Path: "/b", Name: "b", Branch: "bb", SHA: "s2"},
		}})
	m := &Model{state: s, read: emptyRead(), render: layout.Renderer{}, dir: t.TempDir()}

	for _, c := range []struct {
		name  string
		index int
		path  string
		want  bool
	}{
		{"the first tree, where it was", 0, "/a", true},
		{"the second tree, where it was", 1, "/b", true},
		{"a tree that moved", 0, "/b", false},
		{"an index past the list", 2, "/a", false},
		{"an index below the list", -1, "/a", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := m.worktreeStillAt(c.index, c.path); got != c.want {
				t.Errorf("still there = %v, want %v", got, c.want)
			}
		})
	}
}

// What the bound costs on screen: a status read that failed on the first tree
// carries no notice, so the reader is left with a row that did not change and
// nothing saying why.
func TestAFailedStatusReadOnTheFirstWorktreeIsReported(t *testing.T) {
	t.Parallel()
	s := state.State{Width: 90, Height: 30}
	s = state.Apply(s, state.WorktreesLoaded{Base: "main", Here: "/a",
		Worktrees: []git.WorktreeRow{
			{Path: "/a", Name: "a", Branch: "ba", SHA: "s1"},
		}})
	s = state.Apply(s, state.TabChanged{Tab: state.TabWorktrees})
	m := &Model{state: s, read: emptyRead(), render: layout.Renderer{}, dir: t.TempDir()}
	m.probe.settled = true

	next, _ := m.Update(worktreeStatusMsg{
		index: 0, path: "/a", err: errors.New("git worktree: refused")})
	m = next.(*Model)

	if m.state.Notice == "" {
		t.Error("a status read that failed on the first tree said nothing")
	}
}

// The tree the reader is already in is the one place g cannot take them, so the
// verb is withheld for it. The third argument is what says which tree that is:
// inverted, g does nothing on every other tree and moves to the one the reader
// is already standing in.
//
// remove reads the same argument and does not use it — git refuses the main
// tree and a dirty one, which CanRemove answers on its own — so only the go
// side draws this.
func TestGoingToATreeIsOfferedForEveryTreeButTheOneTheReaderIsIn(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name   string
		cursor int
		moves  bool
	}{
		{"the tree the reader is in", 0, false},
		{"another tree", 1, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := state.State{Width: 90, Height: 30}
			s = state.Apply(s, state.WorktreesLoaded{Base: "main", Here: "/a",
				Worktrees: []git.WorktreeRow{
					{Path: "/a", Name: "a", Branch: "ba", SHA: "s1"},
					{Path: "/b", Name: "b", Branch: "bb", SHA: "s2"},
				}})
			s = state.Apply(s, state.TabChanged{Tab: state.TabWorktrees})
			m := &Model{state: s, read: emptyRead(), render: layout.Renderer{}, dir: t.TempDir()}
			m.probe.settled = true
			for range c.cursor {
				m.state = state.Apply(m.state, state.CursorMoved{By: 1})
			}
			row, ok := m.currentWorktree()
			if !ok {
				t.Fatal("the cursor is on no tree, so this proves nothing")
			}

			_, cmd := m.requestWorktreeGo()

			if got := cmd != nil; got != c.moves {
				t.Errorf("g on %q with the reader in /a answered with a command = %v, want %v",
					row.Path, got, c.moves)
			}
			// The command has to be the one that moves. Where it lands is the
			// subject of TestGoingToAWorktreeTakesTheReadMarksWithIt, which has
			// a worktree to land in; the trees here are names only.
			if c.moves && !isReboundMsg(cmd) {
				t.Error("g answered with a command that does not start a move")
			}
		})
	}
}

// o opens the commit in a browser, and it needs both halves: a browser on the
// machine and a remote URL that turns into an https address. Joining them with
// or offers the verb when only one holds, and the reader presses a key that
// either starts nothing or is handed an address no browser can open.
func TestOpeningACommitNeedsBothABrowserAndAnAddress(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name    string
		browser bool
		remote  string
		opens   bool
	}{
		{"a browser and an https remote", true, "https://host/owner/repo", true},
		{"a browser and no remote at all", true, "", false},
		{"a browser and a remote that is not a URL", true, "-n", false},
		{"an https remote and no browser", false, "https://host/owner/repo", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := state.State{Width: 90, Height: 30}
			s = state.Apply(s, state.StatusLoaded{
				Head: git.Head{Branch: "main"}, RemoteURL: c.remote})
			s = state.Apply(s, state.HistoryLoaded{
				Commits: []git.CommitInfo{{SHA: "abc", ShortSHA: "abc", Subject: "one"}}})
			s = state.Apply(s, state.TabChanged{Tab: state.TabHistory})
			m := &Model{state: s, read: emptyRead(), dir: t.TempDir(),
				render:  layout.Renderer{Browser: c.browser},
				browser: alwaysCommand("true")}
			m.probe.settled = true

			_, cmd := m.requestOpenCommit()
			if got := cmd != nil; got != c.opens {
				t.Errorf("o answered with a command = %v, want %v", got, c.opens)
			}
		})
	}
}

// The commit under the cursor is a question only the history tab can answer.
// The other tabs hold rows that carry no commit, and a Row hands back a value
// rather than a pointer, so a zero CommitInfo reads the same as a missing one.
// Answering "yes, here is a commit" for them copies an empty sha to the
// reader's clipboard and offers verbs for a commit that is not there.
func TestTheCommitUnderTheCursorIsAskedOnlyOnTheHistoryTab(t *testing.T) {
	t.Parallel()
	for _, tab := range []state.Tab{
		state.TabChanges, state.TabStashed, state.TabWorktrees, state.TabHistory,
	} {
		t.Run(state.Facts[tab].Name, func(t *testing.T) {
			t.Parallel()
			s := state.State{Width: 90, Height: 30}
			s = state.Apply(s, state.StatusLoaded{
				Head: git.Head{Branch: "main"},
				Rows: []git.Entry{{Path: "a.txt", Worktree: git.Modified}},
			})
			s = state.Apply(s, state.HistoryLoaded{
				Commits: []git.CommitInfo{{SHA: "abc", ShortSHA: "abc", Subject: "one"}}})
			s = state.Apply(s, state.StashedLoaded{
				Stashes: []git.StashRow{{Ref: "stash@{0}", SHA: "s0", Message: "m"}}})
			s = state.Apply(s, state.WorktreesLoaded{Base: "main", Here: "/a",
				Worktrees: []git.WorktreeRow{{Path: "/a", Name: "a", Branch: "b", SHA: "s1"}}})
			s = state.Apply(s, state.TabChanged{Tab: tab})
			m := &Model{state: s, read: emptyRead(), dir: t.TempDir(),
				render:    layout.Renderer{Clipboard: true},
				clipboard: alwaysCommand("true")}
			m.probe.settled = true

			_, _, ok := m.currentCommitAt()
			if want := tab == state.TabHistory; ok != want {
				t.Errorf("a commit under the cursor = %v, want %v", ok, want)
			}

			_, cmd := m.requestCopySHA()
			if want := tab == state.TabHistory; (cmd != nil) != want {
				t.Errorf("y answered with a command = %v, want %v", cmd != nil, want)
			}
		})
	}
}

// stashStillAt is worktreeStillAt for the stashed tab, and the bound is read
// the same way: an index past the end names no row, and reading rows[index]
// there is a read past the end of the list.
func TestAStashIndexPastTheListIsNotStillThere(t *testing.T) {
	t.Parallel()
	s := state.State{Width: 90, Height: 30}
	s = state.Apply(s, state.StashedLoaded{Stashes: []git.StashRow{
		{Ref: "stash@{0}", SHA: "s0", Message: "one"},
		{Ref: "stash@{1}", SHA: "s1", Message: "two"},
	}})
	m := &Model{state: s, read: emptyRead(), render: layout.Renderer{}, dir: t.TempDir()}

	for _, c := range []struct {
		name  string
		index int
		sha   string
		want  bool
	}{
		{"the first stash, where it was", 0, "s0", true},
		{"the second stash, where it was", 1, "s1", true},
		{"a stash that moved", 0, "s1", false},
		{"one past the end of the list", 2, "s0", false},
		{"far past the end", 99, "s0", false},
		{"below the list", -1, "s0", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := m.stashStillAt(c.index, c.sha); got != c.want {
				t.Errorf("still there = %v, want %v", got, c.want)
			}
		})
	}
}

// The diff panel shares the pane with the list, and every tab that opens a file
// draws one. Whether there is room for it is a question about the list and the
// pane, and the same question on each tab: a list longer than the pane leaves a
// diff nothing until the list gives rows up.
func TestEveryTabAsksWhetherTheDiffHasRoom(t *testing.T) {
	t.Parallel()
	for _, tab := range []state.Tab{
		state.TabChanges, state.TabStashed, state.TabWorktrees, state.TabHistory,
	} {
		t.Run(state.Facts[tab].Name, func(t *testing.T) {
			t.Parallel()
			for _, c := range []struct {
				name string
				rows int
				want bool
			}{
				{"a list the pane holds", 1, false},
				{"a list longer than the pane", 60, true},
			} {
				t.Run(c.name, func(t *testing.T) {
					t.Parallel()
					m := modelShowing(t, tab, c.rows)
					if got := m.diffHasNoRoom(); got != c.want {
						t.Errorf("the diff has no room = %v, want %v", got, c.want)
					}
				})
			}
		})
	}
}

// modelShowing builds a pane on tab with rows rows in its list.
func modelShowing(t *testing.T, tab state.Tab, rows int) *Model {
	t.Helper()
	entries := make([]git.Entry, rows)
	commits := make([]git.CommitInfo, rows)
	stashes := make([]git.StashRow, rows)
	trees := make([]git.WorktreeRow, rows)
	for i := range rows {
		name := fmt.Sprintf("f%02d", i)
		entries[i] = git.Entry{Path: name + ".txt", Worktree: git.Modified}
		commits[i] = git.CommitInfo{SHA: name, ShortSHA: name, Subject: name}
		stashes[i] = git.StashRow{Ref: "stash@{" + name + "}", SHA: name, Message: name}
		trees[i] = git.WorktreeRow{Path: "/" + name, Name: name, Branch: name, SHA: name}
	}
	s := state.State{Width: 90, Height: 30}
	s = state.Apply(s, state.StatusLoaded{Head: git.Head{Branch: "main"}, Rows: entries})
	s = state.Apply(s, state.HistoryLoaded{Commits: commits})
	s = state.Apply(s, state.StashedLoaded{Stashes: stashes})
	s = state.Apply(s, state.WorktreesLoaded{Base: "main", Here: "/f00", Worktrees: trees})
	s = state.Apply(s, state.TabChanged{Tab: tab})
	m := &Model{state: s, read: emptyRead(), render: layout.Renderer{}, dir: t.TempDir()}
	m.probe.settled = true
	return m
}

// The expanded commit is the one the cursor row is: a file row under it is not
// it, and neither is anything on another tab. Answering with a sha for those
// rows names a commit the reader is not on, and the pane expands or collapses
// the wrong one.
func TestOnlyACommitRowNamesTheExpandedCommit(t *testing.T) {
	t.Parallel()
	s := state.State{Width: 90, Height: 30}
	s = state.Apply(s, state.HistoryLoaded{Commits: []git.CommitInfo{
		{SHA: "aaa", ShortSHA: "aaa", Subject: "tip"},
		{SHA: "bbb", ShortSHA: "bbb", Subject: "old"},
	}})
	s = state.Apply(s, state.TabChanged{Tab: state.TabHistory})
	s = state.Apply(s, state.CommitExpanded{SHA: "aaa", Files: []git.Entry{{Path: "a.txt"}}})

	commit, file := -1, -1
	for i, row := range s.Rows {
		if row.Kind() == state.RowCommit && commit < 0 {
			commit = i
		}
		if row.Kind() == state.RowFile {
			file = i
		}
	}
	if commit < 0 || file < 0 {
		t.Fatalf("no commit row or file row in %d rows", len(s.Rows))
	}

	m := &Model{state: s, read: emptyRead(), render: layout.Renderer{}, dir: t.TempDir()}
	m.state.Cursor = commit
	if got := m.expandedCommitSHA(); got != "aaa" {
		t.Errorf("on the commit row the sha is %q, want aaa", got)
	}
	m.state.Cursor = file
	if got := m.expandedCommitSHA(); got != "" {
		t.Errorf("on a file row the sha is %q, want none", got)
	}
	m.state.Cursor = len(m.state.Rows)
	if got := m.expandedCommitSHA(); got != "" {
		t.Errorf("with the cursor past the list the sha is %q, want none", got)
	}
}

// The key that jumps to a section puts the cursor on that section's heading.
// Both halves of the search matter: without the kind it lands on the first row
// of the section, which is a file, and without the section it lands on the
// first heading whatever was asked for.
func TestJumpingToASectionLandsOnThatSectionsHeading(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		sec  state.Section
	}{
		{"staged", state.SectionStaged},
		{"unstaged", state.SectionUnstaged},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := state.State{Width: 90, Height: 30}
			s = state.Apply(s, state.StatusLoaded{
				Head: git.Head{Branch: "main"},
				Rows: []git.Entry{
					{Path: "staged.txt", Index: git.Modified},
					{Path: "unstaged.txt", Worktree: git.Modified},
				},
			})
			m := &Model{state: s, read: emptyRead(), render: layout.Renderer{}, dir: t.TempDir()}
			m.probe.settled = true

			m.cursorToSectionHeading(int(c.sec))

			row, ok := state.CursorRow(m.state)
			if !ok {
				t.Fatalf("the cursor is on no row at %d", m.state.Cursor)
			}
			if row.Kind() != state.RowSectionHeading {
				t.Errorf("the cursor landed on a %v, want a section heading", row.Kind())
			}
			if row.Section() != c.sec {
				t.Errorf("the cursor landed on the %v heading, want %v", row.Section(), c.sec)
			}
		})
	}
}
