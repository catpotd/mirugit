package layout

import (
	"strings"
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// The characters mirugit paints are marks it draws itself: the dot that says a
// row is unread, the bar that is the cursor, the box that says a row is
// selected, the words that say what a stash or a tree is doing. A repository is
// free to use the same characters in a file name, a branch, a commit subject or
// a stash message.
//
// Painting one of those says something about the row that is not true, and the
// checks that compare the colored frame with the plain one cannot see it: the
// text is the same either way. The names below carry one painted character
// each, and each has to come out of the frame with nothing of ours inside it.
var paintedCharacterNames = []string{
	"a·b.txt", "c●d.txt", "e█f.txt", "g▌h.txt", "i[✓]j.txt",
	"conflicts.txt", "merges.txt", "applies.txt", "k+12l.txt", "m−3n.txt",
	"s stage.txt",
}

func paintedCharacterState(tab state.Tab) state.State {
	entries := make([]git.Entry, 0, len(paintedCharacterNames))
	for _, name := range paintedCharacterNames {
		entries = append(entries, git.Entry{Path: name, Worktree: git.Modified,
			WorktreeCount: git.Count{Added: 12, Deleted: 3}})
	}
	s := state.State{Width: paneWidth, Height: 40, Tab: state.TabChanges}
	s = state.Apply(s, state.StatusLoaded{Rows: entries, Head: git.Head{Branch: "a·b"}})
	s = state.Apply(s, state.HistoryLoaded{Commits: []git.CommitInfo{
		{SHA: "aaa1234", ShortSHA: "aaa1234", Subject: "a·b ● merges +12 aaa1234",
			Author: "c●d", Age: "3d"}}})
	s = state.Apply(s, state.StashedLoaded{Stashes: []git.StashRow{
		{Ref: "stash@{0}", SHA: "s0", Message: "a·b ● merges applies conflicts",
			Branch: "m−3n", FileCount: 2, Status: git.StashApplies}}})
	s = state.Apply(s, state.WorktreesLoaded{Base: "main", Here: "/a",
		Worktrees: []git.WorktreeRow{
			{Path: "/a", Name: "a·b", Branch: "merges", Main: true},
			{Path: "/b", Name: "c●d", Branch: "conflicts", Dirty: 2, WroteAge: "3d"}}})
	return state.Apply(s, state.TabChanged{Tab: tab})
}

func TestNoColorOfOursLandsOnTextFromTheRepository(t *testing.T) {
	t.Parallel()
	w := Renderer{Palette: Palette{Enabled: true, Theme: cozmic}}
	fromRepository := append(append([]string{}, paintedCharacterNames...),
		"a·b ● merges", "a·b ● merges applies conflicts", "aaa1234", "m−3n")

	for tab := range state.TabCount {
		s := paintedCharacterState(tab)
		frames := map[string]Frame{"the list": Pane(s, nil, w)}
		if tab == state.TabChanges {
			open := state.Apply(s, state.DiffOpened{Path: "a·b.txt",
				Origin: state.WorkingTree(state.SectionUnstaged)})
			open = state.Apply(open, state.DiffLoaded{Diff: git.FileDiff{
				Path: "a·b.txt",
				Blocks: []git.Block{{Header: "@@ -1 +1 @@ a·b", Hash: "h1",
					Lines: []string{"+a·b ● merges", "-m−3n"}}}}})
			frames["an open diff"] = Pane(open, nil, w)
		}
		for what, f := range frames {
			for _, text := range fromRepository {
				for i, line := range f.Lines {
					// Drawn whole on this row, and not drawn whole once the
					// escapes are counted as part of it: something of ours was
					// written inside it.
					if strings.Contains(stripSGR(line), text) && !strings.Contains(line, text) {
						t.Errorf("%s, %s, row %d: %q carries a color of ours\n%q",
							state.Facts[tab].Name, what, i, text, line)
					}
				}
			}
		}
	}
}
