package layout

import (
	"github.com/catpotd/mirugit/internal/state"
)

func stashPane(s state.State, r ReadMarks, w Renderer, f Frame) Frame {
	heading, _ := w.Heading(state.Facts[state.TabStashed].Name, len(s.Stashed.Stashes), 0, 0, state.SectionNone, "", "", s.Width)
	f.Lines = append(f.Lines, heading)
	return finishListPane(s, r, w, f, func(win lineWindow) ([]string, []Region) {
		return renderFor(s.Tab).List(s, r, w, win)
	})
}

func stashListOf(s state.State, r ReadMarks, w Renderer, win lineWindow) ([]string, []Region) {
	var lines []string
	var regions []Region
	for i, row := range s.Rows {
		switch row.Kind() {
		case state.RowStash:
			if !win.draws(len(lines)) {
				lines, regions = appendHiddenRow(win, lines, regions,
					Target{Kind: TargetFile, Path: row.Stash().Ref, Row: i})
				continue
			}
			unread := marksPresent(r) && r.StashMark(row.Stash().SHA) == state.Unread
			verbs := stashRowVerbs(s, row)
			line, rowRegions := w.StashRow(stashRowLayout{
				Info:  row.Stash(),
				State: stashRowState{Cursor: i == s.Cursor, Unread: unread},
				Verbs: verbs,
				Index: i,
			}, s.Width)
			rowIdx := len(lines)
			lines = append(lines, line)
			for _, reg := range rowRegions {
				reg.Row = rowIdx
				regions = append(regions, reg)
			}
		case state.RowFile:
			if !win.draws(len(lines)) {
				lines, regions = appendHiddenRow(win, lines, regions,
					Target{Kind: TargetFile, Path: row.Path(), Row: i})
				continue
			}
			lr := stashLayoutFileRow(s, row, i)
			line, rowRegions := w.FileRow(lr, s.Width)
			rowIdx := len(lines)
			lines = append(lines, line)
			for _, reg := range rowRegions {
				reg.Row = rowIdx
				regions = append(regions, reg)
			}
		case state.RowCommit, state.RowWorktree, state.RowSectionHeading, state.RowDirectory:
			// The stashed tab holds stashes and the files inside them; no other
			// row kind is built for it.

		}
	}
	return lines, regions
}

func stashLayoutFileRow(s state.State, row state.Row, index int) Row {
	// The stash the file is printed under is what knows where it would land.
	// Scanning the file list for a flag copied onto each file answered the same
	// question from a second place.
	conflicts := false
	if parent, _, ok := state.ParentAbove(s.Rows, index, state.RowStash); ok {
		conflicts = parent.Stash().CollidesWith(row.Path())
	}
	return Row{
		Entry:   row.Entry(),
		Count:   row.Entry().WorktreeCount,
		Index:   index,
		Section: row.Section(),
		State: rowState{
			Cursor:    index == s.Cursor,
			Read:      state.Read,
			Indent:    1,
			NoSelect:  true,
			Conflicts: conflicts,
			FullPath:  true,
		},
		Verbs: stashFileVerbs(s),
	}
}

func stashFileVerbs(s state.State) []state.VerbName {
	if !state.CursorRowVerbsAllowed(s) {
		return nil
	}
	return []state.VerbName{state.VerbNameDiff}
}

func stashRowVerbs(s state.State, row state.Row) []state.VerbName {
	if !state.CursorRowVerbsAllowed(s) {
		return nil
	}
	return state.StashVerbs(row.Stash().Status)
}
