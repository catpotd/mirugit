package tui

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/osproc"
	"github.com/catpotd/mirugit/internal/state"
)

// Finding the coordinate rather than writing one down stops a moved column from
// silently turning a click in a test into a miss.
func hitOf(m *Model, path string) (row, col int) {
	m.View()
	for _, r := range m.frame.Regions {
		if r.Target.Kind == layout.TargetFile && r.Target.Path == path {
			return r.Row, r.ColStart
		}
	}
	return -1, -1
}

func offCursorFileRow(m *Model) (screenRow int, path string, ok bool) {
	m.View()
	cursor := m.state.Cursor
	for _, r := range m.frame.Regions {
		if r.Target.Kind != layout.TargetFile || r.Target.Row < 0 || r.Target.Row == cursor {
			continue
		}
		if r.Target.Path == "" {
			continue
		}
		return r.Row, r.Target.Path, true
	}
	return 0, "", false
}

func initSyntheticRepo(t *testing.T, dir string) {
	t.Helper()
	env := []string{
		"HOME=" + dir,
		"GIT_CONFIG_GLOBAL=" + dir + "/.gitconfig",
		"GIT_CONFIG_SYSTEM=/dev/null",
		// git forks a repack of its own once a repository crosses a
		// threshold, and that child outlives the test: it races
		// t.TempDir's removal, and it takes the repository's locks
		// while another test is reading it.
		"GIT_CONFIG_COUNT=2",
		"GIT_CONFIG_KEY_0=gc.auto", "GIT_CONFIG_VALUE_0=0",
		"GIT_CONFIG_KEY_1=maintenance.auto", "GIT_CONFIG_VALUE_1=false",
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
	}
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"commit", "-q", "--allow-empty", "-m", "base"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func syntheticModel(t *testing.T) *Model {
	t.Helper()
	dir := t.TempDir()
	initSyntheticRepo(t, dir)
	return &Model{
		dir:      dir,
		startDir: dir,
		read:     &state.ReadState{Finished: map[string]bool{}, Blocks: map[string]bool{}},
		watch:    watchState{focused: true},
		render: layout.Renderer{
			Clipboard: osproc.ClipboardReady(),
			Browser:   osproc.BrowserReady(),
		},
		state: state.State{Changes: state.Changes{Folded: map[string]bool{}, Selected: map[string]bool{}, Stale: map[string]bool{}}},
	}
}

func historyModelForHitArea(t *testing.T) *Model {
	t.Helper()
	m := fixed(t)
	m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabHistory})
	m.state = state.Apply(m.state, state.HistoryLoaded{
		Commits: []git.CommitInfo{{SHA: "deadbeef", Subject: "base"}},
	})
	m.state = state.Apply(m.state, state.CommitExpanded{
		SHA:   "deadbeef",
		Files: []git.Entry{{Path: "app/a.swift", Worktree: git.Modified}},
	})
	m.state.Cursor = 0
	m.probe.settled = true
	return m
}

func stashedModelForHitArea(t *testing.T) *Model {
	t.Helper()
	stash0 := git.StashRow{
		Ref: "stash@{0}", SHA: "abc", Message: "one", Branch: "main",
		FileCount: 1, Age: "1h", Status: git.StashApplies,
	}
	stash1 := git.StashRow{
		Ref: "stash@{1}", SHA: "def", Message: "two", Branch: "main",
		FileCount: 1, Age: "2h", Status: git.StashApplies,
	}
	m := syntheticModel(t)
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 53})
	m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabStashed})
	m.state = state.Apply(m.state, state.StashedLoaded{Stashes: []git.StashRow{stash0, stash1}})
	m.state.Cursor = 0
	m.probe.settled = true
	return m
}

func worktreesModelForHitArea(t *testing.T) *Model {
	t.Helper()
	m := worktreeTabWithTwoRows(t)
	m.state.Cursor = 0
	return m
}

func copyState(s state.State) state.State {
	c := s
	c.Rows = slices.Clone(s.Rows)
	c.Changes.Entries = slices.Clone(s.Changes.Entries)
	c.History.Commits = slices.Clone(s.History.Commits)
	c.Stashed.Stashes = slices.Clone(s.Stashed.Stashes)
	c.Worktrees.List = slices.Clone(s.Worktrees.List)
	c.History.ExpandedFiles = slices.Clone(s.History.ExpandedFiles)
	c.Stashed.Files = slices.Clone(s.Stashed.Files)
	c.Worktrees.Files = slices.Clone(s.Worktrees.Files)
	if s.Changes.Folded != nil {
		c.Changes.Folded = maps.Clone(s.Changes.Folded)
	}
	if s.Changes.Selected != nil {
		c.Changes.Selected = maps.Clone(s.Changes.Selected)
	}
	if s.Changes.Stale != nil {
		c.Changes.Stale = maps.Clone(s.Changes.Stale)
	}
	return c
}

func assertClickArmsTheRowAcrossColumns(t *testing.T, setup func(t *testing.T) *Model, wantOpenPath string) {
	t.Helper()
	m := setup(t)
	m.View()
	screenRow, path, ok := offCursorFileRow(m)
	if !ok {
		t.Fatal("no off-cursor file row")
	}
	wantIndex := -1
	for _, r := range m.frame.Regions {
		if r.Row == screenRow && r.Target.Kind == layout.TargetFile && r.Target.Row >= 0 {
			wantIndex = r.Target.Row
			break
		}
	}
	if wantIndex < 0 {
		t.Fatalf("no file region on screen row %d", screenRow)
	}
	baseline := copyState(m.state)
	for col := 0; col < 77; col++ {
		region, _, _ := m.frame.Hit(screenRow, col)
		kind := region.Kind
		if kind != layout.TargetFile && kind != layout.TargetDirectory && kind != layout.TargetNone {
			continue
		}
		m.state = copyState(baseline)
		m.View()
		next, _ := m.Update(tea.MouseClickMsg{X: col, Y: screenRow, Button: tea.MouseLeft})
		clickM := next.(*Model)
		if clickM.state.Cursor != wantIndex {
			t.Errorf("col %d on %q: armed row %d, want %d",
				col, path, clickM.state.Cursor, wantIndex)
		}
		if wantOpenPath != "" && clickM.state.Open.Path != wantOpenPath {
			t.Errorf("col %d on %q: OpenPath %q, want %q",
				col, path, clickM.state.Open.Path, wantOpenPath)
		}
	}
}

func fixed(t *testing.T) *Model {
	t.Helper()
	m := syntheticModel(t)
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 53})
	m.state = state.Apply(m.state, state.StatusLoaded{
		Rows: []git.Entry{
			{Path: "app/a.swift", Worktree: git.Modified},
			{Path: "app/b.swift", Index: git.Modified},
		},
		Head: git.Head{Branch: "main"},
	})
	if i := state.FirstFileRow(m.state.Rows); i >= 0 {
		path := m.state.Rows[i].Path()
		m.state.Cursor = i
		m.state = state.Apply(m.state, state.DiffOpened{
			Path: path, Origin: state.WorkingTree(m.state.Rows[i].Section())})
	}
	m.probe.settled = true
	return m
}

func fixedBothSides(t *testing.T) *Model {
	t.Helper()
	m := syntheticModel(t)
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 53})
	m.state = state.Apply(m.state, state.StatusLoaded{
		Rows: []git.Entry{
			{Path: "app/a.swift", Worktree: git.Modified, Index: git.Modified},
		},
		Head: git.Head{Branch: "main"},
	})
	if i := state.FirstFileRow(m.state.Rows); i >= 0 {
		path := m.state.Rows[i].Path()
		m.state.Cursor = i
		m.state = state.Apply(m.state, state.DiffOpened{
			Path: path, Origin: state.WorkingTree(m.state.Rows[i].Section())})
	}
	m.probe.settled = true
	return m
}

// hasReloadCmd walks nested batches: a batch that holds another batch is what
// a reload is once it composes the tab reload rather than repeating it, and a
// walk one level deep reported the repo read as missing.
func hasReloadCmd(cmd tea.Cmd) bool {
	repo, diff := false, false
	walkCmds(cmd, func(msg tea.Msg) {
		switch msg.(type) {
		case repoMsg:
			repo = true
		case diffMsg:
			diff = true
		}
	})
	return repo && diff
}

// walkCmds runs cmd and every command its batches hold, handing each message
// that is not itself a batch to see.
func walkCmds(cmd tea.Cmd, see func(tea.Msg)) {
	if cmd == nil {
		return
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			walkCmds(c, see)
		}
		return
	}
	if msg != nil {
		see(msg)
	}
}

func hasDiffCmd(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	msg := cmd()
	switch v := msg.(type) {
	case diffMsg:
		return true
	case tea.BatchMsg:
		for _, c := range v {
			if c == nil {
				continue
			}
			if _, ok := c().(diffMsg); ok {
				return true
			}
		}
	}
	return false
}

func applyDiffCmd(t *testing.T, m *Model, cmd tea.Cmd) *Model {
	t.Helper()
	if cmd == nil {
		return m
	}
	msg := cmd()
	switch v := msg.(type) {
	case diffMsg:
		next, _ := m.Update(v)
		return next.(*Model)
	case tea.BatchMsg:
		for _, c := range v {
			if c == nil {
				continue
			}
			if dm, ok := c().(diffMsg); ok {
				next, _ := m.Update(dm)
				m = next.(*Model)
			}
		}
		return m
	default:
		t.Fatalf("want a diff command, got %T", msg)
		return m
	}
}

func bootDiff(t *testing.T, m *Model) *Model {
	t.Helper()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	m.state.Open.Path = ""
	m.state.Open.Diff = git.FileDiff{}
	_, cmd := m.Update(tea.WindowSizeMsg{Width: 77, Height: 53})
	return applyDiffCmd(t, m, cmd)
}

func applyReadCmd(t *testing.T, m *Model, cmd tea.Cmd) *Model {
	t.Helper()
	cmd = readCmdOf(t, cmd)
	msg := cmd()
	rm, ok := msg.(readMsg)
	if !ok {
		t.Fatalf("want readMsg, got %T", msg)
	}
	next, _ := m.Update(rm)
	return next.(*Model)
}

func readCmdOf(t *testing.T, cmd tea.Cmd) tea.Cmd {
	t.Helper()
	if cmd == nil {
		t.Fatal("want a read command")
	}
	msg := cmd()
	switch v := msg.(type) {
	case readMsg:
		return func() tea.Msg { return v }
	case tea.BatchMsg:
		for _, c := range v {
			if c == nil {
				continue
			}
			if _, ok := c().(readMsg); ok {
				return c
			}
		}
		t.Fatal("batch has no readMsg")
	default:
		t.Fatalf("want readMsg, got %T", msg)
	}
	return nil
}

func messagesFrom(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	switch v := msg.(type) {
	case tea.BatchMsg:
		var msgs []tea.Msg
		for _, c := range v {
			msgs = append(msgs, messagesFrom(c)...)
		}
		return msgs
	default:
		return []tea.Msg{msg}
	}
}

// applyOnce feeds back what cmd answers and stops there. applyCmds follows the
// commands those answers return, and a rebind answers with a watcher that waits
// for a filesystem event, so a test about the answer itself would wait with it.
// repoMsgFor builds the message loadRepo builds. The block hashes travel with
// the rest, so a test that hands the model a reload without them hands it a
// reload production never sends.
func repoMsgFor(t *testing.T, dir string, repo git.Repo) repoMsg {
	t.Helper()
	hashes, err := state.ReadBlockHashes(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	return repoMsg{Repo: repo, Hashes: hashes}
}

func applyOnce(t *testing.T, m *Model, cmd tea.Cmd) *Model {
	t.Helper()
	for _, msg := range messagesFrom(cmd) {
		next, _ := m.Update(msg)
		m = next.(*Model)
	}
	return m
}

func applyCmds(t *testing.T, m *Model, cmd tea.Cmd) *Model {
	t.Helper()
	queue := []tea.Cmd{}
	if cmd != nil {
		queue = append(queue, cmd)
	}
	for step := 0; len(queue) > 0 && step < 30; step++ {
		c := queue[0]
		queue = queue[1:]
		for _, msg := range messagesFrom(c) {
			var more tea.Cmd
			var next tea.Model
			next, more = m.Update(msg)
			m = next.(*Model)
			if more != nil {
				queue = append(queue, more)
			}
		}
	}
	return m
}

func fileLineOf(m *Model, path string) string {
	for _, line := range strings.Split(m.View().Content, "\n") {
		if strings.Contains(line, path) {
			return line
		}
	}
	return ""
}

func lastFileLineOf(m *Model, path string) string {
	var out string
	for _, line := range strings.Split(m.View().Content, "\n") {
		if strings.Contains(line, path) {
			out = line
		}
	}
	return out
}

func testRepo(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := t.TempDir()
	env := []string{
		"HOME=" + dir,
		"GIT_CONFIG_GLOBAL=" + dir + "/.gitconfig",
		"GIT_CONFIG_SYSTEM=/dev/null",
		// git forks a repack of its own once a repository crosses a
		// threshold, and that child outlives the test: it races
		// t.TempDir's removal, and it takes the repository's locks
		// while another test is reading it.
		"GIT_CONFIG_COUNT=2",
		"GIT_CONFIG_KEY_0=gc.auto", "GIT_CONFIG_VALUE_0=0",
		"GIT_CONFIG_KEY_1=maintenance.auto", "GIT_CONFIG_VALUE_1=false",
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
	}
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"commit", "-q", "--allow-empty", "-m", "base"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func TestReadKeyClearsTheUnreadMark(t *testing.T) {
	t.Parallel()
	dir := repoWithUnstagedChange(t)
	m, err := New(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.probe.settled = true
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 53})
	repo, err := git.Load(context.Background(), dir, mustGitDir(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	m.state = state.Apply(m.state, state.StatusLoaded{Rows: repo.Entries, Head: repo.Head})
	path := "a.txt"
	if line := fileLineOf(m, path); !strings.Contains(line, "·") {
		t.Fatalf("want unread mark before r: %q", line)
	}
	next, cmd := m.Update(tea.KeyPressMsg{Code: 'r'})
	m = applyReadCmd(t, next.(*Model), cmd)
	if line := fileLineOf(m, path); strings.Contains(line, "·") {
		t.Fatalf("unread mark still on the row: %q", line)
	}
}

func TestReadKeyMarksEveryBlockSoAChangeShowsChanged(t *testing.T) {
	t.Parallel()
	dir := repoWithUnstagedChange(t)
	m, err := New(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.probe.settled = true
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 53})
	repo, err := git.Load(context.Background(), dir, mustGitDir(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	m.state = state.Apply(m.state, state.StatusLoaded{Rows: repo.Entries, Head: repo.Head})
	path := "a.txt"
	next, cmd := m.Update(tea.KeyPressMsg{Code: 'r'})
	m = applyReadCmd(t, next.(*Model), cmd)
	if got := m.read.MarkIn(state.WorkingTree(state.SectionUnstaged), path, []string{"new"}); got != state.Changed {
		t.Errorf("after the file changed: got %v, want Changed", got)
	}
}

func sixBlockDiff() git.FileDiff {
	blocks := make([]git.Block, 6)
	for i := range blocks {
		blocks[i] = git.Block{
			Header: "@@ -1 +1 @@",
			Lines:  []string{"+line"},
			Added:  1,
			Hash:   fmt.Sprintf("h%d", i+1),
		}
	}
	return git.FileDiff{Path: "a.txt", Blocks: blocks}
}

func modelWithSixBlockDiff(t *testing.T, height int) *Model {
	t.Helper()
	m := syntheticModel(t)
	m.probe.settled = true
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: height})
	m.state = state.Apply(m.state, state.StatusLoaded{
		Rows: []git.Entry{{Path: "a.txt", Worktree: git.Modified}},
		Head: git.Head{Branch: "main"},
	})
	diff := sixBlockDiff()
	m.state = state.Apply(m.state, state.DiffOpened{Path: "a.txt", Origin: state.WorkingTree(state.SectionUnstaged)})
	m.state = state.Apply(m.state, state.DiffLoaded{Diff: diff})
	return m
}

func TestClosingAfterTwoBlocksLeavesTheFileUnread(t *testing.T) {
	t.Parallel()
	m := modelWithSixBlockDiff(t, 20)
	all := make([]string, 6)
	for i := range all {
		all[i] = fmt.Sprintf("h%d", i+1)
	}
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = next.(*Model)
	if got := m.read.MarkIn(state.WorkingTree(state.SectionUnstaged), "a.txt", all); got != state.Unread {
		t.Errorf("got %v, want Unread", got)
	}
}

func TestClosingAfterEveryBlockMarksTheFileRead(t *testing.T) {
	t.Parallel()
	m := modelWithSixBlockDiff(t, 53)
	all := make([]string, 6)
	for i := range all {
		all[i] = fmt.Sprintf("h%d", i+1)
	}
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = next.(*Model)
	if got := m.read.MarkIn(state.WorkingTree(state.SectionUnstaged), "a.txt", all); got != state.Read {
		t.Errorf("got %v, want Read", got)
	}
}

func TestReadKeyMarksEveryBlockWithoutOpening(t *testing.T) {
	t.Parallel()
	dir := repoWithUnstagedChange(t)
	m, err := New(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.probe.settled = true
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 53})
	repo, err := git.Load(context.Background(), dir, mustGitDir(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	next, _ := m.Update(repoMsgFor(t, dir, repo))
	m = next.(*Model)
	next, cmd := m.Update(tea.KeyPressMsg{Code: 'r'})
	m = applyReadCmd(t, next.(*Model), cmd)
	hashes := m.read.HashesIn(state.WorkingTree(state.SectionUnstaged), "a.txt")
	if len(hashes) == 0 {
		t.Fatal("want block hashes from git")
	}
	if got := m.read.MarkIn(state.WorkingTree(state.SectionUnstaged), "a.txt", hashes); got != state.Read {
		t.Errorf("got %v, want Read", got)
	}
}

func TestTheViewShowsTheHeader(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	lines := strings.Split(m.View().Content, "\n")
	if len(lines) < 4 {
		t.Fatalf("got %d lines, want at least 4 for the header", len(lines))
	}
	if !strings.Contains(lines[0], "changes") {
		t.Errorf("first line = %q, want the tab bar", lines[0])
	}
	if !strings.HasPrefix(lines[2], "[ ") {
		t.Errorf("third line = %q, want the commit box", lines[2])
	}
}

func TestEveryRenderedLineIsExactlyThePaneWidth(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	for i, line := range strings.Split(m.View().Content, "\n") {
		if got := m.render.Of(line); got != 77 {
			t.Errorf("line %d is %d cells: %q", i, got, line)
		}
	}
}

func TestTheViewFillsTheHeight(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	if n := len(strings.Split(m.View().Content, "\n")); n != 53 {
		t.Errorf("got %d lines, want 53", n)
	}
}

// View draws two things: the width probe until the answer arrives, and the pane
// after. Both hand the terminal the same three settings, and they have to: the
// terminal is set up from whichever view is on screen, so a probe that asks for
// less turns focus reporting or motion off for as long as it is up, and the
// watcher reads focus to decide whether to reload.
func TestTheViewAsksForMouseAndFocus(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name    string
		settled bool
	}{
		{"while the width probe is up", false},
		{"once the width is known", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := fixed(t)
			m.probe.settled = c.settled
			v := m.View()
			if v.MouseMode != tea.MouseModeCellMotion {
				t.Error("clicks and the wheel are what the pane reads, and a pointer that is only passing over is not")
			}
			if !v.ReportFocus {
				t.Error("the pane stops refreshing while unfocused")
			}
			if !v.AltScreen {
				t.Error("want the alternate screen")
			}
		})
	}
}

// A file row's checkbox carries no section, and a section heading's carries one.
// Reading a file's box as its heading's ticks every file under that heading
// instead of the one the reader aimed at.
func TestClickingAFileCheckboxTicksThatFileNotItsSection(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	m.View()
	screenRow := -1
	for _, r := range m.frame.Regions {
		if r.Target.Kind == layout.TargetFile && r.Target.Path == "app/b.swift" {
			screenRow = r.Row
			break
		}
	}
	if screenRow < 0 {
		t.Fatal("no region for app/b.swift")
	}
	baseline := copyState(m.state)
	for col := 1; col <= 3; col++ {
		m.state = copyState(baseline)
		m.View()
		next, _ := m.Update(tea.MouseClickMsg{X: col, Y: screenRow, Button: tea.MouseLeft})
		m = next.(*Model)
		if !state.SelectedIn(m.state, state.SectionStaged, "app/b.swift") {
			t.Errorf("col %d: app/b.swift is not ticked: %v", col, m.state.Changes.Selected)
		}
		if state.SelectionCount(m.state) != 1 {
			t.Errorf("col %d: ticked %d rows, want only app/b.swift: %v",
				col, state.SelectionCount(m.state), m.state.Changes.Selected)
		}
	}
}

// A file's name covers a handful of the seventy-seven cells, and the rest of the
// row is where the pointer usually is. A click anywhere on a row arms that row,
// which is the only way the cursor follows the mouse at all.
func TestClickArmsTheRowItLandsOnAcrossEveryColumn(t *testing.T) {
	t.Parallel()
	t.Run("changes", func(t *testing.T) {
		base := fixed(t)
		base.View()
		_, path, ok := offCursorFileRow(base)
		if !ok {
			t.Fatal("no off-cursor file row")
		}
		assertClickArmsTheRowAcrossColumns(t, fixed, path)
	})
	t.Run("history", func(t *testing.T) {
		assertClickArmsTheRowAcrossColumns(t, historyModelForHitArea, "")
	})
	t.Run("stashed", func(t *testing.T) {
		assertClickArmsTheRowAcrossColumns(t, stashedModelForHitArea, "")
	})
	t.Run("worktrees", func(t *testing.T) {
		assertClickArmsTheRowAcrossColumns(t, worktreesModelForHitArea, "")
	})
}

func TestClickingBlankOnADirectoryFoldsIt(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	var dirPath string
	var dirIndex int
	var screenRow int
	m.View()
	for _, r := range m.frame.Regions {
		if r.Target.Kind == layout.TargetDirectory {
			dirPath = r.Target.Path
			dirIndex = r.Target.Row
			screenRow = r.Row
			break
		}
	}
	if dirPath == "" {
		t.Fatal("no directory row")
	}
	blankCol := -1
	for col := 0; col < 77; col++ {
		region, _, _ := m.frame.Hit(screenRow, col)
		if region.Kind == layout.TargetNone {
			blankCol = col
			break
		}
	}
	if blankCol < 0 {
		t.Fatal("no blank column on the directory row")
	}
	next, _ := m.Update(tea.MouseClickMsg{X: blankCol, Y: screenRow, Button: tea.MouseLeft})
	m = next.(*Model)
	if !state.FoldedIn(m.state, m.state.Rows[dirIndex].Section(), dirPath) {
		t.Fatal("the directory should be folded")
	}
}

func hitVerb(m *Model, name state.VerbName) (row, col int) {
	m.View()
	for _, r := range m.frame.Regions {
		if r.Target.Kind == layout.TargetVerb && r.Target.Verb == name {
			return r.Row, r.ColStart
		}
	}
	return -1, -1
}

func regionOfCommit(m *Model, sha string) (row, col int) {
	m.View()
	for _, r := range m.frame.Regions {
		if r.Target.Kind == layout.TargetFile && r.Target.Commit == sha {
			return r.Row, r.ColStart
		}
	}
	return -1, -1
}

func countMsgType(cmd tea.Cmd, want string) int {
	n := 0
	for _, msg := range messagesFrom(cmd) {
		switch want {
		case "diffMsg":
			if _, ok := msg.(diffMsg); ok {
				n++
			}
		case "commitFilesMsg":
			if _, ok := msg.(commitFilesMsg); ok {
				n++
			}
		case "stashFilesMsg":
			if _, ok := msg.(stashFilesMsg); ok {
				n++
			}
		case "worktreeFilesMsg":
			if _, ok := msg.(worktreeFilesMsg); ok {
				n++
			}
		}
	}
	return n
}

func cursorIndexForPath(m *Model, path string) int {
	for i, row := range m.state.Rows {
		if row.Kind() == state.RowFile && row.Path() == path {
			return i
		}
	}
	return -1
}

func TestVerbClickDoesNotOpenADiff(t *testing.T) {
	t.Parallel()
	t.Run("changes", func(t *testing.T) {
		m := fixed(t)
		if i := cursorIndexForPath(m, "app/b.swift"); i < 0 {
			t.Fatal("no row for app/b.swift")
		} else {
			m.state.Cursor = i
		}
		m.state.Open.Path = ""
		m.View()
		row, col := hitVerb(m, state.VerbNameUnstage)
		if row < 0 {
			t.Fatal("no unstage verb region")
		}
		next, cmd := m.Update(tea.MouseClickMsg{X: col, Y: row, Button: tea.MouseLeft})
		m = next.(*Model)
		if m.state.Open.Path != "" {
			t.Errorf("OpenPath = %q, want empty", m.state.Open.Path)
		}
		if countMsgType(cmd, "diffMsg") != 0 {
			t.Error("verb click should not emit diffMsg")
		}
		if cmd == nil {
			t.Fatal("want a verb command")
		}
	})
	t.Run("history", func(t *testing.T) {
		m := fixed(t)
		m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabHistory})
		m.state = state.Apply(m.state, state.HistoryLoaded{
			Commits: []git.CommitInfo{
				{SHA: "aaa", Subject: "one"},
				{SHA: "bbb", Subject: "two"},
			},
		})
		m.state.Cursor = 0
		m.probe.settled = true
		m.View()
		row, col := hitVerb(m, state.VerbNameDiff)
		if row < 0 {
			t.Fatal("no diff verb region")
		}
		next, cmd := m.Update(tea.MouseClickMsg{X: col, Y: row, Button: tea.MouseLeft})
		m = next.(*Model)
		if m.state.Open.Path != "" {
			t.Errorf("OpenPath = %q, want empty", m.state.Open.Path)
		}
		if countMsgType(cmd, "diffMsg") != 0 {
			t.Error("verb click should not emit diffMsg")
		}
		if countMsgType(cmd, "commitFilesMsg") > 1 {
			t.Errorf("commitFilesMsg count = %d, want at most 1", countMsgType(cmd, "commitFilesMsg"))
		}
	})
	t.Run("stashed", func(t *testing.T) {
		m := stashedModelForHitArea(t)
		m.View()
		row, col := hitVerb(m, state.VerbNameRestore)
		if row < 0 {
			t.Fatal("no restore verb region")
		}
		next, cmd := m.Update(tea.MouseClickMsg{X: col, Y: row, Button: tea.MouseLeft})
		m = next.(*Model)
		if m.state.Open.Path != "" {
			t.Errorf("OpenPath = %q, want empty", m.state.Open.Path)
		}
		if countMsgType(cmd, "diffMsg") != 0 {
			t.Error("verb click should not emit diffMsg")
		}
	})
	t.Run("worktrees", func(t *testing.T) {
		m := worktreesModelForHitArea(t)
		m.state.Cursor = 1
		m.View()
		row, col := hitVerb(m, state.VerbNameGo)
		if row < 0 {
			t.Fatal("no go verb region")
		}
		next, cmd := m.Update(tea.MouseClickMsg{X: col, Y: row, Button: tea.MouseLeft})
		m = next.(*Model)
		if m.state.Open.Path != "" {
			t.Errorf("OpenPath = %q, want empty", m.state.Open.Path)
		}
		if countMsgType(cmd, "diffMsg") != 0 {
			t.Error("verb click should not emit diffMsg")
		}
	})
}

func TestRowClickEmitsOneOpenMessagePerTab(t *testing.T) {
	t.Parallel()
	t.Run("changes", func(t *testing.T) {
		m := fixed(t)
		row, col := hitOf(m, "app/b.swift")
		if row < 0 {
			t.Fatal("no region for app/b.swift")
		}
		next, cmd := m.Update(tea.MouseClickMsg{X: col, Y: row, Button: tea.MouseLeft})
		m = next.(*Model)
		if m.state.Open.Path != "app/b.swift" {
			t.Errorf("OpenPath = %q, want app/b.swift", m.state.Open.Path)
		}
		if countMsgType(cmd, "diffMsg") != 1 {
			t.Errorf("diffMsg count = %d, want 1", countMsgType(cmd, "diffMsg"))
		}
		if countMsgType(cmd, "commitFilesMsg") != 0 ||
			countMsgType(cmd, "stashFilesMsg") != 0 ||
			countMsgType(cmd, "worktreeFilesMsg") != 0 {
			t.Error("unexpected non-diff open message on changes tab")
		}
	})
	t.Run("history", func(t *testing.T) {
		m := fixed(t)
		m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabHistory})
		m.state = state.Apply(m.state, state.HistoryLoaded{
			Commits: []git.CommitInfo{
				{SHA: "aaa", Subject: "one"},
				{SHA: "bbb", Subject: "two"},
			},
		})
		m.state.Cursor = 0
		m.probe.settled = true
		row, col := regionOfCommit(m, "bbb")
		if row < 0 {
			t.Fatal("no region for second commit")
		}
		next, cmd := m.Update(tea.MouseClickMsg{X: col, Y: row, Button: tea.MouseLeft})
		m = next.(*Model)
		if m.state.Open.Path != "" {
			t.Errorf("OpenPath = %q, want empty", m.state.Open.Path)
		}
		if countMsgType(cmd, "commitFilesMsg") != 1 {
			t.Errorf("commitFilesMsg count = %d, want 1", countMsgType(cmd, "commitFilesMsg"))
		}
		if countMsgType(cmd, "diffMsg") != 0 {
			t.Error("commit row click should not emit diffMsg")
		}
	})
	t.Run("stashed", func(t *testing.T) {
		m := stashedModelForHitArea(t)
		row, col := hitOf(m, "stash@{1}")
		if row < 0 {
			t.Fatal("no region for stash@{1}")
		}
		next, cmd := m.Update(tea.MouseClickMsg{X: col, Y: row, Button: tea.MouseLeft})
		m = next.(*Model)
		if m.state.Open.Path != "" {
			t.Errorf("OpenPath = %q, want empty", m.state.Open.Path)
		}
		if countMsgType(cmd, "stashFilesMsg") != 1 {
			t.Errorf("stashFilesMsg count = %d, want 1", countMsgType(cmd, "stashFilesMsg"))
		}
		if countMsgType(cmd, "diffMsg") != 0 {
			t.Error("stash row click should not emit diffMsg")
		}
	})
	t.Run("worktrees", func(t *testing.T) {
		m := worktreesModelForHitArea(t)
		row, col := hitOf(m, "/repo/wt")
		if row < 0 {
			t.Fatal("no region for /repo/wt")
		}
		next, cmd := m.Update(tea.MouseClickMsg{X: col, Y: row, Button: tea.MouseLeft})
		m = next.(*Model)
		if m.state.Open.Path != "" {
			t.Errorf("OpenPath = %q, want empty", m.state.Open.Path)
		}
		if countMsgType(cmd, "worktreeFilesMsg") != 1 {
			t.Errorf("worktreeFilesMsg count = %d, want 1", countMsgType(cmd, "worktreeFilesMsg"))
		}
		if countMsgType(cmd, "diffMsg") != 0 {
			t.Error("worktree row click should not emit diffMsg")
		}
	})
}

func TestClickingAFileNameOpensItsDiff(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	row, col := hitOf(m, "app/a.swift")
	next, _ := m.Update(tea.MouseClickMsg{X: col, Y: row})
	m = next.(*Model)
	if m.state.Open.Path != "app/a.swift" {
		t.Errorf("got %q", m.state.Open.Path)
	}
}

func TestEscClearsSelectionWithoutClosingTheDiff(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	m.state = state.Apply(m.state, state.SelectionToggled{Path: "app/a.swift"})
	m.state = state.Apply(m.state, state.DiffOpened{Path: "app/b.swift", Origin: state.WorkingTree(state.SectionUnstaged)})
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = next.(*Model)
	if len(m.state.Changes.Selected) != 0 {
		t.Fatalf("selection should be cleared: %+v", m.state.Changes.Selected)
	}
	if m.state.Open.Path != "app/b.swift" {
		t.Errorf("diff was closed: got %q", m.state.Open.Path)
	}
}

func TestCtrlCQuitsFromAnywhere(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("want a quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("got %T, want tea.QuitMsg", cmd())
	}
}

func TestStageKeyRequestsTheVerb(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	for i, row := range m.state.Rows {
		if row.Kind() == state.RowFile && row.Section() == state.SectionUnstaged {
			m.state.Cursor = i
			break
		}
	}
	next, cmd := m.Update(tea.KeyPressMsg{Code: 's'})
	// Staging asks whether the diff on screen still matches the file first,
	// so what it stages is read after that answer is back.
	m = applyOnce(t, next.(*Model), cmd)
	if m.state.Changes.Pending == nil || m.state.Changes.Pending.Verb != state.VerbStage {
		t.Fatalf("got %+v", m.state.Changes.Pending)
	}
	if cmd == nil {
		t.Fatal("want a verb command")
	}
}

func TestDiscardKeyRequestsSnapshot(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	for i, row := range m.state.Rows {
		if row.Kind() == state.RowFile && row.Section() == state.SectionUnstaged {
			m.state.Cursor = i
			break
		}
	}
	next, cmd := m.Update(tea.KeyPressMsg{Code: 'x'})
	_ = next.(*Model)
	if cmd == nil {
		t.Fatal("want a snapshot command")
	}
}

func TestUndoKeyRunsUndiscard(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	undo := git.Undo{Ref: "refs/mirugit/undo/1", Commit: "abc"}
	m.state.LastUndo = &undo
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'u', Text: "U", Mod: tea.ModShift})
	if cmd == nil {
		t.Fatal("want an undiscard command")
	}
}

func TestUndoAsksWhenTheDiscardRecordedNoDigests(t *testing.T) {
	t.Parallel()
	// 記録が無い undo は「編集されていない」ではなく「分からない」である。
	// 素通しすると、確認なしで作業ツリーを上書きする
	m := fixed(t)
	m.state.LastUndo = &git.Undo{
		Ref: "refs/mirugit/undo/1", Commit: "abc", Paths: []string{"app/a.swift"},
	}
	next, cmd := m.Update(tea.KeyPressMsg{Code: 'u', Text: "U", Mod: tea.ModShift})
	// U asks which of the discarded paths have been written since. That answer
	// is what decides between asking and putting them back.
	m = applyOnce(t, next.(*Model), cmd)
	if m.state.Changes.UndiscardConfirm == nil {
		t.Fatal("undo should ask before overwriting")
	}
}

func TestStashKeyRequestsTheVerb(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	for i, row := range m.state.Rows {
		if row.Kind() == state.RowFile && row.Section() == state.SectionUnstaged {
			m.state.Cursor = i
			break
		}
	}
	next, cmd := m.Update(tea.KeyPressMsg{Code: 'z'})
	m = next.(*Model)
	if m.state.Changes.Pending == nil || m.state.Changes.Pending.Verb != state.VerbStash {
		t.Fatalf("got %+v", m.state.Changes.Pending)
	}
	if cmd == nil {
		t.Fatal("want a verb command")
	}
}

func TestUnstageKeyRequestsTheVerbOnStagedRow(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	for i, row := range m.state.Rows {
		if row.Kind() == state.RowFile && row.Section() == state.SectionStaged {
			m.state.Cursor = i
			break
		}
	}
	next, cmd := m.Update(tea.KeyPressMsg{Code: 'u'})
	m = next.(*Model)
	if m.state.Changes.Pending == nil || m.state.Changes.Pending.Verb != state.VerbUnstage {
		t.Fatalf("got %+v", m.state.Changes.Pending)
	}
	if cmd == nil {
		t.Fatal("want a verb command")
	}
}

// A terminal sends an uppercase letter as the lowercase code with the shift
// modifier set, so matching on the uppercase rune never fires.
func TestShiftedULooksLikeUndoToTheModel(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	if !isUndoKey(tea.KeyPressMsg{Code: 'u', Text: "U", Mod: tea.ModShift}) {
		t.Error("a shifted u was not recognized as undo")
	}
	if isUndoKey(tea.KeyPressMsg{Code: 'u', Text: "u"}) {
		t.Error("a plain u was taken for undo")
	}
	_ = m
}

func TestStageDoesNotRunWhileTypingACommitMessage(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	m.state = state.Apply(m.state, state.MessageFocused{})
	next, cmd := m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	m = next.(*Model)
	if m.state.Changes.Pending != nil {
		t.Fatalf("stage ran while typing: %+v", m.state.Changes.Pending)
	}
	if cmd != nil {
		t.Fatalf("want no command, got %T", cmd())
	}
	if m.state.Changes.Message != "s" {
		t.Errorf("got message %q", m.state.Changes.Message)
	}
}

func TestDWithNoRoomForADiffShowsANotice(t *testing.T) {
	t.Parallel()
	dir := testRepo(t)
	m, err := New(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.probe.settled = true
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 14})
	m.state = state.Apply(m.state, state.StatusLoaded{
		Rows: []git.Entry{{Path: "a.txt", Worktree: git.Modified}},
		Head: git.Head{Branch: "main"},
	})
	next, _ := m.Update(tea.KeyPressMsg{Code: 'j'})
	m = next.(*Model)
	next, cmd := m.Update(tea.KeyPressMsg{Code: 'd'})
	m = next.(*Model)
	if hasDiffCmd(cmd) {
		t.Fatal("want no diff command when the pane is too short")
	}
	if m.state.Open.Peek {
		t.Error("DiffPeek should stay false")
	}
	want := "no room for a diff · make the pane taller"
	if m.state.Notice != want {
		t.Errorf("got Notice %q, want %q", m.state.Notice, want)
	}
	m.View()
	found := false
	for _, line := range m.frame.Lines {
		if strings.Contains(line, "no room for a diff") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("notice should appear in the drawn frame")
	}
}

func TestDWithRoomForThePeekOpensSixRows(t *testing.T) {
	t.Parallel()
	dir := testRepo(t)
	m, err := New(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.probe.settled = true
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 16})
	m.state = state.Apply(m.state, state.StatusLoaded{
		Rows: []git.Entry{{Path: "a.txt", Worktree: git.Modified}},
		Head: git.Head{Branch: "main"},
	})
	next, _ := m.Update(tea.KeyPressMsg{Code: 'j'})
	m = next.(*Model)
	next, _ = m.Update(tea.KeyPressMsg{Code: 'd'})
	m = next.(*Model)
	if !m.state.Open.Peek {
		t.Error("DiffPeek should be true at height 16")
	}
	if m.state.Notice != "" {
		t.Errorf("got Notice %q, want empty", m.state.Notice)
	}
}

func TestEnterWithNothingStagedExplainsInsteadOfCommitting(t *testing.T) {
	t.Parallel()
	dir := testRepo(t)
	m, err := New(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 53})
	m.state = state.Apply(m.state, state.StatusLoaded{
		Rows: []git.Entry{
			{Path: "app/a.swift", Worktree: git.Modified},
		},
		Head: git.Head{Branch: "main"},
	})
	if i := state.FirstFileRow(m.state.Rows); i >= 0 {
		if path := m.state.Rows[i].Path(); path != "" {
			m.state.Cursor = i
			m.state = state.Apply(m.state, state.DiffOpened{
				Path: path, Origin: state.WorkingTree(m.state.Rows[i].Section())})
		}
	}
	m.probe.settled = true
	m.state = state.Apply(m.state, state.MessageFocused{})
	m.state = state.Apply(m.state, state.MessageEdited{Text: "wip"})
	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(*Model)
	if cmd != nil {
		t.Fatalf("want no commit command, got %T", cmd())
	}
	if m.state.Notice != "stage a file first" {
		t.Errorf("got Notice %q, want %q", m.state.Notice, "stage a file first")
	}
}

func TestCtrlCQuitsFromTheCommitMessageField(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	m.state = state.Apply(m.state, state.MessageFocused{})
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("want a quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("got %T, want tea.QuitMsg", cmd())
	}
}

func helpLineOf(m *Model, desc string) (row, col int) {
	m.View()
	for i, line := range m.frame.Lines {
		if strings.Contains(line, desc) {
			return i, 2
		}
	}
	return -1, -1
}

func footerHelpOf(m *Model) (row, col int) {
	m.View()
	for _, r := range m.frame.Regions {
		if r.Target.Kind == layout.TargetHelp {
			return r.Row, r.ColStart
		}
	}
	return -1, -1
}

func TestQuestionMarkOpensHelp(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	next, cmd := m.Update(tea.KeyPressMsg{Code: '?'})
	m = next.(*Model)
	if cmd != nil {
		t.Fatalf("want no command, got %T", cmd())
	}
	if !m.state.HelpOpen {
		t.Fatal("help should be open")
	}
}

func TestEscClosesHelp(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	m.state = state.Apply(m.state, state.HelpOpened{})
	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = next.(*Model)
	if cmd != nil {
		t.Fatalf("want no command, got %T", cmd())
	}
	if m.state.HelpOpen {
		t.Fatal("help should be closed")
	}
}

func TestHelpOpenScrollsWithJK(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 14})
	cursorBefore := m.state.Cursor

	next, cmd := m.Update(tea.KeyPressMsg{Code: '?'})
	m = next.(*Model)
	if cmd != nil {
		t.Fatalf("open help: want no command, got %T", cmd())
	}
	if !m.state.HelpOpen {
		t.Fatal("help should be open")
	}

	next, cmd = m.Update(tea.KeyPressMsg{Code: 'j'})
	m = next.(*Model)
	if cmd != nil {
		t.Fatalf("scroll down: want no command, got %T", cmd())
	}
	if m.state.HelpScroll != 1 {
		t.Errorf("HelpScroll = %d, want 1", m.state.HelpScroll)
	}
	if m.state.Cursor != cursorBefore {
		t.Errorf("Cursor moved from %d to %d while help is open", cursorBefore, m.state.Cursor)
	}

	next, cmd = m.Update(tea.KeyPressMsg{Code: 'k'})
	m = next.(*Model)
	if cmd != nil {
		t.Fatalf("scroll up: want no command, got %T", cmd())
	}
	if m.state.HelpScroll != 0 {
		t.Errorf("HelpScroll = %d, want 0", m.state.HelpScroll)
	}

	next, cmd = m.Update(tea.KeyPressMsg{Code: 'k'})
	m = next.(*Model)
	if cmd != nil {
		t.Fatalf("scroll up at top: want no command, got %T", cmd())
	}
	if m.state.HelpScroll != 0 {
		t.Errorf("HelpScroll = %d, want 0 at top", m.state.HelpScroll)
	}

	// 本番の式を期待値に使わない。changes は29行で、高さ12は内容10行なので
	// 最後まで送っても19で止まる。
	const wantMax = 19
	for range 50 {
		next, cmd = m.Update(tea.KeyPressMsg{Code: 'j'})
		m = next.(*Model)
		if cmd != nil {
			t.Fatalf("scroll down loop: want no command, got %T", cmd())
		}
	}
	if m.state.HelpScroll != wantMax {
		t.Errorf("HelpScroll = %d, want max %d", m.state.HelpScroll, wantMax)
	}

	next, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = next.(*Model)
	if cmd != nil {
		t.Fatalf("close help: want no command, got %T", cmd())
	}
	if m.state.HelpOpen {
		t.Fatal("help should be closed")
	}
}

func TestClickingStageInHelpRunsStageAndCloses(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	for i, row := range m.state.Rows {
		if row.Kind() == state.RowFile && row.Section() == state.SectionUnstaged {
			m.state.Cursor = i
			break
		}
	}
	m.state = state.Apply(m.state, state.HelpOpened{})
	row, col := helpLineOf(m, "stage")
	if row < 0 {
		t.Fatal("no stage line in help")
	}
	next, cmd := m.Update(tea.MouseClickMsg{X: col, Y: row})
	// Staging asks whether the diff on screen still matches the file first,
	// so what it stages is read after that answer is back.
	m = applyOnce(t, next.(*Model), cmd)
	if m.state.HelpOpen {
		t.Fatal("help should be closed after clicking stage")
	}
	if m.state.Changes.Pending == nil || m.state.Changes.Pending.Verb != state.VerbStage {
		t.Fatalf("got %+v", m.state.Changes.Pending)
	}
	if cmd == nil {
		t.Fatal("want a verb command")
	}
}

func TestClickingFoldInHelpFoldsTheDirectory(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	var dirIdx int
	var dirPath string
	for i, r := range m.state.Rows {
		if r.Kind() == state.RowDirectory && r.Section() == state.SectionUnstaged {
			dirIdx = i
			dirPath = r.DirPath()
			break
		}
	}
	if dirIdx < 0 {
		t.Fatal("no unstaged directory row")
	}
	m.state.Cursor = dirIdx
	m.state = state.Apply(m.state, state.HelpOpened{})
	row, col := helpLineOf(m, "fold the directory")
	if row < 0 {
		t.Fatal("no fold line in help")
	}
	next, cmd := m.Update(tea.MouseClickMsg{X: col, Y: row})
	m = next.(*Model)
	if cmd != nil {
		t.Fatalf("want no command, got %T", cmd())
	}
	if m.state.HelpOpen {
		t.Fatal("help should be closed after clicking fold")
	}
	if !state.FoldedIn(m.state, state.SectionUnstaged, dirPath) {
		t.Fatal("the directory should be folded")
	}
}

func TestClickingRestoreInHelpOnChangesTabOnlyCloses(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	m.state = state.Apply(m.state, state.HelpOpened{})
	row, _ := helpLineOf(m, "restore the stash to the working tree")
	if row >= 0 {
		t.Fatal("restore line should not appear on changes tab")
	}
}

func TestClickingDiscardInHelpOnStashedTabRequestsDrop(t *testing.T) {
	t.Parallel()
	stash := git.StashRow{
		Ref: "stash@{0}", SHA: "abc", Message: "hold", Branch: "main",
		FileCount: 1, Age: "1h", Status: git.StashApplies,
	}
	m, err := New(context.Background(), testRepo(t), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 53})
	m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabStashed})
	m.state = state.Apply(m.state, state.StashedLoaded{Stashes: []git.StashRow{stash}})
	m.state.Rows = []state.Row{state.StashRowOf(stash)}
	m.probe.settled = true
	m.state = state.Apply(m.state, state.HelpOpened{})
	row, col := helpLineOf(m, "drop the stash")
	if row < 0 {
		t.Fatal("no drop line in help")
	}
	next, cmd := m.Update(tea.MouseClickMsg{X: col, Y: row})
	m = next.(*Model)
	if runsGit(cmd) {
		t.Fatalf("want no git before confirm, got %T", cmd())
	}
	if m.state.HelpOpen {
		t.Fatal("help should be closed after clicking discard")
	}
	if m.state.Stashed.DropConfirm == nil {
		t.Fatal("drop should await confirmation")
	}
}

func TestClickingFooterHelpOpensHelp(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	row, col := footerHelpOf(m)
	if row < 0 {
		t.Fatal("no footer help region")
	}
	next, _ := m.Update(tea.MouseClickMsg{X: col, Y: row})
	m = next.(*Model)
	if !m.state.HelpOpen {
		t.Fatal("help should be open")
	}
}

// A discard awaiting confirmation has not run yet, so undo has nothing of its
// own to reverse; letting it through here restores the previous snapshot over
// a working tree the reader was about to change.
func TestUndoIsWithheldWhileADiscardAwaitsConfirmation(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	prior := git.Undo{Ref: "refs/mirugit/undo/1", Commit: "deadbeef"}
	m.state.LastUndo = &prior
	m.state.Changes.DiscardConfirm = &state.DiscardConfirm{Files: 3, Added: 142, Deleted: 4}

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'u', Text: "U", Mod: tea.ModShift})
	if cmd != nil {
		t.Error("undo ran while a discard was still awaiting confirmation")
	}
	if m.state.Changes.DiscardConfirm != nil {
		t.Error("the confirmation was left standing")
	}
}

func TestAnyOtherKeyDuringADiscardConfirmationLeavesANotice(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	m.state.Changes.DiscardConfirm = &state.DiscardConfirm{Files: 1, Added: 1, Deleted: 1}
	cursor := m.state.Cursor

	next, cmd := m.Update(tea.KeyPressMsg{Code: 'j'})
	m = next.(*Model)
	if cmd != nil {
		t.Fatalf("want no command, got %T", cmd())
	}
	if m.state.Changes.DiscardConfirm != nil {
		t.Fatal("the confirmation should be cleared")
	}
	if m.state.Cursor != cursor {
		t.Errorf("cursor moved from %d to %d", cursor, m.state.Cursor)
	}
	if m.state.Notice != "canceled · nothing discarded" {
		t.Errorf("got notice %q", m.state.Notice)
	}
}

func TestHoverIsWithheldWhileADiscardAwaitsConfirmation(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	cursor := m.state.Cursor
	m.state.Changes.DiscardConfirm = &state.DiscardConfirm{Files: 1, Added: 1, Deleted: 1}
	// カーソルと同じ行を指すと、遮断が無くてもカーソルは動かない
	other := ""
	for _, r := range m.state.Rows {
		if r.Kind() == state.RowFile && r.Path() != m.state.Rows[cursor].Path() {
			other = r.Path()
			break
		}
	}
	if other == "" {
		t.Fatal("the fixture needs a second file row")
	}
	row, col := hitOf(m, other)
	if row < 0 {
		t.Fatalf("no region for %s", other)
	}
	next, cmd := m.Update(tea.MouseMotionMsg{X: col, Y: row})
	m = next.(*Model)
	if cmd != nil {
		t.Fatalf("want no command, got %T", cmd())
	}
	if m.state.Cursor != cursor {
		t.Errorf("cursor moved from %d to %d during discard confirmation", cursor, m.state.Cursor)
	}
	if m.state.Changes.DiscardConfirm == nil {
		t.Fatal("discard confirmation should stay open")
	}
}

func TestAClickDuringADiscardConfirmationCancelsIt(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	for i, row := range m.state.Rows {
		if row.Path() == "app/a.swift" {
			m.state.Cursor = i
			m.state = state.Apply(m.state, state.DiffOpened{
				Path: "app/a.swift", Origin: state.WorkingTree(row.Section())})
			break
		}
	}
	m.state.Changes.DiscardConfirm = &state.DiscardConfirm{Files: 1, Added: 1, Deleted: 1}
	row, col := hitOf(m, "app/b.swift")
	if row < 0 {
		t.Fatal("no region for app/b.swift")
	}
	next, cmd := m.Update(tea.MouseClickMsg{X: col, Y: row, Button: tea.MouseLeft})
	m = next.(*Model)
	if cmd != nil {
		t.Fatalf("want no command, got %T", cmd())
	}
	if m.state.Changes.DiscardConfirm != nil {
		t.Fatal("discard confirmation should be canceled")
	}
	if m.state.Open.Path != "app/a.swift" {
		t.Errorf("OpenPath = %q, want app/a.swift", m.state.Open.Path)
	}
}

func TestAClickDuringAStashDropConfirmationCancelsIt(t *testing.T) {
	t.Parallel()
	stash0 := git.StashRow{
		Ref: "stash@{0}", SHA: "abc", Message: "first", Branch: "main",
		FileCount: 1, Age: "1h", Status: git.StashApplies,
	}
	stash1 := git.StashRow{
		Ref: "stash@{1}", SHA: "def", Message: "second", Branch: "main",
		FileCount: 1, Age: "2h", Status: git.StashApplies,
	}
	m, err := New(context.Background(), testRepo(t), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 53})
	m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabStashed})
	m.state = state.Apply(m.state, state.StashedLoaded{Stashes: []git.StashRow{stash0, stash1}})
	m.state.Rows = []state.Row{
		state.StashRowOf(stash0),
		state.StashRowOf(stash1),
	}
	m.probe.settled = true

	next, cmd := m.Update(tea.KeyPressMsg{Code: 'x'})
	m = next.(*Model)
	if runsGit(cmd) {
		t.Fatalf("want no git before confirm, got %T", cmd())
	}
	if m.state.Stashed.DropConfirm == nil {
		t.Fatal("drop should await confirmation")
	}
	if strings.Contains(m.View().Content, "restore") {
		t.Fatal("row verbs should be hidden during confirmation")
	}

	m.View()
	row, col := -1, -1
	for _, r := range m.frame.Regions {
		if r.Target.Kind == layout.TargetFile && r.Target.Path == "stash@{1}" {
			row, col = r.Row, r.ColStart
			break
		}
	}
	if row < 0 {
		t.Fatal("no region for stash@{1}")
	}
	next, _ = m.Update(tea.MouseClickMsg{X: col, Y: row, Button: tea.MouseLeft})
	m = next.(*Model)
	if m.state.Stashed.DropConfirm != nil {
		t.Fatal("stash drop confirmation should be canceled")
	}
}

func TestUndoIsWithheldWhileHelpIsOpen(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	m.state.LastUndo = &git.Undo{Ref: "refs/mirugit/undo/1", Commit: "abc"}
	m.state = state.Apply(m.state, state.HelpOpened{})

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'u', Text: "U", Mod: tea.ModShift})
	if cmd != nil {
		t.Error("undo ran while help was open")
	}
	if !m.state.HelpOpen {
		t.Error("help should still be open")
	}
}

func TestReloadIsWithheldWhileHelpIsOpen(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	m.state = state.Apply(m.state, state.HelpOpened{})

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'r', Mod: tea.ModShift})
	if cmd != nil {
		t.Error("reload ran while help was open")
	}
	if !m.state.HelpOpen {
		t.Error("help should still be open")
	}
}

func TestHistoryUndoIsWithheldWhileHelpIsOpen(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabHistory})
	m.state = state.Apply(m.state, state.HistoryLoaded{
		Commits: []git.CommitInfo{{SHA: "a", Subject: "one", Unpushed: true}},
	})
	m.state.Cursor = 0
	m.state = state.Apply(m.state, state.HelpOpened{})

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'u', Text: "u"})
	if cmd != nil {
		t.Error("history undo ran while help was open")
	}
	if !m.state.HelpOpen {
		t.Error("help should still be open")
	}
}

func TestCtrlCQuitsWhileHelpIsOpen(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	m.state = state.Apply(m.state, state.HelpOpened{})

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("want a quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("got %T, want tea.QuitMsg", cmd())
	}
}

func TestReloadIsWithheldWhileADiscardAwaitsConfirmation(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	m.state.Changes.DiscardConfirm = &state.DiscardConfirm{Files: 1, Added: 1, Deleted: 1}

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'r', Text: "R", Mod: tea.ModShift})
	if cmd != nil {
		t.Error("reload ran while a discard was still awaiting confirmation")
	}
	if m.state.Changes.DiscardConfirm != nil {
		t.Error("the confirmation was left standing")
	}
}

func TestReloadIsWithheldWhileAStashDropAwaitsConfirmation(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	m.state.Stashed.DropConfirm = &state.StashDropConfirm{SHA: "abc", Message: "hold"}

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'r', Text: "R", Mod: tea.ModShift})
	if cmd != nil {
		t.Error("reload ran while a stash drop was still awaiting confirmation")
	}
	if m.state.Stashed.DropConfirm != nil {
		t.Error("the confirmation was left standing")
	}
}

func TestYRunsNoDiscardWhenTheTargetsAreGone(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	m.state.Changes.DiscardConfirm = &state.DiscardConfirm{
		Targets: []string{"gone.txt"},
		Files:   1,
		Added:   1,
		Deleted: 1,
	}

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'y'})
	if cmd != nil {
		t.Error("discard ran when the targets were gone")
	}
	if m.state.Changes.DiscardConfirm != nil {
		t.Error("the confirmation was left standing")
	}
	if m.state.LastUndo != nil {
		t.Error("undo was registered for a zero-file discard")
	}
	if m.state.Notice != "nothing to discard · the changes are gone" {
		t.Errorf("got notice %q", m.state.Notice)
	}
}

func TestShiftRReloadsStatusAndDiff(t *testing.T) {
	t.Parallel()
	m := bootDiff(t, fixed(t))
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'r', Mod: tea.ModShift})
	if !hasReloadCmd(cmd) {
		t.Fatal("shift+r should reload status and the open diff")
	}
}

func TestJToAnotherFileReloadsTheDiff(t *testing.T) {
	t.Parallel()
	m := bootDiff(t, fixed(t))
	start := m.cursorPath()
	for range len(m.state.Rows) {
		next, cmd := m.Update(tea.KeyPressMsg{Code: 'j'})
		m = next.(*Model)
		path := m.cursorPath()
		if path != "" && path != start {
			if !hasDiffCmd(cmd) {
				t.Fatal("moving to another file should reload the diff")
			}
			return
		}
	}
	t.Fatal("never reached another file")
}

// The two rows of one path hold different diffs, so arriving at the second one
// must load it. Skipping the load on a matching path left the reader looking at
// the other side's content while the action bar offered this side's verbs.
func TestCursorMoveToTheOtherSectionRowReloadsTheDiff(t *testing.T) {
	t.Parallel()
	m := bootDiff(t, fixedBothSides(t))
	start := state.FirstFileRow(m.state.Rows)
	m.state.Cursor = start
	path := m.cursorPath()
	for {
		next, cmd := m.Update(tea.KeyPressMsg{Code: 'j'})
		m = next.(*Model)
		if m.cursorPath() == path && m.state.Cursor != start {
			if !hasDiffCmd(cmd) {
				t.Fatal("the other section row for the same file did not reload")
			}
			return
		}
		if m.state.Cursor >= len(m.state.Rows)-1 {
			t.Fatal("never reached the other section row for the same file")
		}
	}
}

func TestEscDoesNotReopenTheDiffUntilTheCursorMoves(t *testing.T) {
	t.Parallel()
	m := bootDiff(t, fixed(t))
	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = next.(*Model)
	if hasDiffCmd(cmd) {
		t.Fatal("esc should close without reloading")
	}
	if m.state.Open.Path != "" {
		t.Fatalf("diff should be closed, got %q", m.state.Open.Path)
	}
	_, cmd = m.Update(tea.WindowSizeMsg{Width: 77, Height: 53})
	if hasDiffCmd(cmd) {
		t.Fatal("the diff should stay closed until the cursor moves")
	}
}

func bigChangesModel(t *testing.T) *Model {
	t.Helper()
	m := syntheticModel(t)
	rows := make([]state.Row, 90)
	entries := make([]git.Entry, 90)
	for i := range rows {
		path := fmt.Sprintf("f%02d.txt", i)
		rows[i] = state.FileRow(
			git.Entry{Path: path, Worktree: git.Modified},
			state.SectionUnstaged)

		entries[i] = git.Entry{Path: path, Worktree: git.Modified}
	}
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 53})
	m.state = state.Apply(m.state, state.StatusLoaded{
		Rows: entries, Head: git.Head{Branch: "main"},
	})
	m.state.Rows = rows
	m.state = state.Apply(m.state, state.DiffOpened{Path: "f00.txt", Origin: state.WorkingTree(state.SectionUnstaged)})
	m.probe.settled = true
	return m
}

func TestDAfterEscReopensTheDiffAtSixRows(t *testing.T) {
	t.Parallel()
	m := bootDiff(t, bigChangesModel(t))
	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = next.(*Model)
	if hasDiffCmd(cmd) {
		t.Fatal("esc should close without reloading")
	}
	next, cmd = m.Update(tea.KeyPressMsg{Code: 'd'})
	m = next.(*Model)
	if !m.state.Open.Peek {
		t.Fatal("d after esc should peek the diff")
	}
	m = applyDiffCmd(t, m, cmd)
	m.syncScroll()
	m.View()

	var header bool
	diffContent := 0
	inDiff := false
	for _, line := range m.frame.Lines {
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
		t.Fatal("d should reopen the diff header")
	}
	if diffContent != 6 {
		t.Fatalf("peek diff body = %d lines, want 6", diffContent)
	}
	found := false
	for _, line := range m.frame.Lines {
		if strings.Contains(line, "f00.txt") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("the cursor row should stay visible after peek")
	}
}

func syncTestEnv(base string) []string {
	return []string{
		"HOME=" + base,
		"GIT_CONFIG_GLOBAL=" + base + "/.gitconfig",
		"GIT_CONFIG_SYSTEM=/dev/null",
		// git forks a repack of its own once a repository crosses a
		// threshold, and that child outlives the test: it races
		// t.TempDir's removal, and it takes the repository's locks
		// while another test is reading it.
		"GIT_CONFIG_COUNT=2",
		"GIT_CONFIG_KEY_0=gc.auto", "GIT_CONFIG_VALUE_0=0",
		"GIT_CONFIG_KEY_1=maintenance.auto", "GIT_CONFIG_VALUE_1=false",
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
	}
}

func syncTestGit(t *testing.T, env []string, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = slices.Concat(env, os.Environ())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
}

func newDivergedSyncRepo(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	base := t.TempDir()
	env := syncTestEnv(base)
	remoteDir := base + "/remote.git"
	workDir := base + "/work"
	peerDir := base + "/work2"
	syncTestGit(t, env, base, "init", "--bare", "-q", "-b", "main", remoteDir)
	syncTestGit(t, env, base, "clone", "-q", remoteDir, "work")
	syncTestGit(t, env, workDir, "commit", "-q", "--allow-empty", "-m", "base")
	syncTestGit(t, env, workDir, "push", "-q", "-u", "origin", "main")
	syncTestGit(t, env, base, "clone", "-q", remoteDir, "work2")
	syncTestGit(t, env, workDir, "commit", "-q", "--allow-empty", "-m", "local")
	syncTestGit(t, env, peerDir, "commit", "-q", "--allow-empty", "-m", "remote")
	syncTestGit(t, env, peerDir, "push", "-q")
	if err := git.Fetch(context.Background(), workDir); err != nil {
		t.Fatal(err)
	}
	return workDir
}

func summaryLineOf(m *Model) string {
	m.View()
	if len(m.frame.Lines) < 4 {
		return ""
	}
	return m.frame.Lines[3]
}

func TestBlockStageIsRefusedWhileTheDiffIsStale(t *testing.T) {
	t.Parallel()
	dir := repoWithUnstagedChange(t)
	m, err := New(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.probe.settled = true
	m.state.Width = 77
	m.state.Height = 53

	repo, err := git.Load(context.Background(), dir, mustGitDir(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	next, cmd := m.Update(repoMsgFor(t, dir, repo))
	m = next.(*Model)
	m = applyDiffCmd(t, m, cmd)

	if err := os.WriteFile(dir+"/a.txt", []byte("brand new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m = applyOnce(t, m, m.checkStaleFirst(noBlockYet))
	if !m.state.Changes.Stale["a.txt"] {
		t.Fatal("the open diff should be stale after the file changed")
	}

	next, cmd = m.Update(tea.KeyPressMsg{Code: 's'})
	// s asks whether the diff on screen still matches the file before it
	// stages. The answer is what refuses, so the refusal is read after it.
	m = applyOnce(t, next.(*Model), cmd)
	if m.state.Changes.Pending != nil {
		t.Fatalf("%s should be refused: %+v", "block stage", m.state.Changes.Pending)
	}
	if !m.state.Changes.Stale[m.state.Open.Path] {
		t.Errorf("the row stopped saying it is stale: %v", m.state.Changes.Stale)
	}
}

func TestFetchKeyRunsFetch(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	env := syncTestEnv(base)
	remoteDir := base + "/remote.git"
	workDir := base + "/work"
	peerDir := base + "/work2"
	syncTestGit(t, env, base, "init", "--bare", "-q", "-b", "main", remoteDir)
	syncTestGit(t, env, base, "clone", "-q", remoteDir, "work")
	syncTestGit(t, env, workDir, "commit", "-q", "--allow-empty", "-m", "base")
	syncTestGit(t, env, workDir, "push", "-q", "-u", "origin", "main")
	syncTestGit(t, env, base, "clone", "-q", remoteDir, "work2")
	syncTestGit(t, env, workDir, "commit", "-q", "--allow-empty", "-m", "local")
	syncTestGit(t, env, peerDir, "commit", "-q", "--allow-empty", "-m", "remote")
	syncTestGit(t, env, peerDir, "push", "-q")

	m, err := New(context.Background(), workDir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo, err := git.Load(context.Background(), workDir, mustGitDir(t, workDir))
	if err != nil {
		t.Fatal(err)
	}
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 53})
	m.state = state.Apply(m.state, state.StatusLoaded{
		Rows: repo.Entries, Head: repo.Head,
	})
	m.probe.settled = true

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'f', Text: "f"})
	if cmd == nil {
		t.Fatal("want a fetch command")
	}
	msg := cmd()
	vm, ok := msg.(verbMsg)
	if !ok {
		t.Fatalf("want verbMsg, got %T", msg)
	}
	if vm.err != nil {
		t.Fatalf("fetch failed: %v", vm.err)
	}

	originMain := syncTestGitOutput(t, env, workDir, "rev-parse", "origin/main")
	remoteMain := syncTestGitOutput(t, env, remoteDir, "rev-parse", "main")
	if originMain != remoteMain {
		t.Fatalf("origin/main = %q, remote main = %q", originMain, remoteMain)
	}
}

func syncTestGitOutput(t *testing.T, env []string, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = slices.Concat(env, os.Environ())
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v in %s: %v", args, dir, err)
	}
	return strings.TrimSpace(string(out))
}

func TestSyncRejectionShowsInTheNoticeLine(t *testing.T) {
	t.Parallel()
	workDir := newDivergedSyncRepo(t)
	m, err := New(context.Background(), workDir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo, err := git.Load(context.Background(), workDir, mustGitDir(t, workDir))
	if err != nil {
		t.Fatal(err)
	}
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 53})
	m.state = state.Apply(m.state, state.StatusLoaded{
		Rows: repo.Entries, Head: repo.Head,
	})
	m.probe.settled = true

	next, cmd := m.requestSync()
	m = next.(*Model)
	if cmd == nil {
		t.Fatal("want a sync command")
	}
	next, _ = m.Update(cmd())
	m = next.(*Model)

	want := "pull would merge · resolve in your terminal"
	if m.state.Notice != want {
		t.Fatalf("got notice %q, want %q", m.state.Notice, want)
	}
	summary := summaryLineOf(m)
	if strings.Contains(summary, want) {
		t.Errorf("notice should not replace the summary line: %q", summary)
	}
	found := false
	for _, line := range m.frame.Lines {
		if strings.Contains(line, want) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("notice missing from pane; summary = %q", summary)
	}
}

func TestSyncKeyRunsSync(t *testing.T) {
	t.Parallel()
	workDir := newDivergedSyncRepo(t)
	m, err := New(context.Background(), workDir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo, err := git.Load(context.Background(), workDir, mustGitDir(t, workDir))
	if err != nil {
		t.Fatal(err)
	}
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 53})
	m.state = state.Apply(m.state, state.StatusLoaded{
		Rows: repo.Entries, Head: repo.Head,
	})
	m.probe.settled = true

	next, cmd := m.Update(tea.KeyPressMsg{Code: 's', Text: "S", Mod: tea.ModShift})
	m = next.(*Model)
	if cmd == nil {
		t.Fatal("want a sync command")
	}
	msg := cmd()
	vm, ok := msg.(verbMsg)
	if !ok {
		t.Fatalf("want verbMsg, got %T", msg)
	}
	if vm.err != nil {
		t.Fatalf("sync failed: %v", vm.err)
	}

	next, _ = m.Update(msg)
	m = next.(*Model)

	want := "pull would merge · resolve in your terminal"
	if m.state.Notice != want {
		t.Fatalf("got notice %q, want %q", m.state.Notice, want)
	}
}

func TestSyncKeyDoesNothingWithoutUpstream(t *testing.T) {
	t.Parallel()
	dir := testRepo(t)
	m, err := New(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo, err := git.Load(context.Background(), dir, mustGitDir(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	if repo.Head.HasUpstream {
		t.Fatal("test repo should have no upstream")
	}
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 53})
	m.state = state.Apply(m.state, state.StatusLoaded{
		Rows: repo.Entries, Head: repo.Head,
	})
	m.state.Head = git.Head{Branch: repo.Head.Branch, Ahead: 2, Behind: 1}
	m.probe.settled = true

	line := summaryLineOf(m)
	if strings.Contains(line, "[sync") || strings.Contains(line, "[pull") || strings.Contains(line, "[push") {
		t.Errorf("no upstream: %q", line)
	}

	next, cmd := m.Update(tea.KeyPressMsg{Code: 's', Text: "S", Mod: tea.ModShift})
	m = next.(*Model)
	if cmd != nil {
		t.Fatalf("want no command, got %T", cmd())
	}
	if m.state.Changes.Pending != nil {
		t.Fatalf("sync should not run: %+v", m.state.Changes.Pending)
	}
}

func TestSyncKeyDoesNothingWithoutAheadOrBehind(t *testing.T) {
	t.Parallel()
	dir := repoWithUnstagedChange(t)
	m, err := New(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo, err := git.Load(context.Background(), dir, mustGitDir(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 53})
	m.state = state.Apply(m.state, state.StatusLoaded{
		Rows: repo.Entries, Head: repo.Head,
	})
	if i := state.FirstFileRow(m.state.Rows); i >= 0 {
		if path := m.state.Rows[i].Path(); path != "" {
			m.state.Cursor = i
			m.state = state.Apply(m.state, state.DiffOpened{
				Path: path, Origin: state.WorkingTree(m.state.Rows[i].Section())})
		}
	}
	m.probe.settled = true

	next, cmd := m.Update(tea.KeyPressMsg{Code: 's', Text: "S", Mod: tea.ModShift})
	m = next.(*Model)
	if cmd != nil {
		t.Fatalf("want no command, got %T", cmd())
	}
	if m.state.Changes.Pending != nil {
		t.Fatalf("sync should not run: %+v", m.state.Changes.Pending)
	}

	_, cmd = m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	if cmd == nil {
		t.Fatal("want a stage command")
	}
}

// The design's key table lists tab / 1-4 alongside the click, under the rule
// that no operation exists for only one of the two.
func TestDropKeyRequestsConfirmationOnTheStashedTab(t *testing.T) {
	t.Parallel()
	stash := git.StashRow{
		Ref: "stash@{0}", SHA: "abc", Message: "hold", Branch: "main",
		FileCount: 1, Age: "1h", Status: git.StashApplies,
	}
	m, err := New(context.Background(), testRepo(t), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 53})
	m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabStashed})
	m.state = state.Apply(m.state, state.StashedLoaded{Stashes: []git.StashRow{stash}})
	m.state.Rows = []state.Row{state.StashRowOf(stash)}
	m.probe.settled = true

	next, cmd := m.Update(tea.KeyPressMsg{Code: 'x'})
	m = next.(*Model)
	if runsGit(cmd) {
		t.Fatalf("want no git before confirm, got %T", cmd())
	}
	if m.state.Stashed.DropConfirm == nil {
		t.Fatal("drop should await confirmation")
	}
	if !strings.Contains(m.View().Content, "stash cannot be undone") {
		t.Fatal("confirmation should say stash cannot be undone")
	}
}

func TestDropKeyOnUnknownStashDoesNothing(t *testing.T) {
	t.Parallel()
	stash := git.StashRow{
		Ref: "stash@{0}", SHA: "abc", Message: "hold", Branch: "main",
		FileCount: 1, Age: "1h", Status: git.StashUnknown,
	}
	m, err := New(context.Background(), testRepo(t), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 53})
	m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabStashed})
	m.state = state.Apply(m.state, state.StashedLoaded{Stashes: []git.StashRow{stash}})
	m.state.Rows = []state.Row{state.StashRowOf(stash)}
	m.probe.settled = true
	m.asked.expandRow = stash.Ref

	next, _ := m.Update(tea.KeyPressMsg{Code: 'x'})
	m = next.(*Model)
	if m.state.Stashed.DropConfirm != nil {
		t.Fatal("drop should not open while stash status is unknown")
	}
}

func TestHelpNextTabMatchesKeyboardTab(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	m.state = state.Apply(m.state, state.HistoryLoaded{
		Commits: []git.CommitInfo{{SHA: "a", Subject: "one"}},
	})
	m.state = state.Apply(m.state, state.WorktreesLoaded{
		Worktrees: []git.WorktreeRow{
			{Path: m.dir, Name: "repo", Branch: "main", Main: true, SHA: "abc"},
			{Path: m.dir + "/wt", Name: "wt", Branch: "feat", SHA: "def"},
		},
		Base: "main",
	})
	m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabHistory})
	m.probe.settled = true

	keyModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	keyTab := keyModel.(*Model).state.Tab

	m = fixed(t)
	m.state = state.Apply(m.state, state.HistoryLoaded{
		Commits: []git.CommitInfo{{SHA: "a", Subject: "one"}},
	})
	m.state = state.Apply(m.state, state.WorktreesLoaded{
		Worktrees: []git.WorktreeRow{
			{Path: m.dir, Name: "repo", Branch: "main", Main: true, SHA: "abc"},
			{Path: m.dir + "/wt", Name: "wt", Branch: "feat", SHA: "def"},
		},
		Base: "main",
	})
	m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabHistory})
	m.probe.settled = true
	m.state = state.Apply(m.state, state.HelpOpened{})
	row, col := helpLineOf(m, "switch tabs")
	if row < 0 {
		t.Fatal("no switch tabs line in help")
	}
	helpModel, _ := m.Update(tea.MouseClickMsg{X: col, Y: row})
	helpTab := helpModel.(*Model).state.Tab

	if keyTab != helpTab {
		t.Fatalf("keyboard tab = %d, help click = %d", keyTab, helpTab)
	}
	if keyTab == state.TabStashed {
		t.Fatal("should skip hidden stashed tab")
	}
}

func TestDropConfirmedAfterAReloadDropsTheArmedStash(t *testing.T) {
	t.Parallel()
	dir := testRepo(t)
	if err := os.WriteFile(dir+"/a.txt", []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitIn(t, dir, "add", "a.txt")
	runGitIn(t, dir, "commit", "-q", "-m", "add")
	for _, msg := range []string{"first", "second"} {
		if err := os.WriteFile(dir+"/a.txt", []byte(msg+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		runGitIn(t, dir, "stash", "push", "-q", "-m", msg)
	}
	m, err := New(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.probe.settled = true
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 53})
	m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabStashed})
	head := git.Head{Branch: "main"}
	rows := stashRowsFilled(t, dir, head)
	m.state = state.Apply(m.state, state.StashedLoaded{Stashes: rows, Head: head})
	if len(m.state.Rows) != 2 {
		t.Fatalf("want two stash rows, got %d", len(m.state.Rows))
	}
	m.state.Cursor = 1
	if m.state.Rows[1].Stash().Message != "first" {
		t.Fatalf("cursor row = %q, want first", m.state.Rows[1].Stash().Message)
	}

	next, cmd := m.Update(tea.KeyPressMsg{Code: 'x'})
	m = next.(*Model)
	if runsGit(cmd) {
		t.Fatalf("want no git before confirm, got %T", cmd())
	}
	firstSHA := rows[1].SHA

	if err := os.WriteFile(dir+"/a.txt", []byte("third\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitIn(t, dir, "stash", "push", "-q", "-m", "third")
	rows = stashRowsFilled(t, dir, head)
	m.state = state.Apply(m.state, state.StashedLoaded{Stashes: rows, Head: head})

	next, cmd = m.Update(tea.KeyPressMsg{Code: 'y'})
	m = next.(*Model)
	if cmd == nil {
		t.Fatal("want drop command")
	}
	applyCmds(t, m, cmd)

	rows = stashRowsFilled(t, dir, head)
	if len(rows) != 2 {
		t.Fatalf("want two stashes, got %d", len(rows))
	}
	if rows[0].Message != "third" || rows[1].Message != "second" {
		t.Fatalf("order = %q then %q", rows[0].Message, rows[1].Message)
	}
	for _, row := range rows {
		if row.SHA == firstSHA {
			t.Fatal("armed stash still present")
		}
	}
}

func TestHistoryFileRowRunsNoWorktreeVerbs(t *testing.T) {
	t.Parallel()
	m, err := New(context.Background(), testRepo(t), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 53})
	m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabHistory})
	m.state = state.Apply(m.state, state.HistoryLoaded{Commits: []git.CommitInfo{
		{SHA: "a", Subject: "one"},
	}})
	m.state = state.Apply(m.state, state.CommitExpanded{
		SHA:   "a",
		Files: []git.Entry{{Path: "fourth.txt", Worktree: git.Added}},
	})
	m.state.Cursor = 1
	m.state = state.Apply(m.state, state.DiffOpened{
		Path: "fourth.txt", Origin: state.FromCommit("a"),
	})
	m.probe.settled = true

	for _, key := range []rune{'s', 'x', 'z'} {
		next, cmd := m.Update(tea.KeyPressMsg{Code: key})
		m = next.(*Model)
		if cmd != nil {
			t.Fatalf("key %q: want no command, got %T", key, cmd())
		}
	}
	if m.state.Changes.DiscardConfirm != nil {
		t.Fatal("discard should not await confirmation")
	}
}

func TestTabsAreReachableFromTheKeyboard(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	// tab only reaches a tab the bar draws, so there has to be one to reach.
	m.state = state.Apply(m.state, state.HistoryLoaded{
		Commits: []git.CommitInfo{{SHA: "a", Subject: "one"}},
	})
	if m.state.Tab != state.TabChanges {
		t.Fatalf("start tab = %d", m.state.Tab)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.state.Tab == state.TabChanges {
		t.Error("tab did not move to the next tab")
	}
	m.Update(tea.KeyPressMsg{Code: '1'})
	if m.state.Tab != state.TabChanges {
		t.Errorf("1 did not return to changes, got %d", m.state.Tab)
	}
}

func TestGoOnCurrentWorktreeDoesNothing(t *testing.T) {
	t.Parallel()
	here := "/repo"
	m, err := New(context.Background(), testRepo(t), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.probe.settled = true
	m.dir = here
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 53})
	m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabWorktrees})
	m.state = state.Apply(m.state, state.WorktreesLoaded{
		Worktrees: []git.WorktreeRow{
			{Path: here, Name: "repo", Branch: "main", Main: true},
			{Path: "/repo/wt", Name: "wt", Branch: "feat", SHA: "abc"},
		},
		Base: "main",
		Here: here,
	})
	m.state.Cursor = 0
	before := m.dir
	next, cmd := m.requestWorktreeGo()
	m = next.(*Model)
	if cmd != nil {
		t.Errorf("want nil cmd on current tree, got %v", cmd)
	}
	if m.dir != before {
		t.Errorf("dir = %q, want %q", m.dir, before)
	}
}

func TestGoAfterWorktreeClearsSelectionDiffAndCursor(t *testing.T) {
	t.Parallel()
	// A tree the move can reach: the directory, its git directory and its read
	// marks are all read before any of them is swapped, so a path that is not a
	// worktree moves nothing.
	dir := repoWithUnstagedChange(t)
	second := filepath.Join(t.TempDir(), "second")
	runGitIn(t, dir, "worktree", "add", "-q", second, "-b", "feat")
	wt := git.WorktreeRow{Path: second, Name: "second", Branch: "feat", SHA: "abc"}
	m, err := New(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.probe.settled = true
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 53})
	m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabWorktrees})
	// A lone worktree hides the tab, so the reader can only be here with two.
	m.state = state.Apply(m.state, state.WorktreesLoaded{
		Worktrees: []git.WorktreeRow{
			{Path: dir, Name: "repo", Branch: "main", Main: true},
			wt,
		},
		Base: "main",
	})
	m.state.Cursor = 1
	m.state = state.Apply(m.state, state.SelectionToggled{Path: wt.Path})
	m.state = state.Apply(m.state, state.DiffOpened{Path: "a.txt", Origin: state.WorkingTree(state.SectionUnstaged)})

	next, cmd := m.requestWorktreeGo()
	m = applyOnce(t, next.(*Model), cmd)
	if m.state.Worktrees.Here != wt.Path {
		t.Errorf("WorktreeHere = %q, want %q", m.state.Worktrees.Here, wt.Path)
	}
	if m.state.Cursor != 0 {
		t.Errorf("cursor = %d, want 0", m.state.Cursor)
	}
	if len(m.state.Changes.Selected) != 0 {
		t.Errorf("selection = %v", m.state.Changes.Selected)
	}
	if m.state.Open.Path != "" {
		t.Errorf("open path = %q", m.state.Open.Path)
	}
	if cmd == nil {
		t.Fatal("want reload commands after go")
	}
}

func worktreeTabWithTwoRows(t *testing.T) *Model {
	t.Helper()
	m := syntheticModel(t)
	m.probe.settled = true
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 53})
	m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabWorktrees})
	m.state = state.Apply(m.state, state.WorktreesLoaded{
		Worktrees: []git.WorktreeRow{
			{Path: "/repo", Name: "repo", Branch: "main", Main: true},
			{Path: "/repo/wt", Name: "wt", Branch: "feat", SHA: "abc"},
		},
		Base: "main",
	})
	return m
}

func TestWorktreeCursorDoesNotLoadADiff(t *testing.T) {
	t.Parallel()
	m := worktreeTabWithTwoRows(t)
	next, _ := m.Update(tea.KeyPressMsg{Code: 'j'})
	m = next.(*Model)
	if m.state.Open.Path != "" {
		t.Errorf("open path = %q, want empty", m.state.Open.Path)
	}
}

func TestDiffKeyIsANoOpOnAWorktreeRow(t *testing.T) {
	t.Parallel()
	m := worktreeTabWithTwoRows(t)
	m.state.Cursor = 1
	next, cmd := m.Update(tea.KeyPressMsg{Code: 'd'})
	m = next.(*Model)
	if m.state.Open.Path != "" {
		t.Errorf("open path = %q, want empty", m.state.Open.Path)
	}
	if hasDiffCmd(cmd) {
		t.Error("d should not open a diff on a worktree row")
	}
}

func TestWorktreeRowClickDoesNotLoadADiff(t *testing.T) {
	t.Parallel()
	m := worktreeTabWithTwoRows(t)
	row, col := hitOf(m, "/repo/wt")
	if row < 0 {
		t.Fatal("no region for /repo/wt")
	}
	next, cmd := m.Update(tea.MouseClickMsg{X: col, Y: row, Button: tea.MouseLeft})
	m = next.(*Model)
	if m.state.Open.Path != "" {
		t.Errorf("open path = %q, want empty", m.state.Open.Path)
	}
	if hasDiffCmd(cmd) {
		t.Error("click should not open a diff on a worktree row")
	}
}

func TestWorktreeRowClickDoesNotPassAnAbsolutePathspec(t *testing.T) {
	t.Parallel()
	m := worktreeTabWithTwoRows(t)
	row, col := hitOf(m, "/repo/wt")
	if row < 0 {
		t.Fatal("no region for /repo/wt")
	}
	next, cmd := m.Update(tea.MouseClickMsg{X: col, Y: row, Button: tea.MouseLeft})
	m = next.(*Model)
	if hasDiffCmd(cmd) {
		m = applyDiffCmd(t, m, cmd)
	}
	if strings.HasPrefix(m.state.Open.Path, "/") {
		t.Errorf("open path = %q, want repo-relative or empty", m.state.Open.Path)
	}
	if strings.Contains(m.state.Notice, "outside") {
		t.Errorf("got notice %q", m.state.Notice)
	}
}

// Moving the cursor does not read what a row holds. The row that is open is
// the one the reader opened, and the reads are a git call each: following the
// cursor ran one for every row they passed through and closed the row they had
// opened to look at.
func TestMovingTheCursorDoesNotReadAWorktree(t *testing.T) {
	t.Parallel()
	m := worktreeTabWithTwoRows(t)
	m.state.Cursor = 1
	m.asked.expandRow = "/repo/wt"

	next, cmd := m.Update(tea.KeyPressMsg{Code: 'k'})
	m = next.(*Model)
	if m.state.Cursor != 0 {
		t.Fatalf("cursor = %d, want 0", m.state.Cursor)
	}
	for _, msg := range messagesFrom(cmd) {
		if _, ok := msg.(worktreeFilesMsg); ok {
			t.Fatal("moving the cursor read a worktree")
		}
	}
}

// The diff key opens the row under the cursor, the way it opens a commit.
func TestTheDiffKeyOpensTheWorktreeUnderTheCursor(t *testing.T) {
	t.Parallel()
	m := worktreeTabWithTwoRows(t)
	m.state.Cursor = 0

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'd'})
	for _, msg := range messagesFrom(cmd) {
		if wf, ok := msg.(worktreeFilesMsg); ok && wf.path == "/repo" {
			return
		}
	}
	t.Fatal("want worktreeFilesMsg for /repo")
}

func TestWorktreeFilesAreNotReloadedWhenTheCursorStays(t *testing.T) {
	t.Parallel()
	m := worktreeTabWithTwoRows(t)
	m.state.Cursor = 0
	m.asked.expandRow = "/repo"

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'k'})
	for _, msg := range messagesFrom(cmd) {
		if _, ok := msg.(worktreeFilesMsg); ok {
			t.Fatal("did not expect worktreeFilesMsg when cursor stayed")
		}
	}
}

// The stashed tab answers this the same way the worktrees tab does.
func TestMovingTheCursorDoesNotReadAStash(t *testing.T) {
	t.Parallel()
	stashes := []git.StashRow{
		{Ref: "stash@{0}", SHA: "abc", Message: "one", Status: git.StashApplies},
		{Ref: "stash@{1}", SHA: "def", Message: "two", Status: git.StashApplies},
	}
	m, err := New(context.Background(), testRepo(t), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.probe.settled = true
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 53})
	m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabStashed})
	m.state = state.Apply(m.state, state.StashedLoaded{Stashes: stashes})
	m.state.Cursor = 0
	m.asked.expandRow = "stash@{0}"

	next, cmd := m.Update(tea.KeyPressMsg{Code: 'j'})
	m = next.(*Model)
	if m.state.Cursor != 1 {
		t.Fatalf("cursor = %d, want 1", m.state.Cursor)
	}
	for _, msg := range messagesFrom(cmd) {
		if _, ok := msg.(stashFilesMsg); ok {
			t.Fatal("moving the cursor read a stash")
		}
	}
}

func headingIndex(rows []state.Row, sec state.Section) int {
	for i, r := range rows {
		if r.Kind() == state.RowSectionHeading && r.Section() == sec {
			return i
		}
	}
	return -1
}

func TestJKeyMovesCursorToSectionHeadings(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	start := m.state.Cursor
	sawHeading := false
	for range len(m.state.Rows) {
		next, _ := m.Update(tea.KeyPressMsg{Code: 'j'})
		m = next.(*Model)
		if m.state.Rows[m.state.Cursor].Kind() == state.RowSectionHeading {
			sawHeading = true
			break
		}
	}
	if !sawHeading {
		t.Fatal("j never reached a section heading")
	}
	if m.state.Cursor == start {
		t.Fatal("the cursor did not move")
	}
}

func TestSelectionOpsPerTab(t *testing.T) {
	t.Parallel()
	type setupFn func(t *testing.T) *Model
	cases := []struct {
		name    string
		setup   setupFn
		wantSel bool
	}{
		{"changes", fixed, true},
		{"history", historyModelForHitArea, false},
		{"stashed", stashedModelForHitArea, false},
		{"worktrees", worktreesModelForHitArea, false},
	}
	ops := []struct {
		name  string
		apply func(t *testing.T, m *Model) *Model
	}{
		{"space", func(t *testing.T, m *Model) *Model {
			t.Helper()
			next, _ := m.Update(tea.KeyPressMsg{Code: ' '})
			return next.(*Model)
		}},
		{"a", func(t *testing.T, m *Model) *Model {
			t.Helper()
			next, _ := m.Update(tea.KeyPressMsg{Code: 'a'})
			return next.(*Model)
		}},
		{"shift+j", func(t *testing.T, m *Model) *Model {
			t.Helper()
			next, _ := m.Update(tea.KeyPressMsg{Code: 'j', Mod: tea.ModShift})
			return next.(*Model)
		}},
		{"shift+click", func(t *testing.T, m *Model) *Model {
			t.Helper()
			row, col, ok := shiftClickCoords(m)
			if !ok {
				t.Fatal("no row to shift-click")
			}
			next, _ := m.Update(tea.MouseClickMsg{X: col, Y: row, Button: tea.MouseLeft, Mod: tea.ModShift})
			return next.(*Model)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for _, op := range ops {
				t.Run(op.name, func(t *testing.T) {
					m := c.setup(t)
					m = op.apply(t, m)
					got := len(m.state.Changes.Selected)
					if c.wantSel && got == 0 {
						t.Error("expected selection to grow")
					}
					if !c.wantSel && got != 0 {
						t.Errorf("expected no selection, got %d: %v", got, m.state.Changes.Selected)
					}
				})
			}
		})
	}
}

func shiftClickCoords(m *Model) (row, col int, ok bool) {
	m.View()
	if screenRow, _, found := offCursorFileRow(m); found {
		return screenRow, 1, true
	}
	for _, r := range m.frame.Regions {
		if r.Target.Row < 0 {
			continue
		}
		switch r.Target.Kind {
		case layout.TargetFile, layout.TargetNone, layout.TargetCommit:
			return r.Row, r.ColStart + 1, true
		case layout.TargetDirectory, layout.TargetSectionHeading, layout.TargetTab, layout.TargetBlock, layout.TargetButton, layout.TargetCommitBox, layout.TargetCheckbox, layout.TargetVerb, layout.TargetHelp, layout.TargetHelpLine:
			// Only a row that opens on click can stand in for one here.
		}
	}
	return 0, 0, false
}

func TestSpaceOnHeadingSelectsTheSection(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	idx := headingIndex(m.state.Rows, state.SectionUnstaged)
	if idx < 0 {
		t.Fatal("no unstaged heading")
	}
	m.state.Cursor = idx
	next, _ := m.Update(tea.KeyPressMsg{Code: ' '})
	m = next.(*Model)
	for _, r := range m.state.Rows {
		if r.Kind() != state.RowFile || r.Section() != state.SectionUnstaged {
			continue
		}
		if !state.SelectedIn(m.state, r.Section(), r.Path()) {
			t.Fatalf("unstaged path %q not selected: %+v", r.Path(), m.state.Changes.Selected)
		}
	}
}

func TestLeftKeyOnDirectoryFolds(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	var dirIdx int
	var dirPath string
	for i, r := range m.state.Rows {
		if r.Kind() == state.RowDirectory && r.Section() == state.SectionUnstaged {
			dirIdx = i
			dirPath = r.DirPath()
			break
		}
	}
	if dirIdx < 0 {
		t.Fatal("no unstaged directory row")
	}
	m.state.Cursor = dirIdx
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	m = next.(*Model)
	if !state.FoldedIn(m.state, state.SectionUnstaged, dirPath) {
		t.Fatal("the directory should be folded")
	}
}

func TestRightKeyOnFoldedDirectoryUnfolds(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	var dirIdx int
	var dirPath string
	for i, r := range m.state.Rows {
		if r.Kind() == state.RowDirectory && r.Section() == state.SectionUnstaged {
			dirIdx = i
			dirPath = r.DirPath()
			break
		}
	}
	if dirIdx < 0 {
		t.Fatal("no unstaged directory row")
	}
	m.state.Cursor = dirIdx
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	m = next.(*Model)
	if !state.FoldedIn(m.state, state.SectionUnstaged, dirPath) {
		t.Fatal("the directory should be folded")
	}
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m = next.(*Model)
	if state.FoldedIn(m.state, state.SectionUnstaged, dirPath) {
		t.Fatal("the directory should be unfolded")
	}
}

func TestRightKeyOnOpenDirectoryLeavesItOpen(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	var dirIdx int
	var dirPath string
	for i, r := range m.state.Rows {
		if r.Kind() == state.RowDirectory && r.Section() == state.SectionUnstaged {
			dirIdx = i
			dirPath = r.DirPath()
			break
		}
	}
	if dirIdx < 0 {
		t.Fatal("no unstaged directory row")
	}
	m.state.Cursor = dirIdx
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m = next.(*Model)
	if state.FoldedIn(m.state, state.SectionUnstaged, dirPath) {
		t.Fatal("the open directory should stay open")
	}
}

func TestSelectionCountMatchesFilesNotHeadings(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	m.state = state.Apply(m.state, state.TabSelectionToggled{})
	filePaths := map[string]bool{}
	for _, r := range m.state.Rows {
		if r.Kind() == state.RowFile {
			filePaths[r.Path()] = true
		}
	}
	if len(m.state.Changes.Selected) != len(filePaths) {
		t.Fatalf("selected %d paths, want %d files: %+v",
			len(m.state.Changes.Selected), len(filePaths), m.state.Changes.Selected)
	}
	if len(m.state.Changes.Selected) == len(m.state.Rows) {
		t.Fatal("selection counted headings as files")
	}
}

func TestRangeSelectSkipsHeadings(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	last := len(m.state.Rows) - 1
	m.state = state.Apply(m.state, state.SelectionRangeExtended{From: m.state.Cursor, To: last})
	ticked := 0
	for _, r := range m.state.Rows {
		if !state.SelectedIn(m.state, r.Section(), r.Path()) {
			continue
		}
		if r.Kind() != state.RowFile {
			t.Fatalf("selected a %v row at %q, which is not a file", r.Kind(), r.Path())
		}
		ticked++
	}
	if ticked != state.SelectionCount(m.state) {
		t.Fatalf("%d ticked rows but the count is %d: %+v",
			ticked, state.SelectionCount(m.state), m.state.Changes.Selected)
	}
}

// A tab the bar does not draw has nowhere to put the underline, so the reader
// lands somewhere the bar cannot name. The key does nothing instead.
func TestNumberKeyDoesNotReachAHiddenTab(t *testing.T) {
	t.Parallel()
	dir := repoWithUnstagedChange(t)
	m, err := New(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.probe.settled = true
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 53})
	m = pumpInit(t, m)

	if got := layout.VisibleTabs(m.state, m.read); slices.Contains(got, "stashed") {
		t.Fatalf("this repository has no stashes but the bar draws %v", got)
	}
	before := m.state.Tab
	next, _ := m.Update(tea.KeyPressMsg{Code: '3'})
	m = next.(*Model)
	if m.state.Tab != before {
		t.Errorf("moved to tab %d, which the bar does not draw", m.state.Tab)
	}
}

// git stash branch is the design's guaranteed way out of a conflicting stash,
// so the name it is given has to be one that does not exist. Naming the branch
// the stash was taken from names one that always does.
func TestStashBranchDoesNotReuseTheOriginBranch(t *testing.T) {
	t.Parallel()
	m, err := New(context.Background(), testRepo(t), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 53})
	m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabStashed})
	m.state = state.Apply(m.state, state.StashedLoaded{Stashes: []git.StashRow{{
		Ref: "stash@{0}", SHA: "abc", Branch: "main", Status: git.StashConflicts,
	}}})
	m.state.Cursor = 0
	if got := m.stashBranchBase(); got == "main" {
		t.Errorf("branch name = %q, which already exists", got)
	}
}

// The name above is free in a fresh repository, so that test passes whether or
// not the existing names were read at all. This one takes the first choice
// away: git refuses a branch that exists, and the reader sees the stash they
// cannot get out of.
func TestStashBranchStepsPastANameThatExists(t *testing.T) {
	t.Parallel()
	dir := testRepo(t)
	runGitIn(t, dir, "branch", "main-stash")
	m, err := New(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 53})
	m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabStashed})
	m.state = state.Apply(m.state, state.StashedLoaded{Stashes: []git.StashRow{{
		Ref: "stash@{0}", SHA: "abc", Branch: "main", Status: git.StashConflicts,
	}}})
	m.state.Cursor = 0
	// The base and the search for a free name are asked in that order by the
	// command the key issues, so they are asked in that order here.
	got := git.FreeBranchName(context.Background(), dir, m.stashBranchBase())
	if got != "main-stash-2" {
		t.Errorf("branch name = %q, want main-stash-2", got)
	}
}

func TestStashBranchSuccessLeavesANotice(t *testing.T) {
	t.Parallel()
	m, err := New(context.Background(), testRepo(t), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 53})
	next, _ := m.Update(stashVerbMsg{branch: "main-stash"})
	m = next.(*Model)
	if m.state.Notice != "branched to main-stash" {
		t.Errorf("got %q", m.state.Notice)
	}
	next, _ = m.Update(stashVerbMsg{err: errors.New("boom"), branch: "main-stash"})
	m = next.(*Model)
	if m.state.Notice != "boom" {
		t.Errorf("got %q", m.state.Notice)
	}
	if strings.Contains(m.state.Notice, "branched to") {
		t.Errorf("error path should not set branch notice: %q", m.state.Notice)
	}
}

// Past about eighty files the list takes the whole pane and the diff closes.
// Pressing d there has to reopen it. The flag that drove this was set only when
// the reader closed the diff themselves, so d did nothing in this case.
func TestDOpensASixRowDiffWhenTheListFilledThePane(t *testing.T) {
	t.Parallel()
	dir := testRepo(t)
	for i := range 90 {
		name := fmt.Sprintf("%s/f%02d.txt", dir, i)
		if err := os.WriteFile(name, []byte("base\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runGitIn(t, dir, "add", "-A")
	runGitIn(t, dir, "commit", "-q", "-m", "base")
	for i := range 90 {
		name := fmt.Sprintf("%s/f%02d.txt", dir, i)
		if err := os.WriteFile(name, []byte("base\nmod\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	m, err := New(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.probe.settled = true
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 53})
	m = pumpInit(t, m)

	m.View()
	if drawn := countLines(m.frame.Lines, "block "); drawn != 0 {
		t.Fatalf("%d block lines with ninety files, want the diff closed", drawn)
	}
	next, cmd := m.Update(tea.KeyPressMsg{Code: 'd'})
	m = next.(*Model)
	m = applyDiffCmd(t, m, cmd)
	m.View()
	if drawn := countLines(m.frame.Lines, "block "); drawn == 0 {
		t.Error("d did not open the diff")
	}
}

func countLines(lines []string, needle string) int {
	n := 0
	for _, l := range lines {
		if strings.Contains(l, needle) {
			n++
		}
	}
	return n
}

func runGitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestSelectAllKeySelectsEveryFileInTab(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	next, _ := m.Update(tea.KeyPressMsg{Code: 'a'})
	m = next.(*Model)
	for _, r := range m.state.Rows {
		if r.Kind() != state.RowFile {
			continue
		}
		if !state.SelectedIn(m.state, r.Section(), r.Path()) {
			t.Fatalf("path %q not selected after a", r.Path())
		}
	}
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = next.(*Model)
	for _, r := range m.state.Rows {
		if r.Kind() != state.RowFile {
			continue
		}
		if state.SelectedIn(m.state, r.Section(), r.Path()) {
			t.Fatalf("path %q still selected after esc", r.Path())
		}
	}
}

func diffText(d git.FileDiff) string {
	var b strings.Builder
	for _, block := range d.Blocks {
		for _, line := range block.Lines {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func bootStashFileModel(t *testing.T) (*Model, string, string, string) {
	t.Helper()
	dir := testRepo(t)
	const path = "held.txt"
	const stashLine = "stashed line"
	const localLine = "local line"
	if err := os.WriteFile(dir+"/"+path, []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitIn(t, dir, "add", path)
	runGitIn(t, dir, "commit", "-q", "-m", "base")
	if err := os.WriteFile(dir+"/"+path, []byte(stashLine+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitIn(t, dir, "stash", "push", "-q", "-m", "hold")
	if err := os.WriteFile(dir+"/"+path, []byte(localLine+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, head, err := git.Status(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	stashes := stashRowsFilled(t, dir, head)
	files, err := git.StashFiles(context.Background(), dir, stashes[0].Ref)
	if err != nil {
		t.Fatal(err)
	}
	m, err := New(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.probe.settled = true
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 53})
	m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabStashed})
	m.state = state.Apply(m.state, state.StashedLoaded{Stashes: stashes, Head: head})
	m.state = state.Apply(m.state, state.StashFilesLoaded{Ref: stashes[0].Ref, Files: files})
	for i, row := range m.state.Rows {
		if row.Kind() == state.RowFile && row.Path() == path {
			m.state.Cursor = i
			break
		}
	}
	return m, path, stashLine, localLine
}

func TestClickingAStashFileOpensItsDiff(t *testing.T) {
	t.Parallel()
	m, path, stashLine, localLine := bootStashFileModel(t)
	row, col := hitOf(m, path)
	if row < 0 {
		t.Fatal("no click target for stash file")
	}
	next, cmd := m.Update(tea.MouseClickMsg{X: col, Y: row, Button: tea.MouseLeft})
	m = applyDiffCmd(t, next.(*Model), cmd)
	if m.state.Open.Path != path {
		t.Errorf("OpenPath = %q, want %q", m.state.Open.Path, path)
	}
	text := diffText(m.state.Open.Diff)
	if !strings.Contains(text, stashLine) {
		t.Errorf("diff = %q, want %q", text, stashLine)
	}
	if strings.Contains(text, localLine) {
		t.Errorf("diff = %q, must not contain working tree line %q", text, localLine)
	}
}

func TestPressingDOnAStashFileOpensItsDiff(t *testing.T) {
	t.Parallel()
	m, path, stashLine, localLine := bootStashFileModel(t)
	next, cmd := m.Update(tea.KeyPressMsg{Code: 'd'})
	m = applyDiffCmd(t, next.(*Model), cmd)
	if m.state.Open.Path != path {
		t.Errorf("OpenPath = %q, want %q", m.state.Open.Path, path)
	}
	text := diffText(m.state.Open.Diff)
	if !strings.Contains(text, stashLine) {
		t.Errorf("diff = %q, want %q", text, stashLine)
	}
	if strings.Contains(text, localLine) {
		t.Errorf("diff = %q, must not contain working tree line %q", text, localLine)
	}
}

func TestPressingEnterOnAStashFileOpensItsDiff(t *testing.T) {
	t.Parallel()
	m, path, stashLine, localLine := bootStashFileModel(t)
	m.state.Cursor = 0
	next, _ := m.Update(tea.KeyPressMsg{Code: 'j'})
	m = next.(*Model)
	if m.state.Cursor != 1 || m.state.Rows[m.state.Cursor].Kind() != state.RowFile {
		t.Fatalf("cursor = %d, want file row at 1", m.state.Cursor)
	}
	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = applyDiffCmd(t, next.(*Model), cmd)
	if m.state.Open.Path != path {
		t.Errorf("OpenPath = %q, want %q", m.state.Open.Path, path)
	}
	text := diffText(m.state.Open.Diff)
	if !strings.Contains(text, stashLine) {
		t.Errorf("diff = %q, want %q", text, stashLine)
	}
	if strings.Contains(text, localLine) {
		t.Errorf("diff = %q, must not contain working tree line %q", text, localLine)
	}
}

func TestClickingAWorktreeFileOpensItsDiff(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	env := []string{
		"HOME=" + base,
		"GIT_CONFIG_GLOBAL=" + base + "/.gitconfig",
		"GIT_CONFIG_SYSTEM=/dev/null",
		// git forks a repack of its own once a repository crosses a
		// threshold, and that child outlives the test: it races
		// t.TempDir's removal, and it takes the repository's locks
		// while another test is reading it.
		"GIT_CONFIG_COUNT=2",
		"GIT_CONFIG_KEY_0=gc.auto", "GIT_CONFIG_VALUE_0=0",
		"GIT_CONFIG_KEY_1=maintenance.auto", "GIT_CONFIG_VALUE_1=false",
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
	}
	mainDir := filepath.Join(base, "main")
	runGitInEnv(t, env, base, "init", "-q", "-b", "main", "main")
	runGitInEnv(t, env, mainDir, "commit", "-q", "--allow-empty", "-m", "base")
	wtDir := filepath.Join(base, "wt")
	runGitInEnv(t, env, mainDir, "worktree", "add", "-q", wtDir, "-b", "feat")
	const path = "wt-only.txt"
	const body = "only in worktree"
	if err := os.WriteFile(filepath.Join(wtDir, path), []byte(body+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	trees, treeBase, err := git.WorktreeList(context.Background(), mainDir)
	if err != nil {
		t.Fatal(err)
	}
	var wtPath string
	for _, row := range trees {
		if row.Main {
			continue
		}
		wtPath = row.Path
		break
	}
	if wtPath == "" {
		t.Fatal("linked worktree missing from list")
	}
	files, err := git.WorktreeFiles(context.Background(), wtPath)
	if err != nil {
		t.Fatal(err)
	}
	m, err := New(context.Background(), mainDir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.probe.settled = true
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 53})
	m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabWorktrees})
	m.state = state.Apply(m.state, state.WorktreesLoaded{
		Worktrees: trees, Base: treeBase, Here: mainDir,
	})
	for i, row := range m.state.Rows {
		if row.Kind() == state.RowWorktree && row.Worktree().Path == wtPath {
			m.state.Cursor = i
			break
		}
	}
	m.state = state.Apply(m.state, state.WorktreeFilesLoaded{Path: wtPath, Files: files})
	if len(files) == 0 {
		t.Fatalf("WorktreeFiles empty for %s", wtPath)
	}
	var fileRow = -1
	for i, row := range m.state.Rows {
		if row.Kind() == state.RowFile && row.Path() == path {
			fileRow = i
			break
		}
	}
	if fileRow < 0 {
		t.Fatalf("Rows after load: %+v files=%+v cursor=%d wtPath=%q", m.state.Rows, files, m.state.Cursor, wtPath)
	}
	m.state.Cursor = fileRow
	row, col := hitOf(m, path)
	if row < 0 {
		t.Fatal("no click target for worktree file")
	}
	next, cmd := m.Update(tea.MouseClickMsg{X: col, Y: row, Button: tea.MouseLeft})
	m = applyDiffCmd(t, next.(*Model), cmd)
	if m.state.Open.Path != path {
		t.Errorf("OpenPath = %q, want %q", m.state.Open.Path, path)
	}
	if strings.HasPrefix(m.state.Open.Path, "/") {
		t.Errorf("OpenPath = %q, want repo-relative", m.state.Open.Path)
	}
	text := diffText(m.state.Open.Diff)
	if !strings.Contains(text, body) {
		t.Errorf("diff = %q, want %q", text, body)
	}
}

func runGitInEnv(t *testing.T, env []string, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = slices.Concat(env, os.Environ())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
}

// A row the reader opened stays open while they look at the rows around it.
// The cursor used to decide which row was open, so hovering onto a neighbor
// closed what they had opened and opened something else — the one tab where
// that happened, and the reason this reads the rows rather than a command.
func TestMovingOffAnOpenWorktreeLeavesItOpen(t *testing.T) {
	t.Parallel()
	trees := []git.WorktreeRow{
		{Path: "/repo", Name: "repo", Branch: "main", SHA: "aaa", Main: true},
		{Path: "/repo/wt", Name: "wt", Branch: "feat", SHA: "bbb"},
	}
	m := syntheticModel(t)
	m.probe.settled = true
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 53})
	m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabWorktrees})
	m.state = state.Apply(m.state, state.WorktreesLoaded{
		Worktrees: trees, Base: "main", Here: "/repo"})
	m.state = state.Apply(m.state, state.WorktreeFilesLoaded{
		Path: "/repo/wt", Files: []git.Entry{{Path: "only-in-wt.go"}}})

	opened := rowShape(m.state)
	for _, path := range []string{"/repo", "/repo/wt", "/repo"} {
		row, _ := hitOf(m, path)
		next, _ := m.Update(tea.MouseMotionMsg{X: 10, Y: row})
		m = next.(*Model)
	}
	if got := rowShape(m.state); got != opened {
		t.Errorf("hovering closed the open tree:\n  before %s\n  after  %s", opened, got)
	}
}

func TestMovingOffAnOpenStashLeavesItOpen(t *testing.T) {
	t.Parallel()
	stashes := []git.StashRow{
		{Ref: "stash@{0}", SHA: "abc", Message: "one", Status: git.StashApplies},
		{Ref: "stash@{1}", SHA: "def", Message: "two", Status: git.StashApplies},
	}
	m := syntheticModel(t)
	m.probe.settled = true
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 53})
	m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabStashed})
	m.state = state.Apply(m.state, state.StashedLoaded{Stashes: stashes})
	m.state = state.Apply(m.state, state.StashFilesLoaded{
		Ref: "stash@{0}", Files: []git.Entry{{Path: "only-in-stash.go"}}})

	opened := rowShape(m.state)
	for _, ref := range []string{"stash@{1}", "stash@{0}", "stash@{1}"} {
		row, _ := hitOf(m, ref)
		next, _ := m.Update(tea.MouseMotionMsg{X: 10, Y: row})
		m = next.(*Model)
	}
	if got := rowShape(m.state); got != opened {
		t.Errorf("hovering closed the open stash:\n  before %s\n  after  %s", opened, got)
	}
}

// rowShape names each row by kind and key, which is what changes when something
// opens or closes. The cursor is left out; it is what moved.
func rowShape(s state.State) string {
	out := ""
	for _, r := range s.Rows {
		switch r.Kind() {
		case state.RowWorktree:
			out += " [wt " + r.Worktree().Path + "]"
		case state.RowStash:
			out += " [stash " + r.Stash().Ref + "]"
		case state.RowCommit:
			out += " [commit " + r.Commit().SHA + "]"
		case state.RowFile:
			out += " ->" + r.Path()
		case state.RowDirectory:
			out += " {" + r.DirPath() + "}"
		case state.RowSectionHeading:
			out += " #"
		}
	}
	return out
}

func TestClickingASectionHeadingMovesTheCursorOntoIt(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	m.View()
	var screenRow, col int
	var wantSec state.Section
	found := false
	for _, r := range m.frame.Regions {
		if r.Target.Kind != layout.TargetSectionHeading {
			continue
		}
		screenRow = r.Row
		col = r.ColStart + 2
		wantSec = r.Target.Section
		found = true
		break
	}
	if !found {
		t.Fatal("no section heading region")
	}
	for range len(m.state.Rows) {
		row := m.state.Rows[m.state.Cursor]
		if row.Kind() == state.RowSectionHeading && row.Section() == wantSec {
			t.Fatal("the cursor already sits on the heading")
		}
		next, _ := m.Update(tea.KeyPressMsg{Code: 'j'})
		m = next.(*Model)
	}
	next, _ := m.Update(tea.MouseClickMsg{X: col, Y: screenRow, Button: tea.MouseLeft})
	m = next.(*Model)
	row := m.state.Rows[m.state.Cursor]
	if row.Kind() != state.RowSectionHeading || row.Section() != wantSec {
		t.Errorf("cursor on %v %v, want heading %v", row.Kind(), row.Section(), wantSec)
	}
}

// stashRowsFilled is what the stashed tab reads: the list, then what each stash
// holds. The list alone is what every reload costs, so the fill is separate.
func stashRowsFilled(t *testing.T, dir string, head git.Head) []git.StashRow {
	t.Helper()
	rows, err := git.StashList(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	for i := range rows {
		if err := git.FillStashStatus(context.Background(), dir, &rows[i], head); err != nil {
			t.Fatal(err)
		}
	}
	return rows
}

// runsGit answers whether a command does more than record that a row was read.
// A confirmation must not touch the repository before it is answered; saving a
// read mark is not touching it.
func runsGit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	switch cmd().(type) {
	case readSavedMsg, nil:
		return false
	}
	return true
}

// Closing a diff records which file it was closed for, and reopening that file
// with d opens it peeked: the reader is going back to where they were. Another
// file is a new place, and it opens full height.
//
// Both halves are needed. Without the path test, every file opens peeked once
// any diff has been closed, so the reader who closes one file and opens the
// next sees six rows of it instead of the pane.
func TestOnlyTheFileTheDiffWasClosedForReopensPeeked(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 40})
	m.state = state.Apply(m.state, state.StatusLoaded{
		Rows: []git.Entry{
			{Path: "a.txt", Worktree: git.Modified},
			{Path: "b.txt", Worktree: git.Modified},
		},
		Head: git.Head{Branch: "main"},
	})
	m.state.Cursor = state.FirstFileRow(m.state.Rows)

	next, _ := m.Update(tea.KeyPressMsg{Code: 'd'})
	m = next.(*Model)
	if m.state.Open.Path != "a.txt" {
		t.Fatalf("d opened %q, want a.txt", m.state.Open.Path)
	}
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = next.(*Model)
	if !m.state.Open.Closed || m.state.Open.ClosedFor != "a.txt" {
		t.Fatalf("esc left Closed=%v ClosedFor=%q, want a.txt closed",
			m.state.Open.Closed, m.state.Open.ClosedFor)
	}

	next, _ = m.Update(tea.KeyPressMsg{Code: 'j'})
	m = next.(*Model)
	next, _ = m.Update(tea.KeyPressMsg{Code: 'd'})
	m = next.(*Model)
	if m.state.Open.Path != "b.txt" {
		t.Fatalf("d opened %q, want b.txt", m.state.Open.Path)
	}
	if m.state.Open.Peek {
		t.Error("a file the diff was not closed for opened peeked")
	}
}

// Staging one block reloads the diff of the file that is open, and the side it
// is read from is the side that file's row sits on. Reading the other side
// hands the reader the blocks they did not stage: the unstaged row shows what
// is left, the staged row shows what was taken, and the two are different
// lists.
func TestStagingABlockReloadsTheSideTheFileIsOpenOn(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to git")
	}
	t.Parallel()
	dir := t.TempDir()
	initSyntheticRepo(t, dir)
	// One file with a change on each side: the index holds the first line and
	// the working tree holds a second one on top of it.
	if err := os.WriteFile(filepath.Join(dir, "split.txt"), []byte("staged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitIn(t, dir, "add", "split.txt")
	if err := os.WriteFile(filepath.Join(dir, "split.txt"),
		[]byte("staged\nunstaged\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := New(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo, err := git.Load(context.Background(), dir, mustGitDir(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	next, _ := m.Update(repoMsgFor(t, dir, repo))
	m = next.(*Model)

	unstaged := state.WorkingTree(state.SectionUnstaged)
	m.state = state.Apply(m.state, state.DiffOpened{Path: "split.txt", Origin: unstaged})
	m.state.Changes.Pending = &state.VerbRequested{Verb: state.VerbStage, Block: 0,
		Targets: []string{"split.txt"}}

	// The cursor sits on a heading, so nothing follows it: the only diff the
	// batch asks for is the one staging a block reloads.
	for i, row := range m.state.Rows {
		if row.Kind() == state.RowSectionHeading {
			m.state.Cursor = i
			break
		}
	}
	_, after := m.handleVerbMsg(verbMsg{finished: state.VerbFinished{}})
	var reloaded *git.FileDiff
	walkCmds(after, func(msg tea.Msg) {
		if d, ok := msg.(diffMsg); ok && d.Err == nil {
			copy := d.Diff
			reloaded = &copy
		}
	})
	if reloaded == nil {
		t.Fatal("staging a block reloaded no diff")
	}
	body := ""
	for _, b := range reloaded.Blocks {
		body += strings.Join(b.Lines, "\n")
	}
	if !strings.Contains(body, "unstaged") {
		t.Errorf("the reloaded diff holds %q, want the side the file is open on", body)
	}
}

// A pane that has just started is looking at the reader: the terminal gave it
// the focus by running it. Starting blurred means the watcher's events are
// taken and dropped until the terminal happens to say "focused", and a
// terminal that never sends focus events — or one the reader never leaves —
// never says it, so the pane would sit still while the repository changed.
func TestANewPaneStartsFocused(t *testing.T) {
	if testing.Short() {
		t.Skip("reads the repository")
	}
	t.Parallel()
	dir := t.TempDir()
	initSyntheticRepo(t, dir)
	m, err := New(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	// A change under the repository starts the wait before a reload, and that
	// wait is only set while the pane has the focus.
	before := m.watch.debounceGen
	if _, cmd := m.Update(watchEventMsg{}); cmd == nil {
		t.Fatal("a change under the repository issued no command")
	}
	if m.watch.debounceGen == before {
		t.Error("a change under the repository started no wait: the pane is blurred")
	}
}

// The read marks are written off the update goroutine: the bytes are taken on
// it and the file is replaced on the command's. A command that answers without
// writing loses every mark the reader made in that session, and it answers
// "saved" while doing it, so nothing says the file was not replaced.
func TestSavingTheReadMarksWritesTheFile(t *testing.T) {
	if testing.Short() {
		t.Skip("reads the repository")
	}
	t.Parallel()
	dir := t.TempDir()
	initSyntheticRepo(t, dir)
	// A file the repository knows: the marks of a path it does not have are
	// dropped when they are read back, which is what Prune is for.
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitIn(t, dir, "add", "a.txt")
	runGitIn(t, dir, "commit", "-q", "-m", "add")

	stateDir := t.TempDir()
	m, err := New(context.Background(), dir, stateDir)
	if err != nil {
		t.Fatal(err)
	}
	m.read.MarkBlock("a.txt", "h1")
	m.read.MarkFileIn(state.WorkingTree(state.SectionUnstaged), "a.txt", []string{"h1"})

	cmd := m.saveRead()
	if cmd == nil {
		t.Fatal("saving the marks issued no command")
	}
	msg, ok := cmd().(readSavedMsg)
	if !ok {
		t.Fatalf("saving the marks answered %T", cmd())
	}
	if msg.err != nil {
		t.Fatal(msg.err)
	}

	// The marks are on disk: a pane pointed at the same repository reads them
	// back.
	again, err := New(context.Background(), dir, stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if got := again.read.MarkIn(state.WorkingTree(state.SectionUnstaged),
		"a.txt", []string{"h1"}); got != state.Read {
		t.Errorf("the mark came back as %v, want read", got)
	}
}
