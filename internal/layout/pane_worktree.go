package layout

import (
	"github.com/catpotd/mirugit/internal/state"
)

func worktreePane(s state.State, r ReadMarks, w Renderer, f Frame) Frame {
	heading, _ := w.Heading(state.Facts[state.TabWorktrees].Name, len(s.Worktrees.List), 0, 0, state.SectionNone, "", "vs "+s.Worktrees.Base, s.Width)
	f.Lines = append(f.Lines, heading)
	return finishListPane(s, r, w, f, func(win lineWindow) ([]string, []Region) {
		return renderFor(s.Tab).List(s, r, w, win)
	})
}

func worktreeListOf(s state.State, r ReadMarks, w Renderer, win lineWindow) ([]string, []Region) {
	var lines []string
	var regions []Region
	for i, row := range s.Rows {
		switch row.Kind() {
		case state.RowWorktree:
			if !win.draws(len(lines)) {
				lines, regions = appendHiddenRow(win, lines, regions,
					Target{Kind: TargetFile, Path: row.Worktree().Path, Row: i})
				continue
			}
			unread := marksPresent(r) && row.Worktree().SHA != "" &&
				r.WorktreeMark(row.Worktree().Path, row.Worktree().SHA) == state.Unread
			verbs := worktreeRowVerbs(s, row)
			line, rowRegions := w.WorktreeRow(worktreeRowLayout{
				Info: row.Worktree(),
				State: worktreeRowState{
					Cursor: i == s.Cursor,
					Unread: unread,
					Here:   row.Worktree().Path == s.Worktrees.Here,
				},
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
			lr := worktreeLayoutFileRow(s, row, i)
			line, rowRegions := w.FileRow(lr, s.Width)
			rowIdx := len(lines)
			lines = append(lines, line)
			for _, reg := range rowRegions {
				reg.Row = rowIdx
				regions = append(regions, reg)
			}
		case state.RowCommit, state.RowStash, state.RowSectionHeading, state.RowDirectory:
			// The worktrees tab holds trees and the files each has changed.

		}
	}
	return lines, regions
}

func worktreeLayoutFileRow(s state.State, row state.Row, index int) Row {
	return Row{
		Entry:   row.Entry(),
		Count:   row.Entry().WorktreeCount,
		Index:   index,
		Section: row.Section(),
		State: rowState{
			Cursor:   index == s.Cursor,
			Read:     state.Read,
			Indent:   1,
			NoSelect: true,
			FullPath: true,
		},
		Verbs: worktreeFileVerbs(s),
	}
}

func worktreeFileVerbs(s state.State) []state.VerbName {
	if !state.CursorRowVerbsAllowed(s) {
		return nil
	}
	return []state.VerbName{state.VerbNameDiff}
}

func worktreeRowVerbs(s state.State, row state.Row) []state.VerbName {
	if !state.CursorRowVerbsAllowed(s) {
		return nil
	}
	return state.WorktreeVerbs(row.Worktree(), row.Worktree().Path == s.Worktrees.Here)
}
