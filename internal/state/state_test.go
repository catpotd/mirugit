package state

import (
	"reflect"
	"strings"
	"testing"

	"github.com/catpotd/mirugit/internal/git"
)

func entries(paths ...string) []git.Entry {
	out := make([]git.Entry, 0, len(paths))
	for _, p := range paths {
		out = append(out, git.Entry{Path: p, Worktree: git.Modified})
	}
	return out
}

func rows(paths ...string) []Row {
	out := make([]Row, 0, len(paths))
	for _, p := range paths {
		out = append(out, FileRow(

			git.Entry{Path: p, Worktree: git.Modified},
			SectionUnstaged))
	}
	return out
}

func TestTabConstantsMatchTheHeaderOrder(t *testing.T) {
	t.Parallel()
	if TabChanges != 0 || TabHistory != 1 || TabStashed != 2 || TabWorktrees != 3 {
		t.Errorf("got %d %d %d %d", TabChanges, TabHistory, TabStashed, TabWorktrees)
	}
}

func TestTheCursorStopsAtTheEnds(t *testing.T) {
	t.Parallel()
	s := State{Rows: rows("a", "b", "c")}
	s = Apply(s, CursorMoved{By: -1})
	if s.Cursor != 0 {
		t.Errorf("above the top: got %d", s.Cursor)
	}
	for range 10 {
		s = Apply(s, CursorMoved{By: 1})
	}
	if s.Cursor != 2 {
		t.Errorf("below the bottom: got %d", s.Cursor)
	}
}

func TestOpeningAFileClosesThePreviousOne(t *testing.T) {
	t.Parallel()
	s := State{Rows: rows("a", "b")}
	s = Apply(s, DiffOpened{Path: "a", Origin: WorkingTree(SectionUnstaged)})
	s = Apply(s, DiffOpened{Path: "b", Origin: WorkingTree(SectionUnstaged)})
	if s.Open.Path != "b" {
		t.Errorf("got %q", s.Open.Path)
	}
	if s.Open.BlockCursor != 0 {
		t.Error("the block cursor should start at the top of the new file")
	}
	if s.Open.Peek {
		t.Error("a normal open should not peek the diff")
	}
}

func TestDiffPeekOpensFromAClosedDiff(t *testing.T) {
	t.Parallel()
	s := State{Rows: rows("a"), Open: OpenDiff{Closed: true, ClosedFor: "a"}}
	s = Apply(s, DiffOpened{Path: "a", Peek: true, Origin: WorkingTree(SectionUnstaged)})
	if !s.Open.Peek || s.Open.Path != "a" {
		t.Fatalf("got open=%q peek=%v", s.Open.Path, s.Open.Peek)
	}
}

func TestTheOpenFileLeavingTheListClosesTheDiff(t *testing.T) {
	t.Parallel()
	s := State{Rows: rows("a", "b"), Cursor: 0, Open: OpenDiff{Path: "a"}}
	s = Apply(s, StatusLoaded{Rows: entries("b")})
	if s.Open.Path != "" {
		t.Errorf("the diff stayed open on a file that is gone: %q", s.Open.Path)
	}
}

func TestStagingAWholeFileClosesItsUnstagedDiff(t *testing.T) {
	t.Parallel()
	s := Apply(State{}, StatusLoaded{Rows: []git.Entry{
		{Path: "a", Worktree: git.Modified},
		{Path: "b", Worktree: git.Modified},
	}})
	s = Apply(s, DiffOpened{Path: "a", Origin: WorkingTree(SectionUnstaged)})
	s.Open.Diff = git.FileDiff{Path: "a", Blocks: []git.Block{{}}}
	s = Apply(s, StatusLoaded{Rows: []git.Entry{
		{Path: "a", Index: git.Modified},
		{Path: "b", Worktree: git.Modified},
	}})
	if s.Open.Path != "" {
		t.Errorf("OpenPath = %q, want empty", s.Open.Path)
	}
	if s.Open.Origin.Side() != SectionNone {
		t.Errorf("OpenSection = %v, want SectionNone", s.Open.Origin.Side())
	}
	if len(s.Open.Diff.Blocks) != 0 {
		t.Errorf("len(Diff.Blocks) = %d, want 0", len(s.Open.Diff.Blocks))
	}
}

func TestAStagedDiffStaysOpenWhileItsRowRemains(t *testing.T) {
	t.Parallel()
	s := Apply(State{}, StatusLoaded{Rows: []git.Entry{
		{Path: "a", Index: git.Modified},
	}})
	s = Apply(s, DiffOpened{Path: "a", Origin: WorkingTree(SectionStaged)})
	rows := []git.Entry{{Path: "a", Index: git.Modified}}
	s = Apply(s, StatusLoaded{Rows: rows})
	if s.Open.Path != "a" {
		t.Errorf("OpenPath = %q, want a", s.Open.Path)
	}
}

func TestReloadFallsBackToTheRowNumberWhenTheCursorRowIsGone(t *testing.T) {
	t.Parallel()
	s := State{Rows: rows("a", "b", "c"), Cursor: 2}
	s = Apply(s, StatusLoaded{Rows: entries("a", "b")})
	if s.Cursor != 2 {
		t.Errorf("got %d, want the same row index clamped in range", s.Cursor)
	}
}

func TestReloadKeepsTheCursorOnTheSameFile(t *testing.T) {
	t.Parallel()
	s := Apply(State{}, StatusLoaded{Rows: entries("aa", "b")})
	for i, r := range s.Rows {
		if r.Kind() == RowFile && r.Path() == "b" {
			s.Cursor = i
			break
		}
	}
	s = Apply(s, StatusLoaded{Rows: entries("aa", "ab", "b")})
	r := s.Rows[s.Cursor]
	if r.Kind() != RowFile || r.Path() != "b" {
		t.Errorf("got kind=%v path=%q at cursor %d", r.Kind(), r.Path(), s.Cursor)
	}
}

func TestReloadKeepsTheCursorOnTheSameDirectory(t *testing.T) {
	t.Parallel()
	s := Apply(State{}, StatusLoaded{Rows: entries("z", "src/c")})
	for i, r := range s.Rows {
		if r.Kind() == RowDirectory && r.DirPath() == "src" {
			s.Cursor = i
			break
		}
	}
	s = Apply(s, StatusLoaded{Rows: entries("a", "z", "src/c")})
	r := s.Rows[s.Cursor]
	if r.Kind() != RowDirectory || r.DirPath() != "src" {
		t.Errorf("got kind=%v dir=%q at cursor %d", r.Kind(), r.DirPath(), s.Cursor)
	}
}

func TestReloadKeepsTheCursorOnTheSameSectionOfADuplicatedPath(t *testing.T) {
	t.Parallel()
	dup := git.Entry{Path: "i", Index: git.Modified, Worktree: git.Modified}
	s := Apply(State{}, StatusLoaded{Rows: []git.Entry{dup}})
	for i, r := range s.Rows {
		if r.Kind() == RowFile && r.Section() == SectionUnstaged && r.Path() == "i" {
			s.Cursor = i
			break
		}
	}
	s = Apply(s, StatusLoaded{Rows: []git.Entry{
		{Path: "h", Index: git.Modified},
		dup,
	}})
	r := s.Rows[s.Cursor]
	if r.Section() != SectionUnstaged || r.Path() != "i" {
		t.Errorf("got section=%v path=%q at cursor %d", r.Section(), r.Path(), s.Cursor)
	}
}

func TestChangingTabClosesTheDiffAndClearsScroll(t *testing.T) {
	t.Parallel()
	s := State{Rows: rows("a", "b"), Cursor: 1, Tab: 0, ScrollTop: 5, Open: OpenDiff{Path: "a"}}
	s = Apply(s, TabChanged{Tab: 2})
	if s.Tab != 2 || s.Open.Path != "" || s.ScrollTop != 0 {
		t.Errorf("got %+v", s)
	}
}

func TestReturningToATabRestoresTheCursor(t *testing.T) {
	t.Parallel()
	s := State{Tab: TabChanges, Changes: Changes{Entries: entries("a", "b", "c")}}
	s.Rows = displayOrder(s.Changes.Entries)
	s.Cursor = len(s.Rows) - 1
	left := s.Cursor
	s = Apply(s, TabChanged{Tab: TabHistory})
	s = Apply(s, TabChanged{Tab: TabChanges})
	if s.Cursor != left {
		t.Errorf("got %d, want %d", s.Cursor, left)
	}
}

func TestChangingToTheCurrentTabChangesNothing(t *testing.T) {
	t.Parallel()
	s := State{Tab: TabChanges, Changes: Changes{Entries: entries("a", "b")}}
	s.Rows = displayOrder(s.Changes.Entries)
	s.Cursor = FirstFileRow(s.Rows)
	s.Open.Path = "a"
	s.ScrollTop = 5
	s = Apply(s, SelectionToggled{Path: "a", Section: SectionUnstaged})
	beforeCursor := s.Cursor
	beforeOpen := s.Open.Path
	beforeScroll := s.ScrollTop
	beforeSelected := len(s.Changes.Selected)
	s = Apply(s, TabChanged{Tab: TabChanges})
	if s.Cursor != beforeCursor || s.Open.Path != beforeOpen || s.ScrollTop != beforeScroll || len(s.Changes.Selected) != beforeSelected {
		t.Errorf("got cursor=%d open=%q scroll=%d selected=%d", s.Cursor, s.Open.Path, s.ScrollTop, len(s.Changes.Selected))
	}
}

func TestChangingTabsReportsTheClearedSelection(t *testing.T) {
	t.Parallel()
	s := State{Tab: TabChanges, Changes: Changes{Entries: entries("a", "b")}}
	s.Rows = displayOrder(s.Changes.Entries)
	s = Apply(s, SelectionToggled{Path: "a", Section: SectionUnstaged})
	s = Apply(s, SelectionToggled{Path: "b", Section: SectionUnstaged})
	s = Apply(s, TabChanged{Tab: TabHistory})
	if len(s.Changes.Selected) != 0 {
		t.Errorf("got %d selected, want 0", len(s.Changes.Selected))
	}
	if s.Notice != "2 deselected · tab changed" {
		t.Errorf("got notice %q", s.Notice)
	}
	s = Apply(s, TabChanged{Tab: TabChanges})
	if s.Notice != "2 deselected · tab changed" {
		t.Errorf("notice cleared on return: got %q", s.Notice)
	}
}

func TestChangingTabsWithoutSelectionKeepsTheNotice(t *testing.T) {
	t.Parallel()
	s := State{Tab: TabChanges, Notice: "committed abc1234 · 1 file"}
	s = Apply(s, TabChanged{Tab: TabHistory})
	if s.Notice != "committed abc1234 · 1 file" {
		t.Errorf("got notice %q", s.Notice)
	}
}

func TestChangingTabsClearsAFailedNotice(t *testing.T) {
	t.Parallel()
	s := State{Tab: TabChanges, Notice: "fatal: outside repository", NoticeFailed: true}
	s = Apply(s, TabChanged{Tab: TabHistory})
	if s.Notice != "" {
		t.Errorf("got notice %q, want empty", s.Notice)
	}
	if s.NoticeFailed {
		t.Error("NoticeFailed should be cleared")
	}
}

func TestARestoredCursorIsClampedWhenTheListShrank(t *testing.T) {
	t.Parallel()
	s := State{Tab: TabChanges, Changes: Changes{Entries: entries("a", "b", "c")}}
	s.Rows = displayOrder(s.Changes.Entries)
	s.Cursor = len(s.Rows) - 1
	s = Apply(s, TabChanged{Tab: TabHistory})
	s = Apply(s, StatusLoaded{Rows: entries("a")})
	s = Apply(s, TabChanged{Tab: TabChanges})
	if s.Cursor < 0 || s.Cursor > len(s.Rows)-1 {
		t.Errorf("cursor %d out of range [0, %d]", s.Cursor, len(s.Rows)-1)
	}
}

func TestFoldingADirectoryToggles(t *testing.T) {
	t.Parallel()
	s := State{Changes: Changes{Folded: map[string]bool{}}}
	s = Apply(s, DirectoryFolded{Path: "app", Section: SectionUnstaged})
	if !FoldedIn(s, SectionUnstaged, "app") {
		t.Error("want folded")
	}
	s = Apply(s, DirectoryFolded{Path: "app", Section: SectionUnstaged})
	if FoldedIn(s, SectionUnstaged, "app") {
		t.Error("want unfolded")
	}
}

func TestCursorSkipsRowsHiddenByAFoldedDirectory(t *testing.T) {
	t.Parallel()
	s := State{Tab: TabChanges, Cursor: 1, Rows: []Row{
		SectionHeadingRow(SectionUnstaged),
		DirectoryRow("app", SectionUnstaged),
		DirectoryRow("app/internal", SectionUnstaged),
		FileRow(git.Entry{Path: "app/internal/a.go", Worktree: git.Modified}, SectionUnstaged),
		FileRow(git.Entry{Path: "other/b.go", Worktree: git.Modified}, SectionUnstaged),
	}, Changes: Changes{Folded: map[string]bool{foldKey(SectionUnstaged, "app"): true}}}

	s = Apply(s, CursorMoved{By: 1})
	if s.Cursor != 4 {
		t.Errorf("cursor = %d, want the next visible row 4", s.Cursor)
	}
}

func TestVerbRequestedSetsPending(t *testing.T) {
	t.Parallel()
	s := State{Changes: Changes{Selected: map[string]bool{}}}
	s = Apply(s, VerbRequested{Verb: VerbStage, Targets: []string{"a.txt"}})
	if s.Changes.Pending == nil || s.Changes.Pending.Verb != VerbStage {
		t.Fatalf("got %+v", s.Changes.Pending)
	}
}

func TestVerbFinishedClearsPendingAndSetsNotice(t *testing.T) {
	t.Parallel()
	req := VerbRequested{Verb: VerbStage, Targets: []string{"a.txt"}}
	s := State{Changes: Changes{Pending: &req, Selected: map[string]bool{}}}
	s = Apply(s, VerbFinished{Verb: VerbStage, Ignored: []string{"a.txt"}})
	if s.Changes.Pending != nil {
		t.Fatal("pending should be cleared")
	}
	if s.Notice != "1 unchanged" {
		t.Errorf("got %q", s.Notice)
	}
}

func TestStashFinishedSetsNothingStashedNotice(t *testing.T) {
	t.Parallel()
	s := State{Changes: Changes{Selected: map[string]bool{}}}
	s = Apply(s, VerbFinished{Verb: VerbStash, Ignored: []string{"a.txt", "b.txt"}})
	if s.Notice != "stashed nothing · 2 files had no changes" {
		t.Errorf("got %q", s.Notice)
	}
}

func TestStashFinishedConsumesSelectionAndSaysStashed(t *testing.T) {
	t.Parallel()
	s := State{Rows: rows("a", "b", "c"), Changes: Changes{Selected: map[string]bool{}}}
	for _, p := range []string{"a", "b", "c"} {
		s = Apply(s, SelectionToggled{Path: p, Section: SectionUnstaged})
	}
	s = Apply(s, VerbRequested{Verb: VerbStash, Targets: []string{"a", "b", "c"}, Block: -1})
	s = Apply(s, VerbFinished{Verb: VerbStash, Applied: 3})
	if len(s.Changes.Selected) != 0 {
		t.Fatalf("got %v", s.Changes.Selected)
	}
	if s.Notice != "stashed 3 files" {
		t.Errorf("got %q", s.Notice)
	}
	s = Apply(s, StatusLoaded{Rows: nil})
	if s.Notice != "stashed 3 files" {
		t.Errorf("after reload got %q", s.Notice)
	}
}

func TestStashFinishedSaysStashedOneFile(t *testing.T) {
	t.Parallel()
	s := State{Changes: Changes{Selected: map[string]bool{}}}
	s = Apply(s, VerbRequested{Verb: VerbStash, Targets: []string{"a"}, Block: -1})
	s = Apply(s, VerbFinished{Verb: VerbStash, Applied: 1})
	if s.Notice != "stashed 1 file" {
		t.Errorf("got %q", s.Notice)
	}
}

func TestStashFinishedKeepsIgnoredPathsSelected(t *testing.T) {
	t.Parallel()
	s := State{Rows: rows("a", "b"), Changes: Changes{Selected: map[string]bool{}}}
	for _, p := range []string{"a", "b"} {
		s = Apply(s, SelectionToggled{Path: p, Section: SectionUnstaged})
	}
	s = Apply(s, VerbRequested{Verb: VerbStash, Targets: []string{"a", "b"}, Block: -1})
	s = Apply(s, VerbFinished{Verb: VerbStash, Applied: 1, Ignored: []string{"b"}})
	if len(s.Changes.Selected) != 1 {
		t.Fatalf("got %v", s.Changes.Selected)
	}
	if !SelectedIn(s, SectionUnstaged, "b") {
		t.Error("b should remain selected")
	}
}

func TestBlockStageFinishedKeepsTheSelection(t *testing.T) {
	t.Parallel()
	s := State{Rows: rows("a"), Changes: Changes{Selected: map[string]bool{}}}
	s = Apply(s, SelectionToggled{Path: "a", Section: SectionUnstaged})
	s = Apply(s, VerbRequested{Verb: VerbStage, Targets: []string{"a"}, Block: 0})
	s = Apply(s, VerbFinished{Verb: VerbStage, Applied: 1})
	if len(s.Changes.Selected) != 1 {
		t.Fatalf("got %v", s.Changes.Selected)
	}
}

func TestStageFinishedConsumesOnlyTheUnstagedTick(t *testing.T) {
	t.Parallel()
	s := State{Rows: []Row{
		FileRow(
			git.Entry{Path: "i", Worktree: git.Modified}, SectionUnstaged),

		FileRow(
			git.Entry{Path: "i", Index: git.Modified}, SectionStaged),
	}, Changes: Changes{Selected: map[string]bool{}}}
	s = Apply(s, SelectionToggled{Path: "i", Section: SectionUnstaged})
	s = Apply(s, SelectionToggled{Path: "i", Section: SectionStaged})
	s = Apply(s, VerbRequested{Verb: VerbStage, Targets: []string{"i"}, Block: -1})
	s = Apply(s, VerbFinished{Verb: VerbStage, Applied: 1})
	if !SelectedIn(s, SectionStaged, "i") {
		t.Error("staged tick should remain")
	}
	if SelectedIn(s, SectionUnstaged, "i") {
		t.Error("unstaged tick should be consumed")
	}
	if len(s.Changes.Selected) != 1 {
		t.Fatalf("got %v", s.Changes.Selected)
	}
}

func TestDiscardFinishedNoticeSurvivesTheReload(t *testing.T) {
	t.Parallel()
	s := State{Rows: rows("a", "b"), Changes: Changes{Selected: map[string]bool{}}}
	for _, p := range []string{"a", "b"} {
		s = Apply(s, SelectionToggled{Path: p, Section: SectionUnstaged})
	}
	s.Changes.Pending = &VerbRequested{Verb: VerbDiscard, Targets: []string{"a", "b"}, Block: -1}
	s = Apply(s, DiscardFinished{Applied: 2})
	if strings.Contains(s.Notice, "still here") {
		t.Errorf("got %q", s.Notice)
	}
	if !strings.Contains(s.Notice, "U undo") {
		t.Errorf("got %q", s.Notice)
	}
	if len(s.Changes.Selected) != 0 {
		t.Fatalf("got %v", s.Changes.Selected)
	}
	s = Apply(s, StatusLoaded{Rows: entries("c")})
	if strings.Contains(s.Notice, "still here") {
		t.Errorf("after reload got %q", s.Notice)
	}
}

func TestDiscardConfirmationShownSetsPendingConfirm(t *testing.T) {
	t.Parallel()
	s := State{Changes: Changes{Selected: map[string]bool{}}}
	confirm := DiscardConfirm{Files: 1, Targets: []string{"a.txt"}}
	s = Apply(s, DiscardConfirmationShown{Confirm: confirm})
	if s.Changes.DiscardConfirm == nil || s.Changes.DiscardConfirm.Files != 1 {
		t.Fatalf("got %+v", s.Changes.DiscardConfirm)
	}
}

func TestStatusLoadedDropsAConfirmWhoseTargetIsGone(t *testing.T) {
	t.Parallel()
	s := State{Tab: TabChanges, Changes: Changes{Selected: map[string]bool{}, DiscardConfirm: &DiscardConfirm{Targets: []string{"a.txt"}}}}
	s = Apply(s, StatusLoaded{Rows: entries("b.txt")})
	if s.Changes.DiscardConfirm != nil {
		t.Fatal("confirmation should be dropped when the target is gone")
	}
	if s.Notice != "nothing to discard · the changes are gone" {
		t.Errorf("got notice %q", s.Notice)
	}
}

func TestStatusLoadedKeepsAConfirmWhoseTargetsRemain(t *testing.T) {
	t.Parallel()
	s := State{Tab: TabChanges, Changes: Changes{Selected: map[string]bool{}, DiscardConfirm: &DiscardConfirm{Targets: []string{"a.txt"}}}}
	s = Apply(s, StatusLoaded{Rows: entries("a.txt")})
	if s.Changes.DiscardConfirm == nil {
		t.Fatal("confirmation should remain when targets are still present")
	}
}

func TestDiscardFinishedSetsUndoNotice(t *testing.T) {
	t.Parallel()
	s := State{Changes: Changes{Selected: map[string]bool{}}}
	s = Apply(s, DiscardFinished{Applied: 3, Added: 142, Deleted: 4, Undo: git.Undo{Commit: "abc"}})
	if s.LastUndo == nil || s.LastUndo.Commit != "abc" {
		t.Fatalf("got %+v", s.LastUndo)
	}
	if !strings.Contains(s.Notice, "discarded 3 files") || !strings.Contains(s.Notice, "U undo") {
		t.Errorf("got %q", s.Notice)
	}
	// The figures say how much was thrown away, and the minus is U+2212 rather
	// than a hyphen because that is what every other line carrying a count uses.
	if !strings.Contains(s.Notice, "+142 −4") {
		t.Errorf("the notice does not say what was discarded: %q", s.Notice)
	}
}

func TestUndiscardConfirmedClearsTheConfirm(t *testing.T) {
	t.Parallel()
	undo := git.Undo{Ref: "refs/mirugit/undo/1", Commit: "abc"}
	s := State{
		Changes: Changes{UndiscardConfirm: &UndiscardConfirm{Undo: undo, Files: 1}},
	}
	s = Apply(s, UndiscardConfirmed{})
	if s.Changes.UndiscardConfirm != nil {
		t.Fatalf("got %+v, want nil", s.Changes.UndiscardConfirm)
	}
}

func TestCommitFinishedClearsTheMessageAndSetsNotice(t *testing.T) {
	t.Parallel()
	s := State{Changes: Changes{Message: "fix", MessageFocused: true, Selected: map[string]bool{}}}
	s = Apply(s, CommitFinished{SHA: "3f2a1b", Files: 2})
	if s.Changes.Message != "" || s.Changes.MessageFocused {
		t.Errorf("got message=%q focused=%v", s.Changes.Message, s.Changes.MessageFocused)
	}
	if s.Notice != "committed 3f2a1b · 2 files" {
		t.Errorf("got %q", s.Notice)
	}
}

func TestMessageFocusedAndEdited(t *testing.T) {
	t.Parallel()
	s := State{Changes: Changes{Selected: map[string]bool{}}}
	s = Apply(s, MessageFocused{})
	if !s.Changes.MessageFocused {
		t.Fatal("want focused")
	}
	s = Apply(s, MessageEdited{Text: "feat"})
	if s.Changes.Message != "feat" {
		t.Errorf("got %q", s.Changes.Message)
	}
	s = Apply(s, MessageBlurred{})
	if s.Changes.MessageFocused {
		t.Error("want blurred")
	}
}

func TestHelpOpenedAndClosed(t *testing.T) {
	t.Parallel()
	s := State{Changes: Changes{Selected: map[string]bool{}}}
	s = Apply(s, HelpOpened{})
	if !s.HelpOpen {
		t.Fatal("want help open")
	}
	if s.HelpScroll != 0 {
		t.Errorf("HelpScroll = %d, want 0 on open", s.HelpScroll)
	}
	s = Apply(s, HelpClosed{})
	if s.HelpOpen {
		t.Error("want help closed")
	}
}

func TestHelpScrolledMovesHelpScroll(t *testing.T) {
	t.Parallel()
	s := State{HelpScroll: 2, Changes: Changes{Selected: map[string]bool{}}}
	s = Apply(s, HelpScrolled{By: 1})
	if s.HelpScroll != 3 {
		t.Errorf("HelpScroll = %d, want 3", s.HelpScroll)
	}
}

func TestHelpOpenedResetsHelpScroll(t *testing.T) {
	t.Parallel()
	s := State{HelpScroll: 5, Changes: Changes{Selected: map[string]bool{}}}
	s = Apply(s, HelpOpened{})
	if s.HelpScroll != 0 {
		t.Errorf("HelpScroll = %d, want 0", s.HelpScroll)
	}
}

func TestStaleChangedMarksAndClearsAPath(t *testing.T) {
	t.Parallel()
	s := State{Changes: Changes{Selected: map[string]bool{}, Stale: map[string]bool{}}}
	s = Apply(s, StaleChanged{Path: "a.txt", Stale: true})
	if !s.Changes.Stale["a.txt"] {
		t.Fatal("want stale")
	}
	s = Apply(s, StaleChanged{Path: "a.txt", Stale: false})
	if s.Changes.Stale["a.txt"] {
		t.Fatal("want stale cleared")
	}
}

func TestDiffLoadedClearsStaleForTheOpenFile(t *testing.T) {
	t.Parallel()
	s := State{Open: OpenDiff{Path: "a.txt"}, Changes: Changes{Stale: map[string]bool{"a.txt": true}, Selected: map[string]bool{}}}
	s = Apply(s, DiffLoaded{Diff: git.FileDiff{Path: "a.txt"}})
	if s.Changes.Stale["a.txt"] {
		t.Fatal("reload should clear stale")
	}
}

func TestApplyDoesNotMutateTheStateItWasGiven(t *testing.T) {
	t.Parallel()
	before := State{Rows: rows("a", "b"), Cursor: 0, Changes: Changes{Folded: map[string]bool{}, Selected: map[string]bool{}, Stale: map[string]bool{}}}
	_ = Apply(before, CursorMoved{By: 1})
	if before.Cursor != 0 {
		t.Error("Apply mutated its argument")
	}
	_ = Apply(before, SelectionToggled{Path: "a"})
	if before.Changes.Selected["a"] {
		t.Error("Apply mutated the caller's Selected map")
	}
	_ = Apply(before, StaleChanged{Path: "a", Stale: true})
	if before.Changes.Stale["a"] {
		t.Error("Apply mutated the caller's Stale map")
	}
}

func TestVerbFinishedSetsPullWouldMergeNotice(t *testing.T) {
	t.Parallel()
	s := Apply(State{}, VerbFinished{
		Notice: "pull would merge · resolve in your terminal",
	})
	if s.Notice != "pull would merge · resolve in your terminal" {
		t.Errorf("got %q", s.Notice)
	}
}

func TestHistoryLoadedBuildsRowsOnTheHistoryTab(t *testing.T) {
	t.Parallel()
	s := State{Tab: TabHistory, Changes: Changes{Selected: map[string]bool{}}}
	s = Apply(s, HistoryLoaded{Commits: []git.CommitInfo{
		{SHA: "a", Subject: "one"},
		{SHA: "b", Subject: "two"},
	}})
	if len(s.Rows) != 2 || s.Rows[0].Kind() != RowCommit {
		t.Fatalf("got %+v", s.Rows)
	}
}

func TestHistoryMoreLoadedAppendsTheRequestedPage(t *testing.T) {
	t.Parallel()
	s := State{Tab: TabHistory, Cursor: 1, Changes: Changes{Selected: map[string]bool{}}}
	s = Apply(s, HistoryLoaded{Commits: []git.CommitInfo{
		{SHA: "c", Subject: "third"},
		{SHA: "b", Subject: "second"},
	}, HasMore: true})
	s = Apply(s, HistoryMoreRequested{})
	s = Apply(s, HistoryMoreLoaded{Offset: 2, Commits: []git.CommitInfo{
		{SHA: "a", Subject: "first"},
	}, HasMore: false})
	if got := len(s.History.Commits); got != 3 {
		t.Fatalf("commits = %d, want 3", got)
	}
	if s.History.Commits[2].SHA != "a" {
		t.Errorf("last commit = %q, want a", s.History.Commits[2].SHA)
	}
	if s.Cursor != 1 {
		t.Errorf("cursor = %d, want 1", s.Cursor)
	}
	if s.History.HasMore || s.History.LoadingMore {
		t.Errorf("history after final page = %+v", s.History)
	}
}

func TestHistoryMoreLoadedDropsAnOutOfOrderPage(t *testing.T) {
	t.Parallel()
	s := State{Tab: TabHistory, Changes: Changes{Selected: map[string]bool{}}}
	s = Apply(s, HistoryLoaded{Commits: []git.CommitInfo{{SHA: "b", Subject: "second"}}, HasMore: true})
	s = Apply(s, HistoryMoreRequested{})
	s = Apply(s, HistoryMoreLoaded{Offset: 0, Commits: []git.CommitInfo{{SHA: "a", Subject: "first"}}})
	if got := len(s.History.Commits); got != 1 {
		t.Fatalf("commits = %d, want the existing one", got)
	}
	if s.History.LoadingMore {
		t.Error("an out-of-order page left history loading")
	}
}

func historyWithExpandedC(t *testing.T) State {
	t.Helper()
	s := State{Tab: TabHistory, Changes: Changes{Selected: map[string]bool{}}}
	s = Apply(s, HistoryLoaded{Commits: []git.CommitInfo{
		{SHA: "c", Subject: "third"},
		{SHA: "b", Subject: "second"},
		{SHA: "a", Subject: "first"},
	}})
	s = Apply(s, CommitExpanded{
		SHA: "c",
		Files: []git.Entry{
			{Path: "f1.txt", Worktree: git.Added},
			{Path: "f2.txt", Worktree: git.Added},
		},
	})
	return s
}

func TestExpandingALowerCommitKeepsTheCursorOnIt(t *testing.T) {
	t.Parallel()
	s := historyWithExpandedC(t)
	for i, r := range s.Rows {
		if r.Kind() == RowCommit && r.Path() == "b" {
			s.Cursor = i
			break
		}
	}
	s = Apply(s, CommitExpanded{
		SHA:   "b",
		Files: []git.Entry{{Path: "p.txt", Worktree: git.Added}},
	})
	r := s.Rows[s.Cursor]
	if r.Kind() != RowCommit || r.Path() != "b" {
		t.Errorf("got kind=%v path=%q at cursor %d", r.Kind(), r.Path(), s.Cursor)
	}
}

func TestCollapsingACommitKeepsTheCursorOnAnotherCommit(t *testing.T) {
	t.Parallel()
	s := historyWithExpandedC(t)
	for i, r := range s.Rows {
		if r.Kind() == RowCommit && r.Path() == "b" {
			s.Cursor = i
			break
		}
	}
	s = Apply(s, RowCollapsed{})
	r := s.Rows[s.Cursor]
	if r.Kind() != RowCommit || r.Path() != "b" {
		t.Errorf("got kind=%v path=%q at cursor %d", r.Kind(), r.Path(), s.Cursor)
	}
}

func TestExpandFallsBackToTheRowNumberWhenTheCursorRowIsGone(t *testing.T) {
	t.Parallel()
	s := historyWithExpandedC(t)
	for i, r := range s.Rows {
		if r.Kind() == RowFile && r.Path() == "f2.txt" {
			s.Cursor = i
			break
		}
	}
	s = Apply(s, CommitExpanded{
		SHA:   "b",
		Files: []git.Entry{{Path: "p.txt", Worktree: git.Added}},
	})
	if s.Cursor != 2 {
		t.Errorf("got %d, want the same row index clamped in range", s.Cursor)
	}
}

// A file a commit holds sits on no side of this index. Which commit it came
// from is its Origin, not its section: the section used to say "history" for
// every commit at once, and the read marks keyed on it shared one mark.
func TestCommitExpandedFileRowsSitOnNoSide(t *testing.T) {
	t.Parallel()
	s := State{Tab: TabHistory, Changes: Changes{Selected: map[string]bool{}}}
	s = Apply(s, HistoryLoaded{Commits: []git.CommitInfo{
		{SHA: "a", Subject: "one"},
	}})
	s = Apply(s, CommitExpanded{
		SHA:   "a",
		Files: []git.Entry{{Path: "fourth.txt", Worktree: git.Added}},
	})
	if len(s.Rows) != 2 || s.Rows[1].Kind() != RowFile {
		t.Fatalf("got %+v", s.Rows)
	}
	if s.Rows[1].Section() != SectionNone {
		t.Errorf("section = %v, want SectionNone", s.Rows[1].Section())
	}
	origin, ok := OriginOf(s, 1)
	if !ok {
		t.Fatal("the file belongs to no commit")
	}
	if sha, fromCommit := origin.CommitSHA(); !fromCommit || sha != "a" {
		t.Errorf("origin = %+v, want the file to come from commit a", origin)
	}
}

func TestHistoryLoadedWithNoCommitsLeavesForChanges(t *testing.T) {
	t.Parallel()
	s := Apply(State{Tab: TabHistory, Changes: Changes{Selected: map[string]bool{}}}, HistoryLoaded{})
	if s.Tab != TabChanges {
		t.Errorf("on history with no commits: Tab = %v, want TabChanges", s.Tab)
	}
	s = Apply(State{Tab: TabChanges, Changes: Changes{Selected: map[string]bool{}}}, HistoryLoaded{})
	if s.Tab != TabChanges {
		t.Errorf("on changes with no commits: Tab = %v, want TabChanges", s.Tab)
	}
}

func TestUndoFinishedSetsResetNotice(t *testing.T) {
	t.Parallel()
	s := Apply(State{Changes: Changes{Selected: map[string]bool{}}}, UndoFinished{})
	if s.Notice != "undid commit · reset" {
		t.Errorf("got %q", s.Notice)
	}
}

func TestBlockCursorMovedClampsToTheBlocks(t *testing.T) {
	t.Parallel()
	blocks := make([]git.Block, 3)
	for i := range blocks {
		blocks[i] = git.Block{Hash: "h"}
	}
	s := State{Open: OpenDiff{Diff: git.FileDiff{Blocks: blocks}}, Changes: Changes{Selected: map[string]bool{}}}
	for range 5 {
		s = Apply(s, BlockCursorMoved{By: 1})
	}
	if s.Open.BlockCursor != 2 {
		t.Errorf("after five forward moves got cursor %d, want 2", s.Open.BlockCursor)
	}
	for range 5 {
		s = Apply(s, BlockCursorMoved{By: -1})
	}
	if s.Open.BlockCursor != 0 {
		t.Errorf("after five backward moves got cursor %d, want 0", s.Open.BlockCursor)
	}
	empty := Apply(State{Changes: Changes{Selected: map[string]bool{}}}, BlockCursorMoved{By: 1})
	if empty.Open.BlockCursor != 0 {
		t.Errorf("with no blocks got cursor %d, want 0", empty.Open.BlockCursor)
	}
}

func TestMovingToAnotherBlockClearsItsLineOffset(t *testing.T) {
	t.Parallel()
	blocks := make([]git.Block, 2)
	s := State{Open: OpenDiff{BlockCursor: 0, BlockLine: 4,
		Diff: git.FileDiff{Blocks: blocks}}, Changes: Changes{Selected: map[string]bool{}}}
	s = Apply(s, BlockCursorMoved{By: 1})
	if s.Open.BlockCursor != 1 || s.Open.BlockLine != 0 {
		t.Errorf("after moving got block=%d line=%d, want block=1 line=0", s.Open.BlockCursor, s.Open.BlockLine)
	}
	s.Open.BlockLine = 3
	s = Apply(s, BlockCursorSet{Block: 1})
	if s.Open.BlockLine != 3 {
		t.Errorf("setting the current block cleared line %d", s.Open.BlockLine)
	}
	s = Apply(s, BlockCursorSet{Block: 0})
	if s.Open.BlockLine != 0 {
		t.Errorf("setting another block left line %d", s.Open.BlockLine)
	}
}

func TestStashBranchFinishedSetsNotice(t *testing.T) {
	t.Parallel()
	s := Apply(State{}, StashBranchFinished{Branch: "main-stash"})
	if s.Notice != "branched to main-stash" {
		t.Errorf("got %q", s.Notice)
	}
}

func TestStashedLoadedBuildsRowsOnTheStashedTab(t *testing.T) {
	t.Parallel()
	s := State{Tab: TabStashed, Changes: Changes{Selected: map[string]bool{}}}
	s = Apply(s, StashedLoaded{Stashes: []git.StashRow{
		{Ref: "stash@{0}", SHA: "abc", Message: "one"},
		{Ref: "stash@{1}", SHA: "def", Message: "two"},
	}})
	if len(s.Rows) != 2 || s.Rows[0].Kind() != RowStash {
		t.Fatalf("got %+v", s.Rows)
	}
}

func TestCursorMovedDescendsIntoStashFileRows(t *testing.T) {
	t.Parallel()
	stash := git.StashRow{Ref: "stash@{0}", SHA: "abc", Message: "one"}
	files := []git.Entry{{Path: "a.txt"}, {Path: "b.txt"}}
	s := State{
		Tab: TabStashed, Cursor: 0,
		Changes: Changes{Selected: map[string]bool{}},
		Stashed: Stashed{Stashes: []git.StashRow{stash}, Files: files, Expanded: stash.SHA},
		Rows: StashRowsFor(State{Stashed: Stashed{
			Stashes: []git.StashRow{stash}, Files: files, Expanded: stash.SHA}}),
	}
	if len(s.Rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(s.Rows))
	}
	s = Apply(s, CursorMoved{By: 1})
	if s.Cursor != 1 {
		t.Fatalf("after first j: cursor = %d, want 1", s.Cursor)
	}
	s = Apply(s, CursorMoved{By: 1})
	if s.Cursor != 2 {
		t.Fatalf("after second j: cursor = %d, want 2", s.Cursor)
	}
}

func TestCursorMovedDescendsIntoWorktreeFileRows(t *testing.T) {
	t.Parallel()
	wt := git.WorktreeRow{Path: "/repo/wt", Name: "wt", Branch: "feat", SHA: "abc"}
	files := []git.Entry{{Path: "a.txt"}, {Path: "b.txt"}}
	s := State{
		Tab: TabWorktrees, Cursor: 0,
		Changes:   Changes{Selected: map[string]bool{}},
		Worktrees: Worktrees{List: []git.WorktreeRow{wt}, Files: files, Expanded: wt.Path},
		Rows: WorktreeRowsFor(State{Worktrees: Worktrees{
			List: []git.WorktreeRow{wt}, Files: files, Expanded: wt.Path}}),
	}
	if len(s.Rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(s.Rows))
	}
	s = Apply(s, CursorMoved{By: 1})
	if s.Cursor != 1 {
		t.Fatalf("after first j: cursor = %d, want 1", s.Cursor)
	}
	s = Apply(s, CursorMoved{By: 1})
	if s.Cursor != 2 {
		t.Fatalf("after second j: cursor = %d, want 2", s.Cursor)
	}
}

func TestStashFilesLoadedInsertsFileRows(t *testing.T) {
	t.Parallel()
	stash := git.StashRow{Ref: "stash@{0}", SHA: "abc", Message: "one"}
	s := State{Tab: TabStashed, Cursor: 0, Rows: []Row{StashRowOf(stash)}, Changes: Changes{Selected: map[string]bool{}}, Stashed: Stashed{Stashes: []git.StashRow{stash}}}
	s = Apply(s, StashFilesLoaded{Ref: "stash@{0}", Files: []git.Entry{{Path: "a.txt"}}})
	if len(s.Rows) != 2 || s.Rows[1].Kind() != RowFile || s.Rows[1].Path() != "a.txt" {
		t.Fatalf("got %+v", s.Rows)
	}
	s = Apply(s, SelectionToggled{Path: "a.txt", Section: SectionNone})
	if len(s.Changes.Selected) != 0 {
		t.Errorf("selected = %v, want empty on stashed tab", s.Changes.Selected)
	}
}

func TestWorktreesLoadedRecordsHere(t *testing.T) {
	t.Parallel()
	s := State{Tab: TabWorktrees, Changes: Changes{Selected: map[string]bool{}}}
	s = Apply(s, WorktreesLoaded{
		Worktrees: []git.WorktreeRow{
			{Name: "main", Path: "/repo/main", Branch: "main", SHA: "a"},
			{Name: "wt", Path: "/repo/b", Branch: "feat", SHA: "b"},
		},
		Base: "main",
		Here: "/repo/b",
	})
	if s.Worktrees.Here != "/repo/b" {
		t.Errorf("WorktreeHere = %q, want /repo/b", s.Worktrees.Here)
	}
}

func TestWorktreeGoMovesHere(t *testing.T) {
	t.Parallel()
	s := State{Tab: TabWorktrees, Changes: Changes{Selected: map[string]bool{}}}
	s = Apply(s, WorktreeGo{Here: "/repo/wt"})
	if s.Worktrees.Here != "/repo/wt" {
		t.Errorf("WorktreeHere = %q, want /repo/wt", s.Worktrees.Here)
	}
	if s.Cursor != 0 {
		t.Errorf("cursor = %d, want 0", s.Cursor)
	}
}

func TestWorktreesLoadedBuildsRowsOnTheWorktreesTab(t *testing.T) {
	t.Parallel()
	s := State{Tab: TabWorktrees, Changes: Changes{Selected: map[string]bool{}}}
	s = Apply(s, WorktreesLoaded{Worktrees: []git.WorktreeRow{
		{Name: "main", Path: "/repo/main", Branch: "main", SHA: "a"},
		{Name: "wt", Path: "/repo/wt", Branch: "feat", SHA: "b"},
	}, Base: "main"})
	if len(s.Rows) != 2 || s.Rows[0].Kind() != RowWorktree {
		t.Fatalf("got %+v", s.Rows)
	}
	if s.Worktrees.Base != "main" {
		t.Errorf("base = %q", s.Worktrees.Base)
	}
}

func TestWorktreeGoClearsSelectionAndDiff(t *testing.T) {
	t.Parallel()
	s := State{Tab: TabWorktrees, Cursor: 2, Rows: rows("a", "b", "c"), Open: OpenDiff{Path: "a.txt"}, Changes: Changes{Selected: map[string]bool{"a.txt": true}}}
	s = Apply(s, WorktreeGo{Here: "/repo/wt"})
	if s.Cursor != 0 || s.Open.Path != "" || len(s.Changes.Selected) != 0 {
		t.Errorf("got cursor=%d open=%q selected=%v", s.Cursor, s.Open.Path, s.Changes.Selected)
	}
}

// The notice reports what a reload took away, so a reload that took nothing
// away has nothing to report: "3 of 3 still here" tells the reader something
// happened to a selection that is exactly as they left it, and it pushes off
// whatever the pane was saying before.
func TestAReloadThatKeepsEverySelectedFileSaysNothing(t *testing.T) {
	t.Parallel()
	s := State{Rows: rows("a", "b", "c"), Changes: Changes{Selected: map[string]bool{}}}
	for _, p := range []string{"a", "b", "c"} {
		s = Apply(s, SelectionToggled{Path: p, Section: SectionUnstaged})
	}
	s = Apply(s, StatusLoaded{Rows: entries("a", "b", "c")})
	if strings.Contains(s.Notice, "still here") {
		t.Errorf("nothing was dropped and the pane says %q", s.Notice)
	}
	if len(s.Changes.Selected) != 3 {
		t.Fatalf("got %v", s.Changes.Selected)
	}

	// A reload with nothing selected has nothing to report either.
	empty := State{Rows: rows("a"), Changes: Changes{Selected: map[string]bool{}}}
	empty = Apply(empty, StatusLoaded{Rows: entries("a")})
	if strings.Contains(empty.Notice, "still here") {
		t.Errorf("nothing was selected and the pane says %q", empty.Notice)
	}
}

func TestVanishedSelectionSetsStillHereNotice(t *testing.T) {
	t.Parallel()
	s := State{Rows: rows("a", "b", "c"), Changes: Changes{Selected: map[string]bool{}}}
	for _, p := range []string{"a", "b", "c"} {
		s = Apply(s, SelectionToggled{Path: p, Section: SectionUnstaged})
	}
	s = Apply(s, StatusLoaded{Rows: entries("a", "b")})
	if s.Notice != "2 of 3 still here" {
		t.Errorf("got %q", s.Notice)
	}
	if len(s.Changes.Selected) != 2 {
		t.Fatalf("got %v", s.Changes.Selected)
	}
}

func TestStatusLoadedStoresRemoteURL(t *testing.T) {
	t.Parallel()
	s := Apply(State{}, StatusLoaded{RemoteURL: "git@github.com:owner/repo.git"})
	if s.History.RemoteURL != "git@github.com:owner/repo.git" {
		t.Fatalf("got %q", s.History.RemoteURL)
	}
}

func TestStillHereNoticeClearsOnTheNextVerb(t *testing.T) {
	t.Parallel()
	s := State{Notice: "2 of 3 still here", Changes: Changes{Selected: map[string]bool{"a": true, "b": true}}}
	s = Apply(s, VerbRequested{Verb: VerbStage, Targets: []string{"a"}})
	if s.Notice != "" {
		t.Errorf("got %q", s.Notice)
	}
}

// A verb that fails and says nothing is the worst outcome: the reader presses
// the key, the tree does not move, and the screen offers no reason.
func TestAFailureReachesTheSummaryLine(t *testing.T) {
	t.Parallel()
	s := Apply(State{}, Failed{Line: "git stash pop: exit 129: unknown option"})
	if s.Notice == "" {
		t.Fatal("the failure left nothing on the summary line")
	}
	if strings.Contains(s.Notice, "\n") {
		t.Errorf("notice spans lines: %q", s.Notice)
	}
}

// Restoring the last stash empties the tab the reader is standing on, and the
// bar stops drawing it. Leaving them there is the same dead end a hidden tab
// is, reached by finishing the work rather than by a key.
func TestEmptyingATabMovesTheReaderOff(t *testing.T) {
	t.Parallel()
	s := State{Tab: TabStashed, Stashed: Stashed{Stashes: []git.StashRow{{Ref: "stash@{0}"}}}}
	s = Apply(s, StashedLoaded{})
	if s.Tab != TabChanges {
		t.Errorf("tab = %d, want changes after the last stash went", s.Tab)
	}

	w := State{Tab: TabWorktrees, Worktrees: Worktrees{List: []git.WorktreeRow{{Name: "a"}, {Name: "b"}}}}
	w = Apply(w, WorktreesLoaded{Worktrees: []git.WorktreeRow{{Name: "a"}}})
	if w.Tab != TabChanges {
		t.Errorf("tab = %d, want changes when only the main tree is left", w.Tab)
	}
}

// One path shows in both sections when a file is staged and modified again, or
// when two files under it landed on either side. Folding is a property of the
// row, not of the path, so folding one must leave the other open.
func TestFoldingOneSectionLeavesTheOtherOpen(t *testing.T) {
	t.Parallel()
	s := State{Changes: Changes{Folded: map[string]bool{}}}
	s = Apply(s, DirectoryFolded{Path: "app/Sources", Section: SectionStaged})
	if !FoldedIn(s, SectionStaged, "app/Sources") {
		t.Error("the staged copy did not fold")
	}
	if FoldedIn(s, SectionUnstaged, "app/Sources") {
		t.Error("folding staged also folded the unstaged copy")
	}
}

// A file staged and then edited again shows in both sections. Ticking one row
// must leave the other alone: the two rows are two different pieces of work,
// and the verbs that act on them differ.
func TestSelectingOneSectionLeavesTheOtherUnticked(t *testing.T) {
	t.Parallel()
	s := State{Rows: []Row{
		FileRow(
			git.Entry{Path: "a.go", Index: git.Modified}, SectionStaged),

		FileRow(
			git.Entry{Path: "a.go", Worktree: git.Modified}, SectionUnstaged),
	}, Changes: Changes{Selected: map[string]bool{}}}
	s = Apply(s, SelectionToggled{Path: "a.go", Section: SectionStaged})
	if !SelectedIn(s, SectionStaged, "a.go") {
		t.Error("the staged row did not tick")
	}
	if SelectedIn(s, SectionUnstaged, "a.go") {
		t.Error("ticking the staged row also ticked the unstaged one")
	}
	if got := SelectionCount(s); got != 1 {
		t.Errorf("count = %d, want 1", got)
	}
}

func TestCommitBlockedExplainsWhatToStage(t *testing.T) {
	t.Parallel()
	s := Apply(State{}, CommitBlocked{})
	if s.Notice != "stage a file first" {
		t.Errorf("got Notice %q, want %q", s.Notice, "stage a file first")
	}
}

func TestDiffBlockedExplainsWithoutOpening(t *testing.T) {
	t.Parallel()
	s := Apply(State{}, DiffBlocked{})
	if s.Notice != "no room for a diff · make the pane taller" {
		t.Errorf("got Notice %q, want %q", s.Notice, "no room for a diff · make the pane taller")
	}
	if s.Open.Path != "" {
		t.Errorf("OpenPath = %q, want empty", s.Open.Path)
	}
	if s.Open.Peek {
		t.Error("DiffPeek should stay false")
	}
}

func TestStagedFileCountCountsAPathOnce(t *testing.T) {
	t.Parallel()
	s := State{Rows: []Row{
		FileRow(
			git.Entry{Path: "a.txt", Index: git.Modified}, SectionStaged),

		FileRow(
			git.Entry{Path: "a.txt", Index: git.Modified}, SectionStaged),

		FileRow(
			git.Entry{Path: "b.txt", Worktree: git.Modified}, SectionUnstaged),
	}}
	if got := s.StagedFileCount(); got != 1 {
		t.Errorf("StagedFileCount() = %d, want 1", got)
	}
}

// The count is what the commit box says it will commit, and the list it walks
// holds more than files: the heading over the staged section, and a directory
// row for every folder in it. Counting those said two files were staged where
// one was, and the commit box offered to commit a heading.
func TestStagedFileCountCountsFilesAndNotTheRowsAroundThem(t *testing.T) {
	t.Parallel()
	s := State{}
	s = Apply(s, StatusLoaded{Rows: []git.Entry{
		{Path: "app/a.txt", Index: git.Modified},
		{Path: "app/b.txt", Index: git.Modified},
		{Path: "c.txt", Worktree: git.Modified},
	}})
	var headings, directories int
	for _, row := range s.Rows {
		if row.Kind() == RowSectionHeading {
			headings++
		}
		if row.Kind() == RowDirectory {
			directories++
		}
	}
	if headings == 0 || directories == 0 {
		t.Fatalf("the list holds %d headings and %d directories, so this proves nothing",
			headings, directories)
	}
	if got := s.StagedFileCount(); got != 2 {
		t.Errorf("StagedFileCount() = %d, want 2: %d rows, %d of them headings and "+
			"%d directories", got, len(s.Rows), headings, directories)
	}
}

func TestDiscardCancelledLeavesACanceledNotice(t *testing.T) {
	t.Parallel()
	s := State{Changes: Changes{DiscardConfirm: &DiscardConfirm{Files: 1}}}
	s = Apply(s, DiscardCancelled{})
	if s.Changes.DiscardConfirm != nil {
		t.Fatal("confirmation should be cleared")
	}
	if s.Notice != "canceled · nothing discarded" {
		t.Errorf("got %q", s.Notice)
	}
}

func TestDiscardCancelledKeepsAnExplicitNotice(t *testing.T) {
	t.Parallel()
	s := State{Changes: Changes{DiscardConfirm: &DiscardConfirm{Files: 1}}}
	want := "nothing to discard · the changes are gone"
	s = Apply(s, DiscardCancelled{Notice: want})
	if s.Notice != want {
		t.Errorf("got %q, want %q", s.Notice, want)
	}
}

func TestStashDropCancelledLeavesACanceledNotice(t *testing.T) {
	t.Parallel()
	s := State{Stashed: Stashed{DropConfirm: &StashDropConfirm{SHA: "abc", Message: "wip"}}}
	s = Apply(s, StashDropCancelled{})
	if s.Stashed.DropConfirm != nil {
		t.Fatal("confirmation should be cleared")
	}
	if s.Notice != "canceled · stash kept" {
		t.Errorf("got %q", s.Notice)
	}
}

func TestStateDropsFlatPayloadFields(t *testing.T) {
	t.Parallel()
	dropped := []string{
		"OpenPath", "OpenSection", "OpenCommit", "Diff", "BlockCursor",
		"DiffScroll", "DiffClosed", "DiffClosedFor", "DiffPeek",
		"Entries", "Folded", "Selected", "Stale", "MessageFocused", "Message",
		"Pending", "DiscardConfirm", "UndiscardConfirm",
		"Commits", "ExpandedSHA", "ExpandedFiles", "RemoteURL",
		"Stashes", "StashFiles", "StashDropConfirm",
		"WorktreeBase", "WorktreeHere", "WorktreeFiles",
	}
	typ := reflect.TypeOf(State{})
	for _, name := range dropped {
		if _, ok := typ.FieldByName(name); ok {
			t.Errorf("State still has flat field %q", name)
		}
	}
}

func TestChangingTabZerosOpenDiff(t *testing.T) {
	t.Parallel()
	s := State{
		Rows: rows("a"), Tab: TabChanges, Open: OpenDiff{
			Path: "a", Origin: WorkingTree(SectionUnstaged), Peek: true,
			Diff: git.FileDiff{Blocks: []git.Block{{}}},
		},
	}
	s = Apply(s, TabChanged{Tab: TabHistory})
	if s.Open.Path != "" || s.Open.Peek || len(s.Open.Diff.Blocks) != 0 {
		t.Errorf("Open = %+v, want zero", s.Open)
	}
}

func TestStashedLoadedRestoresCursorByKey(t *testing.T) {
	t.Parallel()
	one := git.StashRow{Ref: "stash@{0}", SHA: "abc", Message: "one"}
	two := git.StashRow{Ref: "stash@{1}", SHA: "def", Message: "two"}
	s := State{
		Tab: TabStashed, Cursor: 1,
		Stashed: Stashed{Stashes: []git.StashRow{one, two},
			Files: []git.Entry{{Path: "a.txt"}}, Expanded: two.SHA},
	}
	s.Rows = StashRowsFor(s)
	// 2つ目の stash の下に開いたファイル行にカーソルを置く
	s.Cursor = 2
	// 新しい stash が先頭に積まれても、展開する位置はカーソルの stash 番号で
	// 決まるので、ファイル行は同じ行番号に来る。stash タブでは restoredCursor と
	// clamp の答えが一致し、この形では両者を見分けられない。見分けるのは
	// TestWorktreesLoadedRestoresCursorByKey で、そちらは行番号がずれる。
	s = Apply(s, StashedLoaded{Stashes: []git.StashRow{
		{Ref: "stash@{0}", SHA: "ghi", Message: "three"},
		{Ref: "stash@{1}", SHA: "abc", Message: "one"},
		{Ref: "stash@{2}", SHA: "def", Message: "two"},
	}})
	for i, r := range s.Rows {
		if r.Kind() == RowFile && r.Path() == "a.txt" {
			if s.Cursor != i {
				t.Errorf("cursor = %d, want %d restored by key", s.Cursor, i)
			}
			return
		}
	}
	t.Fatal("file row missing after reload")
}

func TestWorktreesLoadedRestoresCursorByKey(t *testing.T) {
	t.Parallel()
	wt := git.WorktreeRow{Path: "/repo/wt", Name: "wt", Branch: "feat", SHA: "abc"}
	main := git.WorktreeRow{Path: "/repo/main", Name: "main", Branch: "main", SHA: "aaa"}
	files := []git.Entry{{Path: "a.txt"}}
	trees := Worktrees{List: []git.WorktreeRow{main, wt}, Base: "main", Here: "/repo/main",
		Files: files, Expanded: wt.Path}
	s := State{Tab: TabWorktrees, Worktrees: trees, Cursor: 1}
	s.Rows = WorktreeRowsFor(s)
	s.Cursor = 2
	// 展開中の tree より前に1本増やす。末尾に足すと行番号が変わらず、
	// clamp だけの実装でも通ってしまう。
	s = Apply(s, WorktreesLoaded{
		Worktrees: []git.WorktreeRow{main, {Path: "/repo/extra", Name: "extra"}, wt},
		Base:      "main",
		Here:      "/repo/main",
	})
	for i, r := range s.Rows {
		if r.Kind() == RowFile && r.Path() == "a.txt" {
			if s.Cursor != i {
				t.Errorf("cursor = %d, want %d restored by key", s.Cursor, i)
			}
			return
		}
	}
	t.Fatal("file row missing after reload")
}
