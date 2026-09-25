package layout

import (
	"strings"
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// readMarks answers Read for the one key it is built with and Unread for the
// rest, so a test can say which row it expects the mark on.
type readMarks struct {
	stashSHA     string
	worktreePath string
	worktreeSHA  string
}

func (readMarks) MarkIn(state.Origin, string, []string) state.Mark { return state.Unread }
func (m readMarks) StashMark(sha string) state.Mark {
	if sha != "" && sha == m.stashSHA {
		return state.Read
	}
	return state.Unread
}
func (m readMarks) WorktreeMark(path, sha string) state.Mark {
	if path == m.worktreePath && sha == m.worktreeSHA && sha != "" {
		return state.Read
	}
	return state.Unread
}
func (readMarks) BlockRead(string, string) bool          { return false }
func (readMarks) HashesIn(state.Origin, string) []string { return nil }

// The unread dot is the whole reason the read marks exist: it says which row
// still holds something the reader has not seen. Nothing checked that the
// stashed and worktrees tabs draw it, and a mutation of the comparison in
// pane_worktree.go survived the suite.
func TestTheUnreadMarkFollowsTheMarksOnStashAndWorktree(t *testing.T) {
	t.Parallel()
	const mark = "·"

	t.Run("worktrees", func(t *testing.T) {
		t.Parallel()
		trees := []git.WorktreeRow{
			{Path: "/repo", Name: "repo", Main: true, SHA: "aaa"},
			{Path: "/wt", Name: "wt", SHA: "bbb"},
		}
		s := state.State{Width: 100, Height: 30, Tab: state.TabChanges}
		s = state.Apply(s, state.WorktreesLoaded{Worktrees: trees, Base: "main"})
		s = state.Apply(s, state.TabChanged{Tab: state.TabWorktrees})

		unread, _ := worktreeListOf(s, readMarks{}, Renderer{}, everyLine)
		read, _ := worktreeListOf(s, readMarks{worktreePath: "/wt", worktreeSHA: "bbb"}, Renderer{}, everyLine)
		countMarks(t, "worktrees", unread, read, mark)
	})

	t.Run("stashed", func(t *testing.T) {
		t.Parallel()
		stashes := []git.StashRow{
			{Ref: "stash@{0}", SHA: "s0", Message: "one", Status: git.StashApplies},
			{Ref: "stash@{1}", SHA: "s1", Message: "two", Status: git.StashApplies},
		}
		s := state.State{Width: 100, Height: 30, Tab: state.TabChanges}
		s = state.Apply(s, state.StashedLoaded{Stashes: stashes})
		s = state.Apply(s, state.TabChanged{Tab: state.TabStashed})

		unread, _ := stashListOf(s, readMarks{}, Renderer{}, everyLine)
		read, _ := stashListOf(s, readMarks{stashSHA: "s0"}, Renderer{}, everyLine)
		countMarks(t, "stashed", unread, read, mark)
	})
}

func countMarks(t *testing.T, name string, unread, read []string, mark string) {
	t.Helper()
	before := strings.Count(strings.Join(unread, "\n"), mark)
	after := strings.Count(strings.Join(read, "\n"), mark)
	if before == 0 {
		t.Fatalf("%s: no unread mark drawn at all\n%s", name, strings.Join(unread, "\n"))
	}
	if after >= before {
		t.Errorf("%s: marking one row read left %d marks, was %d\n%s",
			name, after, before, strings.Join(read, "\n"))
	}
}
