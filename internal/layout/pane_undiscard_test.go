package layout

import (
	"strings"
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

func TestRowVerbsHideDuringUndiscardConfirmOnEveryTab(t *testing.T) {
	t.Parallel()
	confirm := &state.UndiscardConfirm{Undo: git.Undo{Ref: "refs/mirugit/undo/1"}, Files: 1}
	w := Renderer{}

	t.Run("changes", func(t *testing.T) {
		row := state.FileRow(

			git.Entry{Path: "a.txt", Worktree: git.Modified}, state.SectionUnstaged)

		s := state.State{Tab: state.TabChanges, Cursor: 0, Rows: []state.Row{row}, Changes: state.Changes{UndiscardConfirm: confirm}}
		if got := rowVerbs(s, row, 0); got != nil {
			t.Fatalf("got %v during undiscard confirm, want nil", got)
		}
	})

	t.Run("history", func(t *testing.T) {
		commit := git.CommitInfo{SHA: "abc", Subject: "one", Unpushed: true}
		s := state.State{Tab: state.TabHistory, Cursor: 0, Width: paneWidth, Height: 20, Rows: []state.Row{state.CommitRow(commit)}, History: state.History{Commits: []git.CommitInfo{commit}}, Changes: state.Changes{UndiscardConfirm: confirm}}
		if got := historyRowVerbs(s, s.Rows[0], 0, w); got != nil {
			t.Fatalf("got %v during undiscard confirm, want nil", got)
		}
		f := Pane(s, nil, w)
		body := strings.Join(f.Lines, "\n")
		if strings.Contains(body, "d diff") || strings.Contains(body, "y sha") {
			t.Fatal("history row should hide verbs during undiscard confirm")
		}
		if !strings.Contains(f.Lines[len(f.Lines)-1], "y restore") {
			t.Fatal("footer should still show undiscard prompt")
		}
	})

	t.Run("stashed", func(t *testing.T) {
		stash := git.StashRow{Ref: "stash@{0}", Message: "hold", Status: git.StashApplies}
		s := state.State{Tab: state.TabStashed, Cursor: 0, Rows: []state.Row{state.StashRowOf(stash)}, Stashed: state.Stashed{Stashes: []git.StashRow{stash}}, Changes: state.Changes{UndiscardConfirm: confirm}}
		if got := stashRowVerbs(s, s.Rows[0]); got != nil {
			t.Fatalf("got %v during undiscard confirm, want nil", got)
		}
	})

	t.Run("worktrees", func(t *testing.T) {
		tree := git.WorktreeRow{Path: "/repo/wt", Name: "wt", Branch: "feat", Dirty: 0}
		main := git.WorktreeRow{Path: "/repo", Name: "main", Main: true}
		s := state.State{Tab: state.TabWorktrees, Cursor: 0, Width: paneWidth, Height: 20, Rows: []state.Row{state.WorktreeRowOf(tree)}, Worktrees: state.Worktrees{List: []git.WorktreeRow{main, tree}}, Changes: state.Changes{UndiscardConfirm: confirm}}
		if got := worktreeRowVerbs(s, s.Rows[0]); got != nil {
			t.Fatalf("got %v during undiscard confirm, want nil", got)
		}
		f := Pane(s, nil, w)
		body := strings.Join(f.Lines, "\n")
		if strings.Contains(body, "g go") || strings.Contains(body, "x remove") {
			t.Fatal("worktree row should hide verbs during undiscard confirm")
		}
	})
}
