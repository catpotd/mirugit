package layout

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

func fourTabs() []Tab {
	return []Tab{
		{Name: "changes", Number: 1},
		{Name: "history", Number: 2},
		{Name: "stashed", Number: 3},
		{Name: "worktrees", Number: 4},
	}
}

func TestHelpOverlayFitsTheViewport(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	lines, _ := w.HelpOverlay(fourTabs(), state.TabChanges, 0, paneWidth, 53)
	if len(lines) != 53 {
		t.Fatalf("got %d lines, want 53", len(lines))
	}
	for i, line := range lines {
		if got := w.Of(line); got != paneWidth {
			t.Errorf("line %d is %d cells: %q", i, got, line)
		}
	}
}

func TestHelpOverlayListsCommands(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	lines, _ := w.HelpOverlay(fourTabs(), state.TabChanges, 0, paneWidth, 53)
	joined := strings.Join(lines, "\n")
	for _, want := range []string{
		"commands",
		"move the cursor",
		"select everything in the tab",
		"mark read without opening",
		"stage",
		"undo the last discard",
		"write a commit message",
		"fold the directory",
		"discard",
		"esc close",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q", want)
		}
	}
	for _, omit := range []string{
		"restore the stash to the working tree",
		"turn the stash into a branch",
		"go to the worktree",
	} {
		if strings.Contains(joined, omit) {
			t.Errorf("forbidden %q on changes tab", omit)
		}
	}
	if !strings.Contains(joined, "fetch") {
		t.Error("missing fetch")
	}

	stashed, _ := w.HelpOverlay(fourTabs(), state.TabStashed, 0, paneWidth, 53)
	stashedJoined := strings.Join(stashed, "\n")
	for _, want := range []string{
		"restore the stash to the working tree",
		"turn the stash into a branch",
	} {
		if !strings.Contains(stashedJoined, want) {
			t.Errorf("missing %q on stashed tab", want)
		}
	}

	worktrees, _ := w.HelpOverlay(fourTabs(), state.TabWorktrees, 0, paneWidth, 53)
	if !strings.Contains(strings.Join(worktrees, "\n"), "go to the worktree") {
		t.Error("missing go to the worktree on worktrees tab")
	}
}

func TestEveryHelpVerbHasARegionOnItsLine(t *testing.T) {
	t.Parallel()
	w := Renderer{Clipboard: true, Browser: true}
	tabs := fourTabs()
	cases := []struct {
		tab   state.Tab
		desc  string
		verb  state.VerbName
		match string
	}{
		{state.TabChanges, "stage", state.VerbNameStage, "stage"},
		{state.TabChanges, "unstage", state.VerbNameUnstage, "unstage"},
		{state.TabChanges, "discard", state.VerbNameDiscard, "discard"},
		{state.TabChanges, "stash key", state.VerbNameStash, "  z"},
		{state.TabChanges, "undo the last discard", state.VerbNameUndo, "undo the last discard"},
		{state.TabChanges, "open the diff", state.VerbNameDiff, "open the diff"},
		{state.TabChanges, "mark read without opening", state.VerbNameRead, "mark read without opening"},
		{state.TabChanges, "reload", state.VerbNameReload, "reload"},
		{state.TabChanges, "fetch", state.VerbNameFetch, "fetch"},
		{state.TabChanges, "write a commit message", state.VerbNameCommit, "write a commit message"},
		{state.TabChanges, "quit", state.VerbNameQuit, "quit"},
		{state.TabChanges, "fold the directory", state.VerbNameFold, "fold the directory"},
		{state.TabStashed, "restore the stash to the working tree", state.VerbNameRestore, "restore the stash to the working tree"},
		{state.TabStashed, "turn the stash into a branch", state.VerbNameBranch, "turn the stash into a branch"},
		{state.TabWorktrees, "go to the worktree", state.VerbNameGo, "go to the worktree"},
		{state.TabHistory, "copy the commit sha", state.VerbNameSHA, "copy the commit sha"},
		{state.TabHistory, "open the commit in a browser", state.VerbNameOpen, "open the commit in a browser"},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			lines, regions := w.HelpOverlay(tabs, c.tab, 0, paneWidth, 53)
			row := -1
			for i, line := range lines {
				if strings.Contains(line, c.match) {
					row = i
					break
				}
			}
			if row < 0 {
				t.Fatalf("no line for %q", c.desc)
			}
			var hit bool
			for _, r := range regions {
				if r.Row != row {
					continue
				}
				if r.Target.Kind == TargetHelpLine && r.Target.Verb == c.verb {
					hit = true
					break
				}
			}
			if !hit {
				t.Errorf("no region for %q on row %d", c.verb, row)
			}
		})
	}
}

func TestEveryHelpVerbIsImplemented(t *testing.T) {
	t.Parallel()
	// These names must stay aligned with the TargetHelpLine branch in keys.go.
	implemented := map[state.VerbName]bool{
		state.VerbNameDown:      true,
		state.VerbNamePage:      true,
		state.VerbNameEnds:      true,
		state.VerbNameSelect:    true,
		state.VerbNameSelectAll: true,
		state.VerbNameExtend:    true,
		state.VerbNameClear:     true,
		state.VerbNameNextTab:   true,
		state.VerbNameStage:     true,
		state.VerbNameUnstage:   true,
		state.VerbNameDiscard:   true,
		state.VerbNameStash:     true,
		state.VerbNameUndo:      true,
		state.VerbNameUncommit:  true,
		state.VerbNameDiff:      true,
		state.VerbNameRead:      true,
		state.VerbNameSHA:       true,
		state.VerbNameOpen:      true,
		state.VerbNameReload:    true,
		state.VerbNameFetch:     true,
		state.VerbNameSync:      true,
		state.VerbNameCommit:    true,
		state.VerbNameNextBlock: true,
		state.VerbNameQuit:      true,
		state.VerbNameRestore:   true,
		state.VerbNameBranch:    true,
		state.VerbNameGo:        true,
		state.VerbNameFold:      true,
		state.VerbNameDrop:      true,
		state.VerbNameRemove:    true,
	}
	for _, verb := range HelpVerbs() {
		if !implemented[verb] {
			t.Errorf("help lists %q but keys.go does not handle it", verb)
		}
	}
	for verb := range implemented {
		found := false
		for _, listed := range HelpVerbs() {
			if listed == verb {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("keys.go handles %q but help does not list it", verb)
		}
	}
}

func TestHelpListsEveryFooterVerb(t *testing.T) {
	t.Parallel()
	// The x key runs a different verb per tab and the help row names that
	// tab's verb, so drop and remove need no alias. Unfold has no row of its
	// own: the fold row toggles.
	aliases := map[state.VerbName]state.VerbName{
		state.VerbNameUnfold: state.VerbNameFold,
	}
	help := make(map[state.VerbName]bool)
	for _, verb := range HelpVerbs() {
		help[verb] = true
	}
	for verb := range footerKeys {
		want := verb
		if mapped, ok := aliases[verb]; ok {
			want = mapped
		}
		if !help[want] {
			t.Errorf("footer verb %q (maps to %q) is not listed in help", verb, want)
		}
	}
}

// A click reports a column, so the region has to be in columns. This test read
// the line by byte offset and so did the code it checked: the footer separates
// its verbs with ·, two bytes and one cell, and the two errors canceled. The
// mark could not be clicked on any footer carrying a separator.
func TestFooterHelpMarkHasRegion(t *testing.T) {
	t.Parallel()
	for _, eastAsian := range []bool{false, true} {
		w := Renderer{Widths: Widths{EastAsian: eastAsian}}
		f := Pane(fixedState(), nil, w)
		var hit bool
		for _, r := range f.Regions {
			if r.Target.Kind != TargetHelp {
				continue
			}
			line := ansi.Strip(f.Lines[r.Row])
			if drawn := drawnCells(w, line, r.ColStart, r.ColEnd); drawn != "?" {
				t.Errorf("eastAsian=%v: the help region sits on %q: %q",
					eastAsian, drawn, line)
			}
			if r.ColEnd > w.Of(line) {
				t.Errorf("eastAsian=%v: the help region ends past the pane: [%d,%d) of %d",
					eastAsian, r.ColStart, r.ColEnd, w.Of(line))
			}
			hit = true
		}
		if !hit {
			t.Fatalf("eastAsian=%v: footer ? has no TargetHelp region", eastAsian)
		}
	}
}

func TestHelpOverlayShowsAssumedWidthWhenNotMeasured(t *testing.T) {
	t.Parallel()
	w := Renderer{Widths: Widths{IsWidthAssumed: true}}
	lines, _ := w.HelpOverlay(fourTabs(), state.TabChanges, 0, paneWidth, 53)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "assumed one column") {
		t.Fatalf("missing assumed width notice: %s", joined)
	}
	if strings.Contains(joined, "character width  measured") {
		t.Error("should not show measured when width was assumed")
	}
}

func TestHelpOverlayTruncatesRowsWiderThanThePane(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	lines, _ := w.HelpOverlay(fourTabs(), state.TabChanges, 0, 51, 53)
	for i, line := range lines {
		if got := w.Of(line); got != 51 {
			t.Errorf("line %d is %d cells, want 51: %q", i, got, line)
		}
	}
	var escLine string
	for _, line := range lines {
		if strings.Contains(line, "esc") {
			escLine = line
			break
		}
	}
	if escLine == "" {
		t.Fatal("no line containing esc")
	}
	if !strings.Contains(escLine, ellipsis) {
		t.Errorf("esc line should be truncated with ellipsis, got %q", escLine)
	}

	lines56, _ := w.HelpOverlay(fourTabs(), state.TabChanges, 0, 56, 53)
	for _, line := range lines56 {
		if strings.Contains(line, "esc") {
			if strings.Contains(line, ellipsis) {
				t.Errorf("esc line at width 56 should not be truncated, got %q", line)
			}
			if !strings.Contains(line, "close the diff") {
				t.Errorf("esc line at width 56 should contain full description, got %q", line)
			}
			break
		}
	}
}

func TestHelpOverlayTruncatesTheWidthNoticeRow(t *testing.T) {
	t.Parallel()
	w := Renderer{Widths: Widths{IsWidthAssumed: true}}
	lines, _ := w.HelpOverlay(fourTabs(), state.TabChanges, 0, 51, 53)
	var noticeLine string
	for _, line := range lines {
		if strings.Contains(line, "assumed") {
			noticeLine = line
			break
		}
	}
	if noticeLine == "" {
		t.Fatal("no assumed-width notice line")
	}
	if got := w.Of(noticeLine); got != 51 {
		t.Errorf("notice line is %d cells, want 51: %q", got, noticeLine)
	}
	if !strings.Contains(noticeLine, ellipsis) {
		t.Errorf("notice line should be truncated with ellipsis, got %q", noticeLine)
	}
}

func TestPaneWithHelpOpenReplacesTheBody(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := fixedState()
	s.HelpOpen = true
	f := Pane(s, nil, w)
	if len(f.Lines) != 53 {
		t.Fatalf("got %d lines, want 53", len(f.Lines))
	}
	if !strings.Contains(f.Lines[2], "commands") {
		t.Errorf("line 2 = %q, want the help overlay", f.Lines[2])
	}
	if strings.Contains(f.Lines[2], "commit message") {
		t.Error("the commit box should be hidden behind help")
	}
	if !strings.Contains(f.Lines[52], "esc close") {
		t.Errorf("last line = %q, want the help footer", f.Lines[52])
	}
}

// Every line of this list runs on click. A line that draws like the
// others and does nothing is worse than no line at all.
func TestEveryHelpLineIsClickable(t *testing.T) {
	t.Parallel()
	for _, row := range helpRows {
		if row.keys == "" {
			continue
		}
		if row.verb.IsZero() {
			t.Errorf("%q (%s) has no verb, so clicking it does nothing",
				row.desc, row.keys)
		}
	}
}

func TestHelpTabRowListsOnlyVisibleTabs(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	cases := []struct {
		name   string
		tabs   []Tab
		want   string
		forbid string
	}{
		{
			name: "two tabs",
			tabs: []Tab{{Name: "changes", Number: 1}, {Name: "history", Number: 2}},
			want: "tab  1-2", forbid: "1-4",
		},
		{
			name: "gap at three",
			tabs: []Tab{
				{Name: "changes", Number: 1},
				{Name: "history", Number: 2},
				{Name: "worktrees", Number: 4},
			},
			want: "tab  1  2  4",
		},
		{
			name: "all four",
			tabs: fourTabs(),
			want: "tab  1-4",
		},
		{
			name:   "one tab",
			tabs:   []Tab{{Name: "changes", Number: 1}},
			want:   "tab  1",
			forbid: "1-",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			lines, _ := w.HelpOverlay(c.tabs, state.TabChanges, 0, paneWidth, 53)
			joined := strings.Join(lines, "\n")
			if !strings.Contains(joined, c.want) {
				t.Errorf("missing %q in:\n%s", c.want, joined)
			}
			if c.forbid != "" && strings.Contains(joined, c.forbid) {
				t.Errorf("forbidden %q in:\n%s", c.forbid, joined)
			}
		})
	}
}

func TestHelpTabRowKeepsItsClickRegion(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	tabs := []Tab{{Name: "changes", Number: 1}, {Name: "history", Number: 2}}
	lines, regions := w.HelpOverlay(tabs, state.TabChanges, 0, paneWidth, 53)
	row := -1
	for i, line := range lines {
		if strings.Contains(line, "switch tabs") {
			row = i
			break
		}
	}
	if row < 0 {
		t.Fatal("no switch tabs line")
	}
	var hit bool
	for _, r := range regions {
		if r.Row != row {
			continue
		}
		if r.Target.Kind == TargetHelpLine && r.Target.Verb == state.VerbNameNextTab {
			hit = true
			break
		}
	}
	if !hit {
		t.Errorf("no next tab region on row %d", row)
	}
}

func TestHelpOverlayScrollReachesTheQuitRow(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	tabs := fourTabs()
	lines, _ := w.HelpOverlay(tabs, state.TabChanges, 0, paneWidth, 12)
	for _, line := range lines {
		if strings.Contains(line, "q  ctrl+c") {
			t.Fatal("quit row should not appear at scroll 0 with height 12")
		}
	}
	scroll := w.HelpScrollClamp(1000, state.TabChanges, 12)
	lines, _ = w.HelpOverlay(tabs, state.TabChanges, scroll, paneWidth, 12)
	var quit bool
	for _, line := range lines {
		if strings.Contains(line, "quit") {
			quit = true
			break
		}
	}
	if !quit {
		t.Fatal("quit row should appear at max scroll with height 12")
	}
}

func TestHelpOverlayFooterShowsScrollRange(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	lines, _ := w.HelpOverlay(fourTabs(), state.TabChanges, 0, paneWidth, 12)
	footer := lines[len(lines)-1]
	wantTotal := w.HelpRowCount(state.TabChanges)
	want := fmt.Sprintf("1-10 / %d", wantTotal)
	if !strings.Contains(footer, "j  k  scroll") {
		t.Errorf("footer %q should mention scroll keys", footer)
	}
	if !strings.Contains(footer, want) {
		t.Errorf("footer %q should contain %q", footer, want)
	}
}

func TestHelpOverlayFooterStaysBareWhenAllRowsFit(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	lines, _ := w.HelpOverlay(fourTabs(), state.TabChanges, 0, paneWidth, 53)
	footer := lines[len(lines)-1]
	want := w.Pad("", "esc close"+strings.Repeat(" ", helpGap)+"?", paneWidth)
	if footer != want {
		t.Errorf("footer = %q, want %q", footer, want)
	}
}

func TestHelpScrollClampBounds(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	if got := w.HelpScrollClamp(-1, state.TabChanges, 12); got != 0 {
		t.Errorf("negative scroll = %d, want 0", got)
	}
	if got := w.HelpScrollClamp(5, state.TabChanges, 53); got != 0 {
		t.Errorf("fitting height scroll = %d, want 0", got)
	}
	// 12 - 2 = 10 rows of content against the changes list's 29 rows, so 19 is
	// the last offset that still fills the overlay.
	if got := w.HelpScrollClamp(1000, state.TabChanges, 12); got != 19 {
		t.Errorf("max scroll = %d, want 19", got)
	}
}

// The clamp has to count the rows this Widths will draw. Counting as if the
// clipboard and browser were present let history scroll two rows past its end,
// pushing the first title off the top and leaving blank rows at the bottom.
func TestHelpScrollClampCountsTheRowsThisWidthsDraws(t *testing.T) {
	t.Parallel()
	plain := Renderer{}
	full := Renderer{Clipboard: true, Browser: true}
	// history draws 20 rows without a clipboard or browser and 22 with both.
	if got := plain.HelpRowCount(state.TabHistory); got != 20 {
		t.Fatalf("Renderer{} の history = %d 行, want 20", got)
	}
	if got := full.HelpRowCount(state.TabHistory); got != 22 {
		t.Fatalf("clipboard と browser ありの history = %d 行, want 22", got)
	}
	// All 20 fit in a 22-line overlay, so there is nothing to scroll to.
	if got := plain.HelpScrollClamp(1000, state.TabHistory, 22); got != 0 {
		t.Errorf("全行が収まる高さでの max scroll = %d, want 0", got)
	}
}

func TestHelpOverlayScrolledRegionsFollowVisibleRows(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	scroll := w.HelpScrollClamp(1000, state.TabChanges, 12)
	lines, regions := w.HelpOverlay(fourTabs(), state.TabChanges, scroll, paneWidth, 12)
	row := -1
	for i, line := range lines {
		if strings.Contains(line, "quit") {
			row = i
			break
		}
	}
	if row < 0 {
		t.Fatal("no quit line at max scroll")
	}
	var hit bool
	for _, r := range regions {
		if r.Row != row {
			continue
		}
		if r.Target.Kind == TargetHelpLine && r.Target.Verb == state.VerbNameQuit {
			hit = true
			break
		}
	}
	if !hit {
		t.Errorf("no quit region on row %d", row)
	}
}

func TestHelpOverlayOmitsSelectOnNonChangesTabs(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	forbidden := []string{
		"select · deselect",
		"select everything in the tab",
		"extend the selection",
		"clear the selection",
	}
	keepOnChanges := forbidden[:3]

	commit := git.CommitInfo{SHA: "a", ShortSHA: "a", Subject: "one", Age: "1m"}
	stash := git.StashRow{
		Ref: "stash@{0}", SHA: "s", Message: "hold", Branch: "main",
		Age: "1h", Status: git.StashApplies,
	}
	wt := git.WorktreeRow{Path: "/repo", Name: "repo", Branch: "main", Main: true}

	nonChanges := []struct {
		name string
		s    state.State
	}{
		{"history", state.State{Width: paneWidth, Height: 53, Tab: state.TabHistory, HelpOpen: true, Rows: []state.Row{state.CommitRow(commit)}, History: state.History{Commits: []git.CommitInfo{commit}}}},
		{"stashed", state.State{Width: paneWidth, Height: 53, Tab: state.TabStashed, HelpOpen: true, Rows: []state.Row{state.StashRowOf(stash)}, Stashed: state.Stashed{Stashes: []git.StashRow{stash}}}},
		{"worktrees", state.State{Width: paneWidth, Height: 53, Tab: state.TabWorktrees, HelpOpen: true, Rows: []state.Row{state.WorktreeRowOf(wt)}, Worktrees: state.Worktrees{Base: "main", List: []git.WorktreeRow{wt}}}},
	}
	for _, c := range nonChanges {
		t.Run(c.name, func(t *testing.T) {
			joined := strings.Join(Pane(c.s, nil, w).Lines, "\n")
			for _, omit := range forbidden {
				if strings.Contains(joined, omit) {
					t.Errorf("forbidden %q in help overlay", omit)
				}
			}
			if !strings.Contains(joined, "close the diff") {
				t.Error("missing close the diff")
			}
		})
	}
	t.Run("changes", func(t *testing.T) {
		s := fixedState()
		s.HelpOpen = true
		joined := strings.Join(Pane(s, nil, w).Lines, "\n")
		for _, want := range keepOnChanges {
			if !strings.Contains(joined, want) {
				t.Errorf("missing %q on changes tab", want)
			}
		}
	})
}

func TestHelpOverlayOmitsFoldAndReadOnNonChangesTabs(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	forbidden := []string{
		"fold the directory",
		"mark read without opening",
	}
	keepOnChanges := forbidden

	commit := git.CommitInfo{SHA: "a", ShortSHA: "a", Subject: "one", Age: "1m"}
	stash := git.StashRow{
		Ref: "stash@{0}", SHA: "s", Message: "hold", Branch: "main",
		Age: "1h", Status: git.StashApplies,
	}
	wt := git.WorktreeRow{Path: "/repo", Name: "repo", Branch: "main", Main: true}

	nonChanges := []struct {
		name string
		s    state.State
	}{
		{"history", state.State{Width: paneWidth, Height: 53, Tab: state.TabHistory, HelpOpen: true, Rows: []state.Row{state.CommitRow(commit)}, History: state.History{Commits: []git.CommitInfo{commit}}}},
		{"stashed", state.State{Width: paneWidth, Height: 53, Tab: state.TabStashed, HelpOpen: true, Rows: []state.Row{state.StashRowOf(stash)}, Stashed: state.Stashed{Stashes: []git.StashRow{stash}}}},
		{"worktrees", state.State{Width: paneWidth, Height: 53, Tab: state.TabWorktrees, HelpOpen: true, Rows: []state.Row{state.WorktreeRowOf(wt)}, Worktrees: state.Worktrees{Base: "main", List: []git.WorktreeRow{wt}}}},
	}
	for _, c := range nonChanges {
		t.Run(c.name, func(t *testing.T) {
			joined := strings.Join(Pane(c.s, nil, w).Lines, "\n")
			for _, omit := range forbidden {
				if strings.Contains(joined, omit) {
					t.Errorf("forbidden %q in help overlay", omit)
				}
			}
		})
	}
	t.Run("changes", func(t *testing.T) {
		s := fixedState()
		s.HelpOpen = true
		joined := strings.Join(Pane(s, nil, w).Lines, "\n")
		for _, want := range keepOnChanges {
			if !strings.Contains(joined, want) {
				t.Errorf("missing %q on changes tab", want)
			}
		}
	})
}

func helpVerbSetForTab(tab state.Tab) map[state.VerbName]bool {
	out := make(map[state.VerbName]bool)
	for _, row := range helpRowsForTab(tab, Renderer{Clipboard: true, Browser: true}) {
		if !row.verb.IsZero() {
			out[row.verb] = true
		}
	}
	return out
}

func footerVerbsUnionForTab(tab state.Tab) map[state.VerbName]bool {
	w := Renderer{Clipboard: true, Browser: true}
	union := make(map[state.VerbName]bool)
	add := func(verbs []state.VerbName) {
		for _, v := range verbs {
			union[v] = true
		}
	}

	commit := git.CommitInfo{SHA: "abc", ShortSHA: "abc", Subject: "one", Age: "1m"}
	stash := git.StashRow{
		Ref: "stash@{0}", SHA: "s", Message: "hold", Branch: "main",
		Age: "1h", Status: git.StashApplies,
	}
	wt := git.WorktreeRow{Path: "/repo", Name: "repo", Branch: "main", Main: false, Dirty: 0}

	switch tab {
	case state.TabChanges:
		folded := map[string]bool{}
		add(footerVerbs(state.State{Width: paneWidth, Height: 53, Tab: state.TabChanges, Changes: state.Changes{Folded: folded}}, w))

		rows := []state.Row{
			state.SectionHeadingRow(state.SectionUnstaged),
			state.DirectoryRow("app", state.SectionUnstaged),
			state.FileRow(git.Entry{Path: "a.txt", Worktree: git.Modified}, state.SectionUnstaged),
			state.FileRow(git.Entry{Path: "b.txt", Index: git.Modified}, state.SectionStaged),
			state.FileRow(git.Entry{Path: "c.txt", Index: git.Unmerged, Worktree: git.Unmerged}, state.SectionUnstaged),
		}
		selected := map[string]bool{
			strconv.Itoa(int(state.SectionUnstaged)) + "\x00" + "a.txt": true,
			strconv.Itoa(int(state.SectionStaged)) + "\x00" + "b.txt":   true,
		}

		for cursor := range rows {
			add(footerVerbs(state.State{Width: paneWidth, Height: 53, Tab: state.TabChanges, Cursor: cursor, Rows: rows, Changes: state.Changes{Folded: folded}}, w))
			add(footerVerbs(state.State{Width: paneWidth, Height: 53, Tab: state.TabChanges, Cursor: cursor, Rows: rows, Changes: state.Changes{Folded: folded, Selected: selected}}, w))
		}
		add(footerVerbs(state.State{Width: paneWidth, Height: 53, Tab: state.TabChanges, Cursor: -1, Rows: rows, Changes: state.Changes{Folded: folded}}, w))
		add(footerVerbs(state.State{Width: paneWidth, Height: 53, Tab: state.TabChanges, Cursor: -1, Rows: rows, Changes: state.Changes{Folded: folded, Selected: selected}}, w))

		foldedApp := map[string]bool{strconv.Itoa(int(state.SectionUnstaged)) + "\x00" + "app": true}
		add(footerVerbs(state.State{Width: paneWidth, Height: 53, Tab: state.TabChanges, Cursor: 1, Rows: []state.Row{state.DirectoryRow("app", state.SectionUnstaged)}, Changes: state.Changes{Folded: foldedApp}}, w))

		add(footerVerbs(state.State{Width: paneWidth, Height: 53, Tab: state.TabChanges, Cursor: 2, Rows: rows[:3], Changes: state.Changes{Folded: folded}, Open: state.OpenDiff{Path: "a.txt", Diff: git.FileDiff{Blocks: []git.Block{{Hash: "a"}, {Hash: "b"}}}}}, w))
	case state.TabHistory:
		s := state.State{Width: paneWidth, Height: 53, Tab: state.TabHistory, Cursor: 0, Rows: []state.Row{state.CommitRow(commit)}, History: state.History{RemoteURL: "git@github.com:owner/repo.git", Commits: []git.CommitInfo{commit}}}
		add(footerVerbs(s, w))
		// The tip of an unpushed branch is the one row that offers uncommit.
		// Without it the union says the footer never shows the key, and the
		// help row for it reads as one the tab cannot reach.
		unpushed := commit
		unpushed.Unpushed = true
		add(footerVerbs(state.State{Width: paneWidth, Height: 53, Tab: state.TabHistory,
			Cursor: 0, Rows: []state.Row{state.CommitRow(unpushed)},
			History: state.History{Commits: []git.CommitInfo{unpushed}}}, w))
		s.Cursor = -1
		add(footerVerbs(s, w))
		s.Open.Path = "a.txt"
		s.Open.Diff = git.FileDiff{Blocks: []git.Block{{Hash: "a"}, {Hash: "b"}}}
		add(footerVerbs(s, w))
	case state.TabStashed:
		for _, status := range []git.StashStatus{git.StashApplies, git.StashConflicts} {
			st := stash
			st.Status = status
			s := state.State{Width: paneWidth, Height: 53, Tab: state.TabStashed, Cursor: 0, Rows: []state.Row{state.StashRowOf(st)}, Stashed: state.Stashed{Stashes: []git.StashRow{st}}}
			add(footerVerbs(s, w))
			s.Cursor = 1
			s.Rows = []state.Row{
				state.StashRowOf(st),
				state.FileRow(git.Entry{Path: "a.txt"}, state.SectionNone),
			}
			add(footerVerbs(s, w))
			s.Open.Path = "a.txt"
			s.Open.Diff = git.FileDiff{Blocks: []git.Block{{Hash: "a"}, {Hash: "b"}}}
			add(footerVerbs(s, w))
		}
	case state.TabWorktrees:
		s := state.State{Width: paneWidth, Height: 53, Tab: state.TabWorktrees, Cursor: 0, Rows: []state.Row{state.WorktreeRowOf(wt)}, Worktrees: state.Worktrees{Base: "main", Here: "/other", List: []git.WorktreeRow{wt}}}
		add(footerVerbs(s, w))
		s.Cursor = 1
		s.Rows = []state.Row{
			state.WorktreeRowOf(wt),
			state.FileRow(git.Entry{Path: "a.txt"}, state.SectionNone),
		}
		add(footerVerbs(s, w))
		s.Open.Path = "a.txt"
		s.Open.Diff = git.FileDiff{Blocks: []git.Block{{Hash: "a"}, {Hash: "b"}}}
		add(footerVerbs(s, w))
	}
	return union
}

func helpVerbsForFooterVerb(footerVerb state.VerbName) []state.VerbName {
	switch footerVerb {
	case state.VerbNameSelect:
		return []state.VerbName{state.VerbNameSelect, state.VerbNameSelectAll, state.VerbNameExtend}
	case state.VerbNameFold, state.VerbNameUnfold:
		return []state.VerbName{state.VerbNameFold}
	default:
		return []state.VerbName{footerVerb}
	}
}

func footerVerbsForHelpVerb(helpVerb state.VerbName) []state.VerbName {
	switch helpVerb {
	case state.VerbNameSelect, state.VerbNameSelectAll, state.VerbNameExtend:
		return []state.VerbName{state.VerbNameSelect}
	case state.VerbNameFold:
		return []state.VerbName{state.VerbNameFold, state.VerbNameUnfold}
	default:
		return []state.VerbName{helpVerb}
	}
}

func TestHelpOverlayVerbsDoNotContradictFooterOnEachTab(t *testing.T) {
	t.Parallel()
	tabs := []struct {
		name string
		tab  state.Tab
	}{
		{"changes", state.TabChanges},
		{"history", state.TabHistory},
		{"stashed", state.TabStashed},
		{"worktrees", state.TabWorktrees},
	}
	for _, tc := range tabs {
		t.Run(tc.name, func(t *testing.T) {
			footerUnion := footerVerbsUnionForTab(tc.tab)
			helpVerbs := helpVerbSetForTab(tc.tab)

			for footerVerb := range footerUnion {
				for _, helpVerb := range helpVerbsForFooterVerb(footerVerb) {
					if helpAlwaysVerbs[helpVerb] || helpVerb == state.VerbNameUndo {
						continue
					}
					if !helpVerbs[helpVerb] {
						t.Errorf("footer may show %q but help omits %q", footerVerb, helpVerb)
					}
				}
			}

			for helpVerb := range helpVerbs {
				if helpAlwaysVerbs[helpVerb] || helpVerb == state.VerbNameUndo || helpVerb == state.VerbNameClear {
					continue
				}
				footerNames := footerVerbsForHelpVerb(helpVerb)
				allowed := false
				for _, footerVerb := range footerNames {
					if _, ok := footerKeys[footerVerb]; !ok {
						allowed = true
						break
					}
					if footerUnion[footerVerb] {
						allowed = true
						break
					}
				}
				if !allowed {
					t.Errorf("help lists %q but footer cannot show it on this tab", helpVerb)
				}
			}
		})
	}
}

func TestStashedHelpListsOpenTheDiff(t *testing.T) {
	t.Parallel()
	w := Renderer{Clipboard: true, Browser: true}
	for _, c := range []struct {
		name string
		s    state.State
	}{
		{"stashed", stashPaneStateWithFiles()},
		{"worktrees", worktreePaneStateWithFiles()},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := c.s
			s.HelpOpen = true
			s.Height = 53
			joined := strings.Join(Pane(s, nil, w).Lines, "\n")
			if !strings.Contains(joined, "open the diff") {
				t.Errorf("help overlay = %q, want open the diff", joined)
			}
		})
	}
}

// The footer of the help says how to leave, and adds where the reader is in
// the list only while the list is longer than the area. A list that fits has
// no scrolling to report, and "1-N / N" beside a list nobody can scroll is a
// line that says nothing and takes the room the way out needs.
func TestTheHelpFooterCountsOnlyWhenTheListScrolls(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	for _, c := range []struct {
		name               string
		total, contentRows int
		wantScroll         bool
	}{
		{"shorter than the area", 5, 10, false},
		{"exactly the area", 10, 10, false},
		{"one row longer", 11, 10, true},
		{"far longer", 40, 10, true},
		{"an area of none", 5, 0, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			line := w.helpOverlayFooter(0, c.contentRows, c.total, paneWidth)
			if got := strings.Contains(line, "scroll"); got != c.wantScroll {
				t.Errorf("the footer is %q, want a scroll count = %v", line, c.wantScroll)
			}
			if !strings.Contains(line, "esc close") {
				t.Errorf("the footer is %q, and the way out is what stays", line)
			}
			if got := w.Of(line); got != paneWidth {
				t.Errorf("the footer is %d cells, want %d: %q", got, paneWidth, line)
			}
		})
	}
}

// The two halves of the help footer sit side by side with a cell between them,
// so a pane exactly that wide holds both. Cutting the left half one cell early
// takes a character off the position the reader is at, on the one pane where
// the line is already as short as it goes.
func TestTheHelpFooterKeepsBothHalvesAtTheWidthThatFitsThem(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	const right = "esc close" + "   " + "?"
	left := "j  k  scroll   1-10 / 40"
	fits := w.Of(left) + 1 + w.Of(right)

	line := w.helpOverlayFooter(0, 10, 40, fits)
	if !strings.Contains(line, left) {
		t.Errorf("at %d columns the footer is %q, and both halves fit", fits, line)
	}
	if narrower := w.helpOverlayFooter(0, 10, 40, fits-1); strings.Contains(narrower, left) {
		t.Errorf("at %d columns the footer is %q, and the left half does not fit",
			fits-1, narrower)
	}
	// The way out stays at every width the overlay is drawn at, and the line
	// is exactly as wide as the pane. Below minPaneWidth the pane says it is
	// too narrow instead of drawing anything, so those widths never reach here.
	for width := minPaneWidth; width <= fits+10; width++ {
		got := w.helpOverlayFooter(0, 10, 40, width)
		if !strings.Contains(got, "esc close") {
			t.Errorf("at %d columns the way out is missing: %q", width, got)
		}
		if w.Of(got) != width {
			t.Errorf("at %d columns the footer is %d cells: %q", width, w.Of(got), got)
		}
	}
}

// A help row is decided by one of three rules: it is shown on every tab, it is
// one of the grouped cases above, or the tabs that explain it are the tabs
// whose footer prints its key. A verb answering to none of them has no rule
// saying where its row belongs.
func TestEveryVerbNameIsAnsweredByOneOfTheHelpsThreeRules(t *testing.T) {
	t.Parallel()
	ownRule := map[state.VerbName]bool{
		state.VerbNameClear:     true,
		state.VerbNameCommit:    true,
		state.VerbNameNextBlock: true,
	}
	grouped := map[state.VerbName]bool{
		state.VerbNameSelect:    true,
		state.VerbNameSelectAll: true,
		state.VerbNameExtend:    true,
		state.VerbNameFold:      true,
		state.VerbNameDiscard:   true,
	}
	for _, verb := range state.AllVerbNames {
		if ownRule[verb] || grouped[verb] || helpAlwaysVerbs[verb] {
			continue
		}
		if _, ok := footerKeys[verb]; !ok {
			t.Errorf("nothing says which tabs explain %q: it is not shown on every "+
				"tab, it is not one of the grouped cases, and no footer prints a "+
				"key for it", verb)
		}
	}
}

// The union is what the help rows and the footer check are compared against, so
// a case it never enumerates is a case neither of them covers. A directory row
// prints two different verbs — fold while it is open, unfold while it is closed
// — and a list built without a closed directory in it holds only the first.
func TestTheChangesUnionCoversAFoldedDirectoryAsWellAsAnOpenOne(t *testing.T) {
	t.Parallel()
	union := footerVerbUnionForTab(state.TabChanges, Renderer{})
	for _, verb := range []state.VerbName{state.VerbNameFold, state.VerbNameUnfold} {
		if !union[verb] {
			t.Errorf("the changes tab never enumerates %q, so nothing checks its key", verb)
		}
	}
}
