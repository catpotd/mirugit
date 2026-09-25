package layout

import (
	"strings"
	"testing"
	"time"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

func TestCommitBoxIsExactlyPaneWidth(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	line, reg := w.CommitBox("", false, paneWidth)
	if got := w.Of(line); got != paneWidth {
		t.Errorf("got %d cells, want %d: %q", got, paneWidth, line)
	}
	if reg.ColStart != 0 || reg.ColEnd != paneWidth {
		t.Errorf("region = %+v, want [0, %d)", reg, paneWidth)
	}
	if reg.Target.Kind != TargetCommitBox {
		t.Errorf("got kind %v, want TargetCommitBox", reg.Target.Kind)
	}
	if !strings.HasPrefix(line, "[ ") || !strings.HasSuffix(line, " ]") {
		t.Errorf("got %q, want brackets around the field", line)
	}
	if !strings.Contains(line, "commit message") {
		t.Errorf("empty box should show the placeholder: %q", line)
	}
}

func TestCommitBoxSurvivesEveryPaneWidth(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	for _, focused := range []bool{false, true} {
		for width := 1; width <= 80; width++ {
			line, _ := w.CommitBox("fix the layout", focused, width)
			if width >= 40 && w.Of(line) != width {
				t.Fatalf("focused=%v width %d: %q is %d cells, want %d",
					focused, width, line, w.Of(line), width)
			}
		}
	}
}

func TestSummaryLineWithSelectionIsExactlyPaneWidth(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	sum := summary{
		Selected:        3,
		SelectedStaged:  1,
		SelectedAdded:   212,
		SelectedDeleted: 1,
		Staged:          1,
	}
	line, regions := w.SummaryLine(sum, paneWidth)
	if got := w.Of(line); got != paneWidth {
		t.Errorf("got %d cells, want %d: %q", got, paneWidth, line)
	}
	var commits int
	for _, r := range regions {
		if r.Target.Kind == TargetCommit {
			commits++
		}
	}
	if commits != 1 {
		t.Fatalf("regions = %+v, want one TargetCommit", regions)
	}
}

func TestSummaryLineSelectedCountsStayPlain(t *testing.T) {
	t.Parallel()
	w := Renderer{Palette: Palette{Enabled: true}}
	sum := summary{
		Selected:        3,
		SelectedStaged:  1,
		SelectedAdded:   2,
		SelectedDeleted: 1,
		Staged:          1,
	}
	line, _ := w.SummaryLine(sum, paneWidth)
	if strings.Contains(line, w.Verb("3 selected")) {
		t.Errorf("selected count should stay plain: %q", line)
	}
	if strings.Contains(line, w.Verb("1 staged")) {
		t.Errorf("staged count should stay plain: %q", line)
	}
	if !strings.Contains(line, "3 selected") {
		t.Errorf("selected count missing: %q", line)
	}
	if !strings.Contains(line, "1 staged") {
		t.Errorf("staged count missing: %q", line)
	}
}

func TestSummaryLineVerbColorReachesTheBarNotTheCounts(t *testing.T) {
	t.Parallel()
	w := Renderer{Palette: Palette{Enabled: true}}
	sum := summary{
		Selected:          3,
		SelectedStaged:    1,
		SelectedAdded:     2,
		SelectedDeleted:   1,
		Staged:            1,
		SelectionBarVerbs: []state.VerbName{state.VerbNameStage, state.VerbNameStash, state.VerbNameDiscard},
	}
	line, _ := w.SummaryLine(sum, paneWidth)
	if !strings.Contains(line, w.Verb("stage")) {
		t.Errorf("bar verb should keep verb color: %q", line)
	}
	if !strings.Contains(line, "1 staged") {
		t.Errorf("staged count should stay intact: %q", line)
	}
}

func TestSummaryLineWithSelectionOmitsStagedWhenZero(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	sum := summary{
		Selected:        2,
		SelectedAdded:   10,
		SelectedDeleted: 1,
	}
	line, _ := w.SummaryLine(sum, paneWidth)
	if strings.Contains(line, "staged") {
		t.Errorf("no staged among selection: %q", line)
	}
}

func TestSummaryLineShowsDiscardConfirmation(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	sum := summary{
		DiscardConfirm: &state.DiscardConfirm{
			Files: 4, Added: 219, Deleted: 2,
		},
	}
	line, _ := w.SummaryLine(sum, paneWidth)
	if !strings.Contains(line, "Discard 4 files") {
		t.Errorf("got %q", line)
	}
	if !strings.Contains(line, "+219") || !strings.Contains(line, "−2") {
		t.Errorf("got %q", line)
	}
}

func TestSummaryLineIsExactlyPaneWidth(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	sum := summary{
		Unread: 3, Files: 5, Added: 266, Deleted: 4,
		Behind: 1, Ahead: 2, Staged: 1,
	}
	line, regions := w.SummaryLine(sum, paneWidth)
	if got := w.Of(line); got != paneWidth {
		t.Errorf("got %d cells, want %d: %q", got, paneWidth, line)
	}
	var commits int
	for _, r := range regions {
		if r.Target.Kind == TargetCommit {
			commits++
		}
	}
	if commits != 1 {
		t.Fatalf("regions = %+v, want one TargetCommit", regions)
	}
}

func TestSummaryLineOmitsUnreadWhenZero(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	line, _ := w.SummaryLine(summary{Files: 2, Added: 1, Deleted: 0}, paneWidth)
	if strings.Contains(line, "unread") {
		t.Errorf("unread count is zero: %q", line)
	}
}

func TestSummaryLineOmitsCommitWhenNothingStaged(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	line, regions := w.SummaryLine(summary{Files: 2, Added: 1, Deleted: 0}, paneWidth)
	if strings.Contains(line, "[commit") {
		t.Errorf("nothing staged: %q", line)
	}
	if len(regions) != 0 {
		t.Errorf("regions = %+v, want none", regions)
	}
}

func TestSummaryOfCountsFromState(t *testing.T) {
	t.Parallel()
	s := state.State{Head: git.Head{Behind: 1, Ahead: 2}, Rows: []state.Row{
		state.FileRow(git.Entry{
			Path: "a", Index: git.Modified,
			IndexCount: git.Count{Added: 3},
		}, state.SectionStaged),

		state.FileRow(git.Entry{
			Path: "b", Worktree: git.Modified,
			WorktreeCount: git.Count{Added: 2, Deleted: 1},
		}, state.SectionUnstaged),
	}}
	sum := summaryOf(s, nil)
	if sum.Unread != 0 || sum.Files != 2 || sum.Added != 5 || sum.Deleted != 1 {
		t.Errorf("got %+v", sum)
	}
	if sum.Staged != 1 || sum.Behind != 1 || sum.Ahead != 2 {
		t.Errorf("got %+v", sum)
	}
	if !sum.NoUpstream {
		t.Errorf("want NoUpstream without HasUpstream, got %+v", sum)
	}
}

func TestSummaryOfSetsNoUpstreamFromHead(t *testing.T) {
	t.Parallel()
	sum := summaryOf(state.State{Head: git.Head{Behind: 2, Ahead: 1, HasUpstream: true}}, nil)
	if sum.NoUpstream {
		t.Errorf("got %+v", sum)
	}
	sum = summaryOf(state.State{Head: git.Head{Behind: 2, Ahead: 1}}, nil)
	if !sum.NoUpstream {
		t.Errorf("got %+v", sum)
	}
}

func TestStashedTabBarStillShowsChangesCount(t *testing.T) {
	t.Parallel()
	s := state.State{
		Tab: state.TabStashed,
		Changes: state.Changes{Entries: []git.Entry{
			{Path: "a.txt", Worktree: git.Modified},
			{Path: "b.txt", Worktree: git.Modified},
		}},
		Stashed: state.Stashed{Stashes: []git.StashRow{{Ref: "stash@{0}"}}},
	}
	tabs := TabsOf(s, nil)
	for _, tb := range tabs {
		if tb.Name == "changes" {
			if tb.Count != 2 {
				t.Errorf("changes count = %d, want 2 from Entries while on stashed tab", tb.Count)
			}
			return
		}
	}
	t.Fatal("changes tab not found")
}

func TestChangesTabCountsARowPerSectionLikeTheSummaryLine(t *testing.T) {
	t.Parallel()
	s := state.State{Tab: state.TabHistory, Changes: state.Changes{Entries: []git.Entry{
		{Path: "i.txt", Index: git.Modified, Worktree: git.Modified},
		{Path: "b.txt", Index: git.Untracked, Worktree: git.Untracked},
	}}}
	tabs := TabsOf(s, nil)
	for _, tb := range tabs {
		if tb.Name == "changes" {
			if tb.Count != 3 {
				t.Fatalf("changes count = %d, want 3 (row count, not %d paths)", tb.Count, len(s.Changes.Entries))
			}
			return
		}
	}
	t.Fatal("changes tab not found")
}

func TestTabsWithZeroCountAreOmitted(t *testing.T) {
	t.Parallel()
	tabs := TabsOf(state.State{Rows: []state.Row{
		state.FileRow(git.Entry{Path: "a"}, state.SectionUnstaged),
	}}, nil)
	if len(tabs) != 1 || tabs[0].Name != "changes" {
		t.Errorf("got %+v, want only changes", tabs)
	}
}

func TestAllFourTabsAppearWhenStashAndWorktreeCountsAreNonzero(t *testing.T) {
	t.Parallel()
	s := state.State{History: state.History{Commits: []git.CommitInfo{{Subject: "a"}}}, Stashed: state.Stashed{Stashes: []git.StashRow{{Ref: "stash@{0}"}, {Ref: "stash@{1}"}}}, Worktrees: state.Worktrees{List: []git.WorktreeRow{{Path: "/a"}, {Path: "/b"}}}}
	tabs := TabsOf(s, nil)
	if len(tabs) != 4 {
		t.Fatalf("got %d tabs, want 4: %+v", len(tabs), tabs)
	}
}

func TestTabsOfKeepsFixedTabNumbers(t *testing.T) {
	t.Parallel()
	s := state.State{History: state.History{Commits: []git.CommitInfo{{Subject: "a"}}}, Worktrees: state.Worktrees{List: []git.WorktreeRow{{Path: "/main"}, {Path: "/wt"}}}}
	tabs := TabsOf(s, nil)
	if len(tabs) != 3 {
		t.Fatalf("got %d tabs, want 3: %+v", len(tabs), tabs)
	}
	want := []int{1, 2, 4}
	for i, n := range want {
		if tabs[i].Number != n {
			t.Errorf("tabs[%d].Number = %d, want %d", i, tabs[i].Number, n)
		}
	}
}

func TestOnlyChangesAndHistoryWhenStashIsEmptyAndOneWorktree(t *testing.T) {
	t.Parallel()
	s := state.State{History: state.History{Commits: []git.CommitInfo{{Subject: "a"}}}, Worktrees: state.Worktrees{List: []git.WorktreeRow{{Path: "/main"}}}}
	tabs := TabsOf(s, nil)
	if len(tabs) != 2 {
		t.Fatalf("got %d tabs, want 2: %+v", len(tabs), tabs)
	}
	if tabs[0].Name != "changes" || tabs[1].Name != "history" {
		t.Errorf("got %+v, want changes and history", tabs)
	}
}

func TestChangesTabStaysWhenTheRepositoryHasNoChanges(t *testing.T) {
	t.Parallel()
	tabs := TabsOf(state.State{}, nil)
	if len(tabs) != 1 || tabs[0].Name != "changes" || tabs[0].Count != 0 {
		t.Errorf("got %+v, want changes at zero", tabs)
	}
}

func TestTabBarIsExactly77CellsWide(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	tabs := TabsOf(state.State{History: state.History{Commits: []git.CommitInfo{{}, {}, {}}}, Stashed: state.Stashed{Stashes: []git.StashRow{{}}}, Worktrees: state.Worktrees{List: []git.WorktreeRow{{}}}}, nil)
	bar, rule, _ := w.TabBar(tabs, 0, "", paneWidth)
	if got := w.Of(bar); got != paneWidth {
		t.Errorf("bar = %d cells, want %d: %q", got, paneWidth, bar)
	}
	if got := w.Of(rule); got != paneWidth {
		t.Errorf("rule = %d cells, want %d", got, paneWidth)
	}
}

func TestUnreadTabsEndTheirLabelWithAMark(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	tabs := []Tab{
		{Name: "changes", Count: 1, Unread: true},
		{Name: "stashed", Count: 2, Unread: true},
	}
	bar, _, _ := w.TabBar(tabs, 0, "", paneWidth)
	if !strings.Contains(bar, "changes 1 ·") || !strings.Contains(bar, "stashed 2 ·") {
		t.Errorf("unread tabs should end with ·: %q", bar)
	}
}

func TestReadTabsOmitTheUnreadMark(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	bar, _, _ := w.TabBar([]Tab{{Name: "history", Count: 3}}, 0, "", paneWidth)
	if strings.Contains(bar, " ·") {
		t.Errorf("read tab should not carry ·: %q", bar)
	}
}

func TestHistoryTabHasNoUnreadMark(t *testing.T) {
	t.Parallel()
	s := state.State{History: state.History{Commits: []git.CommitInfo{{Subject: "a"}, {Subject: "b"}}}}
	tabs := TabsOf(s, nil)
	for _, tb := range tabs {
		if tb.Name != "history" {
			continue
		}
		if tb.Unread {
			t.Fatal("history tab should not be unread")
		}
		w := Renderer{}
		bar, _, _ := w.TabBar(tabs, activeTab(state.TabHistory, tabs), "", paneWidth)
		if strings.Contains(bar, "history 2 ·") {
			t.Errorf("history tab bar should not show unread mark: %q", bar)
		}
		return
	}
	t.Fatal("history tab not found")
}

func TestFormatFetchedAgo(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		at   time.Time
		want string
	}{
		{"never", time.Time{}, "never fetched"},
		{"4m", now.Add(-4 * time.Minute), "fetched 4m ago"},
		{"2h", now.Add(-2 * time.Hour), "fetched 2h ago"},
		{"3d", now.Add(-3 * 24 * time.Hour), "fetched 3d ago"},
	}
	for _, c := range cases {
		if got := FormatFetchedAgo(c.at, now); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestTabBarWithFetchedOnTheRightIsExactly77Cells(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	tabs := TabsOf(state.State{History: state.History{Commits: []git.CommitInfo{{}}}}, nil)
	bar, rule, _ := w.TabBar(tabs, 0, "fetched 4m ago", paneWidth)
	if got := w.Of(bar); got != paneWidth {
		t.Errorf("bar = %d cells, want %d: %q", got, paneWidth, bar)
	}
	if got := w.Of(rule); got != paneWidth {
		t.Errorf("rule = %d cells, want %d", got, paneWidth)
	}
	if !strings.Contains(bar, "fetched 4m ago") {
		t.Errorf("fetched label missing: %q", bar)
	}
}

func TestTabBarWithFourTabsAndFetchedDoesNotOverflow(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	tabs := []Tab{
		{Name: "changes", Count: 5, Unread: true},
		{Name: "history", Count: 3},
		{Name: "stashed", Count: 2},
		{Name: "worktrees", Count: 2, Unread: true},
	}
	bar, _, _ := w.TabBar(tabs, 0, "fetched 4m ago", paneWidth)
	if got := w.Of(bar); got != paneWidth {
		t.Errorf("got %d cells, want %d: %q", got, paneWidth, bar)
	}
}

func TestPaneTabBarShowsFetchedAt77Cells(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := state.State{Width: paneWidth, Height: 24, Fetched: "never fetched", History: state.History{Commits: []git.CommitInfo{{}, {}, {}}}, Stashed: state.Stashed{Stashes: []git.StashRow{{}, {}}}, Worktrees: state.Worktrees{List: []git.WorktreeRow{{Path: "/a"}, {Path: "/b"}}}}
	f := Pane(s, nil, w)
	if got := w.Of(f.Lines[0]); got != paneWidth {
		t.Errorf("tab bar = %d cells, want %d: %q", got, paneWidth, f.Lines[0])
	}
	if !strings.Contains(f.Lines[0], "never fetched") {
		t.Errorf("fetched label missing: %q", f.Lines[0])
	}
}

func TestPaneTabBarShowsBranchBeforeFetched(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := state.State{Width: paneWidth, Height: 24, Fetched: "never fetched", Head: git.Head{Branch: "main"}, History: state.History{Commits: []git.CommitInfo{{}, {}, {}}}, Stashed: state.Stashed{Stashes: []git.StashRow{{}, {}}}, Worktrees: state.Worktrees{List: []git.WorktreeRow{{Path: "/a"}, {Path: "/b"}}}}
	f := Pane(s, nil, w)
	if got := w.Of(f.Lines[0]); got != paneWidth {
		t.Errorf("tab bar = %d cells, want %d: %q", got, paneWidth, f.Lines[0])
	}
	if !strings.Contains(f.Lines[0], "main · never fetched") {
		t.Errorf("branch and fetched label missing: %q", f.Lines[0])
	}
}

// The action bar is where a verb reaches the selection. Without a region the
// words are drawn and cannot be pressed, which is the contract the footer
// states about every verb.
func TestSummaryBarShowsUnstageForStagedOnlySelection(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := state.State{Width: paneWidth, Rows: []state.Row{
		state.SectionHeadingRow(state.SectionStaged),
		state.FileRow(git.Entry{Path: "c.go", Index: git.Modified}, state.SectionStaged),
	}}
	s = state.Apply(s, state.SelectionToggled{Path: "c.go", Section: state.SectionStaged})
	sum := summaryOf(s, nil)
	line, regions := w.SummaryLine(sum, paneWidth)
	if strings.Contains(line, "s stage") {
		t.Fatalf("summary bar shows stage for staged-only selection: %q", line)
	}
	if !strings.Contains(line, "u unstage") {
		t.Fatalf("summary bar missing unstage: %q", line)
	}
	seen := false
	for _, r := range regions {
		if r.Target.Kind == TargetVerb && r.Target.Verb == state.VerbNameUnstage {
			seen = true
		}
		if r.Target.Kind == TargetVerb && r.Target.Verb == state.VerbNameStage {
			t.Fatal("stage is a verb target for staged-only selection")
		}
	}
	if !seen {
		t.Fatal("unstage is not a verb target on the summary bar")
	}
}

func TestTheActionBarOffersItsVerbsAsTargets(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	line, regions := w.SummaryLine(summary{
		Selected: 3, SelectedStaged: 1, SelectedAdded: 212, SelectedDeleted: 1,
		Staged: 1, Files: 5,
		SelectionBarVerbs: []state.VerbName{state.VerbNameStage, state.VerbNameStash, state.VerbNameDiscard},
	}, paneWidth)

	if w.Of(line) != paneWidth {
		t.Fatalf("action bar is %d cells, want %d: %q", w.Of(line), paneWidth, line)
	}

	seen := map[state.VerbName]bool{}
	for _, r := range regions {
		if r.Target.Kind == TargetVerb {
			seen[r.Target.Verb] = true
			if !strings.Contains(cellSlice(line, r.ColStart, r.ColEnd), r.Target.Verb.String()) {
				t.Errorf("the region for %q does not sit on the word", r.Target.Verb)
			}
		}
	}
	// The bar shows whatever fits, dropping from the middle, so the first verb
	// and the way out stay while wider keyed tokens squeeze the middle.
	for _, v := range []state.VerbName{state.VerbNameStage, state.VerbNameClear} {
		if !seen[v] {
			t.Errorf("%q is not a target on the action bar: %q", v, line)
		}
	}
	if len(seen) == 0 {
		t.Errorf("the action bar offered no verb at all: %q", line)
	}
}

func TestActionBarVerbsCarryTheirKeys(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	line, regions := w.SummaryLine(summary{
		Selected:          2,
		Files:             3,
		SelectionBarVerbs: []state.VerbName{state.VerbNameStage, state.VerbNameStash, state.VerbNameDiscard},
	}, paneWidth)
	if !strings.Contains(line, "s stage") {
		t.Fatalf("line = %q, want keyed stage", line)
	}
	if !strings.Contains(line, "esc clear") {
		t.Fatalf("line = %q, want keyed clear", line)
	}
	for _, r := range regions {
		if r.Target.Verb != state.VerbNameClear {
			continue
		}
		if got := cellSlice(line, r.ColStart, r.ColEnd); got != "esc clear" {
			t.Fatalf("clear region = %q, want %q", got, "esc clear")
		}
		return
	}
	t.Fatal("no region for clear")
}

// The confirmation is asked before anything runs, so promising a way back from
// a discard that has not happened points U at the previous one.
func TestTheConfirmationDoesNotPromiseUndo(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	line, _ := w.SummaryLine(summary{
		DiscardConfirm: &state.DiscardConfirm{Files: 3, Added: 142, Deleted: 4},
	}, paneWidth)
	if strings.Contains(line, "U undo") {
		t.Errorf("the confirmation offers undo before the discard ran: %q", line)
	}
	if !strings.Contains(line, "Discard 3 files") {
		t.Errorf("the confirmation lost its count: %q", line)
	}
}

func TestSummaryLineSyncButtonLabels(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	cases := []struct {
		behind, ahead int
		want          string
	}{
		{1, 2, "[sync 1↓ 2↑]"},
		{0, 2, "[push 2↑]"},
		{1, 0, "[pull 1↓]"},
		{0, 0, ""},
	}
	for _, c := range cases {
		line, regions := w.SummaryLine(summary{Behind: c.behind, Ahead: c.ahead, Files: 1, Added: 1}, paneWidth)
		if c.want == "" {
			if strings.Contains(line, "[sync") || strings.Contains(line, "[pull") || strings.Contains(line, "[push") {
				t.Errorf("behind=%d ahead=%d: got %q, want no sync button", c.behind, c.ahead, line)
			}
			continue
		}
		if !strings.Contains(line, c.want) {
			t.Errorf("behind=%d ahead=%d: got %q, want %q", c.behind, c.ahead, line, c.want)
		}
		if w.Of(line) != paneWidth {
			t.Errorf("behind=%d ahead=%d: got %d cells: %q", c.behind, c.ahead, w.Of(line), line)
		}
		var sync int
		for _, r := range regions {
			if r.Target.Kind == TargetButton && r.Target.Verb == state.VerbNameSync {
				sync++
				if cellSlice(line, r.ColStart, r.ColEnd) != c.want {
					t.Errorf("region does not cover %q", c.want)
				}
			}
		}
		if sync != 1 {
			t.Errorf("behind=%d ahead=%d: want one sync region, got %d", c.behind, c.ahead, sync)
		}
	}
}

func TestSummaryLineOmitsSyncWithoutUpstream(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	line, regions := w.SummaryLine(summary{
		Behind:     2,
		Ahead:      1,
		NoUpstream: true,
		Files:      1,
		Added:      1,
	}, paneWidth)
	if strings.Contains(line, "[sync") || strings.Contains(line, "[pull") || strings.Contains(line, "[push") {
		t.Errorf("no upstream: %q", line)
	}
	for _, r := range regions {
		if r.Target.Kind == TargetButton {
			t.Errorf("no upstream but sync region exists: %+v", r)
		}
	}
}

func TestSummaryLineShowsStillHereNotice(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	line, _ := w.SummaryLine(summary{
		Notice:          "2 of 3 still here",
		Selected:        2,
		SelectedAdded:   10,
		SelectedDeleted: 1,
		Staged:          1,
		Files:           2,
	}, paneWidth)
	if strings.Contains(line, "2 of 3 still here") {
		t.Errorf("notice should not replace the summary left: %q", line)
	}
	if !strings.Contains(line, "2 selected") {
		t.Errorf("got %q", line)
	}
	if got := w.Of(line); got != paneWidth {
		t.Errorf("got %d cells, want %d: %q", got, paneWidth, line)
	}
}

func TestSummaryLineIsExactly77CellsInAllModes(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	cases := []summary{
		{Unread: 3, Files: 5, Added: 266, Deleted: 4, Behind: 1, Ahead: 2, Staged: 1},
		{Files: 2, Added: 1, Deleted: 0, Behind: 3, Ahead: 0},
		{Files: 2, Added: 1, Deleted: 0, Ahead: 4},
		{Selected: 3, SelectedStaged: 1, SelectedAdded: 212, SelectedDeleted: 1, Staged: 1, Behind: 1, Ahead: 1},
		{Notice: "pull would merge · resolve in your terminal", Files: 2, Added: 1},
		{Selected: 2, SelectedAdded: 10, SelectedDeleted: 1, Staged: 1, Files: 2},
		{DiscardConfirm: &state.DiscardConfirm{Files: 4, Added: 219, Deleted: 2}},
	}
	for i, sum := range cases {
		line, _ := w.SummaryLine(sum, paneWidth)
		if got := w.Of(line); got != paneWidth {
			t.Errorf("[%d] got %d cells, want %d: %q", i, got, paneWidth, line)
		}
	}
}

// A tab that hides its own count disappears under the zero-count rule, so the
// reader lands on a tab the bar no longer shows.
func TestTheHistoryTabCarriesItsCount(t *testing.T) {
	t.Parallel()
	tabs := TabsOf(state.State{Tab: state.TabHistory, History: state.History{Commits: []git.CommitInfo{{Subject: "a"}, {Subject: "b"}}}}, nil)
	for _, tb := range tabs {
		if tb.Name == "history" {
			if tb.Count != 2 {
				t.Errorf("history count = %d, want 2", tb.Count)
			}
			return
		}
	}
	t.Error("the history tab is not drawn even though there are commits")
}

func TestSummaryCountsFilesNotHeadings(t *testing.T) {
	t.Parallel()
	s := state.State{Rows: []state.Row{
		state.SectionHeadingRow(state.SectionUnstaged),
		state.DirectoryRow("", state.SectionUnstaged),
		state.FileRow(
			git.Entry{Path: "a.go", Worktree: git.Modified}, state.SectionUnstaged),

		state.FileRow(
			git.Entry{Path: "b.go", Worktree: git.Modified}, state.SectionUnstaged),
	}}
	sum := summaryOf(s, nil)
	if sum.Files != 2 {
		t.Errorf("Files = %d, want 2", sum.Files)
	}
}

func TestSummaryCountsStagedFilesNotHeadings(t *testing.T) {
	t.Parallel()
	s := state.State{Rows: []state.Row{
		state.SectionHeadingRow(state.SectionStaged),
		state.DirectoryRow("", state.SectionStaged),
		state.FileRow(
			git.Entry{Path: "a.go", Index: git.Modified}, state.SectionStaged),
	}}
	sum := summaryOf(s, nil)
	if sum.Staged != 1 {
		t.Errorf("Staged = %d, want 1", sum.Staged)
	}
}

// Typing goes into this box only while it holds the focus, and nothing on the
// line said which state it was in. The caret is a character rather than a
// color, so it survives NO_COLOR and a sixteen-color terminal.
func TestCommitBoxShowsWhereTypingLands(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	idle, _ := w.CommitBox("", false, paneWidth)
	typing, _ := w.CommitBox("", true, paneWidth)
	if idle == typing {
		t.Fatal("the box looks the same focused and not")
	}
	if !strings.Contains(idle, "commit message") {
		t.Errorf("idle box lost its prompt: %q", idle)
	}
	if strings.Contains(typing, "commit message") {
		t.Errorf("focused box still shows the prompt: %q", typing)
	}
	for _, line := range []string{idle, typing} {
		if got := w.Of(line); got != paneWidth {
			t.Errorf("%q is %d cells, want %d", line, got, paneWidth)
		}
	}
	withText, _ := w.CommitBox("feat: x", true, paneWidth)
	if !strings.Contains(withText, "feat: x") {
		t.Errorf("typed text missing: %q", withText)
	}
}

func TestCommitBoxKeepsTheTailWhileTyping(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	msg := "HEAD" + strings.Repeat("-", 80) + "TAIL"
	line, _ := w.CommitBox(msg, true, paneWidth)
	if !strings.Contains(line, "TAIL") {
		t.Errorf("focused box should show the tail: %q", line)
	}
	if strings.Contains(line, "HEAD") {
		t.Errorf("focused box should hide the head: %q", line)
	}
	if !strings.Contains(line, "…") {
		t.Errorf("focused box should truncate with ellipsis: %q", line)
	}
	if got := w.Of(line); got != paneWidth {
		t.Errorf("%q is %d cells, want %d", line, got, paneWidth)
	}

	msgJP := strings.Repeat("あ", 40) + "たち"
	lineJP, _ := w.CommitBox(msgJP, true, paneWidth)
	if !strings.Contains(lineJP, "たち") {
		t.Errorf("focused box should show the tail in Japanese: %q", lineJP)
	}
	if got := w.Of(lineJP); got != paneWidth {
		t.Errorf("%q is %d cells, want %d", lineJP, got, paneWidth)
	}

	lineIdle, _ := w.CommitBox(msg, false, paneWidth)
	if !strings.Contains(lineIdle, "HEAD") {
		t.Errorf("idle box should show the head: %q", lineIdle)
	}
	if strings.Contains(lineIdle, "TAIL") {
		t.Errorf("idle box should hide the tail: %q", lineIdle)
	}
}

// clear is the only way out of a selection for a reader using the mouse, so a
// narrow bar has to drop the middle rather than the exit.
func TestTheActionBarKeepsItsWayOut(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	sum := summary{
		Selected: 3, Added: 12, Deleted: 1, Behind: 1, Ahead: 5, Staged: 2,
		SelectionBarVerbs: []state.VerbName{state.VerbNameStage, state.VerbNameStash, state.VerbNameDiscard},
	}
	line, _ := w.SummaryLine(sum, paneWidth)
	if !strings.Contains(line, "clear") {
		t.Errorf("no way out of the selection: %q", line)
	}
	if !strings.Contains(line, "stage") {
		t.Errorf("the first verb went before the last: %q", line)
	}
	if got := w.Of(line); got != paneWidth {
		t.Errorf("%d cells, want %d", got, paneWidth)
	}
}

func TestNarrowSummaryLineDropsTheCommitButtonBeforeCuttingIt(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	sum := summary{Unread: 3, Files: 4, Added: 4, Deleted: 2, Staged: 1}
	line, regions := w.SummaryLine(sum, 40)
	if got := w.Of(line); got != 40 {
		t.Errorf("got %d cells, want 40: %q", got, line)
	}
	if strings.Contains(line, "[commit") {
		t.Errorf("commit button should be dropped: %q", line)
	}
	for _, r := range regions {
		if r.Target.Kind == TargetCommit {
			t.Errorf("unexpected TargetCommit region: %+v", r)
		}
	}
	if !strings.Contains(line, "3 unread") {
		t.Errorf("left side should stay readable: %q", line)
	}
	line42, _ := w.SummaryLine(sum, 42)
	if !strings.Contains(line42, "[commit 1 file]") {
		t.Errorf("at width 42 commit should fit: %q", line42)
	}
}

func TestNarrowSummaryLineKeepsSyncAfterDroppingCommit(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	sum := summary{Unread: 3, Files: 4, Added: 4, Deleted: 2, Staged: 1, Behind: 1, Ahead: 1}
	line, regions := w.SummaryLine(sum, 45)
	if got := w.Of(line); got != 45 {
		t.Errorf("got %d cells, want 45: %q", got, line)
	}
	if !strings.Contains(line, "[sync 1↓ 1↑]") {
		t.Errorf("sync button should remain: %q", line)
	}
	var syncRegions int
	for _, r := range regions {
		if r.Target.Kind == TargetButton && r.Target.Verb == state.VerbNameSync {
			syncRegions++
		}
		if r.Target.Kind == TargetCommit {
			t.Errorf("commit should be dropped: %+v", r)
		}
	}
	if syncRegions != 1 {
		t.Errorf("want one sync region, got %d in %+v", syncRegions, regions)
	}
}

func TestNarrowSummaryLineTruncatesTheLeftOnlyWhenItAloneOverflows(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	sum := summary{Unread: 3, Files: 4, Added: 4, Deleted: 2, Staged: 1}
	line, _ := w.SummaryLine(sum, 20)
	if got := w.Of(line); got != 20 {
		t.Errorf("got %d cells, want 20: %q", got, line)
	}
	if !strings.Contains(line, "…") {
		t.Errorf("left should truncate with ellipsis: %q", line)
	}
	if strings.Contains(line, "[commit") {
		t.Errorf("commit should be dropped: %q", line)
	}
}

func TestCommitBoxPaintsTheClosingBracketWhenTheMessageContainsOne(t *testing.T) {
	t.Parallel()
	w := Renderer{Palette: Palette{Enabled: true}}
	plain := Renderer{}
	close := " ]"
	for _, msg := range []string{"", "fix", `say " ]" here`, `fix [a ]b`} {
		colored, _ := w.CommitBox(msg, true, paneWidth)
		uncol, _ := plain.CommitBox(msg, true, paneWidth)
		if got := stripSGR(colored); got != uncol {
			t.Errorf("msg=%q: stripSGR got %q, want %q", msg, got, uncol)
		}
		if !strings.HasSuffix(stripSGR(colored), close) {
			t.Fatalf("msg=%q: line should end with %q: %q", msg, close, colored)
		}
		idx := strings.LastIndex(colored, w.Verb(close))
		if idx < 0 {
			t.Fatalf("msg=%q: closing bracket not Verb-colored at line end: %q", msg, colored)
		}
		if !strings.HasSuffix(colored[idx:], w.Verb(close)) {
			t.Errorf("msg=%q: Verb-colored close is not at the tail: %q", msg, colored)
		}
	}
}

// The bar says how many of the selected files are already staged, and the count
// comes from the section each selected row sits in. Reading the other section
// counts the ones that are not staged, and the bar then offers to stage what is
// already staged and says nothing about what is not.
//
// The tests around this one build the summary by hand, so the line that fills
// SelectedStaged in is not reached by them.
func TestTheSummaryCountsTheSelectedRowsThatAreStaged(t *testing.T) {
	t.Parallel()
	s := state.State{Width: paneWidth, Height: 40}
	s = state.Apply(s, state.StatusLoaded{
		Head: git.Head{Branch: "main"},
		// Two in one section and one in the other: with one each, counting
		// either section gives the same number.
		Rows: []git.Entry{
			{Path: "staged1.txt", Index: git.Modified},
			{Path: "staged2.txt", Index: git.Modified},
			{Path: "unstaged.txt", Worktree: git.Modified},
		},
	})
	for _, sec := range []state.Section{state.SectionStaged, state.SectionUnstaged} {
		for _, row := range s.Rows {
			if row.Kind() == state.RowFile && row.Section() == sec {
				s = state.Apply(s, state.SelectionToggled{Path: row.Path(), Section: sec})
			}
		}
	}

	sum := summaryOf(s, nil)
	if sum.Selected != 3 {
		t.Fatalf("%d rows are selected, want all three", sum.Selected)
	}
	if sum.SelectedStaged != 2 {
		t.Errorf("%d of the selected rows are staged, want the two in the staged section",
			sum.SelectedStaged)
	}
}

// The selected counts add up the rows that are ticked and no others. Joining
// the two halves of that test with or counts every row as soon as one is
// ticked, and the bar then says the reader picked more lines than they did.
func TestTheSelectedCountsAddUpOnlyTheRowsThatAreTicked(t *testing.T) {
	t.Parallel()
	rows := []git.Entry{
		{Path: "a.txt", Worktree: git.Modified, WorktreeCount: git.Count{Added: 3, Deleted: 1}},
		{Path: "b.txt", Worktree: git.Modified, WorktreeCount: git.Count{Added: 5, Deleted: 2}},
	}
	build := func(t *testing.T, tick []string) summary {
		t.Helper()
		s := state.State{Width: paneWidth, Height: 40}
		s = state.Apply(s, state.StatusLoaded{Head: git.Head{Branch: "main"}, Rows: rows})
		for _, path := range tick {
			for _, row := range s.Rows {
				if row.Kind() == state.RowFile && row.Path() == path {
					s = state.Apply(s, state.SelectionToggled{
						Path: path, Section: row.Section()})
				}
			}
		}
		return summaryOf(s, nil)
	}

	none := build(t, nil)
	if none.Selected != 0 || none.SelectedAdded != 0 || none.SelectedDeleted != 0 {
		t.Errorf("with nothing ticked the counts are %d rows, %d added, %d removed",
			none.Selected, none.SelectedAdded, none.SelectedDeleted)
	}

	one := build(t, []string{"a.txt"})
	if one.Selected != 1 || one.SelectedAdded != 3 || one.SelectedDeleted != 1 {
		t.Errorf("with one ticked the counts are %d rows, %d added, %d removed; "+
			"want 1, 3, 1", one.Selected, one.SelectedAdded, one.SelectedDeleted)
	}

	both := build(t, []string{"a.txt", "b.txt"})
	if both.Selected != 2 || both.SelectedAdded != 8 || both.SelectedDeleted != 3 {
		t.Errorf("with both ticked the counts are %d rows, %d added, %d removed; "+
			"want 2, 8, 3", both.Selected, both.SelectedAdded, both.SelectedDeleted)
	}
}

// The commit button is clicked where it is drawn. The regions are walked from
// the left edge of the right half, adding the four columns that separate one
// piece from the next, and a separator counted before the first piece put the
// button's region four columns right of the button.
func TestTheCommitButtonsRegionCoversTheColumnsItIsDrawnAt(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	for _, c := range []struct {
		name string
		sum  summary
	}{
		{"the button is the only piece drawn",
			summary{Staged: 1, NoUpstream: true}},
		{"a sync button sits left of it",
			summary{Staged: 1, Behind: 2}},
		{"the selection's verbs sit left of it",
			summary{Staged: 1, NoUpstream: true, Selected: 2,
				SelectionBarVerbs: []state.VerbName{state.VerbNameStage}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			line, regions := w.SummaryLine(c.sum, paneWidth)
			_, button := summaryButtons(c.sum)
			at := strings.Index(line, button)
			if at < 0 {
				t.Fatalf("the button %q is not drawn in %q", button, line)
			}
			start, end := w.Of(line[:at]), w.Of(line[:at])+w.Of(button)
			found := false
			for _, r := range regions {
				if r.Target.Kind != TargetCommit {
					continue
				}
				found = true
				if r.ColStart != start || r.ColEnd != end {
					t.Errorf("the button is drawn at columns %d..%d and is clickable "+
						"at %d..%d: %q", start, end, r.ColStart, r.ColEnd, line)
				}
			}
			if !found {
				t.Fatalf("no region names the commit button: %+v", regions)
			}
		})
	}
}
