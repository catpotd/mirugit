package layout

import (
	"strings"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

type rowState struct {
	Cursor   bool
	Selected bool
	Read     state.Mark
	Stale    bool
	// NoSelect leaves the box out. Only the changes tab has verbs that act on a
	// selection, and a box the reader can aim at but never fill reads as broken.
	NoSelect bool
	Indent   int
	// Conflicts draws the stash-file collision label between the name and the
	// figures so the +N column stays fixed when the bar grows or shrinks.
	Conflicts bool
	// FullPath keeps the whole path in the name field. Stash and worktree tabs
	// show paths from the repo root; the changes tab still uses basename.
	FullPath bool
}

type Row struct {
	Entry git.Entry
	Count git.Count
	State rowState
	Verbs []state.VerbName
	// Index is which row of the state this is. A click resolves through it
	// rather than by path, because two sections can show one path twice.
	Index   int
	Section state.Section
}

// markColumn is what tests assert against instead of a literal, so widening the
// gutter moves the rows and their assertions together.
const markColumn = 4

const stashFileConflictsLabel = "        conflicts"

const (
	figuresWidth = 13 // two counts of up to five digits and a sign, and the space between them
	// verbsWidth fits the longest cursor-row offer, u unstage  x discard  z stash.
	verbsWidth = 29
	// stashCommitWorktreeVerbsWidth is the verb field on stash, commit, and
	// worktree cursor rows. File rows alone need the wider verbsWidth.
	stashCommitWorktreeVerbsWidth = 24
	rowVerbGap                    = 1 // between the name and verbs, not inside the verb field
	minRowNameRoom                = 9 // … plus eight cells of trailing name
)

func keyedVerb(v state.VerbName) string {
	if k, ok := footerKeys[v]; ok {
		return k + " " + v.String()
	}
	if v == state.VerbNameClear {
		return "esc " + v.String()
	}
	return v.String()
}

// verbField pads in cells rather than with fmt, which counts runes. The keys
// for fold and unfold are ← and →, one rune of two cells wherever the terminal
// draws ambiguous characters wide, so a field holding either came out a cell
// short of the width it was given and moved the column beside it.
func (w Renderer) verbField(verbs []state.VerbName, fieldWidth int) string {
	tokens := make([]string, len(verbs))
	for i, v := range verbs {
		tokens[i] = keyedVerb(v)
	}
	return w.padTo(strings.Join(tokens, "  "), fieldWidth)
}

func (w Renderer) keyedVerbJoinWidth(verbs []state.VerbName) int {
	tokens := make([]string, len(verbs))
	for i, v := range verbs {
		tokens[i] = keyedVerb(v)
	}
	return w.Of(strings.Join(tokens, "  "))
}

func (w Renderer) fitRowVerbs(verbs []state.VerbName, room int) []state.VerbName {
	for len(verbs) > 0 && w.keyedVerbJoinWidth(verbs) > room {
		verbs = verbs[:len(verbs)-1]
	}
	if len(verbs) == 0 {
		return nil
	}
	return verbs
}

// RowVerbs returns the verbs a cursor row can show on the changes tab.
func RowVerbs(e git.Entry, sec state.Section) []state.VerbName {
	return state.ChangesRowVerbs(state.FileRow(

		e,
		sec))
}

type rowRight struct {
	text       string
	verbs      []state.VerbName
	fieldWidth int
	stale      bool
}

func (w Renderer) rowRightForCursor(r Row, verbRoom int) rowRight {
	if r.State.Stale {
		const staleLabel = "stale · R reload"
		staleWidth := w.Of(staleLabel)
		if staleWidth > verbRoom {
			return rowRight{}
		}
		fieldWidth := staleWidth
		if verbRoom >= verbsWidth {
			fieldWidth = verbsWidth
		}
		return rowRight{
			text:       w.padTo(staleLabel, fieldWidth),
			fieldWidth: fieldWidth,
			stale:      true,
		}
	}
	if len(r.Verbs) == 0 {
		return rowRight{}
	}
	room := min(verbsWidth, verbRoom)
	fittedVerbs := w.fitRowVerbs(r.Verbs, room)
	if len(fittedVerbs) == 0 {
		return rowRight{}
	}
	fieldWidth := w.keyedVerbJoinWidth(fittedVerbs)
	if verbRoom >= verbsWidth {
		fieldWidth = verbsWidth
	}
	return rowRight{
		text:       w.verbField(fittedVerbs, fieldWidth),
		verbs:      fittedVerbs,
		fieldWidth: fieldWidth,
	}
}

func (w Renderer) fileRowRegions(r Row, rr rowRight, leftWidth, nameWidth, width int) []Region {
	regions := []Region{{
		Target:   Target{Kind: TargetFile, Path: r.Entry.Path, Row: r.Index},
		ColStart: leftWidth,
		ColEnd:   leftWidth + nameWidth,
	}}
	if (r.State.Cursor || r.State.Selected) && !r.State.NoSelect {
		boxStart, boxEnd := w.checkboxColumns()
		regions = append(regions, Region{
			Target:   Target{Kind: TargetCheckbox, Path: r.Entry.Path, Row: r.Index},
			ColStart: boxStart,
			ColEnd:   boxEnd,
		})
	}
	if r.State.Cursor && (rr.stale || len(rr.verbs) > 0) {
		rightStart := width - w.Of(rr.text)
		if rr.stale {
			regions = append(regions, w.staleVerbRegions(rightStart, r.Index)...)
		} else {
			regions = append(regions, w.verbRegions(rr.verbs, rightStart, r.Index)...)
		}
	}
	return regions
}

// The fields are named because three of them are strings and two are ints;
// positional arguments let a caller swap them without a compile error.
type subjectRowInput struct {
	Left string
	// Right is what sits at the end of the row when the cursor is elsewhere: a
	// commit's author and age, a stash's branch and verdict, a worktree's
	// counts and merge state. A row that has it in several fixed columns
	// passes them in RightColumns instead, and this stays empty.
	Right string
	// RightColumns is Right in pieces, for a row that gives its trailing
	// columns up one at a time rather than all together. They are dropped from
	// the left, because the rightmost is the one the reader cannot find out
	// another way.
	RightColumns []string
	// Subject is what the row is called. SubjectWidth pins it to a fixed number
	// of cells for a row whose name sits in a column of its own; zero lets it
	// take whatever is left.
	Subject      string
	SubjectWidth int
	Target       Target
	Cursor       bool
	Verbs        []state.VerbName
	// ColorMeta paints Right. A row that builds its right half from
	// RightColumns leaves it nil and gives ColorLastColumn instead, because
	// which of those columns were drawn is settled here rather than by the
	// caller.
	ColorMeta func() string
	// ColorLastColumn paints the rightmost column, the one joinToFit keeps
	// longest. The columns before it carry no color.
	ColorLastColumn func() string
}

// colorRight paints the right half however the caller said to. Painting has to
// hand back exactly the cells paintTail takes away, so a row that colored only
// its last column while the row drew four came out three columns short.
func (r subjectRowInput) colorRight(right string) string {
	if r.ColorMeta != nil {
		return r.ColorMeta()
	}
	if r.ColorLastColumn == nil || len(r.RightColumns) == 0 {
		return right
	}
	last := r.RightColumns[len(r.RightColumns)-1]
	if !strings.HasSuffix(right, last) {
		return right
	}
	return right[:len(right)-len(last)] + r.ColorLastColumn()
}

func (w Renderer) subjectRow(r subjectRowInput, width int) (string, []Region) {
	left, right, subject := r.Left, r.Right, r.Subject
	cursor, verbs := r.Cursor, r.Verbs
	// The subject names the row, so it keeps its room and the column to its
	// right gives way — the same order FileRow settles them in. Both right-hand
	// fields are a fixed width of their own, and neither fits beside a subject
	// in the narrowest pane mirugit agrees to draw.
	//
	// The gap between the two comes out of the right field's room, the same way
	// FileRow takes it: without it a field that fitted exactly left the subject
	// one cell under the smallest name this pane promises to show.
	// The room the right-hand fields may take. A row whose name sits in a
	// column of its own gives them everything past that column; a row whose
	// name takes what is left keeps minRowNameRoom of it and a cell of gap.
	subjectWidth := r.SubjectWidth
	// lastColumn is the trailing column a cursor row keeps beside its verbs.
	// The paint has to cover it too, or it hands back fewer cells than it took.
	lastColumn := ""
	rightRoom := width - w.Of(left) - minRowNameRoom - rowVerbGap
	if subjectWidth > 0 {
		rightRoom = max(width-w.Of(left)-subjectWidth, 0)
	}
	// A row whose trailing columns are given up one at a time keeps the last of
	// them beside the verbs: a tree that is merging is the one thing the reader
	// cannot find out from the row's own name. Its width comes out of the
	// verbs', not out of the pane.
	verbRoom := rightRoom
	if cursor && len(verbs) > 0 && len(r.RightColumns) > 0 {
		lastColumn = r.RightColumns[len(r.RightColumns)-1]
		// One cell between the verbs and the column beside them, the same gap a
		// name keeps from what follows it. Without it "g go" and "merges" read
		// as one word.
		verbRoom = max(rightRoom-w.Of(lastColumn)-rowVerbGap, 0)

	}
	if cursor && len(verbs) > 0 {
		verbs = w.fitRowVerbs(verbs, min(stashCommitWorktreeVerbsWidth, verbRoom))
	}
	if cursor && len(verbs) > 0 {
		fieldWidth := w.keyedVerbJoinWidth(verbs)
		if verbRoom >= stashCommitWorktreeVerbsWidth {
			fieldWidth = stashCommitWorktreeVerbsWidth
		}
		right = w.verbField(verbs, fieldWidth)
		if lastColumn != "" {
			right += strings.Repeat(" ", rowVerbGap) + lastColumn
		}
	} else if len(r.RightColumns) > 0 {
		right = joinToFit(w, r.RightColumns, rightRoom)
		lastColumn = r.RightColumns[len(r.RightColumns)-1]
	}
	if w.Of(right) > rightRoom {
		right = ""
	}
	// One cell between the subject and whatever sits to its right, the same gap
	// FileRow keeps between a name and its verbs. Without it a subject long
	// enough to be cut ran into the next column and read as one word:
	// "…u uncommit", where the mark that says text was dropped looked like part
	// of the key.
	subjectRoom := width - w.Of(left) - w.Of(right)
	if right != "" && subjectWidth == 0 {
		subjectRoom -= rowVerbGap
	}
	if subjectWidth > 0 {
		// A name in a column of its own keeps that column whatever the row
		// beside it is doing, so the columns line up down the list. Padding
		// comes after the cut: padding first and cutting after gives back a
		// row of blanks the width of the column.
		subject = w.padTo(w.Truncate(Printable(subject), min(subjectWidth, subjectRoom)),
			min(subjectWidth, subjectRoom))
	} else {
		subject = w.Truncate(Printable(subject), subjectRoom)
	}
	line := w.Pad(left+subject, right, width)

	regions := []Region{{
		Target:   r.Target,
		ColStart: w.Of(left),
		ColEnd:   w.Of(left) + w.Of(subject),
	}}
	if cursor && len(verbs) > 0 {
		rightStart := width - w.Of(right)
		regions = append(regions, w.verbRegions(verbs, rightStart, r.Target.Row)...)
	}
	// The verbs are what the next keypress would do, so only the row the cursor
	// is on gets them. Every other row keeps the column that says what it is.
	// The two are not the same width, and paintTail replaces by cells, so a row
	// that took the verbs it was not offered also came out four columns short.
	// The field width is the one the row was built with, not the constant: a
	// narrow pane fits fewer verbs, and paintTail cuts by cells and joins what
	// it is handed, so a wider replacement overflows what it replaced. FileRow
	// passes its own rr.fieldWidth here for the same reason.
	if cursor && len(verbs) > 0 {
		// The gap is spaces rather than padTo: Of counts escape bytes as cells,
		// so padding a colored field measures the escapes and adds nothing.
		gap := 0
		if lastColumn != "" {
			gap = rowVerbGap
		}
		// The gap is spaces rather than padTo: Of counts escape bytes as cells,
		// so padding a colored field measures the escapes and adds nothing.
		painted := w.colorVerbField(verbs, w.Of(right)-w.Of(lastColumn)-gap) +
			strings.Repeat(" ", gap)
		if lastColumn != "" && r.ColorLastColumn != nil {
			painted += r.ColorLastColumn()
		} else {
			painted += lastColumn
		}
		line = w.paintTail(line, width, painted, w.Of(right))
	} else if right != "" {
		line = w.paintTail(line, width, r.colorRight(right), w.Of(right))
	}
	if cursor {
		line = replaceOnce(line, left, w.colorCursorLeft(left))
	}
	return line, regions
}

func (w Renderer) FileRow(r Row, width int) (string, []Region) {
	indent := strings.Repeat("  ", r.State.Indent)
	left := w.gutter(r) + indent + string(kindOf(r.Entry, r.Section)) + "  "
	right := w.figures(r)
	var rr rowRight

	if r.State.Cursor {
		verbRoom := width - w.Of(left) - minRowNameRoom - rowVerbGap
		rr = w.rowRightForCursor(r, verbRoom)
		switch {
		case rr.stale || len(rr.verbs) > 0:
			right = rr.text
		case !r.State.Stale && len(r.Verbs) == 0:
			right = w.figures(r)
		default:
			right = ""
		}
	}

	var gap int
	if rr.stale || len(rr.verbs) > 0 {
		gap = rowVerbGap
	}

	rightPart := right
	conflictGap := ""
	if r.State.Conflicts {
		if right != "" && !strings.HasPrefix(right, " ") {
			conflictGap = " "
		}
		rightPart = stashFileConflictsLabel + conflictGap + right
	}
	// The name identifies the row, so it keeps its room and the columns to its
	// right give way. The cursor's verbs were already fitted this way; the
	// figures and the collision label were not, and both are a fixed width.
	// Measured: on a 30-column pane every row that was not the cursor's drew as
	// its figures alone, with no name on it to read or to click.
	if w.Of(rightPart) > width-w.Of(left)-minRowNameRoom-gap {
		right, rightPart = "", ""
		rr, gap = rowRight{}, 0
	}

	namePath := Printable(base(r.Entry.Path))
	if r.State.FullPath {
		namePath = Printable(r.Entry.Path)
	}
	nameRoom := width - w.Of(left) - w.Of(rightPart) - gap
	name := w.TruncateFront(namePath, nameRoom)
	if r.Entry.OldPath != "" {
		name = w.rename(r.Entry, nameRoom)
	}

	line := w.Pad(left+name, rightPart, width)
	// Columns are cells, because that is what a mouse reports. Counting runes
	// would make the right half of a Japanese file name unclickable.
	regions := w.fileRowRegions(r, rr, w.Of(left), w.Of(name), width)
	if r.State.Conflicts && rightPart != "" {
		colored := strings.TrimSuffix(stashFileConflictsLabel, "conflicts") + w.colorStashFileConflicts() + conflictGap
		switch {
		case r.State.Cursor && rr.stale:
			colored += w.colorStaleVerbField(rr.fieldWidth)
		case r.State.Cursor && len(rr.verbs) > 0:
			colored += w.colorVerbField(rr.verbs, rr.fieldWidth)
		default:
			colored += w.colorFigures(r)
		}
		line = w.paintTail(line, width, colored, w.Of(rightPart))
	} else {
		switch {
		case r.State.Cursor && rr.stale:
			line = w.paintTail(line, width, w.colorStaleVerbField(rr.fieldWidth), w.Of(right))
		case r.State.Cursor && len(rr.verbs) > 0:
			line = w.paintTail(line, width, w.colorVerbField(rr.verbs, rr.fieldWidth), w.Of(right))
		case right != "":
			line = w.paintTail(line, width, w.colorFigures(r), w.Of(right))
		}
	}
	line = replaceOnce(line, w.gutter(r), w.colorGutter(r))
	return line, regions
}

// verbRegions maps each drawn verb to a region. ColStart and ColEnd count cells.
func (w Renderer) verbRegions(verbs []state.VerbName, startCol, index int) []Region {
	regions := make([]Region, 0, len(verbs))
	col := startCol
	for i, verb := range verbs {
		if i > 0 {
			col += 2
		}
		token := keyedVerb(verb)
		end := col + w.Of(token)
		regions = append(regions, Region{
			Target:   Target{Kind: TargetVerb, Verb: verb, Row: index},
			ColStart: col,
			ColEnd:   end,
		})
		col = end
	}
	return regions
}

func (w Renderer) staleVerbRegions(startCol, index int) []Region {
	const prefix = "stale · R "
	col := startCol + w.Of(prefix)
	end := col + w.Of(state.VerbNameReload.String())
	return []Region{{
		Target:   Target{Kind: TargetVerb, Verb: state.VerbNameReload, Row: index},
		ColStart: col,
		ColEnd:   end,
	}}
}

func (w Renderer) colorStaleVerbField(fieldWidth int) string {
	plain := w.padTo("stale · R "+state.VerbNameReload.String(), fieldWidth)
	if !w.Enabled {
		return plain
	}
	return replaceOnce(plain, state.VerbNameReload.String(), w.Verb(state.VerbNameReload.String()))
}

// checkboxColumns is where the box sits: the column the cursor mark owns, then
// three of box. The three kinds of row that draw one had three different
// answers, and the section heading's was off by one, so the leftmost cell of
// its box did nothing.
//
// The cursor column is as wide as the mark, which is one cell on most terminals
// and two where ambiguous characters are drawn wide. Reading it as one either
// way put the box's click region a cell to the left of the box on every row of
// such a terminal that draws one.
func (w Renderer) checkboxColumns() (start, end int) {
	start = w.cursorMarkWidth()
	return start, start + 3
}

func (w Renderer) assembleGutter(cursor, selected, noSelect bool, mark rune) string {
	cursorCell := w.cursorCell(cursor)
	box := "   "
	if !noSelect {
		switch {
		case selected:
			box = "[✓]"
		case cursor:
			box = "[ ]"
		}
	}
	if mark == 0 {
		mark = ' '
	}
	return cursorCell + box + string(mark) + " "
}

// cursorCell draws the column the cursor mark owns, marked or blank. Every row
// that draws a box has to fill it the same way, or the boxes below each other
// start in different columns.
func (w Renderer) cursorCell(cursor bool) string {
	if cursor {
		return cursorMark
	}
	return strings.Repeat(" ", w.cursorMarkWidth())
}

func (w Renderer) gutter(r Row) string {
	mark := ' '
	switch r.State.Read {
	case state.Unread:
		mark = '·'
	case state.Changed:
		mark = '●'
	case state.Read:
		// A read row keeps the blank gutter it started with.
	}
	return w.assembleGutter(r.State.Cursor, r.State.Selected, r.State.NoSelect, mark)
}

// figures is the fixed column of counts. It is laid out in cells rather than by
// fmt, which counts runes: the minus sign the counts are written with is U+2212
// and is two cells wide on some terminals, and a column meant to be 12 came out
// wider there, moving every column beside it.
func (w Renderer) figures(r Row) string {
	if r.Count.Binary {
		return w.padFront("binary", figuresWidth)
	}
	counts := w.padFront(state.Plus(r.Count.Added), 6) + " " +
		w.padFront(state.Minus(r.Count.Deleted), 6)
	return w.padFront(counts, figuresWidth)
}

// padFront widens value to cells columns by putting the spaces in front, which
// is what right-aligning a number in its column means.
func (w Renderer) padFront(value string, cells int) string {
	return strings.Repeat(" ", max(cells-w.Of(value), 0)) + value
}

// The old path loses detail first: rename pairing is git's guess, and the new
// path is where the file is now.
func (w Renderer) rename(e git.Entry, room int) string {
	// Both names come from the repository, so both go through Printable. Every
	// other row does; this one read the paths straight out of the entry, and an
	// escape in a file name reached the terminal as an escape.
	newName := Printable(base(e.Path))
	oldName := Printable(base(e.OldPath))
	if state.ParentDir(e.Path) != state.ParentDir(e.OldPath) {
		oldName = Printable(e.OldPath)
	}
	sep := " ← "
	oldRoom := room - w.Of(newName) - w.Of(sep)
	if oldRoom < 8 {
		return w.TruncateFront(newName, room)
	}
	return newName + sep + w.TruncateFront(oldName, oldRoom)
}

func base(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

func kindOf(e git.Entry, sec state.Section) git.Kind {
	if sec == state.SectionStaged {
		return e.Index
	}
	if e.Worktree != git.Unchanged {
		return e.Worktree
	}
	return e.Index
}

// joinToFit drops columns off the left until what is left fits. The rightmost
// stays longest, because it is the one that says something the row's own name
// cannot.
func joinToFit(w Renderer, columns []string, room int) string {
	for {
		joined := strings.Join(columns, "")
		if w.Of(joined) <= room || len(columns) == 0 {
			return joined
		}
		columns = columns[1:]
	}
}
