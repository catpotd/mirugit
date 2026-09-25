package layout

import (
	"fmt"
	"strings"

	"github.com/catpotd/mirugit/internal/state"
)

func changesPane(s state.State, r ReadMarks, w Renderer, f Frame) Frame {
	commitLine, commitReg := w.CommitBox(s.Changes.Message, s.Changes.MessageFocused, s.Width)
	commitReg.Row = len(f.Lines)
	f.Lines = append(f.Lines, commitLine)
	f.Regions = append(f.Regions, commitReg)

	sum := summaryOf(s, r)
	summaryLine, summaryRegions := w.SummaryLine(sum, s.Width)
	summaryRow := len(f.Lines)
	f.Lines = append(f.Lines, summaryLine)
	for _, reg := range summaryRegions {
		reg.Row = summaryRow
		f.Regions = append(f.Regions, reg)
	}
	return finishListPane(s, r, w, f, func(win lineWindow) ([]string, []Region) {
		return renderFor(s.Tab).List(s, r, w, win)
	})
}

func sectionsOf(s state.State, r ReadMarks, w Renderer, win lineWindow) ([]string, []Region) {
	var lines []string
	var regions []Region
	headed := map[state.Section]bool{}
	for i, row := range s.Rows {
		switch row.Kind() {
		case state.RowSectionHeading:
			lines, regions = appendSectionHeading(lines, regions, s, w, row.Section(), i, headed, win)
		case state.RowDirectory:
			if state.IsHiddenByFoldedDirectory(s, row.Section(), row.DirPath()) {
				continue
			}
			rowIdx := len(lines)
			if !win.draws(rowIdx) {
				lines, regions = appendHiddenRow(win, lines, regions,
					Target{Kind: TargetDirectory, Row: i})
				continue
			}
			count := directoryFileCount(s, row.DirPath(), row.Section())
			checkbox := directoryCheckbox(s, row.DirPath(), row.Section(), count, i == s.Cursor)
			line, dirRegions := w.directoryRow(
				row.DirPath(), count, checkbox,
				state.FoldedIn(s, row.Section(), row.DirPath()),
				i == s.Cursor, i, s.Width,
			)
			lines = append(lines, line)
			for _, reg := range dirRegions {
				reg.Row = rowIdx
				regions = append(regions, reg)
			}
		case state.RowFile:
			if !headed[row.Section()] {
				lines, regions = appendSectionHeading(lines, regions, s, w, row.Section(), i, headed, win)
			}
			if state.IsHiddenByFoldedDirectory(s, row.Section(), row.Path()) {
				continue
			}
			rowIdx := len(lines)
			if !win.draws(rowIdx) {
				lines, regions = appendHiddenRow(win, lines, regions,
					Target{Kind: TargetFile, Path: row.Path(), Row: i})
				continue
			}
			lr := layoutFileRow(s, r, row, i)
			line, rowRegions := w.FileRow(lr, s.Width)
			lines = append(lines, line)
			for _, reg := range rowRegions {
				reg.Row = rowIdx
				regions = append(regions, reg)
			}
		case state.RowCommit, state.RowStash, state.RowWorktree:
			// The changes tab holds headings, directories and files. Commits,
			// stashes and worktrees are built by the other three panes.
		}
	}
	return lines, regions
}

func appendSectionHeading(
	lines []string, regions []Region,
	s state.State, w Renderer,
	sec state.Section, cursorIndex int, headed map[state.Section]bool, win lineWindow,
) ([]string, []Region) {
	if headed[sec] {
		return lines, regions
	}
	headed[sec] = true
	if len(lines) > 0 {
		lines = append(lines, blankOr(win, len(lines), s.Width))
	}
	if !win.draws(len(lines)) {
		// TargetNone is what says which state row this screen line is, and
		// CursorLineInList reads only that. A hidden heading without it left
		// the cursor with no line to be on.
		return appendHiddenRow(win, lines, regions,
			Target{Kind: TargetSectionHeading, Section: sec},
			Target{Kind: TargetNone, Row: headingCursorIndex(s, sec, cursorIndex)})
	}
	// Counted over every file in the section, not the drawn ones: folding hides
	// rows without changing what is there, and the directory row it hid keeps
	// saying how many it holds.
	allFiles, added, deleted := sectionTotals(s, sec)
	name := "staged"
	if sec == state.SectionUnstaged {
		name = "changes"
	}
	checkbox := sectionCheckbox(s, sec)
	heading, headingRegions := w.Heading(
		name, allFiles, added, deleted, sec, checkbox, "", s.Width,
	)
	headingCursor := headingCursorIndex(s, sec, cursorIndex)
	if headingCursor == s.Cursor {
		heading = w.cursorSectionHeading(heading)
	}
	headingRow := len(lines)
	lines = append(lines, heading)
	for _, reg := range headingRegions {
		reg.Row = headingRow
		regions = append(regions, reg)
	}
	regions = append(regions, Region{
		Target:   Target{Kind: TargetNone, Row: headingCursor},
		Row:      headingRow,
		ColStart: 0,
		ColEnd:   1,
	})
	return lines, regions
}

func headingCursorIndex(s state.State, sec state.Section, fallback int) int {
	for j, row := range s.Rows {
		if j == s.Cursor && row.Kind() == state.RowSectionHeading && row.Section() == sec {
			return j
		}
	}
	return fallback
}

func layoutFileRow(s state.State, r ReadMarks, row state.Row, index int) Row {
	e := row.Entry()
	count := e.WorktreeCount
	if row.Section() == state.SectionStaged {
		count = e.IndexCount
	}
	path := row.Path()
	return Row{
		Entry:   e,
		Count:   count,
		Index:   index,
		Section: row.Section(),
		State: rowState{
			Cursor:   index == s.Cursor,
			Selected: state.SelectedIn(s, row.Section(), path),
			Read:     fileMark(r, state.WorkingTree(row.Section()), path, s),
			Stale:    s.Changes.Stale[path] && s.Open.Path == path && index == s.Cursor,
			Indent:   fileIndent(path),
		},
		Verbs: rowVerbs(s, row, index),
	}
}

func directoryFileCount(s state.State, dir string, sec state.Section) int {
	n := 0
	for _, row := range s.Rows {
		if row.Kind() != state.RowFile || row.Section() != sec {
			continue
		}
		if state.IsBelowDirectory(row.Path(), dir) {
			n++
		}
	}
	return n
}

// directoryCheckbox mirrors sectionCheckbox for a directory row: the box is
// drawn on the cursor row even when nothing is selected, so the first click
// has something to land on.
func directoryCheckbox(s state.State, dir string, sec state.Section, count int, cursorHere bool) string {
	if count == 0 {
		return ""
	}
	selected, _ := tickedInSection(s, sec, func(path string) bool {
		return state.IsBelowDirectory(path, dir)
	})
	if selected == 0 && !cursorHere {
		return ""
	}
	if selected == 0 {
		return "[ ]"
	}
	if selected == count {
		return "[✓]"
	}
	return "[-]"
}

func (w Renderer) directoryRow(dir string, count int, checkbox string, folded, cursor bool, index, width int) (string, []Region) {
	// Column zero is the cursor's on every row of the list, so the path neither
	// moves when the cursor arrives nor sits under a marker that changes place
	// between one kind of row and the next.
	// The cursor's column, then the box's, then two blank. A row without a box
	// leaves the box's columns blank rather than closing them up: the box
	// appears only once the cursor arrives, and a row that closed them up moved
	// its own text sideways as the reader arrived at it.
	box := checkbox
	if box == "" {
		box = "   "
	}
	prefix := w.cursorCell(cursor) + box + "  "
	arrow := "▾"
	if folded {
		arrow = "▸"
	}
	indent := strings.Repeat("  ", directoryIndent(dir))
	prefix += indent
	heading := prefix + arrow + " " + w.TruncateFront(Printable(dirLabel(dir)), width-12-w.Of(indent))
	line := w.colorDirHeading(w.Pad(heading, fmt.Sprint(count), width), arrow)
	if cursor {
		line = replaceOnce(line, "▌", w.Cursor("▌"))
	}
	if checkbox != "" {
		line = replaceOnce(line, checkbox, colorCheckbox(w.Palette, checkbox))
	}
	regions := []Region{{
		Target:   Target{Kind: TargetDirectory, Path: dir, Row: index},
		ColStart: w.Of(prefix),
		ColEnd:   w.Of(heading),
	}}
	if checkbox != "" {
		boxStart, boxEnd := w.checkboxColumns()
		regions = append([]Region{{
			Target:   Target{Kind: TargetCheckbox, Path: dir, Row: index},
			ColStart: boxStart,
			ColEnd:   boxEnd,
		}}, regions...)
	}
	return line, regions
}

func dirLabel(d string) string {
	if d == "" {
		return "/"
	}
	return d
}

func directoryIndent(dir string) int {
	if dir == "" {
		return 0
	}
	return strings.Count(dir, "/") + 1
}

func fileIndent(path string) int {
	return strings.Count(path, "/") + 1
}

// cursorSectionHeading puts the mark in column zero, where every file row
// carries it. Replacing the first pair of spaces moved it behind the checkbox,
// which appears on the heading only once the cursor is already there.
//
// It takes as many spaces as the mark is wide, and Heading leaves that many.
// The row is already padded to the pane when this runs, so an uneven swap made
// the heading one cell wider than the pane and pushed every row below it down;
// leaving the mark out instead made the cursor invisible on the heading.
func (w Renderer) cursorSectionHeading(line string) string {
	room := strings.Repeat(" ", w.cursorMarkWidth())
	if !strings.HasPrefix(line, room) {
		return line
	}
	return w.Cursor(cursorMark) + line[len(room):]
}

// The cursor test is here as well as in the drawing, because this builds a
// slice per row and the drawing throws away every one but the cursor's. A pane
// of two hundred files allocated two hundred of them per frame, which put the
// frame over its budget.
func rowVerbs(s state.State, row state.Row, index int) []state.VerbName {
	if !state.CursorRowVerbsAllowed(s) || index != s.Cursor {
		return nil
	}
	if row.Kind() != state.RowFile {
		return nil
	}
	path := row.Path()
	if s.Changes.Stale[path] && s.Open.Path == path {
		return nil
	}
	return RowVerbs(row.Entry(), row.Section())
}

// sectionCheckbox draws a box on the cursor row even with nothing selected,
// the way a file row does. Without it the first selection in a section is out
// of the mouse's reach: the box the design says to click is the box that only
// appears once something is already selected.
func sectionCheckbox(s state.State, sec state.Section) string {
	// Not the folded ones: the heading says whether what the reader can see is
	// ticked. Counting them by walking the rows rather than by laying each one
	// out — the heading needs a path and a total, and laying out twenty
	// thousand rows to draw one heading was 3.5 MB and 3.3 ms of every frame.
	selected, total := tickedInSection(s, sec, func(path string) bool {
		return !state.IsHiddenByFoldedDirectory(s, sec, path)
	})
	if total == 0 {
		return ""
	}
	if selected == 0 {
		if row, ok := state.CursorRow(s); ok &&
			row.Kind() == state.RowSectionHeading && row.Section() == sec {
			return "[ ]"
		}
		return ""
	}
	if selected == total {
		return "[✓]"
	}
	return "[-]"
}

// tickedInSection counts the section's files that include accepts, and how many
// there are. The heading and the directory row ask this of different ranges and
// each used to answer it its own way.
func tickedInSection(s state.State, sec state.Section, include func(path string) bool) (ticked, total int) {
	for _, row := range s.Rows {
		if row.Kind() != state.RowFile || row.Section() != sec {
			continue
		}
		if !include(row.Path()) {
			continue
		}
		total++
		if state.SelectedIn(s, sec, row.Path()) {
			ticked++
		}
	}
	return ticked, total
}

// sectionTotals counts the files in a section and their lines. Both callers
// wanted the numbers, and collecting the entries to get them copied every file
// in the section on every frame.
func sectionTotals(s state.State, sec state.Section) (files, added, deleted int) {
	for _, row := range s.Rows {
		if row.Kind() != state.RowFile || row.Section() != sec {
			continue
		}
		files++
		c := row.Entry().WorktreeCount
		if sec == state.SectionStaged {
			c = row.Entry().IndexCount
		}
		added += c.Added
		deleted += c.Deleted
	}
	return files, added, deleted
}

// blankOr is the blank line between sections, or a placeholder when the window
// leaves it out.
func blankOr(win lineWindow, at, width int) string {
	if !win.draws(at) {
		return hiddenLine
	}
	return strings.Repeat(" ", width)
}
