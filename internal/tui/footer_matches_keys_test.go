package tui

import (
	"strings"
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
)

// The footer is the contract: a key that runs from a row has a word for it on
// that row's footer. On the three tabs whose rows expand into files, the keys
// act on the parent from a file row while the footer named only the file's own
// verb. On the stashed tab that meant x dropped a stash from a row whose footer
// read "d diff" — a destructive key with nothing on screen offering it.
//
// The three tabs are driven from one table, and every row of each is checked,
// because the difference only shows on the rows below the first.
func TestEveryKeyTheFooterHidesIsOneThatCannotRun(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		open func(*Model)
		// wantOnFileRow is the verb the parent offers that the file row must
		// also name, since the key reaches the parent from there.
		wantOnFileRow state.VerbName
	}{
		{
			name: "history",
			open: func(m *Model) {
				m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabHistory})
				m.state = state.Apply(m.state, state.HistoryLoaded{Commits: []git.CommitInfo{
					{SHA: "aaa", ShortSHA: "aaa", Subject: "tip", Age: "1m", Unpushed: true},
					{SHA: "bbb", ShortSHA: "bbb", Subject: "old", Age: "2m"},
				}})
				m.state = state.Apply(m.state, state.CommitExpanded{SHA: "aaa",
					Files: []git.Entry{{Path: "a.txt", Index: git.Modified}}})
			},
			wantOnFileRow: state.VerbNameUncommit,
		},
		{
			name: "stashed",
			open: func(m *Model) {
				m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabStashed})
				m.state = state.Apply(m.state, state.StashedLoaded{Stashes: []git.StashRow{
					{Ref: "stash@{0}", SHA: "s0", Message: "one", Status: git.StashApplies},
				}})
				m.state = state.Apply(m.state, state.StashFilesLoaded{Ref: "stash@{0}",
					Files: []git.Entry{{Path: "a.txt"}}})
			},
			wantOnFileRow: state.VerbNameDrop,
		},
		{
			name: "worktrees",
			open: func(m *Model) {
				m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabWorktrees})
				m.state = state.Apply(m.state, state.WorktreesLoaded{Base: "main", Here: "/repo",
					Worktrees: []git.WorktreeRow{
						{Path: "/repo", Name: "repo", Main: true, SHA: "aaa"},
						{Path: "/wt", Name: "wt", SHA: "bbb"},
					}})
				// The tree the reader is already in offers neither go nor
				// remove, so the one below it is the one with verbs to hide.
				m.state.Cursor = 1
				m.state = state.Apply(m.state, state.WorktreeFilesLoaded{Path: "/wt",
					Files: []git.Entry{{Path: "a.txt"}}})
			},
			wantOnFileRow: state.VerbNameGo,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := newParentModel(t)
			m.render.Clipboard, m.render.Browser = true, true
			c.open(m)

			fileRow := -1
			for i, r := range m.state.Rows {
				if r.Kind() == state.RowFile {
					fileRow = i
					break
				}
			}
			if fileRow < 1 {
				t.Fatalf("no file row under a parent: %d rows", len(m.state.Rows))
			}

			key, ok := layout.FooterKeyFor(c.wantOnFileRow)
			if !ok {
				t.Fatalf("%s has no key, so this proves nothing", c.wantOnFileRow)
			}

			m.state.Cursor = fileRow - 1
			onParent := footerLine(t, m)
			if !strings.Contains(onParent, key+" "+c.wantOnFileRow.String()) {
				t.Fatalf("the parent row does not offer %s either, so this proves nothing:\n%s",
					c.wantOnFileRow, onParent)
			}

			m.state.Cursor = fileRow
			onFile := footerLine(t, m)
			if !strings.Contains(onFile, key+" "+c.wantOnFileRow.String()) {
				t.Errorf("%s runs from the file row and the footer does not name it:\n%s",
					c.wantOnFileRow, onFile)
			}
			if !strings.Contains(onFile, "d "+state.VerbNameDiff.String()) {
				t.Errorf("the file row lost its own verb:\n%s", onFile)
			}
		})
	}
}

func footerLine(t *testing.T, m *Model) string {
	t.Helper()
	m.View()
	if len(m.frame.Lines) == 0 {
		t.Fatal("nothing was drawn")
	}
	return m.frame.Lines[len(m.frame.Lines)-1]
}
