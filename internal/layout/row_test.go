package layout

import (
	"strings"
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

const paneWidth = 77

func TestStaleCursorRowShowsStaleReload(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	cursor := Row{
		Entry: git.Entry{Path: "a.txt", Worktree: git.Modified},
		State: rowState{Cursor: true, Stale: true},
		Index: 2,
	}
	line, regions := w.FileRow(cursor, paneWidth)
	if !strings.Contains(line, "stale · R reload") {
		t.Fatalf("line = %q, want stale · R reload", line)
	}
	if got := w.Of(line); got != paneWidth {
		t.Fatalf("line is %d cells, want %d: %q", got, paneWidth, line)
	}
	byteAt := strings.Index(line, "reload")
	if byteAt < 0 {
		t.Fatal("reload is not in the line")
	}
	reloadCol := w.Of(line[:byteAt])
	region, _, _ := Frame{Lines: []string{line}, Regions: regions}.Hit(0, reloadCol)
	if region.Kind != TargetVerb || region.Verb != state.VerbNameReload {
		t.Errorf("reload click = %+v, want TargetVerb reload", region)
	}
}

func TestConflictedRowVerbsOmitStage(t *testing.T) {
	t.Parallel()
	got := RowVerbs(git.Entry{Path: "both.txt", Index: git.Unmerged, Worktree: git.Unmerged}, state.SectionUnstaged)
	for _, v := range got {
		if v == state.VerbNameStage {
			t.Fatalf("conflicted row should not offer stage: %v", got)
		}
	}
}

func TestRowVerbsOmitsStashOnStagedRename(t *testing.T) {
	t.Parallel()
	got := RowVerbs(git.Entry{Path: "new.txt", OldPath: "old.txt", Index: git.Renamed}, state.SectionStaged)
	for _, v := range got {
		if v == state.VerbNameStash {
			t.Fatalf("staged rename should not offer stash: %v", got)
		}
	}
}

func TestFileRowIsExactlyThePaneWidth(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	cases := []Row{
		{Entry: git.Entry{Path: "a.txt", Worktree: git.Modified}, Count: git.Count{Added: 7, Deleted: 1}},
		{Entry: git.Entry{Path: strings.Repeat("long", 20) + ".swift", Worktree: git.Modified},
			Count: git.Count{Added: 1234, Deleted: 567}},
		{Entry: git.Entry{Path: "b.txt", Worktree: git.Modified},
			State: rowState{Cursor: true}, Verbs: []state.VerbName{state.VerbNameStage, state.VerbNameDiscard, state.VerbNameStash}},
		{Entry: git.Entry{Path: "新しいファイル名.swift", Worktree: git.Modified},
			Count: git.Count{Added: 3}},
	}
	for _, c := range cases {
		line, _ := w.FileRow(c, paneWidth)
		if got := w.Of(line); got != paneWidth {
			t.Errorf("%q is %d cells, want %d", line, got, paneWidth)
		}
	}
}

func TestFileRowSurvivesEveryPaneWidth(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	cases := []Row{
		{Entry: git.Entry{Path: "a.txt", Worktree: git.Modified}, Count: git.Count{Added: 7, Deleted: 1}},
		{Entry: git.Entry{Path: strings.Repeat("long", 20) + ".swift", Worktree: git.Modified},
			Count: git.Count{Added: 1234, Deleted: 567}},
		{Entry: git.Entry{Path: "b.txt", Worktree: git.Modified},
			State: rowState{Cursor: true}, Verbs: []state.VerbName{state.VerbNameStage, state.VerbNameDiscard, state.VerbNameStash}},
		{Entry: git.Entry{Path: "新しいファイル名.swift", Worktree: git.Modified},
			Count: git.Count{Added: 3}},
	}
	for _, c := range cases {
		for width := 1; width <= 80; width++ {
			line, _ := w.FileRow(c, width)
			if width >= 40 && w.Of(line) != width {
				t.Fatalf("width %d: %q is %d cells, want %d", width, line, w.Of(line), width)
			}
		}
	}
}

func TestCursorRowWithoutVerbsShowsFigures(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	cursor := Row{
		Entry: git.Entry{Path: "a.txt", Worktree: git.Modified},
		Count: git.Count{Added: 7, Deleted: 1},
		State: rowState{Cursor: true},
	}
	line, _ := w.FileRow(cursor, paneWidth)
	if !strings.Contains(line, "+7") {
		t.Fatalf("line = %q, want figures when the cursor row has no verbs", line)
	}
	if strings.Contains(line, "stage") {
		t.Fatalf("line = %q, should not show verbs", line)
	}
}

func TestCursorRowSwapsFiguresForVerbsWithoutChangingWidth(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	plain := Row{Entry: git.Entry{Path: "a.txt", Worktree: git.Modified}, Count: git.Count{Added: 7}}
	cursor := plain
	cursor.State.Cursor = true
	cursor.Verbs = []state.VerbName{state.VerbNameStage, state.VerbNameDiscard, state.VerbNameStash}

	a, _ := w.FileRow(plain, paneWidth)
	b, _ := w.FileRow(cursor, paneWidth)
	if w.Of(a) != w.Of(b) {
		t.Fatalf("width changed: %d vs %d", w.Of(a), w.Of(b))
	}
	if !strings.Contains(b, "stage") {
		t.Error("the cursor row should show its verbs")
	}
	if strings.Contains(b, "+7") {
		t.Error("the cursor row gives up its figures")
	}
}

func TestSelectionColumnIsReservedSoTickingMovesNothing(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	plain := Row{Entry: git.Entry{Path: "a.txt", Worktree: git.Modified}}
	ticked := plain
	ticked.State.Selected = true

	a, _ := w.FileRow(plain, paneWidth)
	b, _ := w.FileRow(ticked, paneWidth)
	// Compare cells, not bytes: the tick is three bytes and one cell, so a
	// byte index would report a move that the reader never sees.
	if cellsBefore(a, "a.txt") != cellsBefore(b, "a.txt") {
		t.Errorf("the name moved when the box was ticked:\n%q\n%q", a, b)
	}
}

func cellsBefore(line, needle string) int {
	w := Renderer{}
	i := strings.Index(line, needle)
	if i < 0 {
		return -1
	}
	return w.Of(line[:i])
}

func TestReadMarksUseCharactersNotColor(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	for mark, want := range map[state.Mark]string{
		state.Unread:  "·",
		state.Changed: "●",
		state.Read:    " ",
	} {
		r := Row{Entry: git.Entry{Path: "a.txt", Worktree: git.Modified},
			State: rowState{Read: mark}}
		line, _ := w.FileRow(r, paneWidth)
		if got := string([]rune(line)[markColumn]); got != want {
			t.Errorf("mark %v drew %q, want %q in %q", mark, got, want, line[:12])
		}
	}
}

func TestRowStateZeroValueIsUnread(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	line, _ := w.FileRow(Row{Entry: git.Entry{Path: "a.txt", Worktree: git.Modified}}, paneWidth)
	if got := string([]rune(line)[markColumn]); got != "·" {
		t.Errorf("zero rowState drew %q at mark column, want · in %q", got, line[:12])
	}
}

func TestTheRegionCoversTheNameAndNothingElse(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	r := Row{Entry: git.Entry{Path: "a.txt", Worktree: git.Modified}}
	line, regions := w.FileRow(r, paneWidth)

	if len(regions) != 1 {
		t.Fatalf("want one region, got %d", len(regions))
	}
	reg := regions[0]
	if reg.Target.Path != "a.txt" {
		t.Errorf("the region names %q", reg.Target.Path)
	}
	if got := cellSlice(line, reg.ColStart, reg.ColEnd); got != "a.txt" {
		t.Errorf("the region covers %q, not the name", got)
	}
}

func TestClickingACursorRowVerbReturnsTargetVerb(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	cursor := Row{
		Entry: git.Entry{Path: "a.txt", Worktree: git.Modified},
		State: rowState{Cursor: true},
		Verbs: []state.VerbName{state.VerbNameStage, state.VerbNameDiscard, state.VerbNameStash},
		Index: 2,
	}
	line, regions := w.FileRow(cursor, paneWidth)

	byteAt := strings.Index(line, "stage")
	if byteAt < 0 {
		t.Fatalf("stage is not in %q", line)
	}
	stageCol := w.Of(line[:byteAt])
	region, _, _ := Frame{Lines: []string{line}, Regions: regions}.Hit(0, stageCol)
	if region.Kind != TargetVerb || region.Verb != state.VerbNameStage || region.Row != 2 {
		t.Errorf("stage click = %+v, want TargetVerb stage index 2", region)
	}

	byteAt = strings.Index(line, "discard")
	if byteAt < 0 {
		t.Fatalf("discard is not in %q", line)
	}
	discardCol := w.Of(line[:byteAt])
	region, _, _ = Frame{Lines: []string{line}, Regions: regions}.Hit(0, discardCol)
	if region.Kind != TargetVerb || region.Verb != state.VerbNameDiscard {
		t.Errorf("discard click = %+v, want TargetVerb discard", region)
	}
}

func TestJapaneseCursorRowVerbRegionsAlignWithTheDrawnText(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	name := "新しいファイル名.swift"
	cursor := Row{
		Entry: git.Entry{Path: name, Worktree: git.Modified},
		State: rowState{Cursor: true},
		Verbs: []state.VerbName{state.VerbNameStage, state.VerbNameDiscard, state.VerbNameStash},
		Index: 0,
	}
	line, regions := w.FileRow(cursor, paneWidth)

	byteAt := strings.Index(line, "stash")
	if byteAt < 0 {
		t.Fatalf("stash is not in %q", line)
	}
	stashCol := w.Of(line[:byteAt])
	region, _, _ := Frame{Lines: []string{line}, Regions: regions}.Hit(0, stashCol)
	if region.Kind != TargetVerb || region.Verb != state.VerbNameStash {
		t.Errorf("stash click = %+v, want TargetVerb stash", region)
	}
}

func TestBinaryRowDrawsNoBar(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	r := Row{Entry: git.Entry{Path: "img.png", Worktree: git.Modified},
		Count: git.Count{Binary: true}}
	line, _ := w.FileRow(r, paneWidth)
	if strings.Contains(line, "█") {
		t.Error("a binary file has no line counts to draw a bar from")
	}
	if !strings.Contains(line, "binary") {
		t.Errorf("want the word binary in %q", line)
	}
}

// The directory row above already carries the directory, so repeating it on
// every file costs the width that the name itself needs.
func TestCursorRowVerbsCarryTheirKeys(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	cursor := Row{
		Entry: git.Entry{Path: "SyncOwnershipTests.swift", Worktree: git.Modified},
		State: rowState{Cursor: true},
		Verbs: []state.VerbName{state.VerbNameStage, state.VerbNameDiscard, state.VerbNameStash},
	}
	line, regions := w.FileRow(cursor, paneWidth)
	if !strings.Contains(line, "s stage  x discard  z stash") {
		t.Fatalf("line = %q, want keyed verbs under NO_COLOR", line)
	}
	if strings.Contains(line, "stage discard stash") {
		t.Fatalf("line = %q, want no bare verb-only field", line)
	}
	for _, r := range regions {
		if r.Target.Verb != state.VerbNameDiscard {
			continue
		}
		if got := cellSlice(line, r.ColStart, r.ColEnd); got != "x discard" {
			t.Fatalf("discard region = %q, want %q", got, "x discard")
		}
		return
	}
	t.Fatal("no region for discard")
}

func TestCursorRowWithVerbsIsExactly77Cells(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	cursor := Row{
		Entry: git.Entry{Path: "SyncOwnershipTests.swift", Worktree: git.Modified},
		State: rowState{Cursor: true},
		Verbs: []state.VerbName{state.VerbNameStage, state.VerbNameDiscard, state.VerbNameStash},
	}
	line, _ := w.FileRow(cursor, paneWidth)
	if got := w.Of(line); got != paneWidth {
		t.Fatalf("line is %d cells, want %d: %q", got, paneWidth, line)
	}
}

func TestFileRowShowsTheNameWithoutItsDirectory(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	line, _ := w.FileRow(Row{
		Entry: git.Entry{Path: "app/Sources/Infrastructure/Dependencies.swift",
			Worktree: git.Modified},
		Count: git.Count{Added: 7, Deleted: 1},
	}, paneWidth)
	if !strings.Contains(line, "Dependencies.swift") {
		t.Fatalf("no file name in %q", line)
	}
	if strings.Contains(line, "app/Sources") {
		t.Errorf("directory repeated on the file row: %q", line)
	}
}

func shortTxtCursorRow() Row {
	return Row{
		Entry: git.Entry{Path: "short.txt", Worktree: git.Modified},
		State: rowState{Cursor: true},
		Verbs: []state.VerbName{state.VerbNameStage, state.VerbNameDiscard, state.VerbNameStash},
	}
}

func TestCursorRowKeepsAGapBetweenNameAndVerbs(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	line, _ := w.FileRow(shortTxtCursorRow(), 40)
	if got := w.Of(line); got != 40 {
		t.Fatalf("line is %d cells, want 40: %q", got, line)
	}
	if strings.Contains(line, "short.txtstage") {
		t.Fatalf("name and verb touch: %q", line)
	}
	for _, verb := range []state.VerbName{state.VerbNameStage, state.VerbNameDiscard} {
		if !strings.Contains(line, verb.String()) {
			t.Fatalf("line = %q, want verb %q", line, verb)
		}
	}
	idx := strings.Index(line, "short.txt")
	if idx < 0 {
		t.Fatal("short.txt missing")
	}
	nameEndCol := w.Of(line[:idx+len("short.txt")])
	if got := cellSlice(line, nameEndCol, nameEndCol+1); got != " " {
		t.Fatalf("want a gap after the name, got %q in %q", got, line)
	}
}

func TestNarrowCursorRowDropsVerbsBeforeTheName(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	line, _ := w.FileRow(shortTxtCursorRow(), 34)
	if got := w.Of(line); got != 34 {
		t.Fatalf("line is %d cells, want 34: %q", got, line)
	}
	if !strings.Contains(line, "short.txt") {
		t.Fatalf("line = %q, want full short.txt name", line)
	}
	if !strings.Contains(line, "stage") {
		t.Fatalf("line = %q, want at least stage", line)
	}
	if strings.Contains(line, "stash") {
		t.Fatalf("line = %q, stash should be dropped", line)
	}
}

func TestWideCursorRowVerbsStayAtTheirColumn(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	line, _ := w.FileRow(shortTxtCursorRow(), paneWidth)
	wantCol := paneWidth - verbsWidth
	if got := cellsBefore(line, keyedVerb(state.VerbNameStage)); got != wantCol {
		t.Fatalf("verb field at column %d, want %d: %q", got, wantCol, line)
	}
}

func TestNarrowStaleCursorRowDropsTheLabelBeforeTheName(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	stale := shortTxtCursorRow()
	stale.State.Stale = true

	line, _ := w.FileRow(stale, 40)
	if got := w.Of(line); got != 40 {
		t.Fatalf("line is %d cells, want 40: %q", got, line)
	}
	if !strings.Contains(line, "stale · R reload") {
		t.Fatalf("line = %q, want stale label", line)
	}
	idx := strings.Index(line, "short.txt")
	if idx < 0 {
		t.Fatal("short.txt missing")
	}
	nameEndCol := w.Of(line[:idx+len("short.txt")])
	if got := cellSlice(line, nameEndCol, nameEndCol+1); got != " " {
		t.Fatalf("want a gap after the name, got %q in %q", got, line)
	}

	narrow, _ := w.FileRow(stale, 34)
	if got := w.Of(narrow); got != 34 {
		t.Fatalf("line is %d cells, want 34: %q", got, narrow)
	}
	if strings.Contains(narrow, "reload") {
		t.Fatalf("line = %q, stale label should be dropped", narrow)
	}
	if strings.Contains(narrow, "stage") {
		t.Fatalf("line = %q, verbs should not fill in for a dropped stale label", narrow)
	}
	if !strings.Contains(narrow, "short.txt") {
		t.Fatalf("line = %q, want full short.txt name", narrow)
	}
}

func TestStagedRowShowsTheIndexLetterNotTheWorktrees(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	e := git.Entry{Path: "i.txt", Index: git.Added, Worktree: git.Modified}

	staged, _ := w.FileRow(Row{Entry: e, Section: state.SectionStaged}, paneWidth)
	if !strings.Contains(staged, " A  i.txt") {
		t.Fatalf("staged line = %q, want index letter A", staged)
	}

	unstaged, _ := w.FileRow(Row{Entry: e, Section: state.SectionUnstaged}, paneWidth)
	if !strings.Contains(unstaged, " M  i.txt") {
		t.Fatalf("unstaged line = %q, want worktree letter M", unstaged)
	}
}

func TestStashCommitWorktreeCursorRowsShareVerbFieldStart(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	stashCommitCol := paneWidth - stashCommitWorktreeVerbsWidth
	// A worktree row keeps merge beside its verbs, and a cell between the two:
	// without it "x remove" and "merges" read as one word.
	worktreeCol := paneWidth - worktreeMergeWidth - rowVerbGap - stashCommitWorktreeVerbsWidth

	cases := []struct {
		name    string
		verb    state.VerbName
		wantCol int
		draw    func() []Region
	}{
		{
			name:    "stash",
			verb:    state.VerbNameDrop,
			wantCol: stashCommitCol,
			draw: func() []Region {
				_, regions := w.StashRow(stashRowLayout{
					Info:  stashRow("wip", git.StashApplies),
					State: stashRowState{Cursor: true},
					Verbs: []state.VerbName{state.VerbNameDrop},
				}, paneWidth)
				return regions
			},
		},
		{
			name:    "commit",
			verb:    state.VerbNameDiff,
			wantCol: stashCommitCol,
			draw: func() []Region {
				_, regions := w.CommitRow(commitRowLayout{
					Info:  commitInfo("subject", false),
					State: commitRowState{Cursor: true},
					Verbs: []state.VerbName{state.VerbNameDiff},
				}, paneWidth)
				return regions
			},
		},
		{
			name:    "worktree",
			verb:    state.VerbNameRemove,
			wantCol: worktreeCol,
			draw: func() []Region {
				_, regions := w.WorktreeRow(worktreeRowLayout{
					Info:  worktreeRow("wt", "feat/x", 0, git.MergeClean),
					State: worktreeRowState{Cursor: true},
					Verbs: []state.VerbName{state.VerbNameRemove},
				}, paneWidth)
				return regions
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for _, r := range c.draw() {
				if r.Target.Kind != TargetVerb || r.Target.Verb != c.verb {
					continue
				}
				if r.ColStart != c.wantCol {
					t.Fatalf("%s verb ColStart = %d, want %d", c.verb, r.ColStart, c.wantCol)
				}
				return
			}
			t.Fatalf("no region for verb %q", c.verb)
		})
	}
}

func TestCursorSwapLosesOneTrailingField(t *testing.T) {
	t.Parallel()
	w := Renderer{}

	filePlain := Row{Entry: git.Entry{Path: "a.txt", Worktree: git.Modified}, Count: git.Count{Added: 7}}
	fileCursor := filePlain
	fileCursor.State.Cursor = true
	fileCursor.Verbs = []state.VerbName{state.VerbNameStage, state.VerbNameDiscard, state.VerbNameStash}
	filePlainLine, _ := w.FileRow(filePlain, paneWidth)
	fileCursorLine, _ := w.FileRow(fileCursor, paneWidth)
	if !strings.Contains(filePlainLine, "+7") {
		t.Fatalf("plain file line = %q, want +7", filePlainLine)
	}
	if strings.Contains(fileCursorLine, "+7") {
		t.Fatalf("cursor file line = %q, should lose figures", fileCursorLine)
	}
	if !strings.Contains(fileCursorLine, "a.txt") {
		t.Fatalf("cursor file line = %q, want name", fileCursorLine)
	}

	commitPlain, _ := w.CommitRow(commitRowLayout{Info: commitInfo("subject", false)}, paneWidth)
	commitCursor, _ := w.CommitRow(commitRowLayout{
		Info:  commitInfo("subject", false),
		State: commitRowState{Cursor: true},
		Verbs: []state.VerbName{state.VerbNameDiff},
	}, paneWidth)
	if !strings.Contains(commitPlain, "abc123") || !strings.Contains(commitPlain, "2m") {
		t.Fatalf("plain commit line = %q, want the sha and the age", commitPlain)
	}
	if strings.Contains(commitCursor, "abc123") {
		t.Fatalf("cursor commit line = %q, should lose meta", commitCursor)
	}
	if !strings.Contains(commitCursor, "d diff") {
		t.Fatalf("cursor commit line = %q, want diff verb", commitCursor)
	}

	stashPlain, _ := w.StashRow(stashRowLayout{Info: stashRow("wip", git.StashApplies)}, paneWidth)
	stashCursor, _ := w.StashRow(stashRowLayout{
		Info:  stashRow("wip", git.StashApplies),
		State: stashRowState{Cursor: true},
		Verbs: []state.VerbName{state.VerbNameDrop},
	}, paneWidth)
	if !strings.Contains(stashPlain, "applies") {
		t.Fatalf("plain stash line = %q, want applies", stashPlain)
	}
	if strings.Contains(stashCursor, "applies") {
		t.Fatalf("cursor stash line = %q, should lose applies", stashCursor)
	}
	if !strings.Contains(stashCursor, "x drop") {
		t.Fatalf("cursor stash line = %q, want drop verb", stashCursor)
	}

	tree := worktreeRow("wt", "feat", 0, git.MergeClean)
	treePlain, _ := w.WorktreeRow(worktreeRowLayout{Info: tree}, paneWidth)
	treeCursor, _ := w.WorktreeRow(worktreeRowLayout{
		Info:  tree,
		State: worktreeRowState{Cursor: true},
		Verbs: []state.VerbName{state.VerbNameGo, state.VerbNameRemove},
	}, paneWidth)
	if !strings.Contains(treePlain, "↑2") || !strings.Contains(treePlain, "wrote 2m") {
		t.Fatalf("plain worktree line = %q, want ahead and wrote", treePlain)
	}
	if strings.Contains(treeCursor, "↑2") || strings.Contains(treeCursor, "wrote 2m") {
		t.Fatalf("cursor worktree line = %q, should lose ahead and wrote", treeCursor)
	}
	if !strings.Contains(treeCursor, "merges") {
		t.Fatalf("cursor worktree line = %q, want merge", treeCursor)
	}
	if !strings.Contains(treeCursor, "g go") {
		t.Fatalf("cursor worktree line = %q, want go verb", treeCursor)
	}
}

func TestFileRowPaintsTheConflictsLabelNotThePath(t *testing.T) {
	t.Parallel()
	w := Renderer{Palette: Palette{Enabled: true}}
	withConflicts := Row{
		Entry: git.Entry{Path: "pkg/conflicts/handler.go", Worktree: git.Modified},
		Count: git.Count{Added: 5},
		State: rowState{FullPath: true, Conflicts: true},
	}
	without := Row{
		Entry: git.Entry{Path: "pkg/handler.go", Worktree: git.Modified},
		Count: git.Count{Added: 5},
		State: rowState{FullPath: true, Conflicts: true},
	}
	lineA, _ := w.FileRow(withConflicts, paneWidth)
	lineB, _ := w.FileRow(without, paneWidth)
	colA := delConflictsColumn(w, lineA)
	colB := delConflictsColumn(w, lineB)
	if colA < 0 || colB < 0 {
		t.Fatalf("Del(conflicts) missing: %q and %q", lineA, lineB)
	}
	if colA != colB {
		t.Errorf("Del(conflicts) column differs: %d vs %d", colA, colB)
	}
}

func delConflictsColumn(w Renderer, line string) int {
	i := strings.Index(line, w.Del("conflicts"))
	if i < 0 {
		return -1
	}
	return w.Of(stripSGR(line[:i]))
}

// Every cursor row's verbs start in the same column, so the eye finds them in
// the same place down the list. That holds while the field is padded to its
// full width, and the field is padded whenever the room is there. A pane with
// room for exactly the field is the narrowest one where that is still true, and
// giving it the joined width instead shifts the keys of that one row.
func TestTheVerbFieldTakesItsFullWidthAtTheRoomThatFitsIt(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	verbs := RowVerbs(git.Entry{Path: "a.txt", Worktree: git.Modified}, state.SectionUnstaged)
	if len(verbs) == 0 {
		t.Fatal("the fixture row offers no verbs, so this proves nothing")
	}
	joined := w.keyedVerbJoinWidth(verbs)
	if joined >= verbsWidth {
		t.Fatalf("the verbs join to %d, which is not narrower than the field's %d: "+
			"the two widths have to differ for this to measure anything", joined, verbsWidth)
	}

	for _, c := range []struct {
		name  string
		row   Row
		state rowState
	}{
		{"a row offering verbs", Row{
			Entry: git.Entry{Path: "a.txt", Worktree: git.Modified},
			Verbs: verbs, State: rowState{Cursor: true}}, rowState{}},
		{"a row whose diff went stale", Row{
			Entry: git.Entry{Path: "a.txt", Worktree: git.Modified},
			Verbs: verbs, State: rowState{Cursor: true, Stale: true}}, rowState{}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := w.rowRightForCursor(c.row, verbsWidth).fieldWidth; got != verbsWidth {
				t.Errorf("room for exactly %d gives a field of %d", verbsWidth, got)
			}
			if got := w.rowRightForCursor(c.row, verbsWidth+1).fieldWidth; got != verbsWidth {
				t.Errorf("room for %d gives a field of %d", verbsWidth+1, got)
			}
			if got := w.rowRightForCursor(c.row, verbsWidth-1).fieldWidth; got == verbsWidth {
				t.Errorf("room for %d still gives the full field of %d",
					verbsWidth-1, verbsWidth)
			}
		})
	}
}

// The verbs of the cursor row sit in a field of a fixed width, so they start in
// the same column whichever row the cursor is on. Giving the field only the
// width of the words it holds moves that column with the row: the tip of an
// unpushed branch offers a verb the commits under it do not, and its keys would
// then sit in a different place from theirs as the cursor goes down the list.
func TestTheCursorRowsVerbsStartInTheSameColumnOnEveryRow(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	startOf := func(t *testing.T, cursor int, firstVerb string) int {
		t.Helper()
		s := fittingState(state.TabHistory, 40, 20)
		s.Cursor = cursor
		for _, line := range Pane(s, nil, w).Lines {
			if !strings.Contains(line, cursorMark) {
				continue
			}
			at := strings.Index(line, firstVerb)
			if at < 0 {
				t.Fatalf("the cursor row at %d does not offer %q: %q",
					cursor, firstVerb, line)
			}
			// In cells: the row holds marks and names the terminal draws in
			// one or two columns, and a byte count is neither.
			return w.Of(line[:at])
		}
		t.Fatalf("no row at %d drew the cursor", cursor)
		return 0
	}

	// The tip of an unpushed branch is the one row that offers uncommit.
	tip := startOf(t, 0, "u uncommit")
	below := startOf(t, 5, "d diff")
	if tip != below {
		t.Errorf("the tip's verbs start at column %d and a commit below it at %d",
			tip, below)
	}
}

// paintTail swaps the last cells of a laid-out line for the colored text of the
// same width. A swap that starts at the first cell is the whole line: the
// colored text is as wide as the line, and keeping the plain text as well
// leaves a row twice the width of the pane, which wraps and moves every row
// under it.
func TestPaintTailSwapsAWholeLineAsWellAsItsTail(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	const colored = "\x1b[31mabc\x1b[0m"
	for _, c := range []struct {
		name  string
		plain string
		width int
		cells int
		want  string
	}{
		{"the tail alone", "xyzabc", 6, 3, "xyz" + colored},
		{"the whole line", "abc", 3, 3, colored},
		{"wider than the line", "abc", 3, 5, colored},
		{"nothing to swap", "xyzabc", 6, 0, "xyzabc" + colored},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := w.paintTail(c.plain, c.width, colored, c.cells); got != c.want {
				t.Errorf("paintTail(%q, %d, colored, %d) = %q, want %q",
					c.plain, c.width, c.cells, got, c.want)
			}
		})
	}
}

// A renamed file draws both names. The old one is drawn by its name alone
// while the file stayed in its directory, because the directory is the same one
// the row already sits under; a file that moved carries the directory it came
// from, which is the whole of what happened to it. Swapping the two leaves a
// move looking like a rename in place, and a rename in place repeating a
// directory the reader can already see.
func TestARenameNamesTheDirectoryItCameFromOnlyWhenItMoved(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	for _, c := range []struct {
		name string
		e    git.Entry
		want string
	}{
		{"renamed in place", git.Entry{Path: "pkg/new.go", OldPath: "pkg/old.go"},
			"new.go ← old.go"},
		{"moved to another directory",
			git.Entry{Path: "pkg/new.go", OldPath: "other/old.go"},
			"new.go ← other/old.go"},
		{"moved out of the root", git.Entry{Path: "pkg/new.go", OldPath: "old.go"},
			"new.go ← old.go"},
		{"moved into the root", git.Entry{Path: "new.go", OldPath: "pkg/old.go"},
			"new.go ← pkg/old.go"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := w.rename(c.e, 60); got != c.want {
				t.Errorf("rename = %q, want %q", got, c.want)
			}
		})
	}
}

// A renamed file draws where it came from when there is room for eight cells
// of the old name; below that the row keeps the new name alone. Eight is the
// narrowest old name that still says something — the mark that text was
// dropped plus a few characters — so it is the room that is kept, not the
// first one refused.
func TestARenameKeepsItsOldNameAtTheNarrowestRoomThatHoldsIt(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	e := git.Entry{Path: "pkg/new.go", OldPath: "other/a-long-old-name.go"}
	const sep = " ← "
	fits := w.Of("new.go") + w.Of(sep) + 8

	if got := w.rename(e, fits); !strings.Contains(got, sep) {
		t.Errorf("at %d columns the row is %q, and the old name fits", fits, got)
	}
	if got := w.rename(e, fits-1); strings.Contains(got, sep) {
		t.Errorf("at %d columns the row is %q, and the old name does not fit",
			fits-1, got)
	}
	// Whatever the room, the row never draws more than it was given.
	for room := 0; room <= 60; room++ {
		if got := w.Of(w.rename(e, room)); got > room {
			t.Errorf("at %d columns the row is %d cells: %q", room, got, w.rename(e, room))
		}
	}
}

// The stale label is dropped only when it does not fit beside the shortest name
// the pane promises to show. A pane it fills exactly has room for it: dropping
// it there leaves the row of the open file saying nothing about why its diff is
// not being staged.
func TestTheStaleLabelIsDrawnOnTheNarrowestRowThatHoldsIt(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	const label = "stale · R reload"
	stale := shortTxtCursorRow()
	stale.State.Stale = true

	// The marks left of the name are the same width at every pane width, so a
	// wide row is where their width can be measured.
	wide, _ := w.FileRow(stale, 100)
	at := strings.Index(wide, "short.txt")
	if at < 0 {
		t.Fatalf("the name is not drawn, so the left of it cannot be measured: %q", wide)
	}
	fits := w.Of(wide[:at]) + minRowNameRoom + rowVerbGap + w.Of(label)

	for _, c := range []struct {
		name  string
		width int
		want  bool
	}{
		{"the narrowest row that holds it", fits, true},
		{"one cell narrower", fits - 1, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			line, _ := w.FileRow(stale, c.width)
			if got := strings.Contains(line, label); got != c.want {
				t.Errorf("the label is %v on a row of %d cells, want %v: %q",
					got, c.width, c.want, line)
			}
			if got := w.Of(line); got != c.width {
				t.Errorf("the row is %d cells on a pane of %d: %q", got, c.width, line)
			}
		})
	}
}
