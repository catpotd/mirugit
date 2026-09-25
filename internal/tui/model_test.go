package tui

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
	"github.com/charmbracelet/colorprofile"
)

func TestNOColordisablesPalette(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if colorPalette(colorprofile.TrueColor).Enabled {
		t.Fatal("NO_COLOR should disable the palette")
	}
}

// Writing escape codes to a terminal that cannot render them leaves the codes
// on the screen, so the line between the profiles that get color and the ones
// that do not is what every drawn row depends on.
func TestOnlyATerminalWithColorsGetsThem(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	cases := []struct {
		profile colorprofile.Profile
		enabled bool
	}{
		{colorprofile.NoTTY, false},
		{colorprofile.Ascii, false},
		{colorprofile.ANSI, true},
		{colorprofile.ANSI256, true},
		{colorprofile.TrueColor, true},
	}
	for _, c := range cases {
		t.Run(c.profile.String(), func(t *testing.T) {
			if got := colorPalette(c.profile).Enabled; got != c.enabled {
				t.Errorf("%s has color %v, want %v", c.profile, got, c.enabled)
			}
		})
	}
}

func TestShiftedRLooksLikeReloadToTheModel(t *testing.T) {
	t.Parallel()
	if !isReloadKey(tea.KeyPressMsg{Code: 'r', Text: "R", Mod: tea.ModShift}) {
		t.Error("a shifted r was not recognized as reload")
	}
	if isReloadKey(tea.KeyPressMsg{Code: 'r', Text: "r"}) {
		t.Error("a plain r was taken for reload")
	}
}

func TestCopySHASaysWhatItCopied(t *testing.T) {
	t.Parallel()
	msg := runCopySHA(alwaysCommand("true"), "1868950")()
	got, ok := msg.(verbMsg)
	if !ok {
		t.Fatalf("got %T, want verbMsg", msg)
	}
	if got.err != nil {
		t.Fatalf("true: %v", got.err)
	}
	if got.finished.Notice != "copied 1868950" {
		t.Fatalf("notice = %q, want copied 1868950", got.finished.Notice)
	}

	msg = runCopySHA(alwaysCommand("false"), "1868950")()
	got, ok = msg.(verbMsg)
	if !ok {
		t.Fatalf("got %T, want verbMsg", msg)
	}
	if got.err == nil {
		t.Fatal("false should fail")
	}
}

func TestClickOnADirectoryCheckboxTogglesItsFiles(t *testing.T) {
	t.Parallel()
	m := &Model{read: emptyRead(), state: state.State{Rows: []state.Row{
		state.DirectoryRow("src", state.SectionUnstaged),
		state.FileRow(
			git.Entry{Path: "src/a.go"}, state.SectionUnstaged),

		state.FileRow(
			git.Entry{Path: "src/b.go"}, state.SectionUnstaged),
	}, Changes: state.Changes{Selected: map[string]bool{}}}}
	tg := layout.Target{Kind: layout.TargetCheckbox, Path: "src", Row: 0}
	updated, _ := m.checkboxClick(tg, 0)
	m = updated.(*Model)
	if !state.SelectedIn(m.state, state.SectionUnstaged, "src/a.go") ||
		!state.SelectedIn(m.state, state.SectionUnstaged, "src/b.go") {
		t.Fatalf("first click: selected = %v", m.state.Changes.Selected)
	}
	updated, _ = m.checkboxClick(tg, 0)
	m = updated.(*Model)
	if state.SelectedIn(m.state, state.SectionUnstaged, "src/a.go") ||
		state.SelectedIn(m.state, state.SectionUnstaged, "src/b.go") {
		t.Fatalf("second click: selected = %v", m.state.Changes.Selected)
	}
}

func TestStashedMsgSyncsScrollWhenTheListShrinks(t *testing.T) {
	t.Parallel()
	m := &Model{
		state:  twentyStashState(19),
		render: layout.Renderer{},
		read:   &state.ReadState{Finished: map[string]bool{}, Blocks: map[string]bool{}},
	}
	m.syncScroll()

	twoStashes := make([]git.StashRow, 2)
	for i := range twoStashes {
		twoStashes[i] = git.StashRow{
			Ref:     fmt.Sprintf("stash@{%d}", i),
			SHA:     fmt.Sprintf("s%d", i),
			Message: fmt.Sprintf("stash-msg-%d", i),
			Branch:  "main",
			Age:     "1h",
			Status:  git.StashApplies,
		}
	}
	next, _ := m.Update(stashedMsg{stashes: twoStashes, head: m.state.Head})
	m = next.(*Model)

	f := layout.Pane(m.state, nil, m.render)
	for _, want := range []string{"stash-msg-0", "stash-msg-1"} {
		found := false
		for _, line := range f.Lines {
			if strings.Contains(line, want) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("stash row %q not visible after list shrank to 2 (ScrollTop=%d)", want, m.state.ScrollTop)
		}
	}
}

func TestSyncScrollFollowsTheCursorOnChangesAndHistory(t *testing.T) {
	t.Parallel()
	w := layout.Renderer{}
	read := &state.ReadState{Finished: map[string]bool{}, Blocks: map[string]bool{}}
	commits := make([]git.CommitInfo, 20)
	hrows := make([]state.Row, 20)
	for i := range commits {
		commits[i] = git.CommitInfo{
			SHA:      fmt.Sprintf("sha%d", i),
			ShortSHA: fmt.Sprintf("s%d", i),
			Subject:  fmt.Sprintf("commit-%d", i),
			Age:      "1m",
		}
		hrows[i] = state.CommitRow(commits[i])
	}
	for _, c := range []struct {
		name string
		s    state.State
		want string
	}{
		{"changes", twentyChangesState(19), "f19.txt"},
		{"history", state.State{Width: 77, Height: 10, Tab: state.TabHistory, Cursor: 19, Rows: hrows, History: state.History{Commits: commits}}, "commit-19"},
	} {
		m := &Model{state: c.s, render: w, read: read}
		m.syncScroll()
		f := layout.Pane(m.state, read, w)
		if !strings.Contains(f.Lines[len(f.Lines)-1], "?") {
			t.Fatalf("%s: footer missing from the last row", c.name)
		}
		found := false
		for _, line := range f.Lines {
			if strings.Contains(line, c.want) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("%s: cursor row %q not visible after syncScroll", c.name, c.want)
		}
	}
}

func TestSyncScrollFollowsTheCursorOnStashAndWorktree(t *testing.T) {
	t.Parallel()
	w := layout.Renderer{}
	for _, c := range []struct {
		name string
		s    state.State
		want string
	}{
		{"stashed", twentyStashState(19), "stash-msg-19"},
		{"worktrees", twentyWorktreeState(19), "wt19"},
	} {
		m := &Model{state: c.s, render: w, read: emptyRead()}
		m.syncScroll()
		f := layout.Pane(m.state, nil, w)
		if !strings.Contains(f.Lines[len(f.Lines)-1], "?") {
			t.Fatalf("%s: footer missing from the last row", c.name)
		}
		found := false
		for _, line := range f.Lines {
			if strings.Contains(line, c.want) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("%s: cursor row %q not visible after syncScroll", c.name, c.want)
		}
	}
}

func TestStashedPaneScrollsToTheCursorRow(t *testing.T) {
	t.Parallel()
	w := layout.Renderer{}
	m := &Model{state: twentyStashState(19), render: w}
	m.syncScroll()
	f := layout.Pane(m.state, nil, w)
	if !paneShowsRow(f, "stash-msg-19") {
		t.Fatal("the cursor row should be visible in the stashed pane")
	}
	if !paneFooterOnLastRow(f) {
		t.Fatal("the footer should sit on the last row")
	}
}

func TestWorktreePaneScrollsToTheCursorRow(t *testing.T) {
	t.Parallel()
	w := layout.Renderer{}
	m := &Model{state: twentyWorktreeState(19), render: w}
	m.syncScroll()
	f := layout.Pane(m.state, nil, w)
	if !paneShowsRow(f, "wt19") {
		t.Fatal("the cursor row should be visible in the worktrees pane")
	}
	if !paneFooterOnLastRow(f) {
		t.Fatal("the footer should sit on the last row")
	}
}

func paneShowsRow(f layout.Frame, want string) bool {
	for _, line := range f.Lines {
		if strings.Contains(line, want) {
			return true
		}
	}
	return false
}

func paneFooterOnLastRow(f layout.Frame) bool {
	if len(f.Lines) == 0 {
		return false
	}
	return strings.Contains(f.Lines[len(f.Lines)-1], "?")
}

func twentyChangesState(cursor int) state.State {
	rows := make([]state.Row, 20)
	entries := make([]git.Entry, 20)
	for i := range rows {
		path := fmt.Sprintf("f%02d.txt", i)
		rows[i] = state.FileRow(
			git.Entry{Path: path, Worktree: git.Modified},
			state.SectionUnstaged)

		entries[i] = git.Entry{Path: path, Worktree: git.Modified}
	}
	return state.State{Width: 77, Height: 10, Tab: state.TabChanges, Cursor: cursor, Rows: rows, Head: git.Head{Branch: "main"}, Changes: state.Changes{Entries: entries, Folded: map[string]bool{}, Selected: map[string]bool{}}}
}

func twentyStashState(cursor int) state.State {
	stashRows := make([]git.StashRow, 20)
	srows := make([]state.Row, 20)
	for i := range stashRows {
		stashRows[i] = git.StashRow{
			Ref:     fmt.Sprintf("stash@{%d}", i),
			SHA:     fmt.Sprintf("s%d", i),
			Message: fmt.Sprintf("stash-msg-%d", i),
			Branch:  "main",
			Age:     "1h",
			Status:  git.StashApplies,
		}
		srows[i] = state.StashRowOf(stashRows[i])
	}
	return state.State{Width: 77, Height: 10, Tab: state.TabStashed, Cursor: cursor, Rows: srows, Stashed: state.Stashed{Stashes: stashRows}}
}

func TestFoldAndReadKeysLeaveNonChangesTabsUnchanged(t *testing.T) {
	t.Parallel()
	commit := git.CommitInfo{SHA: "a", ShortSHA: "a", Subject: "one", Age: "1m"}
	fileEntry := git.Entry{Path: "a.txt", Worktree: git.Modified}
	stash := git.StashRow{
		Ref: "stash@{0}", SHA: "s", Message: "hold", Branch: "main",
		Age: "1h", Status: git.StashApplies,
	}
	wt := git.WorktreeRow{Path: "/repo", Name: "repo", Branch: "main", Main: true}

	historyWithFile := state.State{Tab: state.TabHistory, Cursor: 1, Rows: []state.Row{
		state.CommitRow(commit),
		state.FileRow(fileEntry, state.SectionNone),
	}, History: state.History{Commits: []git.CommitInfo{commit}, ExpandedSHA: "a", ExpandedFiles: []git.Entry{fileEntry}}}

	nonChanges := []struct {
		name string
		s    state.State
	}{
		{"history", historyWithFile},
		{"stashed", state.State{Tab: state.TabStashed, Cursor: 0, Rows: []state.Row{state.StashRowOf(stash)}, Stashed: state.Stashed{Stashes: []git.StashRow{stash}}}},
		{"worktrees", state.State{Tab: state.TabWorktrees, Cursor: 0, Rows: []state.Row{state.WorktreeRowOf(wt)}, Worktrees: state.Worktrees{Base: "main", List: []git.WorktreeRow{wt}}}},
	}
	for _, c := range nonChanges {
		t.Run(c.name+"/no directory rows", func(t *testing.T) {
			for _, row := range c.s.Rows {
				if row.Kind() == state.RowDirectory {
					t.Fatalf("RowDirectory on %s tab: %+v", c.name, row)
				}
			}
		})
	}

	foldedBefore := map[string]bool{"src": true}
	finishedBefore := map[string]bool{"a.txt": true}

	t.Run("history", func(t *testing.T) {
		s := historyWithFile
		s.Changes.Folded = copyBoolMap(foldedBefore)
		read := &state.ReadState{Finished: copyBoolMap(finishedBefore), Blocks: map[string]bool{}}
		m := &Model{state: s, read: read}
		keys := []tea.KeyPressMsg{
			{Code: tea.KeyLeft},
			{Code: tea.KeyRight},
			{Code: 'r'},
		}
		for _, k := range keys {
			next, cmd := m.key(k)
			if cmd != nil {
				t.Fatalf("key %v: cmd = %v, want nil", k, cmd)
			}
			m = next.(*Model)
		}
		if !mapsEqual(m.state.Changes.Folded, foldedBefore) {
			t.Errorf("Folded = %v, want %v", m.state.Changes.Folded, foldedBefore)
		}
		if !mapsEqual(m.read.Finished, finishedBefore) {
			t.Errorf("Finished = %v, want %v", m.read.Finished, finishedBefore)
		}
	})
}

func copyBoolMap(in map[string]bool) map[string]bool {
	out := make(map[string]bool, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func mapsEqual(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func twentyWorktreeState(cursor int) state.State {
	wtrees := make([]git.WorktreeRow, 20)
	wrows := make([]state.Row, 20)
	for i := range wtrees {
		wtrees[i] = git.WorktreeRow{
			Path:     "/repo/" + fmt.Sprintf("wt%d", i),
			Name:     fmt.Sprintf("wt%d", i),
			Branch:   "feat",
			SHA:      "abc123",
			Ahead:    2,
			WroteAge: "wrote 2m",
			Merge:    git.MergeClean,
		}
		wrows[i] = state.WorktreeRowOf(wtrees[i])
	}
	return state.State{Width: 77, Height: 10, Tab: state.TabWorktrees, Cursor: cursor, Rows: wrows, Worktrees: state.Worktrees{List: wtrees, Base: "main"}}
}

func TestClickingWorktreesLandsOnWorktreesWhenStashIsEmpty(t *testing.T) {
	t.Parallel()
	s := state.State{Width: 77, Height: 24, Tab: state.TabChanges, History: state.History{Commits: []git.CommitInfo{{Subject: "a"}}}, Worktrees: state.Worktrees{List: []git.WorktreeRow{{Path: "/main"}, {Path: "/wt"}}}}
	w := layout.Renderer{}
	f := layout.Pane(s, nil, w)

	var target layout.Target
	for _, r := range f.Regions {
		if r.Target.Kind == layout.TargetTab && r.Target.Name == "worktrees" {
			target = r.Target
			break
		}
	}
	if target.Kind != layout.TargetTab {
		t.Fatal("worktrees tab region not found")
	}
	if target.Tab != state.TabWorktrees {
		t.Fatalf("target.Tab = %v, want TabWorktrees", target.Tab)
	}

	m := &Model{
		state:  s,
		render: w,
		read:   &state.ReadState{Finished: map[string]bool{}, Blocks: map[string]bool{}},
	}
	updated, _ := m.click(target, 0)
	m = updated.(*Model)
	if m.state.Tab != state.TabWorktrees {
		t.Errorf("tab = %v, want TabWorktrees", m.state.Tab)
	}
}

// emptyRead は読んだ記録が空の ReadState を返す。&Model{} を直接組むと read が
// 型付き nil になり、layout が nil レシーバでメソッドを呼んで落ちる。New は
// LoadRead の値を必ず入れるので、本番にこの状態は無い。
func emptyRead() *state.ReadState {
	return &state.ReadState{Finished: map[string]bool{}, Blocks: map[string]bool{}}
}

// alwaysCommand is a clipboard that runs one program whatever it is handed.
// Handing it to the command under test rather than swapping a package variable
// is what lets these tests run beside each other: the detector caught the
// variable being written by one parallel test while another read it.
func alwaysCommand(name string) func(string) *exec.Cmd {
	return func(string) *exec.Cmd { return exec.Command(name) }
}
