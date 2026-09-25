package layout

import (
	"fmt"
	"strings"
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
	"github.com/charmbracelet/x/ansi"
)

func TestSplitGivesTheDiffWhateverTheListDoesNotNeed(t *testing.T) {
	t.Parallel()
	// The pane is sized for the worst case and is mostly empty in the common
	// one: the median change is three files.
	list, diff := splitRows(5, 42)
	if list != 5 {
		t.Errorf("a short list should take only what it needs: got %d", list)
	}
	if diff != 37 {
		t.Errorf("the diff should take the rest: got %d", diff)
	}
}

func TestSplitClosesTheDiffRatherThanTruncateTheList(t *testing.T) {
	t.Parallel()
	list, diff := splitRows(102, 42)
	if diff != 0 {
		t.Errorf("want the diff closed, got %d rows", diff)
	}
	if list != 42 {
		t.Errorf("the list takes the pane and scrolls: got %d", list)
	}
}

func TestSplitClosesTheDiffBelowSixRowsBecauseFewerCannotBeRead(t *testing.T) {
	t.Parallel()
	_, diff := splitRows(38, 42)
	if diff != 0 {
		t.Errorf("want 0, got %d", diff)
	}
	_, diff = splitRows(36, 42)
	if diff != 6 {
		t.Errorf("want 6, got %d", diff)
	}
}

func TestDiffAreaLinesAreExactlyThePaneWidth(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	d := git.FileDiff{Path: "a.swift", Blocks: []git.Block{{
		Header:   "@@ -40,3 +40,5 @@",
		OldStart: 40,
		NewStart: 40,
		Lines:    []string{" }", "+    let owner = SyncOwner(deviceID: .current)", "+    #expect(true)"},
		Added:    2,
	}}}
	lines, _ := w.DiffArea(d, 0, 0, 0, true, paneWidth, 10)
	for _, l := range lines {
		if w.Of(l) != paneWidth {
			t.Errorf("%q is %d cells", l, w.Of(l))
		}
	}
}

func TestLineNumbersIncreaseMonotonicallyAcrossAnInsertion(t *testing.T) {
	t.Parallel()
	b := git.Block{
		Header:   "@@ -2,2 +2,4 @@",
		OldStart: 2,
		NewStart: 2,
		Lines:    []string{" ctx", "+one", "+two", " ctx2"},
	}
	got := lineNumbers(b)
	want := []diffLineNumber{{Old: 2, New: 2}, {New: 3}, {New: 4}, {Old: 3, New: 5}}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestLineNumbersForANewFileStartAtOne(t *testing.T) {
	t.Parallel()
	b := git.Block{
		OldStart: 0,
		NewStart: 1,
		Lines:    []string{"+---", "+name: app-ui-critique", "+日本語の説明"},
	}
	got := lineNumbers(b)
	want := []diffLineNumber{{New: 1}, {New: 2}, {New: 3}}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestDiffAreaKeepsLineNumbersAfterScrollingWithinABlock(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	d := git.FileDiff{Path: "a.txt", Blocks: []git.Block{{
		OldStart: 0,
		NewStart: 1,
		Lines:    []string{"+---", "+one", "+two", "+three"},
	}}}
	lines, _ := w.DiffArea(d, 0, 0, 2, false, paneWidth, 4)
	if !strings.HasPrefix(lines[2], "        3 +two") {
		t.Errorf("got %q, want the third file line", lines[2])
	}
	if strings.Contains(strings.Join(lines, "\n"), "+---") {
		t.Error("the first line remained visible after it was scrolled past")
	}
}

func TestDiffLineScrollDoesNotMoveWithoutABodyRow(t *testing.T) {
	t.Parallel()
	d := git.FileDiff{Blocks: []git.Block{{Lines: []string{"+one", "+two"}}}}
	for _, height := range []int{0, 1, 2} {
		if got := DiffLineScrollFor(d, 0, 1, height); got != 0 {
			t.Errorf("height %d moved to line %d, want 0", height, got)
		}
	}
}

func TestDiffBodyLineNumbersUseFourDigitField(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	d := git.FileDiff{Path: "a.swift", Blocks: []git.Block{{
		OldStart: 40,
		NewStart: 40,
		Lines:    []string{" }"},
	}}}
	lines, _ := w.DiffArea(d, 0, 0, 0, false, paneWidth, 3)
	if len(lines) < 3 {
		t.Fatalf("got %d lines", len(lines))
	}
	if !strings.HasPrefix(lines[2], "  40   40 ") {
		t.Errorf("got %q, want two four-digit line number fields", lines[2])
	}
}

func TestDiffBodyLineNumbersGrowWithoutTruncatingContent(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	d := git.FileDiff{Path: "a.swift", Blocks: []git.Block{{
		OldStart: 10000,
		NewStart: 10000,
		Lines:    []string{"+overflow"},
		Added:    1,
	}}}
	lines, _ := w.DiffArea(d, 0, 0, 0, false, paneWidth, 3)
	if len(lines) < 3 {
		t.Fatalf("got %d lines", len(lines))
	}
	if !strings.Contains(lines[2], "+overflow") {
		t.Errorf("content was truncated: %q", lines[2])
	}
	if got := w.Of(lines[2]); got != paneWidth {
		t.Errorf("line is %d cells, want %d: %q", got, paneWidth, lines[2])
	}
}

func TestLineNumbersPutRemovalsOnTheOldSide(t *testing.T) {
	t.Parallel()
	b := git.Block{
		Header:   "@@ -10,1 +10,1 @@",
		OldStart: 10,
		NewStart: 10,
		Lines:    []string{"-old", "+new"},
	}
	got := lineNumbers(b)
	if got[0] != (diffLineNumber{Old: 10}) {
		t.Errorf("removal = %+v, want 10 on the old side", got[0])
	}
	if got[1] != (diffLineNumber{New: 10}) {
		t.Errorf("addition = %+v, want 10 on the new side", got[1])
	}
}

func TestDiffAreaShowsOldAndNewLineNumbers(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	d := git.FileDiff{Path: "a.txt", Blocks: []git.Block{{
		OldStart: 9,
		NewStart: 10,
		Lines:    []string{" context", "-old", "+new"},
	}}}

	lines, _ := w.DiffArea(d, 0, 0, 0, false, paneWidth, 5)
	for index, want := range []string{
		"   9   10  context",
		"  10      -old",
		"       11 +new",
	} {
		if !strings.HasPrefix(lines[index+2], want) {
			t.Errorf("line %d = %q, want prefix %q", index, lines[index+2], want)
		}
	}
}

func TestDiffAreaBodyLinesAreExactlyPaneWidthWithColor(t *testing.T) {
	t.Parallel()
	w := Renderer{Palette: Palette{Enabled: true}}
	d := git.FileDiff{Path: "a.swift", Blocks: []git.Block{{
		Header:   "@@ -40,3 +40,5 @@",
		OldStart: 40,
		NewStart: 40,
		Lines:    []string{" }", "+    let owner = SyncOwner(deviceID: .current)", "+    #expect(true)"},
		Added:    2,
	}}}
	lines, _ := w.DiffArea(d, 0, 0, 0, true, paneWidth, 10)
	for i, l := range lines {
		plain := ansi.Strip(l)
		if got := w.Of(plain); got != paneWidth {
			t.Errorf("line %d is %d cells after stripping color: %q", i, got, plain)
		}
	}
}

func TestDiffAreaSeparatesBlocksWithABlankLine(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	d := git.FileDiff{Path: "a.swift", Blocks: []git.Block{
		{Header: "@@ -1,1 +1,2 @@", OldStart: 1, NewStart: 1, Lines: []string{" a", "+b"}, Added: 1},
		{Header: "@@ -9,1 +10,2 @@", OldStart: 9, NewStart: 10, Lines: []string{" c", "+d"}, Added: 1},
	}}
	lines, _ := w.DiffArea(d, 0, 0, 0, true, paneWidth, 20)
	var blanks int
	for _, l := range lines {
		if strings.TrimSpace(l) == "" && w.Of(l) == paneWidth {
			blanks++
		}
	}
	if blanks < 1 {
		t.Errorf("want a blank line between blocks, got %d", blanks)
	}
}

func TestDiffBlockHeadingWithoutCursorStartsAtSevenSpaces(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	d := git.FileDiff{Path: "a.swift", Blocks: []git.Block{
		{Header: "@@ -1,1 +1,2 @@", Lines: []string{" a", "+b"}, Added: 2, Hash: "h1"},
		{Header: "@@ -9,1 +10,2 @@", Lines: []string{" c", "+d"}, Added: 5, Deleted: 1, Hash: "h2"},
	}}
	lines, _ := w.DiffArea(d, 1, 0, 0, true, paneWidth, 20)
	if !strings.HasPrefix(lines[1], "       block 1/2") {
		t.Errorf("got %q, want seven spaces before the block heading", lines[1])
	}
}

func TestDiffAreaMarksTheBlockUnderTheCursor(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	d := git.FileDiff{Path: "a.swift", Blocks: []git.Block{
		{Header: "@@ -1,2 +1,3 @@", Lines: []string{" a", "+b"}, Added: 1},
		{Header: "@@ -9,2 +10,3 @@", Lines: []string{" c", "+d"}, Added: 1},
	}}
	lines, _ := w.DiffArea(d, 1, 0, 0, true, paneWidth, 20)

	var marked int
	for _, l := range lines {
		if len([]rune(l)) > 4 && string([]rune(l)[4]) == "▌" {
			marked++
		}
	}
	if marked != 1 {
		t.Errorf("want exactly one marked block header, got %d", marked)
	}
}

func TestDiffAreaOfABinaryFileShowsSizeAndMode(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	d := git.FileDiff{Path: "img.png", Binary: true, Size: 12345, Mode: "100644"}
	lines, regions := w.DiffArea(d, 0, 0, 0, false, paneWidth, 2)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	if got := w.Of(lines[1]); got != paneWidth {
		t.Fatalf("binary line is %d cells, want %d: %q", got, paneWidth, lines[1])
	}
	if !strings.Contains(lines[1], "12345 bytes") || !strings.Contains(lines[1], "100644") {
		t.Errorf("got %q", lines[1])
	}
	for _, r := range regions {
		if r.Target.Kind == TargetBlock {
			t.Error("a binary file has no block to stage")
		}
		if r.Target.Kind == TargetVerb && r.Target.Verb == state.VerbNameStage {
			t.Error("a binary line should not offer block stage")
		}
	}
}

func TestDiffBlockHeadStageUsesVerbColor(t *testing.T) {
	t.Parallel()
	w := Renderer{Palette: Palette{Enabled: true}}
	d := git.FileDiff{Path: "a.swift", Blocks: []git.Block{
		{Header: "@@ -1,2 +1,3 @@", Lines: []string{" a", "+b"}, Added: 1},
	}}
	lines, _ := w.DiffArea(d, 0, 0, 0, true, paneWidth, 10)
	var head string
	for _, l := range lines {
		if strings.Contains(l, "block 1/1") {
			head = l
			break
		}
	}
	if head == "" {
		t.Fatal("want block head line")
	}
	if !strings.Contains(head, w.Verb("stage")) {
		t.Errorf("stage should use Verb color: %q", head)
	}
}

func TestDiffAreaOffersBlockStageOnUnstagedFiles(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	d := git.FileDiff{Path: "a.swift", Blocks: []git.Block{
		{Header: "@@ -1,2 +1,3 @@", Lines: []string{" a", "+b"}, Added: 1},
	}}
	_, regions := w.DiffArea(d, 0, 0, 0, true, paneWidth, 10)
	found := false
	for _, r := range regions {
		if r.Target.Kind == TargetVerb && r.Target.Verb == state.VerbNameStage {
			found = true
		}
	}
	if !found {
		t.Fatal("unstaged blocks should offer stage")
	}
}

func TestNarrowBlockHeadDropsTheFiguresBeforeCuttingThem(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	d := git.FileDiff{Path: "a.swift", Blocks: []git.Block{
		{Header: "@@ -1,2 +1,3 @@", Lines: []string{" a", "+b"}, Added: 1, Deleted: 1},
	}}
	lines, regions := w.DiffArea(d, 0, 0, 0, true, 34, 6)
	if len(lines) < 2 {
		t.Fatalf("got %d lines, want at least 2", len(lines))
	}
	line := lines[1]
	if got := w.Of(line); got != 34 {
		t.Fatalf("got %d cells, want 34: %q", got, line)
	}
	if !strings.Contains(line, "s stage") {
		t.Fatalf("stage offer should remain: %q", line)
	}
	var stageRegion bool
	for _, r := range regions {
		if r.Target.Kind == TargetVerb && r.Target.Verb == state.VerbNameStage {
			stageRegion = true
		}
	}
	if !stageRegion {
		t.Fatal("want TargetVerb stage region")
	}
	if strings.Contains(line, "+1") {
		t.Errorf("figures should be dropped: %q", line)
	}
}

func TestNarrowBlockHeadDropsTheStageOfferWhenEvenItCannotFit(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	d := git.FileDiff{Path: "a.swift", Blocks: []git.Block{
		{Header: "@@ -1,2 +1,3 @@", Lines: []string{" a", "+b"}, Added: 1, Deleted: 1},
	}}
	lines, regions := w.DiffArea(d, 0, 0, 0, true, 20, 6)
	if len(lines) < 2 {
		t.Fatalf("got %d lines, want at least 2", len(lines))
	}
	line := lines[1]
	if got := w.Of(line); got != 20 {
		t.Fatalf("got %d cells, want 20: %q", got, line)
	}
	if !strings.Contains(line, "block 1/1") {
		t.Fatalf("head should remain: %q", line)
	}
	if strings.Contains(line, "s stage") {
		t.Fatalf("stage should be dropped: %q", line)
	}
	for _, r := range regions {
		if r.Target.Kind == TargetVerb && r.Target.Verb == state.VerbNameStage {
			t.Fatal("unexpected stage verb region")
		}
	}
	var blockRegion bool
	for _, r := range regions {
		if r.Target.Kind == TargetBlock {
			blockRegion = true
		}
	}
	if !blockRegion {
		t.Fatal("want TargetBlock region")
	}
}

func TestDiffAreaOmitsBlockStageOnStagedFiles(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	d := git.FileDiff{Path: "a.swift", Blocks: []git.Block{
		{Header: "@@ -1,2 +1,3 @@", Lines: []string{" a", "+b"}, Added: 1},
	}}
	_, regions := w.DiffArea(d, 0, 0, 0, false, paneWidth, 10)
	for _, r := range regions {
		if r.Target.Kind == TargetVerb && r.Target.Verb == state.VerbNameStage {
			t.Error("staged file blocks should not offer stage")
		}
	}
}

func TestDiffAreaStartsAtTheScrolledBlock(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	blocks := make([]git.Block, 3)
	for i := range blocks {
		blocks[i] = git.Block{
			Header: fmt.Sprintf("@@ block %d @@", i+1),
			Lines:  []string{"+a", "+b"},
			Added:  2,
		}
	}
	d := git.FileDiff{Path: "a.txt", Blocks: blocks}
	lines, _ := w.DiffArea(d, 2, 2, 0, false, paneWidth, 10)
	hasThird := false
	hasFirst := false
	for _, line := range lines {
		if strings.Contains(line, "block 3/3") {
			hasThird = true
		}
		if strings.Contains(line, "block 1/3") {
			hasFirst = true
		}
	}
	if !hasThird {
		t.Fatal("want block 3/3 when starting at scroll 2")
	}
	if hasFirst {
		t.Fatal("block 1/3 should not appear when starting at scroll 2")
	}
}

func TestDiffScrollForKeepsTheCursorBlockFullyDrawn(t *testing.T) {
	t.Parallel()
	blocks := make([]git.Block, 3)
	for i := range blocks {
		blocks[i] = git.Block{
			Lines: []string{"+a", "+b", "+c"},
			Added: 3,
		}
	}
	d := git.FileDiff{Blocks: blocks}
	if got := DiffScrollFor(d, 0, 2, 10); got != 1 {
		t.Errorf("DiffScrollFor(d, 0, 2, 10) = %d, want 1", got)
	}
	if got := DiffScrollFor(d, 2, 0, 10); got != 0 {
		t.Errorf("DiffScrollFor(d, 2, 0, 10) = %d, want 0", got)
	}
	if got := DiffScrollFor(d, 0, 1, 10); got != 0 {
		t.Errorf("DiffScrollFor(d, 0, 1, 10) = %d, want 0", got)
	}
	// A pane with no room for the cursor's block still starts at it: the
	// window follows the reader, and the answer is kept for the moment the
	// pane grows. Starting at the top instead hands the reader back to the
	// first block, one they may have scrolled a long way past.
	for height := 1; height < 5; height++ {
		if got := DiffScrollFor(d, 0, 2, height); got != 2 {
			t.Errorf("DiffScrollFor(d, 0, 2, %d) = %d, want 2", height, got)
		}
	}
}

func TestDiffScrollForDrawsAnOversizedBlockFromItsHead(t *testing.T) {
	t.Parallel()
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = "+x"
	}
	d := git.FileDiff{Blocks: []git.Block{{Lines: lines}}}
	if got := DiffScrollFor(d, 0, 0, 10); got != 0 {
		t.Errorf("got %d, want 0 to draw the head of an oversized block", got)
	}
}

// The rule's mark is ambiguous width, so the terminal decides whether it is one
// cell or two, and mirugit asks it at startup. Both answers have to draw a rule
// exactly as wide as the pane: measured, the two-cell answer drew one twice the
// width of the pane, and every row below it moved down by one.
func TestRuleLineIsExactlyPaneWidth(t *testing.T) {
	t.Parallel()
	for _, eastAsian := range []bool{false, true} {
		w := Renderer{Widths: Widths{EastAsian: eastAsian}}
		for width := minPaneWidth; width <= 90; width++ {
			line := w.ruleLine(width)
			if got := w.Of(line); got != width {
				t.Errorf("eastAsian=%v width %d: got %d cells: %q",
					eastAsian, width, got, line)
			}
			if !strings.HasPrefix(line, "─") {
				t.Errorf("eastAsian=%v width %d: got %q", eastAsian, width, line)
			}
		}
	}
}

// The heavier stretch marks which tab is open, and where it starts and ends is
// in columns — the same units a click reports. Marking it by character index
// put it under the wrong tab wherever the mark was not one cell.
func TestTheTabRuleMarksTheColumnsItWasGiven(t *testing.T) {
	t.Parallel()
	for _, eastAsian := range []bool{false, true} {
		w := Renderer{Widths: Widths{EastAsian: eastAsian}}
		for width := minPaneWidth; width <= 90; width++ {
			for _, span := range [][2]int{{10, 20}, {1, 11}, {7, 18}, {0, 3}} {
				start, end := span[0], span[1]
				line := w.ruleWithSpan(width, start, end)
				if got := w.Of(line); got != width {
					t.Errorf("eastAsian=%v width %d: got %d cells: %q",
						eastAsian, width, got, line)
				}
				col := 0
				w.eachCluster(line, func(cluster string, cells, _ int) bool {
					heavy := cluster == "━"
					if want := col >= start && col+cells <= end; heavy != want {
						t.Errorf("eastAsian=%v width %d span [%d,%d) column %d: heavy=%v, want %v",
							eastAsian, width, start, end, col, heavy, want)
					}
					col += cells
					return true
				})
			}
		}
	}
}

func TestDiffHeaderIsExactlyPaneWidth(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	line := w.DiffHeader("app/SyncOwnershipTests.swift", 1, 6, 5, paneWidth)
	if got := w.Of(line); got != paneWidth {
		t.Errorf("got %d cells, want %d: %q", got, paneWidth, line)
	}
	if !strings.Contains(line, "SyncOwnershipTests.swift") {
		t.Errorf("got %q", line)
	}
	if !strings.Contains(line, "block 1/6 · 5 unread") {
		t.Errorf("got %q", line)
	}
	if !strings.Contains(line, "esc close") {
		t.Errorf("got %q", line)
	}
	if strings.Contains(line, "fold") {
		t.Errorf("diff header must not contain fold: %q", line)
	}
}

func TestDiffHeaderSurvivesEveryPaneWidth(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	for width := 1; width <= 80; width++ {
		line := w.DiffHeader("src/very/long/path/name.go", 1, 2, 3, width)
		if width >= 40 && w.Of(line) != width {
			t.Fatalf("width %d: %q is %d cells, want %d", width, line, w.Of(line), width)
		}
	}
}

func TestDiffHeaderOmitsBlockCounterWhenThereAreNoBlocks(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	line := w.DiffHeader("f.txt", 1, 0, 0, paneWidth)
	if strings.Contains(line, "1/0") {
		t.Errorf("got %q", line)
	}
	if !strings.Contains(line, "0 unread") {
		t.Errorf("got %q", line)
	}
	if !strings.Contains(line, "f.txt") {
		t.Errorf("got %q", line)
	}
}

func TestDiffHeaderWithNoBlocksIsExactlyPaneWidth(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	line := w.DiffHeader("f.txt", 1, 0, 0, paneWidth)
	if got := w.Of(line); got != paneWidth {
		t.Errorf("got %d cells, want %d: %q", got, paneWidth, line)
	}
}

func TestPaneWithOpenDiffIncludesTwoRulesAndAHeader(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := state.State{Width: paneWidth, Height: 53, Head: git.Head{Branch: "main"}, Rows: []state.Row{
		state.FileRow(git.Entry{Path: "app/a.swift", Worktree: git.Modified}, state.SectionUnstaged),
	}, Changes: state.Changes{Folded: map[string]bool{}, Selected: map[string]bool{}}, Open: state.OpenDiff{Path: "app/a.swift", Diff: git.FileDiff{Path: "app/a.swift", Blocks: []git.Block{
		{Header: "@@ -1,2 +1,3 @@", Lines: []string{" a", "+b"}, Added: 1, Hash: "h1"},
		{Header: "@@ -9,2 +10,3 @@", Lines: []string{" c", "+d"}, Added: 1, Hash: "h2"},
	}}}}
	f := Pane(s, nil, w)
	if len(f.Lines) != 53 {
		t.Fatalf("got %d lines, want 53", len(f.Lines))
	}

	var rules, headers int
	for _, line := range f.Lines {
		if w.Of(line) == paneWidth && strings.TrimSpace(line) != "" &&
			strings.Trim(line, "─") == "" && !strings.Contains(line, "━") {
			rules++
		}
		if strings.Contains(line, "esc close") {
			headers++
		}
	}
	if rules < 2 {
		t.Errorf("want at least two diff rules, got %d", rules)
	}
	if headers != 1 {
		t.Errorf("want one diff header, got %d", headers)
	}
}

func TestPaneWithClosedDiffOmitsDiffChrome(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := state.State{Width: paneWidth, Height: 53, Head: git.Head{Branch: "main"}, Rows: []state.Row{
		state.FileRow(git.Entry{Path: "app/a.swift", Worktree: git.Modified}, state.SectionUnstaged),
	}, Changes: state.Changes{Folded: map[string]bool{}, Selected: map[string]bool{}}}
	f := Pane(s, nil, w)
	for _, line := range f.Lines {
		if strings.Contains(line, "esc close") {
			t.Errorf("closed diff should not draw a header: %q", line)
		}
	}
}

func TestTruncatedDiffAddLineKeepsAddColor(t *testing.T) {
	t.Parallel()
	w := Renderer{Palette: Palette{Enabled: true}}
	longPlus := "+" + strings.Repeat("x", 80)
	d := git.FileDiff{Path: "a.go", Blocks: []git.Block{{
		OldStart: 1,
		NewStart: 1,
		Lines:    []string{longPlus},
		Added:    1,
	}}}
	lines, _ := w.DiffArea(d, 0, 0, 0, false, paneWidth, 3)
	if len(lines) < 3 {
		t.Fatalf("got %d lines", len(lines))
	}
	body := lines[2]
	plain := ansi.Strip(body)
	numEnd := strings.Index(plain, "+")
	if numEnd < 0 {
		t.Fatalf("no + in %q", plain)
	}
	truncated := strings.TrimSpace(plain[numEnd:])
	if !strings.Contains(body, w.Add(truncated)) {
		t.Errorf("truncated + line lost Add color: %q", body)
	}
}

// A diff too long to hold is drawn as a line saying so. Drawing nothing reads as
// a file with no changes, which is the opposite of what a diff this size means.
func TestDiffAreaOfATooLongDiffSaysHowLongItIs(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	d := git.FileDiff{Path: "pkg/generated.go", TooLong: true, Lines: 800000}
	lines, regions := w.DiffArea(d, 0, 0, 0, false, paneWidth, 2)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	if got := w.Of(lines[1]); got != paneWidth {
		t.Fatalf("the line is %d cells, want %d: %q", got, paneWidth, lines[1])
	}
	if !strings.Contains(lines[1], "800000 lines") {
		t.Errorf("the reader is not told how long it is: %q", lines[1])
	}
	if !strings.Contains(lines[1], "generated.go") {
		t.Errorf("the reader is not told which file: %q", lines[1])
	}
	for _, r := range regions {
		if r.Target.Kind == TargetBlock {
			t.Error("a diff with no blocks held offers a block to stage")
		}
	}
}

// The note beside a mode-only path gives way when the two do not fit, and a
// pane exactly wide enough for both fits. One cell narrower is where it goes.
// Getting that boundary wrong drops the note from a pane that had room for it,
// and the row then says only that the file changed, not that the change was
// its mode.
func TestTheModeNoteStaysOnAPaneExactlyWideEnough(t *testing.T) {
	t.Parallel()
	var w Renderer
	d := git.FileDiff{Path: "a.txt", ModeOnly: true}
	const note = "mode changed"
	exact := w.Of("  "+Printable(d.Path)) + 1 + w.Of(note)

	for _, c := range []struct {
		name  string
		width int
		want  bool
	}{
		{"a cell wider than both need", exact + 1, true},
		{"exactly wide enough for both", exact, true},
		{"a cell too narrow", exact - 1, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			lines := w.diffAreaWithoutBlocks(d, nil, 1, c.width)
			if len(lines) != 1 {
				t.Fatalf("drew %d lines, want 1: %q", len(lines), lines)
			}
			if got := strings.Contains(lines[0], note); got != c.want {
				t.Errorf("the note is %v on a pane %d wide, want %v: %q",
					got, c.width, c.want, lines[0])
			}
		})
	}
}

// The stage offer keeps the block's own figures beside it while both fit, and a
// pane exactly wide enough for both fits. One cell narrower drops the figures
// and keeps the offer. Getting that boundary wrong takes the figures off a pane
// that had room, and the reader loses the count of what pressing s would stage.
func TestTheStageOfferKeepsTheFiguresOnAPaneExactlyWideEnough(t *testing.T) {
	t.Parallel()
	var w Renderer
	const head = "  @@ -1 +1 @@"
	const figures = "+1 −0"
	exact := w.Of(head) + 1 + w.Of("s stage"+strings.Repeat(" ", 8)+figures)

	for _, c := range []struct {
		name  string
		width int
		want  string
	}{
		{"a cell wider than both need", exact + 1, figures},
		{"exactly wide enough for both", exact, figures},
		{"a cell too narrow for both", exact - 1, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			line, onLine, _ := w.stageOffer(head, figures, "a.txt", 0, 0, c.width)
			if onLine != c.want {
				t.Errorf("the figures beside the offer are %q, want %q: %q", onLine, c.want, line)
			}
			if !strings.Contains(line, "s stage") {
				t.Errorf("the offer itself is not on the line: %q", line)
			}
		})
	}
}

// A block's head carries its number and the figures carry what it changes. The
// figures give way when the two do not fit, and a pane exactly wide enough for
// both fits. One cell narrower is where they go.
func TestABlockKeepsItsFiguresOnAPaneExactlyWideEnough(t *testing.T) {
	t.Parallel()
	var w Renderer
	b := git.Block{Header: "@@ -1 +1 @@", Lines: []string{"+x"}, Added: 1}
	headOf := func(width int) string {
		head, _ := w.blockHeadLine(b, 0, 3, false, width)
		return head
	}
	figures := state.Plus(1) + " " + state.Minus(0)
	exact := w.Of(headOf(200)) + 1 + w.Of(figures)

	for _, c := range []struct {
		name  string
		width int
		want  bool
	}{
		{"a cell wider than both need", exact + 1, true},
		{"exactly wide enough for both", exact, true},
		{"a cell too narrow", exact - 1, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			_, got := w.blockHeadLine(b, 0, 3, false, c.width)
			if (got != "") != c.want {
				t.Errorf("the figures are %q on a pane %d wide, want them %v",
					got, c.width, c.want)
			}
		})
	}
}

// The diff area fills the rows it was given and stops. Reading the bound one
// row late writes a row past the end of the panel, and every row under the
// panel shifts by one.
func TestTheDiffAreaStopsAtTheRowsItWasGiven(t *testing.T) {
	t.Parallel()
	var w Renderer
	blocks := make([]git.Block, 8)
	for i := range blocks {
		blocks[i] = git.Block{Header: "@@ -1 +1 @@", Lines: []string{"+a", "+b"},
			Hash: string(rune('a' + i))}
	}
	d := git.FileDiff{Path: "a.txt", Blocks: blocks}

	for want := 0; want <= 12; want++ {
		area, regions := w.DiffArea(d, 0, 0, 0, false, 60, want)
		if len(area) != want {
			t.Errorf("the area drew %d rows, want the %d it was given", len(area), want)
		}
		// The rows are padded out to the height either way, so a bound read one
		// row late shows up in the regions rather than in the count: a region
		// sits on a row the area did not draw, and a click there acts on a block
		// that is not on the screen.
		for _, r := range regions {
			if r.Row >= len(area) {
				t.Errorf("a region sits on row %d of an area %d rows tall: %+v",
					r.Row, len(area), r.Target)
			}
		}
	}
}

// A diff with no body to show — too long, or binary — says so on one row: the
// file's name on the left, how big it is on the right. The two are drawn with a
// cell between them, so a pane exactly that wide holds both. Dropping the right
// half one cell early leaves a reader with a file they cannot find out anything
// about, on the one row that was supposed to tell them.
func TestTheRowForADiffWithNoBodyKeepsItsRightHalfAtTheWidthThatFits(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	for _, c := range []struct {
		name  string
		draw  func(int) string
		right string
	}{
		{"too long to show", func(width int) string {
			return w.tooLongDiffLine(git.FileDiff{Path: "pkg/dir/file.go", Lines: 4200}, width)
		}, "4200 lines · too long to show"},
		{"binary", func(width int) string {
			return w.binaryDiffLine(git.FileDiff{
				Path: "pkg/dir/file.go", Size: 1234567, Mode: "100644"}, width)
		}, "1234567 bytes · 100644"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			fits := w.Of("  file.go") + 1 + w.Of(c.right)
			if got := c.draw(fits); !strings.Contains(got, c.right) {
				t.Errorf("at %d columns the row is %q, and %q fits", fits, got, c.right)
			}
			if got := c.draw(fits - 1); strings.Contains(got, c.right) {
				t.Errorf("at %d columns the row is %q, and %q does not fit",
					fits-1, got, c.right)
			}
			for _, width := range []int{fits - 1, fits, fits + 1} {
				if got := w.Of(c.draw(width)); got != width {
					t.Errorf("the row drawn for %d columns is %d wide", width, got)
				}
			}
		})
	}
}

// The reader standing on the block the window already starts at is not a reason
// to stop there: the area holds what fits, so the blocks above the cursor are
// drawn while there is room for them. Treating that as "the cursor is above the
// window" pins the window to the cursor block and the reader loses the context
// they had a moment ago, on the one block they are reading.
func TestDiffScrollForFillsTheAreaAboveTheCursorBlock(t *testing.T) {
	t.Parallel()
	blocks := make([]git.Block, 3)
	for i := range blocks {
		blocks[i] = git.Block{Lines: []string{"+a", "+b", "+c"}, Added: 3}
	}
	d := git.FileDiff{Blocks: blocks}
	// With the blank row the area opens with, two blocks and the line between
	// them are 10 rows; three are 15.
	if got := DiffScrollFor(d, 2, 2, 10); got != 1 {
		t.Errorf("DiffScrollFor(d, 2, 2, 10) = %d, want 1", got)
	}
	if got := DiffScrollFor(d, 1, 1, 10); got != 0 {
		t.Errorf("DiffScrollFor(d, 1, 1, 10) = %d, want 0", got)
	}
	// Room for all three, so the window starts at the top wherever it was.
	if got := DiffScrollFor(d, 2, 2, 15); got != 0 {
		t.Errorf("DiffScrollFor(d, 2, 2, 15) = %d, want 0", got)
	}
}

// A diff with nothing to draw has no block to scroll to, and a pane with no
// room has nowhere to draw one. Both answer zero before the search begins: the
// search reads the block the cursor is on, and there is no such block.
func TestDiffScrollForAnswersZeroWhenThereIsNothingToShow(t *testing.T) {
	t.Parallel()
	blocks := []git.Block{{Lines: []string{"+a"}}, {Lines: []string{"+b"}}}
	for _, c := range []struct {
		name string
		d    git.FileDiff
		h    int
	}{
		{"no blocks at all", git.FileDiff{}, 10},
		{"no blocks and no room", git.FileDiff{}, 0},
		{"blocks but no room", git.FileDiff{Blocks: blocks}, 0},
		{"blocks but negative room", git.FileDiff{Blocks: blocks}, -3},
		{"a binary file", git.FileDiff{Binary: true, Blocks: blocks}, 10},
		{"a file whose mode changed", git.FileDiff{ModeOnly: true, Blocks: blocks}, 10},
		{"a diff too long to show", git.FileDiff{TooLong: true, Blocks: blocks}, 10},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			for _, scroll := range []int{-2, 0, 1, 9} {
				for _, cursor := range []int{-2, 0, 1, 9} {
					if got := DiffScrollFor(c.d, scroll, cursor, c.h); got != 0 {
						t.Errorf("scroll %d cursor %d answered %d, want 0",
							scroll, cursor, got)
					}
				}
			}
		})
	}
}

// The block the reader is on is marked in the gutter, and the mark is painted
// where color is on. Painting the others instead points the reader at a block
// they are not on: the mark and the paint have to name the same block, and only
// one block at a time.
func TestTheCursorBlockIsTheOneWhoseMarkIsPainted(t *testing.T) {
	t.Parallel()
	w := Renderer{Palette: Palette{Enabled: true, Theme: slate}}
	blocks := make([]git.Block, 3)
	for i := range blocks {
		blocks[i] = git.Block{Header: "@@", Lines: []string{"+a"}, Added: 1}
	}
	d := git.FileDiff{Path: "a.txt", Blocks: blocks}

	for cursor := range blocks {
		lines, _ := w.DiffArea(d, cursor, 0, 0, false, paneWidth, 20)
		painted := 0
		at := -1
		for i, line := range lines {
			if !strings.Contains(line, w.Add(cursorMark)) {
				continue
			}
			painted++
			at = i
		}
		if painted != 1 {
			t.Errorf("cursor on block %d: %d heads are painted, want one", cursor, painted)
			continue
		}
		// The painted head is the one that also carries the block's number.
		if want := fmt.Sprintf("block %d/%d", cursor+1, len(blocks)); !strings.Contains(lines[at], want) {
			t.Errorf("cursor on block %d: the painted head is %q, want %q",
				cursor, lines[at], want)
		}
	}
}

// The gap goes between blocks, so a diff of N blocks draws N-1 of them. A gap
// after the last block says another follows: on a theme that marks the gap it
// is a drawn line, and the reader is told to scroll for a block that is not
// there.
func TestTheGapGoesBetweenBlocksAndNotAfterTheLast(t *testing.T) {
	t.Parallel()
	w := Renderer{Palette: Palette{Enabled: true, Theme: ThemeByName("cozmic")}}
	mark := w.theme().BlockGap
	if mark == "" {
		t.Fatal("this theme draws no gap, so this proves nothing")
	}
	for _, count := range []int{1, 2, 3, 5} {
		blocks := make([]git.Block, count)
		for i := range blocks {
			blocks[i] = git.Block{Header: "@@", Lines: []string{"+x"}, Added: 1}
		}
		d := git.FileDiff{Path: "a.txt", Blocks: blocks}
		// Tall enough that every block and every gap between them fits.
		lines, _ := w.DiffArea(d, 0, 0, 0, false, paneWidth, 4*count+4)
		gaps := 0
		for _, line := range lines {
			if strings.Contains(line, mark) {
				gaps++
			}
		}
		if gaps != count-1 {
			t.Errorf("%d blocks drew %d gaps, want %d", count, gaps, count-1)
		}
	}
}

// A block's body is drawn into the rows that are left, and none are left when
// the head used the last one. Drawing a line anyway pushes the area one row
// past the pane, and the row that then falls off the bottom is the next
// block's head: the reader loses a block rather than a line of one.
func TestABlockBodyDrawsNothingWithoutRoom(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	block := git.Block{Header: "@@ -1,3 +1,3 @@",
		Lines: []string{"+a", "+b", "+c"}, Added: 3}
	for _, room := range []int{-2, -1, 0} {
		if got := w.blockBodyLines(block, paneWidth, 0, room); len(got) != 0 {
			t.Errorf("with room for %d rows the body drew %d: %q", room, len(got), got)
		}
	}
	// The rows that are there are used, so the answers above are not "never".
	for _, room := range []int{1, 2, 3, 9} {
		want := min(room, len(block.Lines))
		if got := w.blockBodyLines(block, paneWidth, 0, room); len(got) != want {
			t.Errorf("with room for %d rows the body drew %d, want %d", room, len(got), want)
		}
	}
}

// The cursor names a block, and the list it names into can shrink under it: a
// reload replaces the diff while the reader is part way down it. A cursor at
// the end of the old list is one past the end of the new one, and reading the
// block there reaches past the list.
func TestTheDiffCursorIsBroughtBackInsideTheBlocksItHas(t *testing.T) {
	t.Parallel()
	blocks := []git.Block{
		{Lines: []string{"+a"}}, {Lines: []string{"+b"}}, {Lines: []string{"+c"}},
	}
	d := git.FileDiff{Blocks: blocks}
	for _, cursor := range []int{len(blocks), len(blocks) + 1, len(blocks) + 99} {
		for _, scroll := range []int{0, 1, len(blocks)} {
			got := DiffScrollFor(d, scroll, cursor, 20)
			if got < 0 || got >= len(blocks) {
				t.Errorf("a cursor of %d answered block %d, which the diff does not have",
					cursor, got)
			}
		}
	}
}

// The offer to stage a block is drawn when the head and it fit side by side
// with a cell between them. The narrowest pane that holds both is one where
// the reader can still stage; refusing it there takes the key away from the
// pane where it is hardest to reach any other way.
func TestTheStageOfferIsDrawnAtTheNarrowestPaneThatHoldsIt(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	const stage = "s stage"
	head := "    ▌  block 1/1"
	fits := w.Of(head) + 1 + w.Of(stage)

	line, _, regions := w.stageOffer(head, "", "a.txt", 0, 0, fits)
	if !strings.Contains(line, stage) {
		t.Errorf("at %d columns the row is %q, and the offer fits", fits, line)
	}
	if len(regions) == 0 {
		t.Errorf("at %d columns the offer has no click region", fits)
	}
	if narrower, _, _ := w.stageOffer(head, "", "a.txt", 0, 0, fits-1); strings.Contains(narrower, stage) {
		t.Errorf("at %d columns the row is %q, and the offer does not fit",
			fits-1, narrower)
	}
	// From the width that holds the head upwards, the row is exactly the width
	// it was given. Below that Pad lets the pair overflow on purpose, so the
	// caller sees a head too long for the pane rather than a quietly cut one.
	for width := w.Of(head); width <= fits+10; width++ {
		row, _, _ := w.stageOffer(head, "", "a.txt", 0, 0, width)
		if got := w.Of(row); got != width {
			t.Errorf("at %d columns the row is %d cells: %q", width, got, row)
		}
	}
}

// The figures beside a block head say how many lines it adds and removes, and
// they are painted the same green and red as the lines themselves. Left plain,
// the one place a reader looks to size up a block reads as ordinary text while
// every line under it is colored.
func TestABlockHeadPaintsItsFigures(t *testing.T) {
	t.Parallel()
	w := Renderer{Palette: Palette{Enabled: true}}
	d := git.FileDiff{Path: "a.swift", Blocks: []git.Block{
		{Header: "@@ -1,2 +1,3 @@", Lines: []string{" a", "+b", "-c"}, Added: 1, Deleted: 1},
	}}
	lines, _ := w.DiffArea(d, 0, 0, 0, false, paneWidth, 10)
	var head string
	for _, l := range lines {
		if strings.Contains(l, "block 1/1") {
			head = l
			break
		}
	}
	if head == "" {
		t.Fatal("the block head is not drawn, so this proves nothing")
	}
	for _, want := range []string{w.Add("+1"), w.Del("−1")} {
		if !strings.Contains(head, want) {
			t.Errorf("the head does not carry %q: %q", want, head)
		}
	}
}
