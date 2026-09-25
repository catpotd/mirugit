package layout

import (
	"github.com/catpotd/mirugit/internal/state"
)

func historyPane(s state.State, r ReadMarks, w Renderer, f Frame) Frame {
	right := ""
	if s.History.LoadingMore {
		right = "loading"
	} else if s.History.HasMore {
		right = "j more"
	}
	heading, _ := w.Heading(state.Facts[state.TabHistory].Name, len(s.History.Commits), 0, 0, state.SectionNone, "", right, s.Width)
	f.Lines = append(f.Lines, heading)
	return finishListPane(s, r, w, f, func(win lineWindow) ([]string, []Region) {
		return renderFor(s.Tab).List(s, r, w, win)
	})
}

func historyListOf(s state.State, w Renderer, win lineWindow) ([]string, []Region) {
	var lines []string
	var regions []Region
	commitIndex := 0
	for i, row := range s.Rows {
		switch row.Kind() {
		case state.RowCommit:
			if !win.draws(len(lines)) {
				lines, regions = appendHiddenRow(win, lines, regions,
					// The same kind the drawn row carries: TargetCommit is the
					// commit button in the header, and CursorLineInList reads
					// only the kinds that are rows.
					Target{Kind: TargetFile, Commit: row.Commit().SHA, Row: i})
				commitIndex++
				continue
			}
			verbs := historyRowVerbs(s, row, commitIndex, w)
			line, rowRegions := w.CommitRow(commitRowLayout{
				Info:  row.Commit(),
				State: commitRowState{Cursor: i == s.Cursor},
				Verbs: verbs,
				Index: i,
			}, s.Width)
			rowIdx := len(lines)
			lines = append(lines, line)
			for _, reg := range rowRegions {
				reg.Row = rowIdx
				regions = append(regions, reg)
			}
			commitIndex++
		case state.RowFile:
			if !win.draws(len(lines)) {
				lines, regions = appendHiddenRow(win, lines, regions,
					Target{Kind: TargetFile, Path: row.Path(), Row: i})
				continue
			}
			lr := Row{
				Entry:   row.Entry(),
				Count:   row.Entry().WorktreeCount,
				Index:   i,
				Section: row.Section(),
				State:   rowState{Cursor: i == s.Cursor, Read: state.Read, NoSelect: true, Indent: 1},
				Verbs:   historyFileVerbs(s),
			}
			line, rowRegions := w.FileRow(lr, s.Width)
			rowIdx := len(lines)
			lines = append(lines, line)
			for _, reg := range rowRegions {
				reg.Row = rowIdx
				regions = append(regions, reg)
			}
		case state.RowStash, state.RowWorktree, state.RowSectionHeading, state.RowDirectory:
			// The history tab holds commits and the files each one touched.

		}
	}
	return lines, regions
}

// Which row shows the verbs is the drawing's answer, not this one: subjectRow
// offers the verb field on the cursor line alone, the same as FileRow. This
// asked it a second time, and the stash and worktree tabs did not, so the three
// tabs disagreed about where the question was settled.
func historyRowVerbs(s state.State, row state.Row, commitIndex int, w Renderer) []state.VerbName {
	if !state.CursorRowVerbsAllowed(s) {
		return nil
	}
	return commitVerbsForRemote(row.Commit(), commitIndex, s.History.RemoteURL, w.Clipboard, w.Browser)
}

func historyFileVerbs(s state.State) []state.VerbName {
	if !state.CursorRowVerbsAllowed(s) {
		return nil
	}
	return []state.VerbName{state.VerbNameDiff}
}
