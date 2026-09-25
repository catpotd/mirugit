package tui

import (
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
)

// historyModel builds the list the way the program does. Writing Rows by hand
// leaves the file rows with no link to the commit above them, which the program
// always sets, and the test then measures a state no reader can reach.
func historyModel(t *testing.T, commits []git.CommitInfo, expanded string, files []git.Entry, cursor int) *Model {
	t.Helper()
	s := state.State{Width: 77, Height: 24}
	s.Changes.Folded = map[string]bool{}
	s.Changes.Selected = map[string]bool{}
	s.Changes.Stale = map[string]bool{}
	s = state.Apply(s, state.TabChanged{Tab: state.TabHistory})
	s = state.Apply(s, state.HistoryLoaded{Commits: commits})
	if expanded != "" {
		s = state.Apply(s, state.CommitExpanded{SHA: expanded, Files: files})
	}
	s.Cursor = cursor
	return &Model{state: s, render: layout.Renderer{}, read: emptyRead(), dir: t.TempDir()}
}

// requestHistoryDiff is what a click on a commit row reaches when the frame
// carried no SHA. It reads the cursor row instead, so it has to answer for
// every row kind the history tab can hold.
func TestHistoryDiffFromTheCursorRow(t *testing.T) {
	t.Parallel()
	commit := git.CommitInfo{SHA: "abc", ShortSHA: "abc", Subject: "one"}

	t.Run("a commit row already expanded collapses", func(t *testing.T) {
		t.Parallel()
		m := historyModel(t, []git.CommitInfo{commit}, "abc", nil, 0)
		next, cmd := m.requestHistoryDiff()
		m = next.(*Model)
		// Closing a row is a state change and nothing else. It used to ask git
		// for the answer and read the reply to find out it had nothing to read.
		if cmd != nil {
			t.Errorf("collapsing asked git for something: %T", cmd())
		}
		if m.state.History.ExpandedSHA != "" {
			t.Errorf("the open commit is %q, want none", m.state.History.ExpandedSHA)
		}
	})

	t.Run("a file row opens its commit's diff", func(t *testing.T) {
		t.Parallel()
		m := historyModel(t, []git.CommitInfo{commit}, "abc",
			[]git.Entry{{Path: "a.txt", Index: git.Modified}}, 1)
		next, cmd := m.requestHistoryDiff()
		if cmd == nil {
			t.Error("opening a file under a commit issued no command")
		}
		open := next.(*Model).state.Open
		sha, fromCommit := open.Origin.CommitSHA()
		if open.Path != "a.txt" || !fromCommit || sha != "abc" {
			t.Errorf("Open = %+v, want a.txt from abc", open)
		}
		if !open.PrioritizeDiff {
			t.Error("the diff key did not reserve space for the commit diff")
		}
	})

	t.Run("a file row with no commit above it does nothing", func(t *testing.T) {
		t.Parallel()
		// A file row that lost its commit is what a reload leaves behind when
		// the commit it belonged to is no longer in the log.
		m := historyModel(t, []git.CommitInfo{commit}, "abc",
			[]git.Entry{{Path: "a.txt", Index: git.Modified}}, 1)
		m.state.Rows = m.state.Rows[1:]
		m.state.Cursor = 0
		next, cmd := m.requestHistoryDiff()
		if cmd != nil {
			t.Error("a file with no commit above it issued a command")
		}
		if next.(*Model).state.Open.Path != "" {
			t.Error("a file with no commit above it opened a diff")
		}
	})

	t.Run("a cursor outside the rows does nothing", func(t *testing.T) {
		t.Parallel()
		m := historyModel(t, nil, "", nil, 3)
		if _, cmd := m.requestHistoryDiff(); cmd != nil {
			t.Error("an out-of-range cursor issued a command")
		}
	})
}

// A click carries the commit the frame drew on that line; a key carries
// nothing and the cursor row is the answer. The two are not the same row: a
// reload that shortened the list leaves the cursor where it was, and the line
// under the pointer is the one the reader meant. Reading the cursor for a click
// opens whichever commit the cursor happens to sit on.
func TestAClickOpensTheCommitOnTheLineAndAKeyOpensTheCursorRow(t *testing.T) {
	t.Parallel()
	commits := []git.CommitInfo{
		{SHA: "aaa", ShortSHA: "aaa", Subject: "one"},
		{SHA: "bbb", ShortSHA: "bbb", Subject: "two"},
		{SHA: "ccc", ShortSHA: "ccc", Subject: "three"},
	}

	t.Run("a click on the open commit closes that commit", func(t *testing.T) {
		t.Parallel()
		m := historyModel(t, commits, "ccc", nil, 0)
		next, _ := m.openRow(2, layout.Target{Kind: layout.TargetFile, Row: 2, Commit: "ccc"})
		if got := next.(*Model).state.History.ExpandedSHA; got != "" {
			t.Errorf("the open commit is %q, want none: the click named ccc", got)
		}
	})

	t.Run("a click on a closed commit opens that commit", func(t *testing.T) {
		t.Parallel()
		m := historyModel(t, commits, "aaa", nil, 0)
		next, cmd := m.openRow(2, layout.Target{Kind: layout.TargetFile, Row: 2, Commit: "ccc"})
		if cmd == nil {
			t.Fatal("the click asked git for nothing")
		}
		if got := next.(*Model).state.History.ExpandedSHA; got != "aaa" {
			t.Errorf("the open commit is %q, want aaa until the files arrive", got)
		}
	})

	t.Run("a key with no commit on it reads the cursor row", func(t *testing.T) {
		t.Parallel()
		m := historyModel(t, commits, "ccc", nil, 2)
		next, _ := m.openRow(2, layout.Target{Kind: layout.TargetFile, Row: 2})
		if got := next.(*Model).state.History.ExpandedSHA; got != "" {
			t.Errorf("the open commit is %q, want none: the cursor is on ccc", got)
		}
	})
}

// The diff follows the cursor into an expanded commit, and only onto the file
// rows there: a commit row is not a file, and opening its sha as a path asks
// git for a file nobody has. The cursor also has to be on a row at all — the
// list shortens under it when the log is reloaded.
func TestTheHistoryDiffFollowsOnlyOntoFileRows(t *testing.T) {
	t.Parallel()
	commits := []git.CommitInfo{{SHA: "aaa", ShortSHA: "aaa", Subject: "one"}}
	files := []git.Entry{{Path: "note.txt", Index: git.Modified}}
	for _, c := range []struct {
		name   string
		cursor int
		want   bool
	}{
		{"the cursor is on the commit row", 0, false},
		{"the cursor is on a file of that commit", 1, true},
		{"the cursor is past the end", 9, false},
		{"the cursor is before the start", -1, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := historyModel(t, commits, "aaa", files, c.cursor)
			if got := m.followHistoryCursor() != nil; got != c.want {
				t.Errorf("following the cursor asked git = %v, want %v", got, c.want)
			}
		})
	}
}

// The read this starts asks git for one commit by name. Both halves of the
// guard are needed: a tab other than history has no commit rows to fill in, and
// an empty name is a read of nothing, which git answers for HEAD.
func TestNoCommitIsReadWithoutATabAndAName(t *testing.T) {
	t.Parallel()
	commits := []git.CommitInfo{{SHA: "aaa", ShortSHA: "aaa", Subject: "one"}}
	for _, c := range []struct {
		name string
		tab  state.Tab
		sha  string
		want bool
	}{
		{"the history tab and a commit", state.TabHistory, "aaa", true},
		{"the history tab and no name", state.TabHistory, "", false},
		{"another tab and a commit", state.TabChanges, "aaa", false},
		{"another tab and no name", state.TabChanges, "", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := historyModel(t, commits, "", nil, 0)
			m.state = state.Apply(m.state, state.TabChanged{Tab: c.tab})
			if got := m.expandCommit(c.sha) != nil; got != c.want {
				t.Errorf("a read was started = %v, want %v", got, c.want)
			}
		})
	}
}
