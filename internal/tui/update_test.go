package tui

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// Update sends a message to updateEvent first and to updateResult only when
// updateEvent says it did not take it. The flag is what separates the two, and
// a case that answers false hands its message on: the model it built and the
// command it asked for are both dropped, and the message reaches a switch that
// does not name it. For a watch event that means the next read of the watcher
// is never issued and the pane stops following the repository.
func TestUpdateEventSaysItTookEveryMessageItNames(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		msg  tea.Msg
	}{
		{"the terminal took focus", tea.FocusMsg{}},
		{"the terminal lost focus", tea.BlurMsg{}},
		{"the watcher answered", watchReadyMsg{}},
		{"the watcher saw a change", watchEventMsg{}},
		{"the wait after a change ran out", watchDebounceDoneMsg{}},
		{"the poll came round", watchPollTickMsg{}},
		{"the window was resized", tea.WindowSizeMsg{Width: 80, Height: 24}},
		{"the terminal said where the cursor is", tea.CursorPositionMsg{}},
		{"the width probe ran out", widthProbeTimeoutMsg{}},
		{"the width probe after a draw", widthProbeAfterDrawMsg{}},
		{"a key was pressed", tea.KeyPressMsg{Code: 'j'}},
		{"the pointer moved", tea.MouseMotionMsg{}},
		{"the pointer was clicked", tea.MouseClickMsg{}},
		{"the wheel was turned", tea.MouseWheelMsg{}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := modelForRange(t, rangeTestRows(), 0)
			if _, _, handled := m.updateEvent(c.msg); !handled {
				t.Errorf("updateEvent passed %T on to updateResult", c.msg)
			}
		})
	}

	// A message it does not name is the other half of the same decision.
	t.Run("a message it does not name", func(t *testing.T) {
		t.Parallel()
		m := modelForRange(t, rangeTestRows(), 0)
		if _, _, handled := m.updateEvent(repoMsg{}); handled {
			t.Error("updateEvent took a message meant for updateResult")
		}
	})
}

// The watch event's command is what keeps the watcher read going and starts the
// wait before a reload. Update has to hand it back.
func TestAWatchEventLeavesACommandBehind(t *testing.T) {
	t.Parallel()
	m := modelForRange(t, rangeTestRows(), 0)
	m.watch.focused = true
	m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabChanges})
	if _, cmd := m.Update(watchEventMsg{}); cmd == nil {
		t.Error("a change under the repository issued no command")
	}
}

// The diff of a file listed under a stash or a worktree is read from a
// directory that is not the one the pane is bound to, so the reader of that
// diff needs both the directory and the command that fills it. Every answer
// carries the two together or neither: openNestedFile tests one of them and
// acts on both, so a pair that disagreed would open a diff with nothing to
// put in it, or throw away a command that was already going to run.
func TestANestedDiffAnswersADirectoryAndACommandTogether(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name  string
		open  func(*Model)
		paths []string
	}{
		{"the changes tab, which reads this repository", func(m *Model) {
			m.state = state.Apply(m.state, state.StatusLoaded{
				Rows: []git.Entry{{Path: "a.txt", Worktree: git.Modified}},
				Head: git.Head{Branch: "main"}})
		}, []string{"a.txt", "", "gone.txt"}},
		{"a commit's files, which are also in this repository", func(m *Model) {
			m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabHistory})
			m.state = state.Apply(m.state, state.HistoryLoaded{Commits: []git.CommitInfo{
				{SHA: "aaa", ShortSHA: "aaa", Subject: "one", Age: "1m"}}})
			m.state = state.Apply(m.state, state.CommitExpanded{SHA: "aaa",
				Files: []git.Entry{{Path: "note.txt", Index: git.Modified}}})
		}, []string{"note.txt", "", "gone.txt"}},
		{"a stash's files", func(m *Model) {
			m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabStashed})
			m.state = state.Apply(m.state, state.StashedLoaded{Stashes: []git.StashRow{
				{Ref: "stash@{0}", SHA: "s0", Message: "one", Status: git.StashApplies}}})
			m.state = state.Apply(m.state, state.StashFilesLoaded{Ref: "stash@{0}",
				Files: []git.Entry{{Path: "held.txt"}}})
		}, []string{"held.txt", "", "gone.txt"}},
		{"a worktree's files", func(m *Model) {
			m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabWorktrees})
			m.state = state.Apply(m.state, state.WorktreesLoaded{Base: "main", Here: "/repo",
				Worktrees: []git.WorktreeRow{
					{Path: "/repo", Name: "repo", Main: true, SHA: "aaa"},
					{Path: "/wt", Name: "wt", SHA: "bbb"}}})
			m.state = state.Apply(m.state, state.WorktreeFilesLoaded{Path: "/repo",
				Files: []git.Entry{{Path: "side.txt"}}})
		}, []string{"side.txt", "", "gone.txt"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			for _, path := range c.paths {
				for cursor := -1; cursor <= 4; cursor++ {
					m := modelForRange(t, nil, 0)
					c.open(m)
					m.state.Cursor = cursor
					dir, cmd := m.nestedFileDiff(path)
					if (dir == "") != (cmd == nil) {
						t.Errorf("path %q at cursor %d answered dir=%q cmd=%v",
							path, cursor, dir, cmd != nil)
					}
				}
			}
		})
	}
}

// The confirmation names the worktree it would remove, so there has to be one.
// A cursor that is not on a worktree row — the list is empty, or the cursor is
// past its end — has no row to name, and a confirmation for a worktree with no
// path asks the reader to agree to nothing.
func TestRemovingAWorktreeAsksForNothingWhenTheCursorIsOnNoRow(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name  string
		build func(*Model)
	}{
		{"the list is empty", func(m *Model) {
			m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabWorktrees})
			m.state = state.Apply(m.state, state.WorktreesLoaded{Base: "main", Here: "/repo"})
		}},
		{"the cursor is past the end", func(m *Model) {
			m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabWorktrees})
			m.state = state.Apply(m.state, state.WorktreesLoaded{Base: "main", Here: "/repo",
				Worktrees: []git.WorktreeRow{{Path: "/repo", Name: "repo", Main: true}}})
			m.state.Cursor = 9
		}},
		{"the cursor is before the start", func(m *Model) {
			m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabWorktrees})
			m.state = state.Apply(m.state, state.WorktreesLoaded{Base: "main", Here: "/repo",
				Worktrees: []git.WorktreeRow{{Path: "/repo", Name: "repo", Main: true}}})
			m.state.Cursor = -1
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := modelForRange(t, nil, 0)
			c.build(m)
			next, cmd := m.requestWorktreeRemove()
			if cmd != nil {
				t.Errorf("a row that is not there issued a command: %T", cmd())
			}
			if confirm := next.(*Model).state.Worktrees.RemoveConfirm; confirm != nil {
				t.Errorf("the pane asks to remove %+v", *confirm)
			}
		})
	}

	// A worktree the reader is not standing in can be removed, so the answers
	// above are not simply "never".
	m := modelForRange(t, nil, 0)
	m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabWorktrees})
	m.state = state.Apply(m.state, state.WorktreesLoaded{Base: "main", Here: "/repo",
		Worktrees: []git.WorktreeRow{
			{Path: "/repo", Name: "repo", Main: true},
			{Path: "/wt", Name: "wt"}}})
	m.state.Cursor = 1
	next, _ := m.requestWorktreeRemove()
	if confirm := next.(*Model).state.Worktrees.RemoveConfirm; confirm == nil {
		t.Error("a worktree that can be removed was not offered")
	} else if confirm.Path != "/wt" {
		t.Errorf("the pane asks to remove %q, want /wt", confirm.Path)
	}
}

// A stash whose refs have moved, or one waiting for the reader to agree to a
// drop, is one the pane must not read: the refs a read names have been
// renumbered by the drop, so the answer would describe another stash, and the
// question is asked again when the list is reloaded.
func TestAStashIsNotReadWhileTheListIsAboutToMove(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name  string
		build func(*Model)
		want  bool
	}{
		{"an ordinary list", func(*Model) {}, true},
		{"a drop is waiting to be agreed to", func(m *Model) {
			m.state = state.Apply(m.state, state.StashDropConfirmationShown{
				Confirm: state.StashDropConfirm{SHA: "s0", Message: "one"}})
		}, false},
		{"the refs have moved", func(m *Model) {
			m.state = state.Apply(m.state, state.StashRefsMoved{})
		}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := modelForRange(t, nil, 0)
			m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabStashed})
			m.state = state.Apply(m.state, state.StashedLoaded{Stashes: []git.StashRow{
				{Ref: "stash@{0}", SHA: "s0", Message: "one", Status: git.StashApplies}}})
			c.build(m)
			if got := m.expandStash("stash@{0}") != nil; got != c.want {
				t.Errorf("asking git = %v, want %v", got, c.want)
			}
		})
	}
}

// Reading what a stash holds costs a git call, and the two things that make the
// call pointless are asked separately: a tab that is not the stashed one is not
// showing stashes, and a row with no ref names no stash. Either alone is reason
// enough to do nothing — running git for a stash named "" asks it about every
// stash, and running it while another tab is drawn spends the call on a list
// nobody is looking at.
func TestReadingAStashNeedsBothTheTabAndTheRef(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		tab  state.Tab
		ref  string
		want bool
	}{
		{"the stashed tab and a ref", state.TabStashed, "stash@{0}", true},
		{"the stashed tab and no ref", state.TabStashed, "", false},
		{"another tab and a ref", state.TabChanges, "stash@{0}", false},
		{"another tab and no ref", state.TabChanges, "", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := modelForRange(t, nil, 0)
			m.state = state.Apply(m.state, state.TabChanged{Tab: c.tab})
			m.state = state.Apply(m.state, state.StashedLoaded{Stashes: []git.StashRow{
				{Ref: "stash@{0}", SHA: "s0", Message: "one", Status: git.StashApplies}}})
			if got := m.expandStash(c.ref) != nil; got != c.want {
				t.Errorf("asking git = %v, want %v", got, c.want)
			}
		})
	}
}

// The worktrees tab reads what a tree has changed the same way, and the two
// reasons to do nothing are the same two.
func TestReadingAWorktreeNeedsBothTheTabAndThePath(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		tab  state.Tab
		path string
		want bool
	}{
		{"the worktrees tab and a path", state.TabWorktrees, "/wt", true},
		{"the worktrees tab and no path", state.TabWorktrees, "", false},
		{"another tab and a path", state.TabChanges, "/wt", false},
		{"another tab and no path", state.TabChanges, "", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := modelForRange(t, nil, 0)
			m.state = state.Apply(m.state, state.TabChanged{Tab: c.tab})
			m.state = state.Apply(m.state, state.WorktreesLoaded{Base: "main", Here: "/repo",
				Worktrees: []git.WorktreeRow{
					{Path: "/repo", Name: "repo", Main: true},
					{Path: "/wt", Name: "wt"}}})
			if got := m.expandWorktree(c.path) != nil; got != c.want {
				t.Errorf("asking git = %v, want %v", got, c.want)
			}
		})
	}
}

// The row under the cursor is a stash only on the stashed tab, and only while
// the cursor is on one. Answering "found" with an empty row hands every caller
// a stash whose ref is the empty string, which git reads as every stash.
func TestTheStashUnderTheCursorIsOnlyFoundWhenThereIsOne(t *testing.T) {
	t.Parallel()
	withStashes := func(m *Model) {
		m.state = state.Apply(m.state, state.StashedLoaded{Stashes: []git.StashRow{
			{Ref: "stash@{0}", SHA: "s0", Message: "one", Status: git.StashApplies}}})
	}
	for _, c := range []struct {
		name   string
		build  func(*Model)
		cursor int
		want   bool
	}{
		{"the cursor is on a stash", func(m *Model) {
			m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabStashed})
			withStashes(m)
		}, 0, true},
		{"the cursor is past the end", func(m *Model) {
			m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabStashed})
			withStashes(m)
		}, 9, false},
		{"the list is empty", func(m *Model) {
			m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabStashed})
		}, 0, false},
		{"another tab is showing", func(m *Model) {
			withStashes(m)
		}, 0, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := modelForRange(t, nil, 0)
			c.build(m)
			m.state.Cursor = c.cursor
			row, ok := m.stashAtCursor()
			if ok != c.want {
				t.Errorf("found = %v, want %v", ok, c.want)
			}
			if ok && row.Ref == "" {
				t.Error("a stash with no ref was reported found")
			}
		})
	}
}

// The branch a stash was taken from is the name this offers, and it is only
// there when the cursor is on a stash. Building the name from a row that was
// not found gives "-stash": git reads a leading hyphen as the start of an
// option, so the one route that cannot fail would fail.
func TestTheNameOfferedForAStashBranchNeedsAStash(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name  string
		build func(*Model)
		want  string
	}{
		{"the cursor is on a stash taken from a branch", func(m *Model) {
			m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabStashed})
			m.state = state.Apply(m.state, state.StashedLoaded{Stashes: []git.StashRow{
				{Ref: "stash@{0}", SHA: "s0", Branch: "feature", Message: "one",
					Status: git.StashApplies}}})
		}, "feature-stash"},
		{"the cursor is on a stash with no branch", func(m *Model) {
			m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabStashed})
			m.state = state.Apply(m.state, state.StashedLoaded{Stashes: []git.StashRow{
				{Ref: "stash@{0}", SHA: "s0", Message: "one", Status: git.StashApplies}}})
		}, "stash"},
		{"the list is empty", func(m *Model) {
			m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabStashed})
		}, "stash"},
		{"another tab is showing", func(m *Model) {}, "stash"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := modelForRange(t, nil, 0)
			c.build(m)
			if got := m.stashBranchBase(); got != c.want {
				t.Errorf("the name offered is %q, want %q", got, c.want)
			}
		})
	}
}

// A file that was read and then renamed is still the file the reader read. The
// marks are kept by path, so the rename has to carry them across: without it
// the row comes back unread, and the reader is told to look again at a diff
// they have already seen. Only a renamed entry carries an old path, and
// handing the empty one to the move would ask it to rename every file at once.
func TestAReadFileStaysReadAcrossARename(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("the repo message reads the repository")
	}
	unstaged := state.WorkingTree(state.SectionUnstaged)
	hashes := []string{"h1"}
	m := syntheticModel(t)
	m.read.MarkBlock("old.txt", "h1")
	m.read.MarkFileIn(unstaged, "old.txt", hashes)
	if m.read.MarkIn(unstaged, "old.txt", hashes) != state.Read {
		t.Fatal("the file was not marked read, so this proves nothing")
	}

	next, _ := m.Update(repoMsg{Repo: git.Repo{
		Entries: []git.Entry{{Path: "new.txt", OldPath: "old.txt", Index: git.Renamed}},
		Head:    git.Head{Branch: "main"},
	}})
	m = next.(*Model)

	if got := m.read.MarkIn(unstaged, "new.txt", hashes); got != state.Read {
		t.Errorf("the file is %v under its new name, want read", got)
	}
	if got := m.read.MarkIn(unstaged, "old.txt", hashes); got == state.Read {
		t.Error("the mark stayed under the name the file no longer has")
	}
}

// The commit under the cursor is a commit only on the history tab, and only
// while the cursor is on a row that has one. Its index is what decides whether
// undo is offered, and an answer of "found" for a row that is not there hands
// the caller a commit with no sha at index -1.
func TestTheCommitUnderTheCursorIsOnlyFoundWhenThereIsOne(t *testing.T) {
	t.Parallel()
	commits := []git.CommitInfo{
		{SHA: "aaa", ShortSHA: "aaa", Subject: "one", Age: "1m"},
		{SHA: "bbb", ShortSHA: "bbb", Subject: "two", Age: "2m"},
	}
	for _, c := range []struct {
		name   string
		build  func(*Model)
		cursor int
		want   bool
	}{
		{"the cursor is on the tip", func(m *Model) {
			m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabHistory})
			m.state = state.Apply(m.state, state.HistoryLoaded{Commits: commits})
		}, 0, true},
		{"the cursor is past the end", func(m *Model) {
			m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabHistory})
			m.state = state.Apply(m.state, state.HistoryLoaded{Commits: commits})
		}, 9, false},
		{"the log is empty", func(m *Model) {
			m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabHistory})
		}, 0, false},
		{"another tab is showing", func(m *Model) {
			m.state = state.Apply(m.state, state.HistoryLoaded{Commits: commits})
		}, 0, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := modelForRange(t, nil, 0)
			c.build(m)
			m.state.Cursor = c.cursor
			commit, index, ok := m.currentCommitAt()
			if ok != c.want {
				t.Errorf("found = %v, want %v", ok, c.want)
			}
			if ok && commit.SHA == "" {
				t.Error("a commit with no sha was reported found")
			}
			if !ok && index != -1 {
				t.Errorf("nothing was found and the index is %d, want -1", index)
			}
		})
	}
}

// What a stash offers depends on what it would do: one that applies cleanly is
// restored, one that would conflict is taken to a branch instead, and one whose
// verdict is not in yet offers neither. Both halves of the check matter — the
// cursor has to be on a stash, and that stash has to offer the verb — because
// running the other verb reaches git with a stash it cannot take.
func TestAStashVerbRunsOnlyWhereTheStashOffersIt(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name    string
		status  git.StashStatus
		noStash bool
		restore bool
		branch  bool
	}{
		{"a stash that applies", git.StashApplies, false, true, false},
		{"a stash that would conflict", git.StashConflicts, false, false, true},
		{"a stash from an unrelated history", git.StashUnrelated, false, false, true},
		{"a stash with no verdict yet", git.StashUnknown, false, false, false},
		{"no stash under the cursor", git.StashApplies, true, false, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			build := func() *Model {
				m := modelForRange(t, nil, 0)
				m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabStashed})
				if !c.noStash {
					m.state = state.Apply(m.state, state.StashedLoaded{
						Stashes: []git.StashRow{{Ref: "stash@{0}", SHA: "s0",
							Message: "one", Status: c.status}}})
				}
				return m
			}
			if _, cmd := build().requestStashRestore(); (cmd != nil) != c.restore {
				t.Errorf("restore issued a command = %v, want %v", cmd != nil, c.restore)
			}
			if _, cmd := build().requestStashBranch(); (cmd != nil) != c.branch {
				t.Errorf("branch issued a command = %v, want %v", cmd != nil, c.branch)
			}
			// Drop is offered by both verdicts and by neither when the cursor
			// is on no stash; it asks before it runs, so the confirmation is
			// what says it was offered.
			next, _ := build().requestStashDrop()
			wantDrop := !c.noStash && c.status != git.StashUnknown
			if got := next.(*Model).state.Stashed.DropConfirm != nil; got != wantDrop {
				t.Errorf("drop asked = %v, want %v", got, wantDrop)
			}
		})
	}
}

// A file listed under a worktree is read from that worktree's index when the
// working tree there holds nothing more, and from its working tree otherwise.
// All three parts of that decide it: a file with work on both sides is read
// from the side the reader can still change, and a conflicted index is not a
// staged file at all.
func TestWhichSideAWorktreeFileIsReadFrom(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		e    git.Entry
		want bool
	}{
		{"staged and nothing else", git.Entry{Path: "a.txt", Index: git.Modified}, true},
		{"added to the index", git.Entry{Path: "a.txt", Index: git.Added}, true},
		{"changed in the working tree", git.Entry{Path: "a.txt",
			Worktree: git.Modified}, false},
		{"changed on both sides", git.Entry{Path: "a.txt",
			Index: git.Modified, Worktree: git.Modified}, false},
		{"untracked", git.Entry{Path: "a.txt", Worktree: git.Untracked}, false},
		{"conflicted", git.Entry{Path: "a.txt",
			Index: git.Unmerged, Worktree: git.Unmerged}, false},
		{"conflicted with a settled working tree", git.Entry{Path: "a.txt",
			Index: git.Unmerged}, false},
		{"nothing at all", git.Entry{Path: "a.txt"}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			row := state.FileRow(c.e, state.SectionNone)
			if got := worktreeFileStaged(row); got != c.want {
				t.Errorf("read from the index = %v, want %v", got, c.want)
			}
		})
	}
}

// Which side a file opens from is decided by the row under the cursor, because
// a path staged and then edited again has a row on each side holding different
// diffs. The row has to be that file's: a row for another file answers for
// another file, and a commit row's path is a sha, which is not a file at all.
func TestTheSideAFileOpensFromComesFromItsOwnRow(t *testing.T) {
	t.Parallel()
	rows := []state.Row{
		state.SectionHeadingRow(state.SectionStaged),
		state.FileRow(git.Entry{Path: "a.txt", Index: git.Modified}, state.SectionStaged),
		state.SectionHeadingRow(state.SectionUnstaged),
		state.FileRow(git.Entry{Path: "b.txt", Worktree: git.Modified}, state.SectionUnstaged),
	}
	for _, c := range []struct {
		name   string
		cursor int
		path   string
		want   state.Section
	}{
		{"the cursor is on the file's staged row", 1, "a.txt", state.SectionStaged},
		{"the cursor is on the file's unstaged row", 3, "b.txt", state.SectionUnstaged},
		{"the cursor is on another file's staged row", 1, "b.txt", state.SectionUnstaged},
		{"the cursor is on a heading", 0, "a.txt", state.SectionUnstaged},
		{"the cursor is on no row", 9, "a.txt", state.SectionUnstaged},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := modelForRange(t, rows, c.cursor)
			if got := m.sectionFor(c.path); got != c.want {
				t.Errorf("%q opens from %v, want %v", c.path, got, c.want)
			}
		})
	}

	// A commit row carries a sha where a file row carries a path, and the sha
	// is not a side of the working tree.
	commit := git.CommitInfo{SHA: "abc", ShortSHA: "abc", Subject: "one"}
	m := modelForRange(t, []state.Row{state.CommitRow(commit)}, 0)
	if got := m.sectionFor(commit.SHA); got != state.SectionUnstaged {
		t.Errorf("a commit row answered %v, want the unstaged side", got)
	}
}

// A diff takes rows from the list only when the list leaves it none. A pane
// that holds the list with rows to spare gives the diff what is left, which is
// more than the six rows a peek keeps; a list longer than the pane leaves
// nothing, and the only way to draw a diff there is to shorten the list.
//
// The answer is about the list and the pane, not about how the reader arrived:
// asking it only where the key is pressed dropped it on the next cursor move,
// and a repository with more changed files than rows showed a diff once and
// then never again.
func TestADiffTakesRoomFromTheListOnlyWhenTheListLeavesNone(t *testing.T) {
	t.Parallel()
	short := []state.Row{
		state.SectionHeadingRow(state.SectionUnstaged),
		state.FileRow(git.Entry{Path: "a.txt", Worktree: git.Modified}, state.SectionUnstaged),
		state.FileRow(git.Entry{Path: "b.txt", Worktree: git.Modified}, state.SectionUnstaged),
	}
	long := append([]state.Row{state.SectionHeadingRow(state.SectionUnstaged)},
		manyFileRows(60)...)

	for _, c := range []struct {
		name string
		rows []state.Row
		want bool
	}{
		{"a list the pane holds", short, false},
		{"a list longer than the pane", long, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := modelForRange(t, c.rows, 1)
			m.state.Height = 30
			next, _ := m.open(c.rows[1].Path())
			m = next.(*Model)
			if m.state.Open.Peek != c.want {
				t.Errorf("the diff took room from the list = %v, want %v",
					m.state.Open.Peek, c.want)
			}

			// The cursor moving onto another file keeps the room the open diff
			// has, whichever way it was split.
			next, _ = m.Update(tea.KeyPressMsg{Code: 'j'})
			if got := next.(*Model).state.Open.Peek; got != c.want {
				t.Errorf("moving the cursor changed the room to %v, want %v", got, c.want)
			}

			// A diff the reader closed reopens the way they left it: the list
			// has the pane back, and opening the same file again has to take
			// the rows a second time.
			path := c.rows[1].Path()
			m.state = state.Apply(m.state, state.DiffClosed{For: path})
			next, _ = m.open(path)
			if !next.(*Model).state.Open.Peek {
				t.Error("reopening the diff that was closed did not take room from the list")
			}

			// Another file, after that close, is an ordinary open.
			m.state = state.Apply(m.state, state.DiffClosed{For: path})
			other := c.rows[len(c.rows)-1].Path()
			next, _ = m.open(other)
			if got := next.(*Model).state.Open.Peek; got != c.want {
				t.Errorf("opening %q after a close of %q took room = %v, want %v",
					other, path, got, c.want)
			}
		})
	}
}

func manyFileRows(n int) []state.Row {
	rows := make([]state.Row, n)
	for i := range rows {
		rows[i] = state.FileRow(
			git.Entry{Path: fmt.Sprintf("f%02d.txt", i), Worktree: git.Modified},
			state.SectionUnstaged)
	}
	return rows
}
