package layout

import (
	"fmt"
	"strings"
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
	"github.com/charmbracelet/x/ansi"
)

func fixedState() state.State {
	return state.State{Width: 77, Height: 53, Head: git.Head{Branch: "main"}, Rows: []state.Row{
		state.FileRow(git.Entry{Path: "app/a.swift", Worktree: git.Modified}, state.SectionUnstaged),
		state.FileRow(git.Entry{Path: "app/b.swift", Index: git.Modified}, state.SectionStaged),
		state.FileRow(git.Entry{Path: "docs/c.md", Worktree: git.Modified}, state.SectionUnstaged),
	}, Changes: state.Changes{Folded: map[string]bool{}}}
}

func TestPaneWithRowCommitKeepsViewportHeight(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := fixedState()
	s.Rows = append(s.Rows, state.CommitRow(git.CommitInfo{}))
	f := Pane(s, nil, w)
	if len(f.Lines) != 53 {
		t.Fatalf("got %d lines, want 53", len(f.Lines))
	}
}

func TestClickingASelectedRowCheckboxReturnsTargetCheckbox(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := fixedState()
	s.Changes.Selected = map[string]bool{"app/a.swift": true}
	f := Pane(s, nil, w)

	var hit bool
	for _, r := range f.Regions {
		if r.Target.Kind == TargetCheckbox && r.Target.Path == "app/a.swift" {
			region, _, _ := f.Hit(r.Row, r.ColStart)
			if region.Kind != TargetCheckbox {
				t.Errorf("Hit on the checkbox returned %v", region.Kind)
			}
			hit = true
		}
	}
	if !hit {
		t.Fatal("no checkbox region for the selected file")
	}
}

func TestPaneHeaderIsFourLines(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	f := Pane(fixedState(), nil, w)
	if len(f.Lines) < 4 {
		t.Fatalf("got %d lines, want at least 4 for the header", len(f.Lines))
	}
	if !strings.Contains(f.Lines[0], "changes") {
		t.Errorf("first line = %q, want the tab bar", f.Lines[0])
	}
	if !strings.HasPrefix(f.Lines[2], "[ ") {
		t.Errorf("third line = %q, want the commit box", f.Lines[2])
	}
	if !strings.Contains(f.Lines[3], "files") {
		t.Errorf("fourth line = %q, want the summary", f.Lines[3])
	}
	for i := 0; i < 4; i++ {
		if got := w.Of(f.Lines[i]); got != 77 {
			t.Errorf("header line %d is %d cells", i, got)
		}
	}
}

func TestFooterStaysOnTheLastRowWhenTheListIsShort(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := fixedState()
	s.Height = 49
	s.Open.Path = ""
	f := Pane(s, nil, w)
	if len(f.Lines) != 49 {
		t.Fatalf("got %d lines, want 49", len(f.Lines))
	}
	footerRow := -1
	for i, line := range f.Lines {
		if strings.Contains(line, "?") && strings.Contains(line, "diff") {
			footerRow = i
			break
		}
	}
	if footerRow != 48 {
		t.Fatalf("footer on row %d, want row 48 (the last line)", footerRow)
	}
}

func TestHistoryFooterStaysOnTheLastRowWithOneCommit(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	commit := git.CommitInfo{SHA: "abc", ShortSHA: "abc", Subject: "init", Unpushed: true, Age: "1m"}
	s := state.State{Width: 77, Height: 49, Tab: state.TabHistory, Cursor: 0, Rows: []state.Row{state.CommitRow(commit)}, History: state.History{Commits: []git.CommitInfo{commit}}}
	f := Pane(s, nil, w)
	if len(f.Lines) != 49 {
		t.Fatalf("got %d lines, want 49", len(f.Lines))
	}
	last := f.Lines[len(f.Lines)-1]
	if !strings.Contains(last, "?") {
		t.Fatalf("last line is not the footer: %q", last)
	}
}

func TestPaneFillsTheViewportExactly(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	f := Pane(fixedState(), nil, w)

	if len(f.Lines) != 53 {
		t.Fatalf("got %d lines, want 53", len(f.Lines))
	}
	for i, l := range f.Lines {
		if got := w.Of(l); got != 77 {
			t.Errorf("line %d is %d cells: %q", i, got, l)
		}
	}
}

func TestEveryRegionLandsOnTheRowThatDrewIt(t *testing.T) {
	t.Parallel()
	// This is the assertion the whole layering exists for. A region that names
	// a file must sit on a line that shows that file's name.
	w := Renderer{}
	f := Pane(fixedState(), nil, w)

	var checked int
	for _, r := range f.Regions {
		if r.Target.Kind != TargetFile {
			continue
		}
		checked++
		if r.Row < 0 || r.Row >= len(f.Lines) {
			t.Fatalf("region for %q points at row %d, outside the frame", r.Target.Path, r.Row)
		}
		line := f.Lines[r.Row]
		if !strings.Contains(line, base(r.Target.Path)) {
			t.Errorf("region for %q sits on %q", r.Target.Path, line)
		}
	}
	if checked != 3 {
		t.Errorf("want a region per file, got %d", checked)
	}
}

func TestHitResolvesAFileRegionToItsOwnPath(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	f := Pane(fixedState(), nil, w)

	for _, want := range []string{"app/a.swift", "app/b.swift", "docs/c.md"} {
		var hit bool
		for _, r := range f.Regions {
			if r.Target.Kind == TargetFile && r.Target.Path == want {
				region, _, _ := f.Hit(r.Row, r.ColStart)
				if region.Path != want {
					t.Errorf("Hit on the region for %q returned %q", want, region.Path)
				}
				hit = true
			}
		}
		if !hit {
			t.Errorf("no region for %q", want)
		}
	}
}

func TestPaneGivesTheDiffWhatTheListDoesNotNeed(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := fixedState()
	s.Open.Path = "app/a.swift"
	s.Open.Diff = git.FileDiff{Path: "app/a.swift", Blocks: []git.Block{
		{Header: "@@ -1,2 +1,3 @@", Lines: []string{" a", "+b"}, Added: 1},
	}}
	f := Pane(s, nil, w)

	var found bool
	for _, r := range f.Regions {
		if r.Target.Kind == TargetBlock {
			found = true
		}
	}
	if !found {
		t.Error("an open file should put its blocks in the frame")
	}
}

func TestStashedPaneFillsTheViewportExactly(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	stash := git.StashRow{
		Ref: "stash@{0}", SHA: "abc", Message: "hold", Branch: "main",
		FileCount: 1, Age: "1h", Status: git.StashApplies,
	}
	s := state.State{Width: 77, Height: 53, Tab: state.TabStashed, Rows: []state.Row{state.StashRowOf(stash)}, Stashed: state.Stashed{Stashes: []git.StashRow{stash}}}
	f := Pane(s, nil, w)
	if len(f.Lines) != 53 {
		t.Fatalf("got %d lines, want 53", len(f.Lines))
	}
	for i, l := range f.Lines {
		if got := w.Of(l); got != 77 {
			t.Errorf("line %d is %d cells: %q", i, got, l)
		}
	}
}

func TestPaneWithNoRowsStillFillsTheViewport(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := fixedState()
	s.Rows = nil
	f := Pane(s, nil, w)
	if len(f.Lines) != 53 {
		t.Fatalf("got %d lines, want 53", len(f.Lines))
	}
}

func TestAFileChangedOnBothSidesAppearsTwiceWithDistinctIndices(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := state.State{Width: 77, Height: 53, Head: git.Head{Branch: "main"}, Rows: []state.Row{
		state.FileRow(git.Entry{
			Path: "app/both.swift", Index: git.Modified, Worktree: git.Modified,
			IndexCount: git.Count{Added: 1}, WorktreeCount: git.Count{Added: 2},
		}, state.SectionStaged),

		state.FileRow(git.Entry{
			Path: "app/both.swift", Index: git.Modified, Worktree: git.Modified,
			IndexCount: git.Count{Added: 1}, WorktreeCount: git.Count{Added: 2},
		}, state.SectionUnstaged),
	}, Changes: state.Changes{Folded: map[string]bool{}}}
	f := Pane(s, nil, w)

	var indices []int
	for _, r := range f.Regions {
		if r.Target.Kind == TargetFile && r.Target.Path == "app/both.swift" {
			indices = append(indices, r.Target.Row)
		}
	}
	if len(indices) != 2 {
		t.Fatalf("want two regions for the same path, got %d", len(indices))
	}
	if indices[0] == indices[1] {
		t.Errorf("both rows claimed the same index %d", indices[0])
	}
}

func TestColoredPaneLinesAreExactlyPaneWidth(t *testing.T) {
	t.Parallel()
	w := Renderer{Palette: Palette{Enabled: true}}
	read := emptyReadMarks{}
	f := Pane(fixedState(), read, w)
	for i, line := range f.Lines {
		plain := ansi.Strip(line)
		if got := w.Of(plain); got != 77 {
			t.Errorf("line %d is %d cells after stripping color: %q", i, got, plain)
		}
	}
}

func TestColoredPaneRegionsMatchUncolored(t *testing.T) {
	t.Parallel()
	plain := Renderer{}
	colored := Renderer{Palette: Palette{Enabled: true}}
	read := emptyReadMarks{}
	s := fixedState()
	s.Cursor = 0
	s.Changes.Selected = map[string]bool{"app/a.swift": true}
	fPlain := Pane(s, read, plain)
	fColor := Pane(s, read, colored)
	if len(fPlain.Regions) != len(fColor.Regions) {
		t.Fatalf("region count differs: %d vs %d", len(fPlain.Regions), len(fColor.Regions))
	}
	for i := range fPlain.Regions {
		if fPlain.Regions[i] != fColor.Regions[i] {
			t.Errorf("region %d: plain=%+v colored=%+v", i, fPlain.Regions[i], fColor.Regions[i])
		}
	}
}

func ninetyFileState() state.State {
	rows := make([]state.Row, 90)
	for i := range rows {
		rows[i] = state.FileRow(
			git.Entry{Path: fmt.Sprintf("f%02d.txt", i), Worktree: git.Modified},
			state.SectionUnstaged)

	}
	return state.State{Width: 77, Height: 53, Head: git.Head{Branch: "main"}, Rows: rows, Changes: state.Changes{Folded: map[string]bool{}, Selected: map[string]bool{}}, Open: state.OpenDiff{Path: "f00.txt", Diff: git.FileDiff{Path: "f00.txt", Blocks: []git.Block{
		{Header: "@@ -1,1 +1,2 @@", Lines: []string{" a", "+b"}, Added: 1},
	}}}}
}

func TestNinetyFilesClosesTheDiff(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	f := Pane(ninetyFileState(), nil, w)
	for _, line := range f.Lines {
		if strings.Contains(line, "esc close") {
			t.Fatal("the diff should close when the list overflows the pane")
		}
	}
}

func TestScrollingReachesEveryListLine(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := ninetyFileState()
	list, _ := sectionsOf(s, nil, w, everyLine)
	body := s.Height - state.Facts[state.TabChanges].HeaderRows - 1
	listRows, _ := ListRowsFor(s, len(list), body)
	seen := map[int]bool{}
	for top := 0; top <= len(list)-listRows; top++ {
		shown, _ := sliceScrolled(list, nil, top, listRows)
		for i := range shown {
			seen[top+i] = true
		}
	}
	for i := range list {
		if !seen[i] {
			t.Fatalf("list line %d was never scrolled into view", i)
		}
	}
}

func TestScrollTopKeepsTheCursorRowVisible(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := ninetyFileState()
	s.Cursor = 89
	list, regions := sectionsOf(s, nil, w, everyLine)
	body := s.Height - state.Facts[state.TabChanges].HeaderRows - 1
	listRows, _ := ListRowsFor(s, len(list), body)
	cursorLine := CursorLineInList(regions, s.Cursor)
	top := ScrollTopFor(0, len(list), listRows, cursorLine)
	shown, _ := sliceScrolled(list, regions, top, listRows)
	found := false
	for _, line := range shown {
		if strings.Contains(line, "f89.txt") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("the cursor row should stay visible at the bottom of the list")
	}
}

func TestDiffPeekOpensSixDiffRows(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := ninetyFileState()
	s.Open.Peek = true
	f := Pane(s, nil, w)
	var header bool
	diffContent := 0
	inDiff := false
	for _, line := range f.Lines {
		if strings.Contains(line, "esc close") {
			header = true
			inDiff = true
			diffContent = 0
			continue
		}
		if inDiff {
			if strings.Trim(line, "─") == "" && strings.Contains(line, "─") {
				break
			}
			diffContent++
		}
	}
	if !header {
		t.Fatal("peek should draw the diff header")
	}
	if diffContent != minDiffRows {
		t.Fatalf("peek diff body = %d lines, want %d", diffContent, minDiffRows)
	}
}

func TestShownBlockHashesStopsAtThePaneHeight(t *testing.T) {
	t.Parallel()
	blocks := make([]git.Block, 6)
	for i := range blocks {
		blocks[i] = git.Block{
			Header: "@@ -1 +1 @@",
			Lines:  []string{"+line"},
			Added:  1,
			Hash:   fmt.Sprintf("h%d", i+1),
		}
	}
	d := git.FileDiff{Path: "a.txt", Blocks: blocks}
	// The area opens with a blank row, and one block takes a head row and a
	// line row. Five rows reach the second block's head but not its line, and
	// a block whose line was never drawn was not shown.
	if got := ShownBlockHashes(d, 0, 0, 5); len(got) != 1 || got[0] != "h1" {
		t.Fatalf("got %v, want only h1", got)
	}
	// Blank, head, line, blank, head, line.
	got := ShownBlockHashes(d, 0, 0, 6)
	if len(got) != 2 || got[0] != "h1" || got[1] != "h2" {
		t.Errorf("got %v, want h1 and h2", got)
	}
}

func TestShownBlockHashesNamesAnOversizedBlockAfterItsTailIsShown(t *testing.T) {
	t.Parallel()
	lines := make([]string, 10)
	for i := range lines {
		lines[i] = fmt.Sprintf("+line-%d", i)
	}
	d := git.FileDiff{Path: "a.txt", Blocks: []git.Block{{Lines: lines, Hash: "long"}}}
	if got := ShownBlockHashes(d, 0, 0, 6); len(got) != 0 {
		t.Errorf("top of oversized block marked %v read", got)
	}
	if got := ShownBlockHashes(d, 0, 6, 6); len(got) != 1 || got[0] != "long" {
		t.Errorf("tail of oversized block marked %v read, want long", got)
	}
}

func TestShownBlockHashesSkipsBlocksBeforeTheScroll(t *testing.T) {
	t.Parallel()
	d := git.FileDiff{Blocks: []git.Block{
		{Hash: "h1", Lines: []string{"+a"}},
		{Hash: "h2", Lines: []string{"+b"}},
	}}
	got := ShownBlockHashes(d, 1, 0, 5)
	if len(got) != 1 || got[0] != "h2" {
		t.Errorf("got %v, want [h2]", got)
	}
}

func TestCanPeekDiffNeedsTenBodyRows(t *testing.T) {
	t.Parallel()
	if CanPeekDiff(9) {
		t.Fatal("body 9 should not peek")
	}
	if !CanPeekDiff(10) {
		t.Fatal("body 10 should peek")
	}
}

func TestListRowsForPeekNeedsAListRow(t *testing.T) {
	t.Parallel()
	s := ninetyFileState()
	s.Open.Peek = true
	listRows, diffRows := ListRowsFor(s, 90, 9)
	if listRows != 9 || diffRows != 0 {
		t.Fatalf("body 9: got listRows=%d diffRows=%d, want 9 and 0", listRows, diffRows)
	}
	listRows, diffRows = ListRowsFor(s, 90, 10)
	if listRows != 1 || diffRows != 6 {
		t.Fatalf("body 10: got listRows=%d diffRows=%d, want 1 and 6", listRows, diffRows)
	}
}

func TestHistoryDiffOpenedFromTheDiffVerbKeepsMostRowsForTheDiff(t *testing.T) {
	t.Parallel()
	s := state.State{Tab: state.TabHistory, Open: state.OpenDiff{
		Path: "internal/layout/pane.go", PrioritizeDiff: true}}
	listRows, diffRows := ListRowsFor(s, 100, 49)
	if listRows != 8 || diffRows != 38 {
		t.Errorf("listRows=%d diffRows=%d, want 8 and 38", listRows, diffRows)
	}
}

func TestDiffPeekAtFifteenRowsKeepsTheFooter(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := ninetyFileState()
	s.Height = 15
	s.Open.Peek = true
	f := Pane(s, nil, w)
	if len(f.Lines) != 15 {
		t.Fatalf("got %d lines, want 15", len(f.Lines))
	}
	if !strings.Contains(f.Lines[14], "?") {
		t.Fatalf("footer missing from last line: %q", f.Lines[14])
	}
	for _, line := range f.Lines {
		if strings.Contains(line, "esc close") {
			t.Fatalf("diff header should not appear at height 15: %q", line)
		}
	}
}

func TestDiffPeekScrollsToTheCursorRow(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := ninetyFileState()
	s.Cursor = 50
	s.Open.Peek = true
	list, regions := sectionsOf(s, nil, w, everyLine)
	body := s.Height - state.Facts[state.TabChanges].HeaderRows - 1
	listRows, _ := ListRowsFor(s, len(list), body)
	cursorLine := CursorLineInList(regions, s.Cursor)
	s.ScrollTop = ScrollTopFor(0, len(list), listRows, cursorLine)
	f := Pane(s, nil, w)
	found := false
	for _, line := range f.Lines {
		if strings.Contains(line, "f50.txt") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("peek should scroll the list until the cursor row is visible")
	}
}

// Staging a conflicted path is what tells git the conflict is settled, so the
// block offer has to be withheld the same way the row's verb is.
func TestDirectoryRowStartsAtSixSpaces(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	line, _ := w.directoryRow("", 1, "", false, false, 0, paneWidth)
	if !strings.HasPrefix(line, "      ▾ /") {
		t.Errorf("got %q, want six spaces before the directory arrow", line)
	}
}

func TestChangesIndentDirectoryGroupsAndFiles(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := state.Apply(state.State{Width: paneWidth, Height: 24}, state.StatusLoaded{Rows: []git.Entry{
		{Path: ".agents/rules/antigravity.md", Worktree: git.Modified},
	}})
	f := Pane(s, nil, w)

	column := func(text string) int {
		for _, line := range f.Lines {
			if at := strings.Index(line, text); at >= 0 {
				return w.Of(line[:at])
			}
		}
		t.Fatalf("%q is not drawn in %q", text, f.Lines)
		return 0
	}
	section := column("changes · 1 file")
	rules := column("▾ .agents/rules")
	file := column("antigravity.md")
	if rules != section+4 {
		t.Errorf(".agents/rules starts in column %d, want %d", rules, section+4)
	}
	if file != rules+5 {
		t.Errorf("file starts in column %d, want %d", file, rules+5)
	}
}

func TestChangesIndentTracksNestedDirectoryDepth(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := state.State{Width: paneWidth, Height: 24, Rows: []state.Row{
		state.SectionHeadingRow(state.SectionUnstaged),
		state.DirectoryRow("app", state.SectionUnstaged),
		state.DirectoryRow("app/internal", state.SectionUnstaged),
		state.DirectoryRow("app/internal/cache", state.SectionUnstaged),
		state.FileRow(git.Entry{Path: "app/internal/cache/deep.go", Worktree: git.Modified},
			state.SectionUnstaged),
	}}
	f := Pane(s, nil, w)
	arrowColumn := func(path string) int {
		for _, region := range f.Regions {
			if region.Target.Kind != TargetDirectory || region.Target.Path != path {
				continue
			}
			line := f.Lines[region.Row]
			arrow := strings.Index(line, "▾")
			if arrow < 0 {
				t.Fatalf("directory %q has no arrow: %q", path, line)
			}
			return w.Of(line[:arrow])
		}
		t.Fatalf("directory %q is not drawn", path)
		return 0
	}

	root := arrowColumn("app")
	if got, want := arrowColumn("app/internal"), root+2; got != want {
		t.Errorf("child directory arrow = %d, want %d", got, want)
	}
	if got, want := arrowColumn("app/internal/cache"), root+4; got != want {
		t.Errorf("grandchild directory arrow = %d, want %d", got, want)
	}
	if got, want := fileIndent("app/internal/cache/deep.go"), 4; got != want {
		t.Errorf("deep file indent = %d, want %d", got, want)
	}
	for _, line := range f.Lines {
		if !strings.Contains(line, "deep.go") {
			continue
		}
		if got := w.Of(line); got != paneWidth {
			t.Errorf("deep file row uses %d columns, want %d: %q", got, paneWidth, line)
		}
		return
	}
	t.Fatal("deep file name is not visible")
}

func TestFoldingADirectoryHidesItsCompleteSubtree(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := state.State{Width: paneWidth, Height: 24, Rows: []state.Row{
		state.SectionHeadingRow(state.SectionUnstaged),
		state.DirectoryRow("app", state.SectionUnstaged),
		state.DirectoryRow("app/internal", state.SectionUnstaged),
		state.FileRow(git.Entry{Path: "app/internal/a.go", Worktree: git.Modified}, state.SectionUnstaged),
		state.FileRow(git.Entry{Path: "other/b.go", Worktree: git.Modified}, state.SectionUnstaged),
	}, Changes: state.Changes{Folded: map[string]bool{"2\x00app": true}}}
	f := Pane(s, nil, w)
	got := strings.Join(f.Lines, "\n")
	for _, hidden := range []string{"internal", "a.go"} {
		if strings.Contains(got, hidden) {
			t.Errorf("folded app still draws %q:\n%s", hidden, got)
		}
	}
	if !strings.Contains(got, "b.go") {
		t.Errorf("file outside the folded directory is missing:\n%s", got)
	}
}

func TestSectionCheckboxOmitsFilesHiddenByAnAncestorDirectory(t *testing.T) {
	t.Parallel()
	s := state.Apply(state.State{Width: paneWidth, Height: 24}, state.StatusLoaded{Rows: []git.Entry{
		{Path: "app/internal/a.go", Worktree: git.Modified},
	}})
	for i, row := range s.Rows {
		if row.Kind() == state.RowSectionHeading {
			s.Cursor = i
			break
		}
	}
	s = state.Apply(s, state.DirectoryFolded{Path: "app", Section: state.SectionUnstaged})
	if got := sectionCheckbox(s, state.SectionUnstaged); got != "" {
		t.Errorf("checkbox = %q, want no selectable visible file", got)
	}
}

func TestDirectoryRowIsExactly77Cells(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	line, _ := w.directoryRow("app/Sources/Infrastructure", 1, "", false, false, 0, paneWidth)
	if got := w.Of(line); got != paneWidth {
		t.Errorf("got %d cells, want %d: %q", got, paneWidth, line)
	}
}

func TestPaneLeavesABlankLineAfterSummary(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	f := Pane(fixedState(), nil, w)
	if len(f.Lines) < 5 {
		t.Fatalf("got %d lines, want at least 5", len(f.Lines))
	}
	if strings.TrimSpace(f.Lines[4]) != "" {
		t.Errorf("line after summary = %q, want blank", f.Lines[4])
	}
}

func TestPaneLeavesABlankLineAfterDiffHeader(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := fixedState()
	s.Open.Path = "app/a.swift"
	s.Open.Diff = git.FileDiff{Path: "app/a.swift", Blocks: []git.Block{
		{Header: "@@ -1,2 +1,3 @@", Lines: []string{" a", "+b"}, Added: 1, Hash: "h1"},
	}}
	f := Pane(s, nil, w)
	var headerRow int
	for i, line := range f.Lines {
		if strings.Contains(line, "esc close") {
			headerRow = i
			break
		}
	}
	if headerRow == 0 {
		t.Fatal("diff header not found")
	}
	if strings.TrimSpace(f.Lines[headerRow+1]) != "" {
		t.Errorf("line after diff header = %q, want blank", f.Lines[headerRow+1])
	}
}

func TestConflictedFileOffersNoBlockStage(t *testing.T) {
	t.Parallel()
	e := git.Entry{Path: "f.txt", Index: git.Unmerged, Worktree: git.Unmerged}
	s := state.State{Tab: state.TabChanges, Cursor: 0, Rows: []state.Row{
		state.FileRow(e, state.SectionUnstaged),
	}, Open: state.OpenDiff{Path: "f.txt"}}
	if diffShowStage(s) {
		t.Error("a conflicted path offered s stage on its blocks")
	}
}

// Folding hides rows; it does not change what the section holds. The directory
// row keeps saying 2 while it is folded, so a heading that says 0 files
// contradicts the line directly under it.
func TestFoldingDoesNotChangeTheSectionCount(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	rows := []state.Row{
		state.SectionHeadingRow(state.SectionUnstaged),
		state.DirectoryRow("app", state.SectionUnstaged),
		state.FileRow(git.Entry{
			Path: "app/a.go", Worktree: git.Modified,
			WorktreeCount: git.Count{Added: 1},
		}, state.SectionUnstaged),

		state.FileRow(git.Entry{
			Path: "app/b.go", Worktree: git.Modified,
			WorktreeCount: git.Count{Added: 1},
		}, state.SectionUnstaged),
	}
	open := state.State{Width: 77, Height: 53, Rows: rows, Changes: state.Changes{Folded: map[string]bool{}}}
	folded := open
	folded.Changes.Folded = map[string]bool{"app": true}

	for _, c := range []struct {
		name string
		s    state.State
	}{{"open", open}, {"folded", folded}} {
		lines, _ := sectionsOf(c.s, nil, w, everyLine)
		var heading string
		for _, l := range lines {
			if strings.Contains(l, "changes") {
				heading = l
				break
			}
		}
		if !strings.Contains(heading, "2 files") {
			t.Errorf("%s: heading = %q, want 2 files", c.name, heading)
		}
	}
}

// The design says a section is selected by clicking its box. The box is only
// drawn once something in the section is selected, so without the cursor
// drawing one the mouse can never reach the first selection.
func TestSectionHeadingUnderTheCursorDrawsItsBox(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := state.State{Width: 77, Height: 53, Cursor: 0, Rows: []state.Row{
		state.SectionHeadingRow(state.SectionUnstaged),
		state.FileRow(git.Entry{
			Path: "a.go", Worktree: git.Modified,
		}, state.SectionUnstaged),
	}, Changes: state.Changes{Folded: map[string]bool{}}}
	lines, regions := sectionsOf(s, nil, w, everyLine)
	if !strings.Contains(lines[0], "[ ]") {
		t.Errorf("heading under the cursor = %q, want a box", lines[0])
	}
	var reachable bool
	for _, r := range regions {
		if r.Row == 0 && r.ColStart <= 40 && 40 < r.ColEnd {
			reachable = true
		}
	}
	if !reachable {
		t.Error("nothing on the heading row answers a pointer at column 40")
	}
}

// The stashed and worktrees tabs have no summary line, so a failure reported
// there had nowhere to go and the verb looked like it did nothing.
func TestStashAndWorktreePanesShowANotice(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	for _, c := range []struct {
		name  string
		state state.State
	}{
		{"stashed", state.State{Width: 77, Height: 20, Tab: state.TabStashed, Notice: "git stash branch: exit 1"}},
		{"worktrees", state.State{Width: 77, Height: 20, Tab: state.TabWorktrees, Notice: "git worktree remove: exit 128"}},
	} {
		f := Pane(c.state, nil, w)
		var found bool
		for _, line := range f.Lines {
			if strings.Contains(line, "exit") {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: the notice is nowhere on screen", c.name)
		}
	}
}

func TestHistoryPaneShowsANotice(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	withNotice := state.State{Width: 77, Height: 20, Tab: state.TabHistory, Notice: "git reset --soft HEAD^: exit 128"}
	f := Pane(withNotice, nil, w)
	if !strings.Contains(f.Lines[3], "exit 128") {
		t.Errorf("notice line = %q, want exit 128 under the heading", f.Lines[3])
	}
	withoutNotice := withNotice
	withoutNotice.Notice = ""
	f = Pane(withoutNotice, nil, w)
	for _, line := range f.Lines {
		if strings.Contains(line, "exit 128") {
			t.Errorf("empty notice still shows exit 128 on %q", line)
		}
	}
}

func TestHistoryHeaderRowsCountTheNoticeLine(t *testing.T) {
	t.Parallel()
	if got := HeaderRowsOf(state.State{Tab: state.TabHistory}); got != 3 {
		t.Errorf("without notice = %d, want 3", got)
	}
	if got := HeaderRowsOf(state.State{Tab: state.TabHistory, Notice: "undid commit · reset"}); got != 4 {
		t.Errorf("with notice = %d, want 4", got)
	}
}

func TestHistoryPaneDrawsAHeading(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	commits := []git.CommitInfo{
		{SHA: "a", ShortSHA: "a", Subject: "one", Age: "1m"},
		{SHA: "b", ShortSHA: "b", Subject: "two", Age: "2m"},
	}
	s := state.State{Width: 77, Height: 20, Tab: state.TabHistory, Rows: []state.Row{
		state.CommitRow(commits[0]),
		state.CommitRow(commits[1]),
	}, History: state.History{Commits: commits}}
	f := Pane(s, nil, w)
	want := fmt.Sprintf("history · %d", len(s.History.Commits))
	var found bool
	for i, line := range f.Lines {
		if i < 2 {
			continue
		}
		plain := stripSGR(line)
		if !strings.Contains(plain, want) {
			continue
		}
		found = true
		if strings.Contains(plain, "+") || strings.Contains(plain, "−") {
			t.Errorf("heading %q should not show added/deleted figures", plain)
		}
	}
	if !found {
		t.Fatalf("no line with %q below the tab bar", want)
	}
}

func TestHistoryPaneOffersTheNextPage(t *testing.T) {
	t.Parallel()
	commit := git.CommitInfo{SHA: "a", ShortSHA: "a", Subject: "one", Age: "1m"}
	s := state.State{Width: 77, Height: 20, Tab: state.TabHistory,
		Rows:    []state.Row{state.CommitRow(commit)},
		History: state.History{Commits: []git.CommitInfo{commit}, HasMore: true}}
	frame := Pane(s, nil, Renderer{})
	for _, line := range frame.Lines {
		if strings.Contains(stripSGR(line), "j more") {
			return
		}
	}
	t.Error("history heading does not offer the next page")
}

func TestTabHeadingsStartAtColumnSix(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	commit := git.CommitInfo{SHA: "a", ShortSHA: "a", Subject: "one", Age: "1m"}
	stash := git.StashRow{
		Ref: "stash@{0}", SHA: "s", Message: "hold", Branch: "main",
		Age: "1h", Status: git.StashApplies,
	}
	wt := worktreeRow("wt", "feat", 0, git.MergeClean)
	cases := []struct {
		name string
		s    state.State
	}{
		{"changes", state.State{Width: 77, Height: 53, Tab: state.TabChanges, Rows: []state.Row{state.FileRow(
			git.Entry{Path: "a.go", Worktree: git.Modified},
			state.SectionUnstaged)}, Changes: state.Changes{Folded: map[string]bool{}}}},
		{"history", state.State{Width: 77, Height: 20, Tab: state.TabHistory, Rows: []state.Row{state.CommitRow(commit)}, History: state.History{Commits: []git.CommitInfo{commit}}}},
		{"stashed", state.State{Width: 77, Height: 20, Tab: state.TabStashed, Rows: []state.Row{state.StashRowOf(stash)}, Stashed: state.Stashed{Stashes: []git.StashRow{stash}}}},
		{"worktrees", state.State{Width: 77, Height: 20, Tab: state.TabWorktrees, Rows: []state.Row{state.WorktreeRowOf(wt)}, Worktrees: state.Worktrees{Base: "main", List: []git.WorktreeRow{wt}}}},
	}
	for _, c := range cases {
		f := Pane(c.s, nil, w)
		if col := headingNameColumn(w, f, c.name); col != 6 {
			t.Errorf("%s: name starts at column %d, want 6", c.name, col)
		}
	}
}

func TestTabHeadingDotsUseTheMarkColor(t *testing.T) {
	t.Parallel()
	w := Renderer{Palette: Palette{Enabled: true, Theme: slate}}
	wantDot := w.Mark("·")
	commit := git.CommitInfo{SHA: "a", ShortSHA: "a", Subject: "one", Age: "1m"}
	stash := git.StashRow{
		Ref: "stash@{0}", SHA: "s", Message: "hold", Branch: "main",
		Age: "1h", Status: git.StashApplies,
	}
	wt := worktreeRow("wt", "feat", 0, git.MergeClean)
	cases := []struct {
		name string
		s    state.State
	}{
		{"changes", state.State{Width: 77, Height: 53, Tab: state.TabChanges, Rows: []state.Row{state.FileRow(
			git.Entry{Path: "a.go", Worktree: git.Modified},
			state.SectionUnstaged)}, Changes: state.Changes{Folded: map[string]bool{}}}},
		{"history", state.State{Width: 77, Height: 20, Tab: state.TabHistory, Rows: []state.Row{state.CommitRow(commit)}, History: state.History{Commits: []git.CommitInfo{commit}}}},
		{"stashed", state.State{Width: 77, Height: 20, Tab: state.TabStashed, Rows: []state.Row{state.StashRowOf(stash)}, Stashed: state.Stashed{Stashes: []git.StashRow{stash}}}},
		{"worktrees", state.State{Width: 77, Height: 20, Tab: state.TabWorktrees, Rows: []state.Row{state.WorktreeRowOf(wt)}, Worktrees: state.Worktrees{Base: "main", List: []git.WorktreeRow{wt}}}},
	}
	for _, c := range cases {
		f := Pane(c.s, nil, w)
		var found bool
		for _, line := range f.Lines {
			if !strings.Contains(stripSGR(line), c.name+" ·") {
				continue
			}
			found = true
			if !strings.Contains(line, wantDot) {
				t.Errorf("%s heading %q should color the dot with Mark", c.name, line)
			}
		}
		if !found {
			t.Fatalf("%s: no heading on screen", c.name)
		}
	}
}

func TestEveryTabHeadingSitsOutsideTheScrolledList(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	commits := make([]git.CommitInfo, 5)
	hrows := make([]state.Row, 5)
	for i := range commits {
		commits[i] = git.CommitInfo{
			SHA: fmt.Sprintf("%d", i), ShortSHA: fmt.Sprintf("%d", i),
			Subject: "x", Age: "1m",
		}
		hrows[i] = state.CommitRow(commits[i])
	}
	history := state.State{Width: 77, Height: 10, Tab: state.TabHistory, ScrollTop: 1, Rows: hrows, History: state.History{Commits: commits}}
	hlist, _ := historyListOf(history, w, everyLine)
	if tabHeadingInList(hlist) {
		t.Error("history heading sits inside the scrollable list")
	}
	if !paneShowsHeading(Pane(history, nil, w), "history") {
		t.Fatal("history heading missing from pane while scrolled")
	}

	stashRows := make([]git.StashRow, 3)
	srows := make([]state.Row, 3)
	for i := range stashRows {
		stashRows[i] = git.StashRow{
			Ref: fmt.Sprintf("stash@{%d}", i), SHA: fmt.Sprintf("s%d", i),
			Message: "hold", Branch: "main", Age: "1h", Status: git.StashApplies,
		}
		srows[i] = state.StashRowOf(stashRows[i])
	}
	stashed := state.State{Width: 77, Height: 10, Tab: state.TabStashed, Rows: srows, Stashed: state.Stashed{Stashes: stashRows}}
	if tabHeadingInList(stashListLines(stashed, nil, w)) {
		t.Error("stashed heading sits inside the scrollable list")
	}
	if !paneShowsHeading(Pane(stashed, nil, w), "stashed") {
		t.Fatal("stashed heading missing from pane")
	}

	wtrees := make([]git.WorktreeRow, 3)
	wrows := make([]state.Row, 3)
	for i := range wtrees {
		wtrees[i] = worktreeRow(fmt.Sprintf("wt%d", i), "feat", 0, git.MergeClean)
		wrows[i] = state.WorktreeRowOf(wtrees[i])
	}
	worktrees := state.State{Width: 77, Height: 10, Tab: state.TabWorktrees, Rows: wrows, Worktrees: state.Worktrees{Base: "main", List: wtrees}}
	if tabHeadingInList(worktreeListLines(worktrees, nil, w)) {
		t.Error("worktrees heading sits inside the scrollable list")
	}
	if !paneShowsHeading(Pane(worktrees, nil, w), "worktrees") {
		t.Fatal("worktrees heading missing from pane")
	}
}

func tabHeadingInList(list []string) bool {
	for _, line := range list {
		plain := stripSGR(line)
		for _, name := range []string{"history ·", "stashed ·", "worktrees ·"} {
			if strings.Contains(plain, name) {
				return true
			}
		}
	}
	return false
}

func stashListLines(s state.State, r ReadMarks, w Renderer) []string {
	list, _ := stashListOf(s, r, w, everyLine)
	return list
}

func worktreeListLines(s state.State, r ReadMarks, w Renderer) []string {
	list, _ := worktreeListOf(s, r, w, everyLine)
	return list
}

func paneShowsHeading(f Frame, name string) bool {
	for _, line := range f.Lines {
		if strings.Contains(stripSGR(line), name+" ·") {
			return true
		}
	}
	return false
}

func headingNameColumn(w Renderer, f Frame, name string) int {
	for _, line := range f.Lines {
		plain := stripSGR(line)
		idx := strings.Index(plain, name+" ·")
		if idx < 0 {
			continue
		}
		return w.Of(plain[:idx])
	}
	return -1
}

func TestDirectoryNameIsNotDimWhileArrowIs(t *testing.T) {
	t.Parallel()
	w := Renderer{Palette: Palette{Enabled: true, Theme: cozmic}}
	dir, _ := w.directoryRow("app/2", 2, "", false, false, 0, paneWidth)
	dim := rgbEscape(cozmic.Dim)
	dirColor := rgbEscape(cozmic.Dir)
	nameStart := strings.Index(dir, "\x1b["+dirColor+"mapp/2")
	if nameStart < 0 {
		t.Fatal("directory name missing")
	}
	nameStart += len("\x1b[") + len(dirColor) + len("m")
	if got := effectiveRGB(dir, nameStart); got != dirColor {
		t.Errorf("directory name fg = %q, want %q: %q", got, dirColor, dir)
	}
	arrow := strings.Index(dir, "▾")
	if arrow < 0 {
		t.Fatal("arrow missing")
	}
	if got := effectiveRGB(dir, arrow); got != dim {
		t.Errorf("arrow fg = %q, want %q: %q", got, dim, dir)
	}
}

// effectiveRGB returns the active 38;2 foreground SGR at line[:pos], or "" for
// the terminal default.
func effectiveRGB(line string, pos int) string {
	var current string
	i := 0
	for i < pos && i < len(line) {
		if line[i] == 0x1b && i+1 < len(line) && line[i+1] == '[' {
			j := i + 2
			for j < len(line) && line[j] != 'm' {
				j++
			}
			if j < len(line) {
				seq := line[i+2 : j]
				if seq == "0" || seq == "00" {
					current = ""
				} else if strings.HasPrefix(seq, "38;2;") {
					current = seq
				}
				i = j + 1
				continue
			}
		}
		i++
	}
	return current
}

func TestFileRowSitsDeeperThanDirectoryHeading(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	dir, _ := w.directoryRow("app/Sources", 2, "", false, false, 0, paneWidth)
	file, _ := w.FileRow(Row{
		Entry: git.Entry{Path: "app/Sources/a.go", Worktree: git.Modified},
		State: rowState{Indent: fileIndent("app/Sources/a.go"), Read: state.Read},
	}, paneWidth)
	if got, want := kindColumn(file), arrowColumn(w, dir); got <= want {
		t.Errorf("kind at column %d, arrow at %d:\n %q\n %q", got, want, file, dir)
	}
}

func TestNestedFileRowTruncatesToPaneWidth(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	line, _ := w.FileRow(Row{
		Entry: git.Entry{Path: strings.Repeat("long", 20) + ".swift", Worktree: git.Modified},
		State: rowState{Indent: 1},
	}, paneWidth)
	if got := w.Of(line); got != paneWidth {
		t.Errorf("got %d cells, want %d: %q", got, paneWidth, line)
	}
}

func TestColoredDirectoryRowStripsToThePlainRow(t *testing.T) {
	t.Parallel()
	plain := Renderer{}
	colored := Renderer{Palette: Palette{Enabled: true, Theme: cozmic}}
	dir := "app/Sources/Infrastructure"
	col, _ := colored.directoryRow(dir, 1, "", false, false, 0, paneWidth)
	uncol, _ := plain.directoryRow(dir, 1, "", false, false, 0, paneWidth)
	if got := stripSGR(col); got != uncol {
		t.Errorf("stripped colored:\n got %q\nwant %q", got, uncol)
	}
}

// The arrow is the only thing saying a row is a directory, and it is also the
// only thing saying the row folds. Drawn in the faint tier it measured 2.07
// against the ground, which is a decoration's contrast, not an affordance's.
func TestDirectoryRowReadsApartFromAFileRow(t *testing.T) {
	t.Parallel()
	w := Renderer{Palette: Palette{Enabled: true, Theme: cozmic}}
	dir, _ := w.directoryRow("app/Sources", 2, "", false, false, 0, paneWidth)
	if !strings.Contains(dir, rgbEscape(cozmic.Dim)) {
		t.Errorf("the directory row uses no dim tier: %q", dir)
	}
	if strings.Contains(dir, rgbEscape(cozmic.Faint)) {
		t.Errorf("the directory row still uses the faint tier: %q", dir)
	}
}

// The cursor takes a column of its own rather than pushing the row across, so
// arriving on a directory does not move the path under the reader's eye.
func TestCursorOnADirectoryDoesNotShiftIt(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	idle, _ := w.directoryRow("app/Sources", 2, "", false, false, 0, paneWidth)
	armed, _ := w.directoryRow("app/Sources", 2, "", false, true, 0, paneWidth)
	if arrowColumn(w, idle) != arrowColumn(w, armed) {
		t.Errorf("the arrow moved:\n idle  %q\n armed %q", idle, armed)
	}
	for _, l := range []string{idle, armed} {
		if got := w.Of(l); got != paneWidth {
			t.Errorf("%q is %d cells, want %d", l, got, paneWidth)
		}
	}
}

func rgbEscape(hex string) string {
	c := hexRGB(hex)
	return fmt.Sprintf("38;2;%d;%d;%d", c[0], c[1], c[2])
}

// arrowColumn is where the arrow sits in cells, which is what the eye sees; a
// byte index moves when a three-byte glyph is inserted ahead of it.
func arrowColumn(w Renderer, line string) int {
	i := strings.Index(line, "▾")
	if i < 0 {
		return -1
	}
	return w.Of(line[:i])
}

func kindColumn(line string) int {
	w := Renderer{}
	plain := stripSGR(line)
	if len(plain) < 6 {
		return -1
	}
	i := 5
	for i < len(plain) && plain[i] == ' ' {
		i++
	}
	if i >= len(plain) {
		return -1
	}
	return w.Of(plain[:i])
}

// The cursor owns column zero on every row of the list. A marker that moves
// between row kinds gives the eye a second thing to track while it is already
// tracking the row.
func TestTheCursorAlwaysSitsInColumnZero(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	dir, _ := w.directoryRow("app/Sources", 2, "", false, true, 0, paneWidth)
	file, _ := w.FileRow(Row{
		Entry: git.Entry{Path: "a.go", Worktree: git.Modified},
		State: rowState{Cursor: true},
		Verbs: []state.VerbName{state.VerbNameStage},
	}, paneWidth)
	heading, _ := w.Heading("staged", 1, 1, 0, state.SectionStaged, "", "", paneWidth)
	heading = w.cursorSectionHeading(heading)

	for name, line := range map[string]string{
		"directory": dir, "file": file, "heading": heading,
	} {
		if col := cursorColumn(w, line); col != 0 {
			t.Errorf("%s: cursor at column %d, want 0: %q", name, col, line)
		}
	}
}

func cursorColumn(w Renderer, line string) int {
	i := strings.Index(line, "▌")
	if i < 0 {
		return -1
	}
	return w.Of(stripSGR(line[:i]))
}

func stripSGR(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func directoryCheckboxState(cursor int) state.State {
	return state.State{Width: 77, Height: 53, Cursor: cursor, Rows: []state.Row{
		state.DirectoryRow("src", state.SectionUnstaged),
		state.FileRow(
			git.Entry{Path: "src/a.go", Worktree: git.Modified}, state.SectionUnstaged),

		state.FileRow(
			git.Entry{Path: "src/b.go", Worktree: git.Modified}, state.SectionUnstaged),
	}, Changes: state.Changes{Selected: map[string]bool{}}}
}

func TestDirectoryRowDrawsItsCheckboxWhereFileBoxesSit(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	line, _ := w.directoryRow("src", 2, "[✓]", false, false, 0, paneWidth)
	if !strings.HasPrefix(line, " [✓]    ▾ src") {
		t.Errorf("got %q, want prefix [✓]    ▾ src", line)
	}
	line, _ = w.directoryRow("src", 2, "[✓]", false, true, 0, paneWidth)
	if !strings.HasPrefix(line, "▌[✓]    ▾ src") {
		t.Errorf("cursor: got %q", line)
	}
}

func TestDirectoryRowWithCheckboxIsExactly77Cells(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	line, _ := w.directoryRow("src", 2, "[✓]", false, false, 0, paneWidth)
	if got := w.Of(line); got != paneWidth {
		t.Errorf("got %d cells, want %d", got, paneWidth)
	}
}

func TestDirectoryRowCheckboxAnswersTheMouse(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	_, regions := w.directoryRow("src", 2, "[✓]", false, false, 0, paneWidth)
	found := false
	for _, r := range regions {
		if r.Target.Kind == TargetCheckbox && r.Target.Path == "src" &&
			r.Target.Row == 0 && r.ColStart == 1 && r.ColEnd == 4 {
			found = true
		}
	}
	if !found {
		t.Fatal("missing checkbox region")
	}
	_, regions = w.directoryRow("src", 2, "", false, false, 0, paneWidth)
	for _, r := range regions {
		if r.Target.Kind == TargetCheckbox {
			t.Fatal("empty checkbox should not get a region")
		}
	}
}

func TestDirectoryCheckboxStates(t *testing.T) {
	t.Parallel()
	s := directoryCheckboxState(2)
	if got := directoryCheckbox(s, "src", state.SectionUnstaged, 2, false); got != "" {
		t.Errorf("unselected, cursor elsewhere: got %q", got)
	}
	s = directoryCheckboxState(0)
	if got := directoryCheckbox(s, "src", state.SectionUnstaged, 2, true); got != "[ ]" {
		t.Errorf("unselected, cursor here: got %q", got)
	}
	s = directoryCheckboxState(0)
	s = state.Apply(s, state.SelectionToggled{Path: "src/a.go", Section: state.SectionUnstaged})
	if got := directoryCheckbox(s, "src", state.SectionUnstaged, 2, true); got != "[-]" {
		t.Errorf("partial: got %q", got)
	}
	s = state.Apply(s, state.SelectionToggled{Path: "src/b.go", Section: state.SectionUnstaged})
	if got := directoryCheckbox(s, "src", state.SectionUnstaged, 2, true); got != "[✓]" {
		t.Errorf("all selected: got %q", got)
	}
}

func TestSelectAllTicksDirectoryRowsToo(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := directoryCheckboxState(0)
	s = state.Apply(s, state.TabSelectionToggled{})
	f := Pane(s, nil, w)
	var dirLine string
	for _, line := range f.Lines {
		if strings.Contains(line, "▾ src") {
			dirLine = line
			break
		}
	}
	if dirLine == "" {
		t.Fatal("directory row not found")
	}
	if !strings.Contains(dirLine, "[✓]") {
		t.Errorf("directory row = %q, want [✓]", dirLine)
	}
}

// The boxes stand in one column so the eye can run down them. A heading's box
// one cell off from the file boxes under it reads as a different control.
func TestEveryCheckboxSharesOneColumn(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	file, _ := w.FileRow(Row{
		Entry: git.Entry{Path: "a.go", Worktree: git.Modified},
		State: rowState{Selected: true},
	}, paneWidth)
	heading, _ := w.Heading("staged", 1, 1, 0, state.SectionStaged, "[✓]", "", paneWidth)
	if got, want := boxColumn(w, heading), boxColumn(w, file); got != want {
		t.Errorf("heading box at column %d, file box at %d:\n %q\n %q",
			got, want, heading, file)
	}
}

func boxColumn(w Renderer, line string) int {
	plain := stripSGR(line)
	i := strings.Index(plain, "[")
	if i < 0 {
		return -1
	}
	return w.Of(plain[:i])
}

// A block only counts as shown when all of it fit. Recording it on the head
// line alone marked a long file read the moment the cursor passed over it.
func TestABlockTallerThanTheDiffAreaIsNotShown(t *testing.T) {
	t.Parallel()
	lines := make([]string, 40)
	for i := range lines {
		lines[i] = "+x"
	}
	d := git.FileDiff{Blocks: []git.Block{{Hash: "h1", Lines: lines}}}

	if got := ShownBlockHashes(d, 0, 0, 20); len(got) != 0 {
		t.Errorf("got %v, want none: 40 lines do not fit in 20 rows", got)
	}
	// The blank row the area opens with, the head and the forty lines: the
	// fortieth line is drawn at 42 rows and not at 41.
	if got := ShownBlockHashes(d, 0, 0, 41); len(got) != 0 {
		t.Errorf("got %v, want none: the last line is one row below the fold", got)
	}
	if got := ShownBlockHashes(d, 0, 0, 42); len(got) != 1 {
		t.Errorf("got %v, want the block once it fits", got)
	}
}

func TestLayoutFileRowCarriesTheSectionOfItsRow(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	e := git.Entry{Path: "i.txt", Index: git.Added, Worktree: git.Modified}
	s := state.State{Changes: state.Changes{Selected: map[string]bool{}, Folded: map[string]bool{}}}

	staged := layoutFileRow(s, nil, state.FileRow(
		e, state.SectionStaged),

		0)
	stagedLine, _ := w.FileRow(staged, paneWidth)
	if !strings.Contains(stagedLine, " A  ") {
		t.Fatalf("staged line = %q, want index letter A", stagedLine)
	}

	unstaged := layoutFileRow(s, nil, state.FileRow(
		e, state.SectionUnstaged),

		1)
	unstagedLine, _ := w.FileRow(unstaged, paneWidth)
	if !strings.Contains(unstagedLine, " M  ") {
		t.Fatalf("unstaged line = %q, want worktree letter M", unstagedLine)
	}
}

func TestFileRowVerbsHideWhileADiscardAwaitsConfirmation(t *testing.T) {
	t.Parallel()
	row := state.FileRow(

		git.Entry{Path: "a.txt", Worktree: git.Modified}, state.SectionUnstaged)

	s := state.State{Cursor: 0, Rows: []state.Row{row}}
	if got := rowVerbs(s, row, 0); got == nil {
		t.Fatal("row verbs should show without confirmation")
	}
	s.Changes.DiscardConfirm = &state.DiscardConfirm{Files: 1}
	if got := rowVerbs(s, row, 0); got != nil {
		t.Fatalf("got %v during discard confirm, want nil", got)
	}
	s.Changes.DiscardConfirm = nil
	s.Stashed.DropConfirm = &state.StashDropConfirm{Message: "hold"}
	if got := rowVerbs(s, row, 0); got != nil {
		t.Fatalf("got %v during stash drop confirm, want nil", got)
	}
}

// The verbs are withheld from a stale row only while its own diff is open: the
// diff on the screen is the one the reader would be staging, and it no longer
// matches the file. A stale row whose diff is not open keeps its verbs, because
// the reader is acting on the row rather than on what is drawn below it.
func TestAStaleRowKeepsItsVerbsUntilItsOwnDiffIsOpen(t *testing.T) {
	t.Parallel()
	row := state.FileRow(
		git.Entry{Path: "a.txt", Worktree: git.Modified}, state.SectionUnstaged)
	other := state.FileRow(
		git.Entry{Path: "b.txt", Worktree: git.Modified}, state.SectionUnstaged)

	for _, c := range []struct {
		name string
		open string
		want bool
	}{
		{"no diff open", "", true},
		{"another file's diff open", "b.txt", true},
		{"its own diff open", "a.txt", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := state.State{Width: 90, Height: 40, Tab: state.TabChanges,
				Cursor: 0, Rows: []state.Row{row, other}}
			s = state.Apply(s, state.StaleChanged{Path: "a.txt", Stale: true})
			if c.open != "" {
				s = state.Apply(s, state.DiffOpened{Path: c.open,
					Origin: state.WorkingTree(state.SectionUnstaged)})
			}
			if got := rowVerbs(s, row, 0) != nil; got != c.want {
				t.Errorf("the stale row offers verbs = %v, want %v", got, c.want)
			}
		})
	}
}

func TestStashRowVerbsHideWhileADropAwaitsConfirmation(t *testing.T) {
	t.Parallel()
	stash := git.StashRow{Ref: "stash@{0}", Message: "hold", Status: git.StashApplies}
	row := state.StashRowOf(stash)
	s := state.State{Cursor: 0}
	got := stashRowVerbs(s, row)
	if len(got) != 2 || got[0] != state.VerbNameRestore || got[1] != state.VerbNameDrop {
		t.Fatalf("got %v, want [restore drop]", got)
	}
	s.Stashed.DropConfirm = &state.StashDropConfirm{Message: "hold"}
	if got := stashRowVerbs(s, row); got != nil {
		t.Fatalf("got %v during drop confirm, want nil", got)
	}
}

func stashPaneStateWithFiles() state.State {
	stash := git.StashRow{
		Ref: "stash@{0}", SHA: "abc", Message: "hold", Branch: "main",
		FileCount: 2, Age: "1h", Status: git.StashApplies,
		Collides: []string{"internal/collect/aggregate.go"},
	}
	s := state.State{Width: paneWidth, Height: 20, Tab: state.TabStashed, Cursor: 0, Stashed: state.Stashed{Stashes: []git.StashRow{stash}, Expanded: stash.SHA, Files: []git.Entry{
		{
			Path: "internal/collect/aggregate.go", Worktree: git.Modified,
			WorktreeCount: git.Count{Added: 38, Deleted: 4},
		},
		{
			Path: "internal/collect/aggregate_other.go", Worktree: git.Modified,
			WorktreeCount: git.Count{Added: 38, Deleted: 1},
		},
	}}}
	s.Rows = state.StashRowsFor(s)
	return s
}

func worktreePaneStateWithFiles() state.State {
	tree := git.WorktreeRow{
		Path: "/repo/wt", Name: "wt", Branch: "feat", SHA: "abc",
		Ahead: 1, Dirty: 1, WroteAge: "wrote 1m", Merge: git.MergeClean,
	}
	s := state.State{Width: paneWidth, Height: 20, Tab: state.TabWorktrees, Cursor: 0, Worktrees: state.Worktrees{List: []git.WorktreeRow{tree}, Base: "main", Expanded: tree.Path, Files: []git.Entry{
		{
			Path: "internal/sync/engine.go", Worktree: git.Modified,
			WorktreeCount: git.Count{Added: 7, Deleted: 2},
		},
	}}}
	s.Rows = state.WorktreeRowsFor(s)
	return s
}

func plusColumn(w Renderer, line string) int {
	plain := stripSGR(line)
	i := strings.Index(plain, "+")
	if i < 0 {
		return -1
	}
	return w.Of(plain[:i])
}

func TestStashAndWorktreeFileRowFiguresCarryAddAndDelColor(t *testing.T) {
	t.Parallel()
	w := Renderer{Palette: Palette{Enabled: true, Theme: cozmic}}
	addColor := rgbEscape(cozmic.Add)
	delColor := rgbEscape(cozmic.Del)

	for name, s := range map[string]state.State{"stashed": stashPaneStateWithFiles(), "worktrees": worktreePaneStateWithFiles()} {
		f := Pane(s, nil, w)
		var coloredAdd, coloredDel bool
		for _, line := range f.Lines {
			plain := stripSGR(line)
			if !strings.Contains(plain, "+") || !strings.Contains(plain, "−") {
				continue
			}
			if strings.Contains(line, addColor) {
				coloredAdd = true
			}
			if strings.Contains(line, delColor) {
				coloredDel = true
			}
		}
		if !coloredAdd || !coloredDel {
			t.Errorf("%s: add=%v del=%v, want both figures colored", name, coloredAdd, coloredDel)
		}
	}
}

func TestStashAndWorktreeFileRowsExposeANameRegion(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	for _, c := range []struct {
		tab  string
		s    state.State
		path string
	}{
		{"stashed", stashPaneStateWithFiles(), "internal/collect/aggregate.go"},
		{"worktrees", worktreePaneStateWithFiles(), "internal/sync/engine.go"},
	} {
		f := Pane(c.s, nil, w)
		var found bool
		for _, r := range f.Regions {
			if r.Target.Kind != TargetFile || r.Target.Path != c.path {
				continue
			}
			found = true
			if r.Row < 0 || r.Row >= len(f.Lines) {
				t.Fatalf("%s: region for %q points outside the frame", c.tab, c.path)
			}
			got := cellSlice(f.Lines[r.Row], r.ColStart, r.ColEnd)
			if !strings.Contains(got, c.path) {
				t.Errorf("%s: region covers %q, want %q", c.tab, got, c.path)
			}
		}
		if !found {
			t.Errorf("%s: no region for %q", c.tab, c.path)
		}
	}
}

func TestStashFileRowPlusColumnDoesNotMoveWhenConflicts(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	f := Pane(stashPaneStateWithFiles(), nil, w)
	var cols []int
	for _, line := range f.Lines {
		plain := stripSGR(line)
		if !strings.Contains(plain, "aggregate.go") && !strings.Contains(plain, "aggregate_other.go") {
			continue
		}
		col := plusColumn(w, line)
		if col < 0 {
			t.Fatalf("no + column on %q", plain)
		}
		cols = append(cols, col)
	}
	if len(cols) != 2 {
		t.Fatalf("want two file rows, got %d", len(cols))
	}
	if cols[0] != cols[1] {
		t.Errorf("+ column moved with conflicts: %v", cols)
	}
}

func TestFileRowKindLetterColumnMatchesAcrossTabs(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	changes, _ := w.FileRow(Row{
		Entry: git.Entry{Path: "app/a.go", Worktree: git.Modified},
		State: rowState{Indent: 1, Read: state.Read},
	}, paneWidth)
	stash, _ := w.FileRow(nestedFileRow(git.Entry{
		Path: "app/a.go", Worktree: git.Modified,
	}), paneWidth)
	worktree, _ := w.FileRow(nestedFileRow(git.Entry{
		Path: "app/a.go", Worktree: git.Modified,
	}), paneWidth)

	got := []int{kindColumn(changes), kindColumn(stash), kindColumn(worktree)}
	if got[0] != got[1] || got[1] != got[2] {
		t.Errorf("kind letter columns differ across tabs: changes=%d stash=%d worktree=%d",
			got[0], got[1], got[2])
	}
}

func TestChangesPaneKeepsSummaryWhenNoticeIsSet(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := fixedState()
	s.Notice = "git stash: exit 1"
	f := Pane(s, nil, w)
	if !strings.Contains(f.Lines[3], "files") {
		t.Errorf("summary line = %q, want file counts", f.Lines[3])
	}
	if strings.Contains(f.Lines[3], "exit 1") {
		t.Errorf("summary line should not carry the notice: %q", f.Lines[3])
	}
	found := false
	for _, line := range f.Lines {
		if strings.Contains(stripSGR(line), "exit 1") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("the notice should appear on its own line")
	}
}

// The window keeps the regions that land on the rows it drew and drops the
// rest, then moves the ones it kept up by the scroll. A region named for a row
// the window does not draw points at a row of someone else's, so a click in one
// place acts somewhere else.
//
// Both bounds matter. Joining them with and keeps everything, because no row is
// both above the window and past its end.
func TestTheScrolledWindowKeepsOnlyTheRegionsOnTheRowsItDrew(t *testing.T) {
	t.Parallel()
	list := []string{"a", "b", "c", "d", "e", "f"}
	regions := make([]Region, len(list))
	for i := range list {
		regions[i] = Region{Target: Target{Kind: TargetFile, Path: list[i], Row: i},
			Row: i, ColStart: 0, ColEnd: 1}
	}

	const top, rows = 2, 3
	shown, adj := sliceScrolled(list, regions, top, rows)
	if len(shown) != rows {
		t.Fatalf("the window drew %d rows, want %d", len(shown), rows)
	}
	if len(adj) != rows {
		t.Fatalf("the window kept %d regions, want the %d on the rows it drew: %+v",
			len(adj), rows, adj)
	}
	for i, r := range adj {
		if r.Row != i {
			t.Errorf("region %d sits on row %d, want %d", i, r.Row, i)
		}
		if want := list[top+i]; r.Target.Path != want {
			t.Errorf("row %d carries %q, want %q", i, r.Target.Path, want)
		}
	}
}

// The list and the diff split the body, and the diff area has a floor: below
// six rows it is not worth the three rows of chrome around it, so the list
// takes the whole body instead. Six is the smallest diff that is kept, not the
// largest that is dropped — a pane one row taller than the floor is the one
// where the reader first sees a diff at all, and moving the line by one costs
// them the whole area.
func TestTheDiffAreaIsKeptAtTheSmallestHeightItFitsIn(t *testing.T) {
	t.Parallel()
	s := ninetyFileState()
	s.Rows = s.Rows[:4]
	for _, c := range []struct {
		name               string
		body               int
		wantList, wantDiff int
	}{
		{"one row of diff short of the floor", 12, 12, 0},
		{"exactly the floor", 13, 4, minDiffRows},
		{"a row above the floor", 14, 4, minDiffRows + 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			list, diff := ListRowsFor(s, len(s.Rows), c.body)
			if list != c.wantList || diff != c.wantDiff {
				t.Errorf("body %d splits into list=%d diff=%d, want %d and %d",
					c.body, list, diff, c.wantList, c.wantDiff)
			}
		})
	}
}

// The diff area offers to stage the block under the reader when the open file
// is the row the cursor is on. A cursor that is not on a row cannot be compared
// with anything: the list moves under the pane — a reload shortens it between
// the frame a key was drawn on and the key arriving — and reading the row at a
// cursor the list no longer has reaches past its end.
func TestTheStageOfferIsWithheldWhenTheCursorIsOnNoRow(t *testing.T) {
	t.Parallel()
	rows := []state.Row{
		state.SectionHeadingRow(state.SectionUnstaged),
		state.FileRow(git.Entry{Path: "a.txt", Worktree: git.Modified}, state.SectionUnstaged),
	}
	for _, c := range []struct {
		name   string
		open   string
		cursor int
	}{
		{"a diff is open and the cursor is past the end", "a.txt", len(rows)},
		{"a diff is open and the cursor is far past the end", "a.txt", len(rows) + 99},
		{"a diff is open and the cursor is before the start", "a.txt", -1},
		{"no diff is open and the cursor is past the end", "", len(rows)},
		{"no diff is open and the cursor is on a row", "", 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := state.State{Width: 80, Height: 24, Tab: state.TabChanges,
				Rows: rows, Cursor: c.cursor}
			s.Changes.Folded = map[string]bool{}
			s.Changes.Selected = map[string]bool{}
			s.Open.Path = c.open
			if diffShowStage(s) {
				t.Error("the pane offers to stage a block of a row it cannot read")
			}
		})
	}

	// The offer stages the block the reader is looking at, and that is the block
	// of the open file. A cursor on another file's row is not on the open one,
	// so the row's own name has to be compared with the open path: staging
	// there would take a block out of a file the reader is not reading.
	twoFiles := []state.Row{
		state.SectionHeadingRow(state.SectionUnstaged),
		state.FileRow(git.Entry{Path: "a.txt", Worktree: git.Modified}, state.SectionUnstaged),
		state.FileRow(git.Entry{Path: "b.txt", Worktree: git.Modified}, state.SectionUnstaged),
	}
	other := state.State{Width: 80, Height: 24, Tab: state.TabChanges,
		Rows: twoFiles, Cursor: 2}
	other.Changes.Folded = map[string]bool{}
	other.Changes.Selected = map[string]bool{}
	other.Open.Path = "a.txt"
	if diffShowStage(other) {
		t.Error("the pane offers to stage a block while the cursor is on another file")
	}
	other.Cursor = 1
	if !diffShowStage(other) {
		t.Error("the pane does not offer to stage the block of the open file")
	}

	// A row that is not a file cannot be the open file, whatever it is called.
	// A directory row carries the name of the directory, and a diff open on a
	// path of that name would otherwise offer to stage a directory row.
	dirRows := []state.Row{
		state.SectionHeadingRow(state.SectionUnstaged),
		state.DirectoryRow("pkg", state.SectionUnstaged),
		state.FileRow(git.Entry{Path: "pkg/a.txt", Worktree: git.Modified},
			state.SectionUnstaged),
	}
	for i, row := range dirRows {
		if row.Kind() == state.RowFile {
			continue
		}
		s := state.State{Width: 80, Height: 24, Tab: state.TabChanges,
			Rows: dirRows, Cursor: i}
		s.Changes.Folded = map[string]bool{}
		s.Changes.Selected = map[string]bool{}
		s.Open.Path = row.Path()
		if diffShowStage(s) {
			t.Errorf("the pane offers to stage row %d, which is a %v", i, row.Kind())
		}
	}

	// The offer is made when the cursor is on the open file, so the answers
	// above are not simply "never".
	s := state.State{Width: 80, Height: 24, Tab: state.TabChanges, Rows: rows, Cursor: 1}
	s.Changes.Folded = map[string]bool{}
	s.Changes.Selected = map[string]bool{}
	s.Open.Path = "a.txt"
	if !diffShowStage(s) {
		t.Error("the pane does not offer to stage the block under the cursor")
	}
}

// The files of a stash, a worktree and a commit are all drawn under the row
// that holds them, and the reader learns the shape once: a row indented one
// level belongs to the row above it. A tab that drops the indent puts its files
// at the level of the rows they belong to, so the list reads as a flat one.
func TestFilesUnderARowAreIndentedTheSameOnEveryTabThatDrawsThem(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	indentOf := func(t *testing.T, tab state.Tab) int {
		t.Helper()
		s := fittingState(tab, 90, 30)
		s.Cursor = 0
		for _, line := range Pane(s, nil, w).Lines {
			if !strings.Contains(line, "nested.go") {
				continue
			}
			return len(line) - len(strings.TrimLeft(line, " "))
		}
		t.Fatalf("%v draws no file row, so this proves nothing", tab)
		return 0
	}

	stashed := indentOf(t, state.TabStashed)
	for _, tab := range []state.Tab{state.TabWorktrees, state.TabHistory} {
		if got := indentOf(t, tab); got != stashed {
			t.Errorf("%v indents its files by %d, and the stashed tab by %d",
				tab, got, stashed)
		}
	}

	// The row a file belongs to sits one level to its left, or the figure above
	// is the same on every tab because nothing is indented at all.
	s := fittingState(state.TabStashed, 90, 30)
	s.Cursor = 0
	for _, line := range Pane(s, nil, w).Lines {
		// Not the cursor row: its mark sits in the gutter and there is no
		// indent to read there.
		if !strings.Contains(line, "a stash message") || strings.Contains(line, cursorMark) {
			continue
		}
		parent := len(line) - len(strings.TrimLeft(line, " "))
		if stashed <= parent {
			t.Errorf("a file is indented by %d and the row holding it by %d",
				stashed, parent)
		}
		return
	}
	t.Fatal("the stashed tab draws no stash row")
}

// The box is what the reader fills to act on several files at once, and only
// the changes tab has verbs that act on a selection. A box on a tab whose verbs
// act on one row is one the reader can aim at and never fill.
func TestOnlyTheChangesTabDrawsTheSelectionBox(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	cursorOnFirstFile := func(t *testing.T, tab state.Tab) (state.State, string) {
		t.Helper()
		s := fittingState(tab, 90, 30)
		for i, row := range s.Rows {
			if row.Kind() == state.RowFile {
				s.Cursor = i
				return s, row.Path()
			}
		}
		t.Fatalf("%v has no file row, so this proves nothing", tab)
		return s, ""
	}
	lineFor := func(t *testing.T, s state.State, path string) string {
		t.Helper()
		want := path
		if i := strings.LastIndex(path, "/"); i >= 0 {
			want = path[i+1:]
		}
		for _, line := range Pane(s, nil, w).Lines {
			if strings.Contains(line, want) && strings.Contains(line, cursorMark) {
				return line
			}
		}
		t.Fatalf("no cursor row drew %q", want)
		return ""
	}

	for _, tab := range []state.Tab{state.TabHistory, state.TabStashed, state.TabWorktrees} {
		s, path := cursorOnFirstFile(t, tab)
		line := lineFor(t, s, path)
		if strings.Contains(line, "[ ]") || strings.Contains(line, "[✓]") {
			t.Errorf("%v draws a box on its file row: %q", tab, line)
		}
	}

	// The rows that hold those files are the same: a stash, a commit and a
	// worktree are acted on one at a time.
	for _, c := range []struct {
		tab  state.Tab
		kind state.RowKind
	}{
		{state.TabHistory, state.RowCommit},
		{state.TabStashed, state.RowStash},
		{state.TabWorktrees, state.RowWorktree},
	} {
		s := fittingState(c.tab, 90, 30)
		found := false
		for i, row := range s.Rows {
			if row.Kind() != c.kind {
				continue
			}
			s.Cursor = i
			found = true
			break
		}
		if !found {
			t.Fatalf("%v holds no %v row, so this proves nothing", c.tab, c.kind)
		}
		for _, line := range Pane(s, nil, w).Lines {
			if !strings.Contains(line, cursorMark) {
				continue
			}
			if strings.Contains(line, "[ ]") || strings.Contains(line, "[✓]") {
				t.Errorf("%v draws a box on its %v row: %q", c.tab, c.kind, line)
			}
		}
	}

	// The changes tab draws it, so the answers above are not "no tab draws one".
	s, path := cursorOnFirstFile(t, state.TabChanges)
	if line := lineFor(t, s, path); !strings.Contains(line, "[ ]") {
		t.Errorf("the changes tab draws no box on its cursor row: %q", line)
	}
}

// A pane with no room at all draws nothing: a terminal that reports zero
// columns or zero rows is one mirugit has nothing to put on, and padding a
// line to zero cells gives an empty string per row of a height that is also
// zero or less. The notice that the pane is too narrow belongs to a pane that
// has room for it.
func TestAPaneWithNoRoomDrawsNothing(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	for _, c := range []struct{ width, height int }{
		{0, 0}, {0, 24}, {80, 0}, {0, -3}, {-5, 24}, {80, -1},
		// Narrower than the pane will draw rows in, so the notice is what
		// would be drawn — and a height of none has no room for it either.
		{10, 0}, {10, -2}, {19, 0},
	} {
		s := state.State{Width: c.width, Height: c.height, Tab: state.TabChanges}
		s.Changes.Folded = map[string]bool{}
		s.Changes.Selected = map[string]bool{}
		if lines := Pane(s, nil, w).Lines; len(lines) != 0 {
			t.Errorf("a %dx%d pane drew %d lines: %q", c.width, c.height, len(lines), lines)
		}
	}

	// A pane too narrow for rows but with room for the notice draws it, so the
	// answers above are not "a narrow pane never draws".
	s := state.State{Width: 10, Height: 3, Tab: state.TabChanges}
	s.Changes.Folded = map[string]bool{}
	s.Changes.Selected = map[string]bool{}
	lines := Pane(s, nil, w).Lines
	if len(lines) != 3 {
		t.Fatalf("a 10x3 pane drew %d lines, want 3", len(lines))
	}
	if !strings.Contains(lines[0], "too narrow") {
		t.Errorf("the first line is %q, want the notice", lines[0])
	}
}

// A pane whose diff leaves no room for the list has no window to scroll: every
// line is below a window of no height, and reading that as "the cursor is past
// the bottom" drags the top down by one on every draw, so the list the reader
// gets back when the diff closes starts somewhere they never went.
func TestAListWithNoRowsDoesNotScroll(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name                     string
		top, listLen, cursorLine int
	}{
		{"the cursor below the top", 3, 10, 5},
		{"the cursor at the top", 3, 10, 3},
		{"the cursor far below", 0, 40, 39},
		{"nothing in the list", 0, 0, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			want := c.top
			if want > c.listLen {
				want = c.listLen
			}
			if got := ScrollTopFor(c.top, c.listLen, 0, c.cursorLine); got != want {
				t.Errorf("with no rows to show, the top moved from %d to %d",
					c.top, got)
			}
		})
	}

	// A window with rows does follow the cursor, so the answers above are not
	// "the top never moves".
	if got := ScrollTopFor(0, 40, 5, 9); got != 5 {
		t.Errorf("a window of five rows put the top at %d, want 5", got)
	}
}

// The number at the end of a directory row is how many files it holds, and it
// is dimmed: the name is what the reader is looking for, and a count drawn as
// brightly reads as part of it. The arrow and the name carry their own colors,
// so the count left plain is the one field that does not.
func TestADirectoryRowDimsTheCountOfFilesInIt(t *testing.T) {
	t.Parallel()
	w := Renderer{Palette: Palette{Enabled: true, Theme: cozmic}}
	line, _ := w.directoryRow("app/Sources", 12, "", false, false, 0, paneWidth)
	if !strings.Contains(line, w.Dim("12")) {
		t.Errorf("the count is not dimmed: %q", line)
	}
	if plain := stripSGR(line); !strings.HasSuffix(strings.TrimRight(plain, " "), "12") {
		t.Errorf("the row does not end with the count: %q", plain)
	}
}
