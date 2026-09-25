package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
)

// A file that is staged and edited again is two rows, one in each section, and
// selecting both selects one file. Handing the verb that file twice ran git
// against the same path twice and the notice counted it as two.
func TestAPathSelectedOnBothSidesIsHandedToTheVerbOnce(t *testing.T) {
	t.Parallel()
	both := git.Entry{Path: "a.txt", Index: git.Modified, Worktree: git.Modified}
	rows := []state.Row{
		state.SectionHeadingRow(state.SectionUnstaged),
		state.FileRow(both, state.SectionUnstaged),
		state.SectionHeadingRow(state.SectionStaged),
		state.FileRow(both, state.SectionStaged),
	}
	m := modelForRange(t, rows, 1)
	m.state = state.Apply(m.state, state.SelectionToggled{Path: "a.txt", Section: state.SectionUnstaged})
	m.state = state.Apply(m.state, state.SelectionToggled{Path: "a.txt", Section: state.SectionStaged})
	if n := len(m.state.Changes.Selected); n != 2 {
		t.Fatalf("%d rows are selected, want both sides of the file", n)
	}

	entries := m.verbEntries(state.VerbDiscard)
	if len(entries) != 1 {
		t.Fatalf("the verb is handed %d entries, want 1: %+v", len(entries), entries)
	}
	if entries[0].Path != "a.txt" {
		t.Errorf("the entry is %q, want a.txt", entries[0].Path)
	}
}

// The confirmation says how many lines a discard removes, and each row counts
// the side it is on: the staged row counts what is staged, the unstaged row
// what the working tree holds. Reading the other side's numbers shows a reader
// a count that belongs to the half they are not discarding.
func TestTheDiscardConfirmationCountsEachRowOnItsOwnSide(t *testing.T) {
	t.Parallel()
	entry := git.Entry{
		Path:          "a.txt",
		Index:         git.Modified,
		Worktree:      git.Modified,
		IndexCount:    git.Count{Added: 5, Deleted: 1},
		WorktreeCount: git.Count{Added: 20, Deleted: 7},
	}
	for _, c := range []struct {
		name           string
		section        state.Section
		added, deleted int
	}{
		{"a row in the staged section", state.SectionStaged, 5, 1},
		{"a row in the unstaged section", state.SectionUnstaged, 20, 7},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := state.State{Rows: []state.Row{
				state.SectionHeadingRow(c.section),
				state.FileRow(entry, c.section),
			}}
			got := discardConfirmOf(s, []git.Entry{entry})
			if got.Files != 1 {
				t.Errorf("the confirmation names %d files, want 1", got.Files)
			}
			if got.Added != c.added || got.Deleted != c.deleted {
				t.Errorf("the confirmation says +%d −%d, want +%d −%d",
					got.Added, got.Deleted, c.added, c.deleted)
			}
		})
	}
}

// The confirmation holds the paths, and the discard that follows it needs the
// entries those paths name. A file staged and edited again has a row on each
// side, and handing both to the discard runs git against the same path twice:
// the second run has nothing left to remove and reports a failure for a discard
// that worked.
func TestAPathOnBothSidesBecomesOneEntryToDiscard(t *testing.T) {
	t.Parallel()
	both := git.Entry{Path: "a.txt", Index: git.Modified, Worktree: git.Modified}
	rows := []state.Row{
		state.SectionHeadingRow(state.SectionStaged),
		state.FileRow(both, state.SectionStaged),
		state.SectionHeadingRow(state.SectionUnstaged),
		state.FileRow(both, state.SectionUnstaged),
	}
	m := modelForRange(t, rows, 1)

	entries := m.entriesForPaths([]string{"a.txt"})
	if len(entries) != 1 {
		t.Fatalf("the discard is handed %d entries, want 1: %+v", len(entries), entries)
	}
	if entries[0].Path != "a.txt" {
		t.Errorf("the entry is %q, want a.txt", entries[0].Path)
	}

	// A path no row carries names nothing to discard.
	if got := m.entriesForPaths([]string{"gone.txt"}); len(got) != 0 {
		t.Errorf("a path the list does not hold answered with %+v", got)
	}
}

// Shift and a click extend the selection, and a selection is over files. A
// commit row carries no selection, so the shift is nothing but a click there
// and the commit opens. Taking the other branch opens a diff under the reader's
// shift-click and leaves a commit they clicked closed.
func TestShiftClickExtendsOnAFileAndOpensOnACommit(t *testing.T) {
	t.Parallel()
	rows := []state.Row{
		state.SectionHeadingRow(state.SectionUnstaged),
		state.FileRow(git.Entry{Path: "a.txt", Worktree: git.Modified}, state.SectionUnstaged),
		state.FileRow(git.Entry{Path: "b.txt", Worktree: git.Modified}, state.SectionUnstaged),
	}
	m := modelForRange(t, rows, 1)
	next, _ := m.fileClick(layout.Target{Kind: layout.TargetFile, Path: "b.txt", Row: 2},
		tea.ModShift)
	got := next.(*Model).state
	if got.Open.Path != "" {
		t.Errorf("a shift-click on a file opened %q", got.Open.Path)
	}
	if len(got.Changes.Selected) == 0 {
		t.Error("a shift-click on a file selected nothing")
	}

	// A commit row on the history tab has no selection to extend.
	h := historyModel(t, []git.CommitInfo{
		{SHA: "aaa", ShortSHA: "aaa", Subject: "one"},
		{SHA: "bbb", ShortSHA: "bbb", Subject: "two"},
	}, "", nil, 0)
	if h.state.Rows[1].Kind() != state.RowCommit {
		t.Fatalf("row 1 is %v, want a commit row", h.state.Rows[1].Kind())
	}
	_, cmd := h.fileClick(layout.Target{Kind: layout.TargetFile, Row: 1, Commit: "bbb"},
		tea.ModShift)
	if cmd == nil {
		t.Error("a shift-click on a commit did not open it")
	}
}
