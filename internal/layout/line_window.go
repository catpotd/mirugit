package layout

// lineWindow is the range of list lines the pane will actually draw. The list
// is built to the size of the repository and then sliced to the size of the
// terminal, so a repository of twenty thousand changed files paid to lay out
// twenty thousand rows to show fifty: measured at 229 ms and 28 MB per frame,
// against 1.4 ms and no allocation for the walk that counts them.
//
// The window is not known before the walk, because how many rows the list gets
// depends on how many lines it has. So the same walk runs twice: once with
// noLines, which lays out nothing and only counts, and once with the window the
// count decided. Running the same function twice is what keeps the two from
// disagreeing about which rows a fold hides.
type lineWindow struct {
	first int
	last  int
	// dropHiddenTargets leaves out the click targets of the rows outside the
	// window. They answer "which row is this screen line" for a row nobody can
	// click, and only TabListShape reads them: sliceScrolled throws away every
	// one. Building twenty thousand of them was 21.6 MB of the 30.4 MB one
	// frame allocated at twenty thousand changed files.
	dropHiddenTargets bool
}

// noLines counts and places the rows without laying any of them out.
var noLines = lineWindow{first: 0, last: 0}

// countLines counts and places nothing. It is noLines for the callers that read
// only the number.
var countLines = lineWindow{first: 0, last: 0, dropHiddenTargets: true}

// everyLine lays out the whole list. The panes that are not a scrolling list —
// the help overlay, the too-narrow notice — have no window to speak of.
var everyLine = lineWindow{first: 0, last: -1}

// draws reports whether the line at this index reaches the screen.
func (w lineWindow) draws(i int) bool {
	if w.last < 0 {
		return true
	}
	return i >= w.first && i < w.last
}

// hiddenLine stands in for a line the window leaves out. sliceScrolled indexes
// the list, so the entries have to keep their places; only their contents are
// skipped.
const hiddenLine = ""

// appendHiddenRow keeps a line's place and its click target without laying the
// line out. The target stays because CursorLineInList finds the cursor through
// it, and the cursor is often outside the window scrolling is about to move.
func appendHiddenRow(win lineWindow, lines []string, regions []Region, targets ...Target) ([]string, []Region) {
	if win.dropHiddenTargets {
		return append(lines, hiddenLine), regions
	}
	row := len(lines)
	for _, t := range targets {
		regions = append(regions, Region{Target: t, Row: row})
	}
	return append(lines, hiddenLine), regions
}
