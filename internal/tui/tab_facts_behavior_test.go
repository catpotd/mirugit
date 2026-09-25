package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// The tab table holds a value per tab, so a wrong entry looks exactly like a
// right one. TestEveryTabAnswersEveryFact checks that each tab names every
// field; nothing checked that the value it names is the one the reader gets. A
// mutation of the worktrees tab's LowerUResetsCommit survived the whole suite.
//
// The expectations below are written out rather than read from the table. A
// test that compares the behavior against the same field the behavior reads
// passes whichever value the field holds, which is how the first version of
// this file went green with AllowsSelection flipped on every tab.

// Only the changes tab has verbs that spend a selection. A box on the other
// three would be one the reader can fill and never spend.
func TestOnlyTheChangesTabTakesATick(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		tab  state.Tab
		want bool
	}{
		{state.TabChanges, true},
		{state.TabHistory, false},
		{state.TabStashed, false},
		{state.TabWorktrees, false},
	} {
		t.Run(state.Facts[c.tab].Name, func(t *testing.T) {
			t.Parallel()
			m := expandedTabModel(t, c.tab)
			row, ok := firstFileRow(m)
			if !ok {
				t.Fatal("this tab draws no file row, so the tick has nothing to land on")
			}
			m.state.Cursor = row
			m.state = state.Apply(m.state, state.SelectionToggled{
				Path:    m.state.Rows[row].Path(),
				Section: m.state.Rows[row].Section(),
			})

			if ticked := state.SelectionCount(m.state) > 0; ticked != c.want {
				t.Errorf("a tick landed %v, want %v", ticked, c.want)
			}
		})
	}
}

// r marks a file read without opening it. The other three tabs read their files
// out of a stash, a commit or another worktree, none of which the read marks of
// this working tree describe.
func TestOnlyTheChangesTabAnswersTheReadKey(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		tab  state.Tab
		want bool
	}{
		{state.TabChanges, true},
		{state.TabHistory, false},
		{state.TabStashed, false},
		{state.TabWorktrees, false},
	} {
		t.Run(state.Facts[c.tab].Name, func(t *testing.T) {
			t.Parallel()
			m := expandedTabModel(t, c.tab)
			row, ok := firstFileRow(m)
			if !ok {
				t.Fatal("this tab draws no file row to read")
			}
			m.state.Cursor = row

			if _, cmd := m.requestRead(); (cmd != nil) != c.want {
				t.Errorf("r read %v, want %v", cmd != nil, c.want)
			}
		})
	}
}

// u is one key with two jobs. On the history tab it undoes the last commit; on
// the changes tab it unstages. Handing the undo to the wrong tab moves HEAD.
//
// The stashed and worktrees tabs draw no row either job acts on, so u leaves
// nothing behind there whichever one it picked; their entry cannot be checked
// from outside the table.
func TestLowerUUnstagesOnChangesAndUndoesOnHistory(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		tab         state.Tab
		wantUnstage bool
		wantUndo    bool
	}{
		{state.TabChanges, true, false},
		{state.TabHistory, false, true},
	} {
		t.Run(state.Facts[c.tab].Name, func(t *testing.T) {
			t.Parallel()
			m := everyTabModel(t)
			// The tip commit has to be one undo is offered on, or the history
			// half proves nothing: reset reaches one commit and only while no
			// remote holds it.
			m.state = state.Apply(m.state, state.HistoryLoaded{Commits: []git.CommitInfo{
				{SHA: "aaa", ShortSHA: "aaa", Subject: "tip", Age: "1m", Unpushed: true},
			}})
			m.state = state.Apply(m.state, state.StatusLoaded{
				Rows: []git.Entry{{Path: "a.txt", Index: git.Modified}},
				Head: git.Head{Branch: "main"}})
			m.state = state.Apply(m.state, state.TabChanged{Tab: c.tab})
			// Each job has a row it acts on: unstage a staged file, undo the
			// commit at the tip. The changes tab opens on its section heading.
			m.state.Cursor = 0
			if c.tab == state.TabChanges {
				row, ok := firstFileRow(m)
				if !ok {
					t.Fatal("no staged file row, so unstage has nothing to act on")
				}
				m.state.Cursor = row
			}

			// rowVerbKey rather than Update: Update batches the follow commands
			// onto the answer, so the command it returns says nothing about
			// which job ran.
			next, cmd := m.rowVerbKey(tea.KeyPressMsg{Code: 'u'})
			m = next.(*Model)

			if armed := m.state.Changes.Pending != nil; armed != c.wantUnstage {
				t.Errorf("u armed unstage %v, want %v", armed, c.wantUnstage)
			}
			if ran := cmd != nil; ran != (c.wantUnstage || c.wantUndo) {
				t.Errorf("u ran a command %v, want %v", ran, c.wantUnstage || c.wantUndo)
			}
		})
	}
}

// expandedTabModel opens a tab with its rows already expanded into files, which
// is the state every one of these keys acts in.
func expandedTabModel(t *testing.T, tab state.Tab) *Model {
	t.Helper()
	m := everyTabModel(t)
	m.state = state.Apply(m.state, state.TabChanged{Tab: tab})
	m.state.Cursor = 0
	switch tab {
	case state.TabHistory:
		m.state = state.Apply(m.state, state.CommitExpanded{SHA: "aaa",
			Files: []git.Entry{{Path: "a.txt", Index: git.Modified}}})
	case state.TabStashed:
		m.state = state.Apply(m.state, state.StashFilesLoaded{Ref: "stash@{0}",
			Files: []git.Entry{{Path: "a.txt"}}})
	case state.TabWorktrees:
		m.state = state.Apply(m.state, state.WorktreeFilesLoaded{Path: "/repo",
			Files: []git.Entry{{Path: "a.txt"}}})
	case state.TabChanges, state.TabCount:
	}
	return m
}

// firstFileRow is where a key that acts on a file has to put the cursor. Every
// tab draws its files somewhere below its own rows.
func firstFileRow(m *Model) (int, bool) {
	for i, r := range m.state.Rows {
		if r.Kind() == state.RowFile {
			return i, true
		}
	}
	return 0, false
}

// The file rows drawn under a worktree carry the change on the side it is on,
// because the diff is asked for with --cached or without it depending on that
// side. Carrying it on the working-tree side whichever side it came from left
// worktreeFileStaged answering false for every file, and a change that is only
// staged opened an empty pane.
func TestAWorktreeFileNamesTheSideItsChangeIsOn(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		file git.Entry
		want bool
	}{
		{"a change only the index holds",
			git.Entry{Path: "a.txt", Index: git.Modified}, true},
		{"a change the working tree holds",
			git.Entry{Path: "b.txt", Worktree: git.Modified}, false},
		{"a file git has never seen",
			git.Entry{Path: "c.txt", Worktree: git.Untracked}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := everyTabModel(t)
			m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabWorktrees})
			m.state.Cursor = 0
			m.state = state.Apply(m.state, state.WorktreeFilesLoaded{
				Path: "/repo", Files: []git.Entry{c.file}})

			row, ok := firstFileRow(m)
			if !ok {
				t.Fatal("the file was not drawn under the worktree")
			}
			if got := worktreeFileStaged(m.state.Rows[row]); got != c.want {
				t.Errorf("the row asks for the staged side %v, want %v", got, c.want)
			}
		})
	}
}

// The read marks are filed under where a file came from. Reading a file inside
// one commit used to mark it read inside every other commit holding that path,
// because the key said "history" and not which commit.
func TestReadingAFileInOneCommitLeavesTheOthersUnread(t *testing.T) {
	t.Parallel()
	m := newParentModel(t)
	m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabHistory})
	m.state = state.Apply(m.state, state.HistoryLoaded{Commits: []git.CommitInfo{
		{SHA: "aaa", ShortSHA: "aaa", Subject: "one", Age: "1m"},
		{SHA: "bbb", ShortSHA: "bbb", Subject: "two", Age: "2m"},
	}})

	readIn := func(sha string) state.Mark {
		m.state = state.Apply(m.state, state.CommitExpanded{SHA: sha,
			Files: []git.Entry{{Path: "a.txt", Index: git.Modified}}})
		row, ok := firstFileRow(m)
		if !ok {
			t.Fatalf("%s drew no file row", sha)
		}
		m.state.Cursor = row
		origin, ok := state.OriginOf(m.state, row)
		if !ok {
			t.Fatalf("the file under %s belongs to no commit", sha)
		}
		return m.read.MarkIn(origin, "a.txt", nil)
	}

	// Finish the file inside the first commit.
	m.state = state.Apply(m.state, state.CommitExpanded{SHA: "aaa",
		Files: []git.Entry{{Path: "a.txt", Index: git.Modified}}})
	row, ok := firstFileRow(m)
	if !ok {
		t.Fatal("the commit drew no file row")
	}
	m.state.Cursor = row
	origin, ok := state.OriginOf(m.state, row)
	if !ok {
		t.Fatal("the file belongs to no commit")
	}
	m.read.MarkFileIn(origin, "a.txt", nil)

	if got := readIn("aaa"); got != state.Read {
		t.Fatalf("the commit that was read answers %v, so this proves nothing", got)
	}
	if got := readIn("bbb"); got != state.Unread {
		t.Errorf("the other commit answers %v for a file never opened there", got)
	}
}
