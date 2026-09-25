package layout

import (
	"strings"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// HeaderRowsOf counts every line above the list through the tab bar, including
// an optional notice on any tab.
func HeaderRowsOf(s state.State) int {
	n := state.Facts[s.Tab].HeaderRows
	if s.Notice != "" {
		n++
	}
	if s.Unfinished != git.NothingInProgress {
		n++
	}
	return n
}

// unfinishedLine says which git operation the repository is part way through
// and how to end it. status --porcelain does not report it, so a merge whose
// conflicts are all resolved draws an empty file list: the reader sees nothing
// left to do while git is waiting for a commit.
//
// The commands are printed rather than bound to keys because ending a rebase
// takes decisions this pane does not ask for.
func (w Renderer) unfinishedLine(s state.State) string {
	if s.Unfinished == git.NothingInProgress {
		return ""
	}
	text := s.Unfinished.Verb() + " in progress · " + s.Unfinished.Finish() + " to finish"
	if abandon := s.Unfinished.Abandon(); abandon != "" {
		text += " · " + abandon + " to stop"
	}
	return w.Pad(w.Truncate(text, s.Width), "", s.Width)
}

// Pane has no terminal, no clock and no repository in reach, so that the
// correspondence between what is drawn and where a click lands can be asserted
// without any of them.
// minPaneWidth is the narrowest pane every row can still be laid out in. Below
// it the tab bar, the commit box, and the file rows each overflow by a cell or
// two, and an overflowing row wraps and shifts every row under it. Measured: the
// last failure is at 19 cells.
const minPaneWidth = 20

// tooNarrowPane says why the pane is blank rather than drawing rows that would
// wrap. A reader who widens the terminal gets the pane back.
func tooNarrowPane(s state.State, w Renderer) Frame {
	f := Frame{}
	if s.Width <= 0 || s.Height <= 0 {
		return f
	}
	f.Lines = append(f.Lines, w.Pad(w.Truncate("too narrow", s.Width), "", s.Width))
	for len(f.Lines) < s.Height {
		f.Lines = append(f.Lines, strings.Repeat(" ", s.Width))
	}
	return f
}

func Pane(s state.State, r ReadMarks, w Renderer) Frame {
	if s.Width < minPaneWidth {
		return tooNarrowPane(s, w)
	}
	f := Frame{}
	tabs := TabsOf(s, r)
	bar, rule, tabRegions := w.TabBar(tabs, activeTab(s.Tab, tabs), tabBarRight(s), s.Width)
	f.Lines = append(f.Lines, bar, rule)
	f.Regions = append(f.Regions, tabRegions...)
	if line := w.unfinishedLine(s); line != "" {
		f.Lines = append(f.Lines, line)
	}

	if s.HelpOpen {
		return helpPane(s, w, f, tabs)
	}

	return renderFor(s.Tab).Pane(s, r, w, f)
}

func helpPane(s state.State, w Renderer, f Frame, tabs []Tab) Frame {
	overlay, overlayRegions := w.HelpOverlay(tabs, s.Tab, s.HelpScroll, s.Width, s.Height-len(f.Lines))
	base := len(f.Lines)
	f.Lines = append(f.Lines, overlay...)
	for _, reg := range overlayRegions {
		reg.Row += base
		f.Regions = append(f.Regions, reg)
	}
	for len(f.Lines) < s.Height {
		f.Lines = append(f.Lines, strings.Repeat(" ", s.Width))
	}
	return f.trimmedTo(s.Height)
}

func appendDiffPanel(f Frame, s state.State, r ReadMarks, w Renderer, diffRows int) Frame {
	if diffRows > 0 {
		f.Lines = append(f.Lines, w.colorRuleLine(w.ruleLine(s.Width)))
		blocks := len(s.Open.Diff.Blocks)
		block := 1
		if blocks > 0 {
			block = s.Open.BlockCursor + 1
		}
		f.Lines = append(f.Lines, w.DiffHeader(
			s.Open.Path, block, blocks,
			unreadBlocks(r, s.Open.Path, s.Open.Diff.Blocks), s.Width))
		area, diffRegions := w.DiffArea(s.Open.Diff, s.Open.BlockCursor, s.Open.Scroll, s.Open.BlockLine, diffShowStage(s), s.Width, diffRows)
		f.Regions = append(f.Regions, offsetRows(diffRegions, len(f.Lines))...)
		f.Lines = append(f.Lines, area...)
		f.Lines = append(f.Lines, w.colorRuleLine(w.ruleLine(s.Width)))
	}
	return f
}

func appendFooterAndPad(f Frame, s state.State, w Renderer) Frame {
	for len(f.Lines) < s.Height-1 {
		f.Lines = append(f.Lines, strings.Repeat(" ", s.Width))
	}
	footerRow := len(f.Lines)
	footerLine := w.footerPlain(s, s.Width)
	if mark, ok := footerHelpRegion(footerLine, footerRow, w); ok {
		f.Regions = append(f.Regions, mark)
	}
	f.Lines = append(f.Lines, w.colorFooter(s, footerLine))
	return f.trimmedTo(s.Height)
}

// trimmedTo cuts the frame to the terminal's height, lines and regions
// together. Cutting only the lines leaves regions naming rows that are not
// drawn, and Frame exists to keep the two from disagreeing.
func (f Frame) trimmedTo(height int) Frame {
	if height < 0 {
		height = 0
	}
	if len(f.Lines) > height {
		f.Lines = f.Lines[:height]
	}
	kept := f.Regions[:0]
	for _, reg := range f.Regions {
		if reg.Row >= 0 && reg.Row < height {
			kept = append(kept, reg)
		}
	}
	f.Regions = kept
	return f
}

func tabBarRight(s state.State) string {
	if s.Head.Branch == "" {
		return s.Fetched
	}
	return Printable(s.Head.Branch) + " · " + s.Fetched
}

func diffShowStage(s state.State) bool {
	if _, ok := state.CursorRow(s); s.Open.Path == "" || !ok {
		return false
	}
	row := s.Rows[s.Cursor]
	if row.Kind() != state.RowFile || row.Path() != s.Open.Path {
		return false
	}
	// Staging a conflicted path is what tells git the conflict is settled, so
	// the offer is withheld here as well as on the row.
	if row.Entry().IsConflicted() {
		return false
	}
	return row.Section() == state.SectionUnstaged
}

// ShownBlockHashes returns the content hashes of blocks DiffArea would draw.
func ShownBlockHashes(d git.FileDiff, startBlock, startLine, height int) []string {
	if d.Binary || d.ModeOnly || d.TooLong || len(d.Blocks) == 0 || height < 1 {
		return nil
	}
	if startBlock < 0 {
		startBlock = 0
	}
	if startBlock >= len(d.Blocks) {
		return nil
	}
	startLine = DiffLineScrollFor(d, startBlock, startLine, height)
	used := 1
	var hashes []string
	for i := startBlock; i < len(d.Blocks); i++ {
		if used >= height {
			break
		}
		if i > startBlock {
			used++
		}
		bodyStart := 0
		if i == startBlock {
			bodyStart = startLine
		}
		if used+1+len(d.Blocks[i].Lines)-bodyStart > height {
			break
		}
		used += 1 + len(d.Blocks[i].Lines) - bodyStart
		hashes = append(hashes, d.Blocks[i].Hash)
	}
	return hashes
}

// ListRowsFor returns how many body rows the list and diff split.
func ListRowsFor(s state.State, listLen, body int) (listRows, diffRows int) {
	if body < 1 {
		body = 1
	}
	if s.Open.Path == "" {
		return body, 0
	}
	if s.Open.PrioritizeDiff {
		return prioritizeDiffRows(listLen, body)
	}
	if s.Open.Peek {
		if !CanPeekDiff(body) {
			return body, 0
		}
		diffRows = minDiffRows
		listRows = body - diffChromeRows - diffRows
		return listRows, diffRows
	}
	return splitListAndDiff(listLen, body)
}

func prioritizeDiffRows(listLen, body int) (listRows, diffRows int) {
	diffRoom := body - diffChromeRows
	minimumListRows := min(listLen, 1)
	if diffRoom < minDiffRows+minimumListRows {
		return body, 0
	}
	listRows = min(listLen, 8)
	if diffRoom-listRows < minDiffRows {
		listRows = diffRoom - minDiffRows
	}
	return listRows, diffRoom - listRows
}

// splitListAndDiff is the split for a diff that is not peeking: the list takes
// the rows it asks for and the diff takes what is left, and the list takes the
// pane when what is left is less than a diff can be read in.
func splitListAndDiff(listLen, body int) (listRows, diffRows int) {
	listRows, diffRows = splitRows(listLen, body)
	if diffRows == 0 {
		return listRows, diffRows
	}
	adjList, adjDiff := splitRows(listLen, body-diffChromeRows)
	if adjDiff < minDiffRows {
		return body, 0
	}
	return adjList, adjDiff
}

// DiffNeedsPeek reports whether a diff can only be drawn by taking rows from
// the list. The ordinary split hands the list what it asks for and the diff
// what is left, which is nothing once the list is longer than the pane: a
// repository with more changed files than rows then had no way to show a diff
// at all.
//
// It asks the list's length and the pane, not the diff that is open. Whether
// the reader arrived by opening a diff or by moving the cursor onto another
// file, the same list in the same pane is split the same way.
func DiffNeedsPeek(listLen, body int) bool {
	if body < 1 {
		body = 1
	}
	_, diffRows := splitListAndDiff(listLen, body)
	return diffRows == 0
}

// CursorLineInList finds which list line holds the cursor row.
func CursorLineInList(regions []Region, cursor int) int {
	for _, r := range regions {
		if r.Target.Row != cursor {
			continue
		}
		switch r.Target.Kind {
		case TargetFile, TargetDirectory, TargetNone:
			return r.Row
		case TargetSectionHeading, TargetTab, TargetBlock, TargetButton, TargetCommitBox, TargetCommit, TargetCheckbox, TargetVerb, TargetHelp, TargetHelpLine:
			// The other kinds sit on a row rather than being one, so they do
			// not answer where the cursor line is.
		}
	}
	return 0
}

// ScrollTopFor keeps the cursor line inside the visible window.
func ScrollTopFor(cur, listLen, listRows, cursorLine int) int {
	top := cur
	if cursorLine < top {
		top = cursorLine
	} else if listRows > 0 && cursorLine >= top+listRows {
		top = cursorLine - listRows + 1
	}
	maxTop := listLen - listRows
	if maxTop < 0 {
		maxTop = 0
	}
	if top > maxTop {
		top = maxTop
	}
	if top < 0 {
		top = 0
	}
	return top
}

func sliceScrolled(list []string, regions []Region, scrollTop, listRows int) ([]string, []Region) {
	if scrollTop < 0 {
		scrollTop = 0
	}
	end := scrollTop + listRows
	if end > len(list) {
		end = len(list)
	}
	if scrollTop > len(list) {
		scrollTop = len(list)
	}
	shown := list[scrollTop:end]
	var adj []Region
	for _, r := range regions {
		if r.Row < scrollTop || r.Row >= end {
			continue
		}
		r.Row -= scrollTop
		adj = append(adj, r)
	}
	return shown, adj
}

func offsetRows(rs []Region, by int) []Region {
	out := make([]Region, len(rs))
	for i, r := range rs {
		r.Row += by
		out[i] = r
	}
	return out
}

// finishListPane puts the list into the frame. build is the tab's own walk over
// its rows: it runs twice, once to count and once to lay out only the lines the
// window keeps, because how many rows the list gets depends on how many lines
// it has and the count has to come first.
func finishListPane(
	s state.State, r ReadMarks, w Renderer, f Frame,
	build func(lineWindow) ([]string, []Region),
) Frame {
	f.Lines = appendNotice(f.Lines, s, w)
	if state.Facts[s.Tab].BlankLineAfterNotice {
		f.Lines = append(f.Lines, strings.Repeat(" ", s.Width))
	}
	counted, _ := build(countLines)
	body := s.Height - len(f.Lines) - 1
	listRows, diffRows := ListRowsFor(s, len(counted), body)
	top := clampListTop(s.ScrollTop, len(counted), listRows)
	list, listRegions := build(lineWindow{first: top, last: top + listRows, dropHiddenTargets: true})
	shown, scrolledRegions := sliceScrolled(list, listRegions, s.ScrollTop, listRows)
	f.Regions = append(f.Regions, offsetRows(scrolledRegions, len(f.Lines))...)
	f.Lines = append(f.Lines, shown...)
	f = appendDiffPanel(f, s, r, w, diffRows)
	return appendFooterAndPad(f, s, w)
}

// clampListTop is where sliceScrolled will start, computed before the list is
// laid out so the window and the slice name the same lines. sliceScrolled
// clamps the same way; asking it afterwards would be asking after the lines
// were already built or skipped.
func clampListTop(top, listLen, listRows int) int {
	if top < 0 {
		return 0
	}
	if top > listLen {
		return listLen
	}
	_ = listRows
	return top
}

// appendNotice puts what just happened on the tabs that have no summary line to
// carry it. A verb that fails and says nothing reads as a verb that did nothing.
func appendNotice(lines []string, s state.State, w Renderer) []string {
	if s.Notice == "" {
		return lines
	}
	return append(lines, w.Pad(w.Truncate(Printable(s.Notice), s.Width), "", s.Width))
}
