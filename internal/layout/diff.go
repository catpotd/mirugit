package layout

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// minDiffRows is where a diff stops being readable. Below it the area closes
// and the list takes the pane, because the list may never be cut.
const minDiffRows = 6

// diffChromeRows is the list-to-diff separator, the header, and the diff-to-
// footer separator. splitRows callers subtract it so the diff body still fits.
const diffChromeRows = 3

// CanPeekDiff reports whether diff chrome, six diff body rows, and one list row
// fit in body.
func CanPeekDiff(body int) bool {
	return body-diffChromeRows >= minDiffRows+1
}

func splitRows(listRows, height int) (list, diff int) {
	if listRows >= height {
		return height, 0
	}
	diff = height - listRows
	if diff < minDiffRows {
		return height, 0
	}
	return listRows, diff
}

// ruleLine is a full-width separator between regions that must not be confused
// with the tab underline, which mixes ━ into the same glyph.
// ruleLine draws a rule exactly width cells wide.
//
// It counts cells rather than characters because the mark it is made of is
// ambiguous width: Unicode lets the terminal choose one cell or two, and
// mirugit asks the terminal at startup which it draws. On a terminal that
// answered two, repeating the mark width times drew a rule twice as wide as
// the pane, which wrapped and pushed every row below it down by one.
func (w Renderer) ruleLine(width int) string {
	return w.ruleWithSpan(width, -1, -1)
}

// ruleWithSpan draws the rule with the columns in [heavyStart, heavyEnd) in the
// heavier mark. The span is in columns, which is what a click reports and what
// the tab bar above it was measured in.
func (w Renderer) ruleWithSpan(width, heavyStart, heavyEnd int) string {
	const light, heavy = "─", "━"
	step := max(w.Of(light), 1)
	var b strings.Builder
	col := 0
	for ; col+step <= width; col += step {
		// The whole mark has to fall inside the span. A mark is one column on
		// most terminals and two where ambiguous characters are drawn wide, and
		// a span that starts or ends on an odd column cannot be met exactly by
		// a two-column mark: this leaves the odd cell plain rather than marking
		// a column that belongs to the tab beside it.
		if col >= heavyStart && col+step <= heavyEnd {
			b.WriteString(heavy)
		} else {
			b.WriteString(light)
		}
	}
	// A pane whose width is not a multiple of the mark's own width has cells
	// left over. A space keeps the row exactly as wide as the pane; half a mark
	// is not a thing the terminal can draw.
	if col < width {
		b.WriteString(strings.Repeat(" ", width-col))
	}
	return b.String()
}

// DiffHeader names the open file, where the reader is in it, and how to leave.
func (w Renderer) DiffHeader(path string, block, blocks, unread, width int) string {
	// The path comes from the repository; Printable is where escapes stop.
	left := Printable(base(path))
	var center string
	if blocks == 0 {
		center = fmt.Sprintf("%d unread", unread)
	} else {
		center = fmt.Sprintf("block %d/%d · %d unread", block, blocks, unread)
	}
	right := "esc close"

	rightW := w.Of(right)
	centerW := w.Of(center)
	leftMax := width - rightW - centerW - 2
	if leftMax < 1 {
		leftMax = 1
	}
	if w.Of(left) > leftMax {
		left = w.Truncate(left, leftMax)
	}
	leftW := w.Of(left)

	// The drawing sets the middle five cells clear of "esc close" rather than
	// centering it, so where the reader is in the file sits next to the way out.
	const gapBeforeRight = 5
	centerStart := width - rightW - gapBeforeRight - centerW
	if centerStart < leftW+1 {
		centerStart = leftW + 1
	}
	head := left + strings.Repeat(" ", centerStart-leftW)
	tail := strings.Repeat(" ", max(width-centerStart-centerW-rightW, 0)) + right
	line := head + center + tail
	if w.Of(line) > width {
		// Too narrow for where the reader is in the file. The name and the way
		// out are what stay, and neither of them holds a mark of ours.
		return w.Pad(left, right, width)
	}
	if w.Of(line) < width {
		tail += strings.Repeat(" ", width-w.Of(line))
	}
	// Only the middle is painted. The mark it holds is the one this line drew,
	// between the block number and the unread count; the same character in a
	// file name is the repository's text, and painting it says the name holds
	// a mark of ours.
	return head + w.colorDots(center) + tail
}

func unreadBlocks(r ReadMarks, path string, blocks []git.Block) int {
	if len(blocks) == 0 {
		return 0
	}
	if !marksPresent(r) {
		return len(blocks)
	}
	n := 0
	for _, b := range blocks {
		if !r.BlockRead(path, b.Hash) {
			n++
		}
	}
	return n
}

type diffLineNumber struct {
	Old int
	New int
}

func lineNumbers(b git.Block) []diffLineNumber {
	old := b.OldStart
	new := b.NewStart
	nums := make([]diffLineNumber, len(b.Lines))
	for i, l := range b.Lines {
		switch {
		case b.LineDeleted(l):
			nums[i].Old = old
			old++
		case b.LineAdded(l):
			nums[i].New = new
			new++
		default:
			nums[i] = diffLineNumber{Old: old, New: new}
			old++
			new++
		}
	}
	return nums
}

func lineNumberWidth(nums []diffLineNumber) int {
	w := 4
	for _, num := range nums {
		if d := len(strconv.Itoa(num.Old)); num.Old > 0 && d > w {
			w = d
		}
		if d := len(strconv.Itoa(num.New)); num.New > 0 && d > w {
			w = d
		}
	}
	return w
}

func formatLineNumbers(num diffLineNumber, width int) string {
	old := strings.Repeat(" ", width+1)
	new := strings.Repeat(" ", width+1)
	if num.Old > 0 {
		old = fmt.Sprintf("%*d ", width, num.Old)
	}
	if num.New > 0 {
		new = fmt.Sprintf("%*d ", width, num.New)
	}
	return old + new
}

func (w Renderer) colorDiffLineNumbered(padded, numPlain, raw string) string {
	if !w.Enabled {
		return padded
	}
	line := w.colorDiffLine(padded, numPlain, raw)
	return replaceOnce(line, numPlain, w.Faint(numPlain))
}

func padToHeight(lines []string, want, width int) []string {
	for len(lines) < want {
		lines = append(lines, strings.Repeat(" ", width))
	}
	return lines[:want]
}

func (w Renderer) diffAreaWithoutBlocks(d git.FileDiff, lines []string, want, width int) []string {
	if len(lines) >= want {
		return padToHeight(lines, want, width)
	}
	switch {
	case d.Binary:
		lines = append(lines, w.binaryDiffLine(d, width))
	case d.TooLong:
		lines = append(lines, w.tooLongDiffLine(d, width))
	default:
		// The same rule as binaryDiffLine: the path is what identifies the row,
		// and the note beside it gives way when the two do not fit. What is cut
		// is the front of the path, because the name is at the end of it — the
		// line beside this one cuts the same way.
		left := "  " + Printable(d.Path)
		right := "mode changed"
		if w.Of(left)+1+w.Of(right) > width {
			right = ""
		}
		lines = append(lines, w.Pad(w.TruncateFront(left, width), right, width))
	}
	return padToHeight(lines, want, width)
}

func (w Renderer) blockHeadLine(b git.Block, index, total int, atCursor bool, width int) (head, figures string) {
	cursor := " "
	if atCursor {
		cursor = "▌"
	}
	head = fmt.Sprintf("    %s  block %d/%d", cursor, index+1, total)
	if w.Of(head) > width {
		head = w.Truncate(head, width)
	}
	figures = fmt.Sprintf("%s %s", state.Plus(b.Added), state.Minus(b.Deleted))
	if w.Of(head)+1+w.Of(figures) > width {
		figures = ""
	}
	return head, figures
}

func (w Renderer) stageOffer(head, figures, path string, block, row, width int) (line, figuresOnLine string, regions []Region) {
	stage := "s stage"
	stageWithFigures := stage + strings.Repeat(" ", 8) + figures
	stageOnLine := false
	switch {
	case figures != "" && w.Of(head)+1+w.Of(stageWithFigures) <= width:
		line = w.Pad(head, stageWithFigures, width)
		figuresOnLine = figures
		stageOnLine = true
	case w.Of(head)+1+w.Of(stage) <= width:
		line = w.Pad(head, stage, width)
		figuresOnLine = ""
		stageOnLine = true
	default:
		return w.Pad(head, figures, width), figures, nil
	}
	if stageOnLine {
		// The offer reads "s stage" and the whole of it is what the reader
		// aims at. Covering only the word left the key hint dead.
		stageAt := strings.LastIndex(line, stage)
		if stageAt >= 0 {
			regions = append(regions, Region{
				Target:   Target{Kind: TargetVerb, Verb: state.VerbNameStage, Path: path, Block: block},
				Row:      row,
				ColStart: w.Of(line[:stageAt]),
				ColEnd:   w.Of(line[:stageAt]) + w.Of(stage),
			})
		}
	}
	return line, figuresOnLine, regions
}

func (w Renderer) blockBodyLines(b git.Block, width, startLine, room int) []string {
	var lines []string
	nums := lineNumbers(b)
	nw := lineNumberWidth(nums)
	startLine = min(max(startLine, 0), len(b.Lines))
	for j := startLine; j < len(b.Lines); j++ {
		if len(lines) >= room {
			break
		}
		l := b.Lines[j]
		numPlain := formatLineNumbers(nums[j], nw)
		content := w.Truncate(Printable(l), width-w.Of(numPlain))
		body := numPlain + content
		lines = append(lines, w.colorDiffLineNumbered(w.Pad(body, "", width), numPlain, content))
	}
	return lines
}

func (w Renderer) DiffArea(d git.FileDiff, blockCursor, startBlock, startLine int, showStage bool, width, height int) ([]string, []Region) {
	var lines []string
	var regions []Region
	want := height

	if startBlock < 0 {
		startBlock = 0
	}

	if want > 0 {
		lines = append(lines, strings.Repeat(" ", width))
	}

	if d.Binary || d.ModeOnly || d.TooLong {
		return w.diffAreaWithoutBlocks(d, lines, want, width), nil
	}

	for i := startBlock; i < len(d.Blocks); i++ {
		b := d.Blocks[i]
		if len(lines) >= want {
			break
		}
		head, figures := w.blockHeadLine(b, i, len(d.Blocks), i == blockCursor, width)
		line := w.Pad(head, figures, width)
		figuresOnLine := figures
		if showStage && i == blockCursor {
			var stageRegions []Region
			line, figuresOnLine, stageRegions = w.stageOffer(head, figures, d.Path, i, len(lines), width)
			regions = append(regions, stageRegions...)
		}
		lines = append(lines, w.colorDiffBlockHead(line, i == blockCursor, figuresOnLine))
		regions = append(regions, Region{
			Target: Target{Kind: TargetBlock, Path: d.Path, Block: i},
			Row:    len(lines) - 1, ColStart: 4, ColEnd: w.Of(head),
		})
		bodyStart := 0
		if i == startBlock {
			bodyStart = startLine
		}
		lines = append(lines, w.blockBodyLines(b, width, bodyStart, want-len(lines))...)
		if i < len(d.Blocks)-1 && len(lines) < want {
			lines = append(lines, w.blockGap(width))
		}
	}
	return padToHeight(lines, want, width), regions
}

func DiffLineScrollFor(d git.FileDiff, block, line, height int) int {
	if d.Binary || d.ModeOnly || d.TooLong || block < 0 || block >= len(d.Blocks) {
		return 0
	}
	bodyRows := max(height-2, 0)
	if bodyRows == 0 {
		return 0
	}
	lastStart := max(len(d.Blocks[block].Lines)-bodyRows, 0)
	return min(max(line, 0), lastStart)
}

// DiffScrollFor returns the first block index to draw so the cursor block fits
// entirely in height. The count matches ShownBlockHashes so read marks agree
// with what the reader could see.
func DiffScrollFor(d git.FileDiff, scroll, cursor, height int) int {
	if d.Binary || d.ModeOnly || d.TooLong || len(d.Blocks) == 0 || height < 1 {
		return 0
	}
	if scroll < 0 {
		scroll = 0
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= len(d.Blocks) {
		cursor = len(d.Blocks) - 1
	}
	if cursor < scroll {
		return cursor
	}
	if diffAreaRows(d, cursor, cursor) > height {
		return cursor
	}
	for s := 0; s <= cursor; s++ {
		if diffAreaRows(d, s, cursor) <= height {
			return s
		}
	}
	return cursor
}

// diffAreaRows is the number of rows DiffArea takes to draw the blocks from
// fromBlock to toBlock: the blank row it opens with, then each block's head and
// lines, with a blank row between one block and the next. Counting the blocks
// alone read one row short, so the block whose last line sat one row below the
// fold was scrolled to as if it fitted and marked read as if it had been drawn.
func diffAreaRows(d git.FileDiff, fromBlock, toBlock int) int {
	return 1 + diffBlockLines(d, fromBlock, toBlock)
}

func diffBlockLines(d git.FileDiff, fromBlock, toBlock int) int {
	lines := 0
	for i := fromBlock; i <= toBlock; i++ {
		lines += 1 + len(d.Blocks[i].Lines)
		if i < toBlock {
			lines++
		}
	}
	return lines
}

// binaryDiffLine names the file and how big it is. Neither half is fitted to
// the pane on its own, and Pad lets an overlong pair overflow: measured, a pane
// of 20 columns drew "  filea.go1234567 bytes · 100644" at 32, with the two
// halves run together and the row wrapped.
//
// The size is what gives way. A reader who cannot see both still has to know
// which file this is, and the pane says nothing else about it.
// tooLongDiffLine says why the body is not on screen. Drawing nothing reads as
// a file with no changes, which is the opposite of what a diff this size means.
func (w Renderer) tooLongDiffLine(d git.FileDiff, width int) string {
	left := "  " + Printable(base(d.Path))
	right := fmt.Sprintf("%d lines · too long to show", d.Lines)
	if w.Of(left)+1+w.Of(right) > width {
		right = ""
	}
	return w.Pad(w.Truncate(left, width), right, width)
}

func (w Renderer) binaryDiffLine(d git.FileDiff, width int) string {
	left := "  " + Printable(base(d.Path))
	right := fmt.Sprintf("%d bytes · %s", d.Size, d.Mode)
	if w.Of(left)+1+w.Of(right) > width {
		right = ""
	}
	return w.Pad(w.Truncate(left, width), right, width)
}
