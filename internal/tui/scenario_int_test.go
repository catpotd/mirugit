package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
)

// A scenario drives the program the way a reader does: real keys against a real
// repository, with every command the program returns actually run. The unit
// tests build State by hand and drop the commands, so a verb that never runs
// and a row that reloads from the wrong side both pass them.
// maxScenarioDepth bounds one key's command chain. Real chains are three or
// four deep; hitting this means a loop, not a long step.
const maxScenarioDepth = 24

// scenarioCommandTimeout bounds one command. The suite runs its tests in
// parallel, so a git chain shares the machine with a few hundred others; three
// seconds was enough alone and not enough together.
const scenarioCommandTimeout = 20 * time.Second

type scenario struct {
	t   *testing.T
	m   *Model
	dir string
	// pending counts commands that never returned, which are the watcher and
	// the tick. A scenario that leans on one of those has to say so.
	pending int
}

func newScenario(t *testing.T, dir string) *scenario {
	t.Helper()
	m, err := New(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.probe.settled = true
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 53})
	s := &scenario{t: t, m: m, dir: dir}
	s.reload()
	// A dropped load leaves a model the test did not ask for, and the failure
	// then lands somewhere else: measured under load, the repository never
	// arrived and TestScenarioALongListScrollsWithTheCursor panicked on
	// Rows[0]. The pump counted the drop and nobody read the count.
	if s.pending > 0 {
		t.Fatalf("%d load(s) did not return in %s; the scenario is not the one this test describes",
			s.pending, scenarioCommandTimeout)
	}
	return s
}

// reload runs the program's own loads, not a hand-built message. Building the
// message here left Commits, Stashes and Worktrees empty, which hid three of
// the four tabs from every scenario.
//
// Init also starts the watcher, the poll and the width probe. Those never
// return, so they are left out; probe.settled stands in for the third.
func (s *scenario) reload() {
	s.t.Helper()
	s.run(loadRepo(context.Background(), s.dir, mustGitDir(s.t, s.dir)), 0)
	s.run(loadWorktrees(context.Background(), s.dir), 0)
}

func (s *scenario) send(msg tea.Msg) {
	s.t.Helper()
	next, cmd := s.m.Update(msg)
	s.m = next.(*Model)
	s.run(cmd, 0)
}

// run resolves a command and feeds what it returns back in, which is what the
// bubbletea loop does. Watchers and tickers never return, so a command that
// takes too long is dropped — and says so, because a step this harness swallows
// is a step the scenario did not actually take.
func (s *scenario) run(cmd tea.Cmd, depth int) {
	s.t.Helper()
	if cmd == nil {
		return
	}
	if depth > maxScenarioDepth {
		s.t.Fatalf("command chain deeper than %d; the scenario stopped short", maxScenarioDepth)
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	var msg tea.Msg
	select {
	case msg = <-done:
	case <-time.After(scenarioCommandTimeout):
		s.pending++
		return
	}
	switch v := msg.(type) {
	case nil:
		return
	case tea.BatchMsg:
		for _, c := range v {
			s.run(c, depth+1)
		}
	case tea.QuitMsg:
		return
	default:
		next, cmd := s.m.Update(msg)
		s.m = next.(*Model)
		s.run(cmd, depth+1)
	}
}

func (s *scenario) key(r rune) {
	s.t.Helper()
	s.send(tea.KeyPressMsg{Code: r, Text: string(r)})
}

func (s *scenario) press(code rune) {
	s.t.Helper()
	s.send(tea.KeyPressMsg{Code: code})
}

func (s *scenario) screen() []string {
	return strings.Split(s.m.View().Content, "\n")
}

// sectionRow is the drawn line for one file in one section. Two sections can
// hold the same name, and reading the first match is how a test passes while
// the second row is wrong.
func (s *scenario) sectionRow(section, name string) string {
	s.t.Helper()
	in := false
	for _, line := range s.screen() {
		if strings.Contains(line, section+" · ") {
			in = true
			continue
		}
		if in && isSectionHeading(line) {
			in = false
		}
		if in && strings.Contains(line, name) {
			return line
		}
	}
	return ""
}

func (s *scenario) sectionHeading(section string) string {
	s.t.Helper()
	for _, line := range s.screen() {
		if strings.Contains(line, section+" · ") {
			return line
		}
	}
	return ""
}

func isSectionHeading(line string) bool {
	return strings.Contains(line, "staged · ") || strings.Contains(line, "changes · ")
}

// cursorTo moves with j until the cursor sits on the named row of the named
// section, so a test says where it is rather than counting keystrokes.
func (s *scenario) cursorTo(section, name string) {
	s.t.Helper()
	for range len(s.m.state.Rows) {
		s.key('k')
	}
	for range len(s.m.state.Rows) + 2 {
		if row := s.sectionRow(section, name); strings.HasPrefix(row, "▌") {
			return
		}
		s.key('j')
	}
	s.t.Fatalf("never reached %s row %q\n%s", section, name, strings.Join(s.screen(), "\n"))
}

// cursorToHeading walks to a section heading, which is a row the reader can
// stand on to tick the whole section.
func (s *scenario) cursorToHeading(section string) {
	s.t.Helper()
	for range len(s.m.state.Rows) {
		s.key('k')
	}
	for range len(s.m.state.Rows) + 2 {
		if strings.HasPrefix(s.sectionHeading(section), "▌") {
			return
		}
		s.key('j')
	}
	s.t.Fatalf("never reached the %s heading\n%s", section, strings.Join(s.screen(), "\n"))
}

// repoWithAPathOnBothSides stages an edit and then edits again, which is the
// state an agent leaves behind and the one every row-keyed bug appeared in.
func repoWithAPathOnBothSides(t *testing.T) string {
	t.Helper()
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
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(name, body string) {
		t.Helper()
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	run("init", "-q", "-b", "main")
	write("app/both.txt", "one\ntwo\nthree\n")
	write("app/other.txt", "alpha\nbeta\n")
	run("add", ".")
	run("commit", "-q", "-m", "base")

	write("app/both.txt", "one\nSTAGED\nthree\n")
	run("add", "app/both.txt")
	write("app/both.txt", "one\nSTAGED\nWORKTREE\n")

	write("app/other.txt", "alpha\nEDITED\n")
	return dir
}

func TestScenarioBothSidesStartUnreadAndUnticked(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	s := newScenario(t, repoWithAPathOnBothSides(t))

	for _, section := range []string{"staged", "changes"} {
		row := s.sectionRow(section, "both.txt")
		if row == "" {
			t.Fatalf("no both.txt row in %s\n%s", section, strings.Join(s.screen(), "\n"))
		}
		if !strings.Contains(row, "·") {
			t.Errorf("%s both.txt starts read: %q", section, row)
		}
	}
}

// Reading one side says nothing about the other. The bug drew ● on the side
// that was never opened, which claims the reader saw it and it changed since.
func TestScenarioReadingOneSideLeavesTheOtherUnread(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	s := newScenario(t, repoWithAPathOnBothSides(t))
	s.cursorTo("staged", "both.txt")
	s.key('r')

	if row := s.sectionRow("staged", "both.txt"); strings.Contains(row, "·") || strings.Contains(row, "●") {
		t.Errorf("staged both.txt is still marked after r: %q", row)
	}
	row := s.sectionRow("changes", "both.txt")
	if strings.Contains(row, "●") {
		t.Errorf("changes both.txt reads as changed-after-read: %q", row)
	}
	if !strings.Contains(row, "·") {
		t.Errorf("changes both.txt lost its unread mark: %q", row)
	}
}

// The tick belongs to the row. Both rows carrying it made the summary say one
// while two headings showed a partial box.
func TestScenarioSelectingOneSideLeavesTheOtherUnticked(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	s := newScenario(t, repoWithAPathOnBothSides(t))
	s.cursorTo("staged", "both.txt")
	s.key(' ')

	if row := s.sectionRow("staged", "both.txt"); !strings.Contains(row, "[✓]") {
		t.Errorf("staged both.txt is not ticked: %q", row)
	}
	if row := s.sectionRow("changes", "both.txt"); strings.Contains(row, "[✓]") {
		t.Errorf("changes both.txt was ticked too: %q", row)
	}
	if h := s.sectionHeading("changes"); strings.Contains(h, "[-]") || strings.Contains(h, "[✓]") {
		t.Errorf("changes heading shows a selection: %q", h)
	}
	if got := s.m.state.Notice; got != "" {
		t.Errorf("notice = %q, want none", got)
	}
}

// Folding is per row for the same reason. The directory holds one file on each
// side and the reader closes one without losing sight of the other.
func TestScenarioFoldingOneSideLeavesTheOtherOpen(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	s := newScenario(t, repoWithAPathOnBothSides(t))
	s.cursorTo("staged", "▾ app")
	s.press(tea.KeyLeft)

	if s.sectionRow("staged", "both.txt") != "" {
		t.Error("the staged directory did not fold")
	}
	if s.sectionRow("changes", "both.txt") == "" {
		t.Errorf("folding staged also folded changes\n%s", strings.Join(s.screen(), "\n"))
	}
}

// diffBody is the diff area under the list, which is what the reader is looking
// at when a row is open.
func (s *scenario) diffBody() string {
	lines := s.screen()
	for i, line := range lines {
		if strings.Contains(line, "esc close") {
			return strings.Join(lines[i:], "\n")
		}
	}
	return ""
}

// The two rows hold different diffs: HEAD against the index, and the index
// against the tree. Moving between them has to swap what the diff area shows.
func TestScenarioMovingBetweenSidesSwapsTheDiff(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	s := newScenario(t, repoWithAPathOnBothSides(t))

	s.cursorTo("staged", "both.txt")
	if body := s.diffBody(); !strings.Contains(body, "+STAGED") {
		t.Errorf("staged row shows the wrong diff:\n%s", body)
	}

	s.cursorTo("changes", "both.txt")
	body := s.diffBody()
	if !strings.Contains(body, "+WORKTREE") {
		t.Errorf("changes row shows the wrong diff:\n%s", body)
	}
	if strings.Contains(body, "+STAGED") {
		t.Errorf("changes row still shows the staged diff:\n%s", body)
	}
}

// Staging the unstaged row leaves nothing on that side, and the staged row now
// holds both edits. A verb that runs against the wrong side is silent: the
// screen redraws and the reader has to read the numbers to notice.
func TestScenarioStagingTheUnstagedRowEmptiesThatSide(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	s := newScenario(t, repoWithAPathOnBothSides(t))
	s.cursorTo("changes", "both.txt")
	s.key('s')
	s.reload()

	if row := s.sectionRow("changes", "both.txt"); row != "" {
		t.Errorf("both.txt is still unstaged after s: %q", row)
	}
	if row := s.sectionRow("staged", "both.txt"); row == "" {
		t.Errorf("both.txt left the staged side too\n%s", strings.Join(s.screen(), "\n"))
	}
	if row := s.sectionRow("changes", "other.txt"); row == "" {
		t.Errorf("s staged other.txt as well\n%s", strings.Join(s.screen(), "\n"))
	}
}

// Unstaging the staged row moves that edit to the unstaged side, where the
// path already had a row. Both edits end up in one row.
func TestScenarioUnstagingTheStagedRowEmptiesThatSide(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	s := newScenario(t, repoWithAPathOnBothSides(t))
	s.cursorTo("staged", "both.txt")
	s.key('u')
	s.reload()

	if row := s.sectionRow("staged", "both.txt"); row != "" {
		t.Errorf("both.txt is still staged after u: %q", row)
	}
	if row := s.sectionRow("changes", "both.txt"); row == "" {
		t.Errorf("both.txt left the unstaged side too\n%s", strings.Join(s.screen(), "\n"))
	}
}

// A tick on one row must send the verb to that row only. Ticking the staged
// copy and pressing s would otherwise stage the unstaged copy as well.
func TestScenarioAVerbRunsOnTheTickedRowOnly(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	s := newScenario(t, repoWithAPathOnBothSides(t))
	s.cursorTo("changes", "other.txt")
	s.key(' ')
	s.cursorTo("changes", "other.txt")
	s.key('s')
	s.reload()

	if row := s.sectionRow("changes", "other.txt"); row != "" {
		t.Errorf("other.txt is still unstaged: %q", row)
	}
	if row := s.sectionRow("changes", "both.txt"); row == "" {
		t.Errorf("staging the ticked row also staged both.txt\n%s", strings.Join(s.screen(), "\n"))
	}
}

// Diff follows the cursor row, so multi-select s must not fall through to
// block stage and stage only the row under the cursor.
func TestScenarioTickingTwoRowsSendsTheVerbToBoth(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	s := newScenario(t, repoWithAPathOnBothSides(t))
	s.cursorTo("changes", "both.txt")
	s.key(' ')
	s.cursorTo("changes", "other.txt")
	s.key(' ')
	s.key('s')
	s.reload()

	if row := s.sectionRow("changes", "both.txt"); row != "" {
		t.Errorf("both.txt is still unstaged: %q", row)
	}
	if row := s.sectionRow("changes", "other.txt"); row != "" {
		t.Errorf("other.txt is still unstaged: %q", row)
	}
	if row := s.sectionRow("staged", "both.txt"); row == "" {
		t.Errorf("both.txt did not appear in staged\n%s", strings.Join(s.screen(), "\n"))
	}
	if row := s.sectionRow("staged", "other.txt"); row == "" {
		t.Errorf("other.txt did not appear in staged\n%s", strings.Join(s.screen(), "\n"))
	}
}

// clickAt sends a click at the cell where sub appears on the named section's
// row, which is what the reader does with the pointer.
func (s *scenario) clickAt(section, name, sub string) {
	s.t.Helper()
	row := s.sectionRow(section, name)
	if row == "" {
		s.t.Fatalf("no %s row %q", section, name)
	}
	y := -1
	for i, line := range s.screen() {
		if line == row {
			y = i
			break
		}
	}
	at := strings.Index(row, sub)
	if y < 0 || at < 0 {
		s.t.Fatalf("cannot place a click on %q at %q", row, sub)
	}
	x := layout.Renderer{}.Of(row[:at])
	s.send(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
}

func TestScenarioClickingARowOpensThatSidesDiff(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	s := newScenario(t, repoWithAPathOnBothSides(t))
	s.clickAt("staged", "both.txt", "both.txt")
	if body := s.diffBody(); !strings.Contains(body, "+STAGED") {
		t.Errorf("clicking the staged row opened the wrong diff:\n%s", body)
	}
	s.clickAt("changes", "both.txt", "both.txt")
	if body := s.diffBody(); !strings.Contains(body, "+WORKTREE") {
		t.Errorf("clicking the changes row opened the wrong diff:\n%s", body)
	}
}

func TestScenarioClickingACheckboxTicksThatRowOnly(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	s := newScenario(t, repoWithAPathOnBothSides(t))
	s.cursorTo("staged", "both.txt")
	s.clickAt("staged", "both.txt", "[ ]")

	if row := s.sectionRow("staged", "both.txt"); !strings.Contains(row, "[✓]") {
		t.Errorf("the staged row is not ticked: %q", row)
	}
	if row := s.sectionRow("changes", "both.txt"); strings.Contains(row, "[✓]") {
		t.Errorf("the changes row was ticked too: %q", row)
	}
}

// Discard takes the whole path, not the row. Stash and discard do not care
// which section a row came from. Standing on the unstaged row and
// pressing x therefore removes the staged edit too, and the confirmation says
// +2 -2 where that row alone is +1 -1.
func TestScenarioDiscardOnEitherRowTakesTheWholePath(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	s := newScenario(t, repoWithAPathOnBothSides(t))
	s.cursorTo("changes", "both.txt")
	s.key('x')
	if s.m.state.Changes.DiscardConfirm == nil {
		t.Fatal("x did not ask for confirmation")
	}
	if got := s.m.state.Changes.DiscardConfirm; got.Added != 2 || got.Deleted != 2 {
		t.Errorf("confirmation says +%d -%d, want the whole path +2 -2", got.Added, got.Deleted)
	}
	s.key('y')
	s.reload()

	if row := s.sectionRow("changes", "both.txt"); row != "" {
		t.Errorf("the unstaged edit survived: %q", row)
	}
	if row := s.sectionRow("staged", "both.txt"); row != "" {
		t.Errorf("the staged edit survived, so discard became row-scoped: %q", row)
	}
	body, err := os.ReadFile(filepath.Join(s.dir, "app/both.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "STAGED") || strings.Contains(string(body), "WORKTREE") {
		t.Errorf("the file kept an edit: %q", body)
	}
}

// esc closes the diff and leaves the list alone, because the reader who folds
// the diff away still wants to see where they were.
func TestScenarioEscClosesTheDiffAndKeepsTheCursor(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	s := newScenario(t, repoWithAPathOnBothSides(t))
	s.cursorTo("changes", "other.txt")
	s.press(tea.KeyEsc)

	if body := s.diffBody(); body != "" {
		t.Errorf("the diff is still open:\n%s", body)
	}
	if row := s.sectionRow("changes", "other.txt"); !strings.HasPrefix(row, "▌") {
		t.Errorf("the cursor moved off the row: %q", row)
	}
}

// Staging an edit the reader has read must not make the row unread again on
// the side it lands on, because nothing about the content changed.
func TestScenarioStagingAReadRowKeepsItRead(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	s := newScenario(t, repoWithAPathOnBothSides(t))
	s.cursorTo("changes", "other.txt")
	s.key('r')
	if row := s.sectionRow("changes", "other.txt"); strings.Contains(row, "·") {
		t.Fatalf("r left other.txt unread: %q", row)
	}
	s.cursorTo("changes", "other.txt")
	s.key('s')
	s.reload()

	if row := s.sectionRow("staged", "other.txt"); strings.Contains(row, "·") || strings.Contains(row, "●") {
		t.Errorf("staging what was read marked it unread again: %q", row)
	}
}

// shift sends the form a terminal reports for a shifted letter: the base code
// with the modifier, not the uppercase rune.
func (s *scenario) shift(r rune) {
	s.t.Helper()
	s.send(tea.KeyPressMsg{Code: r, Text: strings.ToUpper(string(r)), Mod: tea.ModShift})
}

func (s *scenario) type_(text string) {
	s.t.Helper()
	for _, r := range text {
		s.key(r)
	}
}

// Committing empties the staged section and leaves the unstaged edits alone,
// which is the whole point of staging one side of a path.
func TestScenarioCommitTakesOnlyTheStagedSide(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	s := newScenario(t, repoWithAPathOnBothSides(t))
	s.key('c')
	if !s.m.state.Changes.MessageFocused {
		t.Fatal("c did not focus the commit message")
	}
	s.type_("keep the staged half")
	s.press(tea.KeyEnter)
	s.reload()

	if row := s.sectionRow("staged", "both.txt"); row != "" {
		t.Errorf("the staged side survived the commit: %q", row)
	}
	if row := s.sectionRow("changes", "both.txt"); row == "" {
		t.Errorf("the commit took the unstaged edit too\n%s", strings.Join(s.screen(), "\n"))
	}
	body, err := os.ReadFile(filepath.Join(s.dir, "app/both.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "WORKTREE") {
		t.Errorf("the working copy lost its edit: %q", body)
	}
}

// U puts back what the last discard removed. Without it the confirmation is the
// only thing standing between the reader and lost work.
func TestScenarioUndoPutsBackWhatDiscardRemoved(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	s := newScenario(t, repoWithAPathOnBothSides(t))
	s.cursorTo("changes", "other.txt")
	s.key('x')
	s.key('y')
	s.reload()
	if row := s.sectionRow("changes", "other.txt"); row != "" {
		t.Fatalf("the discard did not run: %q", row)
	}

	s.shift('u')
	s.reload()
	body, err := os.ReadFile(filepath.Join(s.dir, "app/other.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "EDITED") {
		t.Errorf("U did not put the edit back: %q", body)
	}
}

// Space on a heading ticks that section and nothing else, so the action bar
// count matches the boxes the reader can see.
func TestScenarioSpaceOnAHeadingTicksThatSectionOnly(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	s := newScenario(t, repoWithAPathOnBothSides(t))
	s.cursorToHeading("changes")
	s.key(' ')

	if h := s.sectionHeading("changes"); !strings.Contains(h, "[✓]") {
		t.Errorf("the changes heading is not fully ticked: %q", h)
	}
	if h := s.sectionHeading("staged"); strings.Contains(h, "[✓]") || strings.Contains(h, "[-]") {
		t.Errorf("the staged heading was ticked too: %q", h)
	}
	if row := s.sectionRow("staged", "both.txt"); strings.Contains(row, "[✓]") {
		t.Errorf("the staged row was ticked: %q", row)
	}
	if got := s.m.state.Notice; got != "" {
		t.Errorf("notice = %q", got)
	}
}

// esc clears a selection before it closes a diff, because the reader who has
// ticked rows is holding something they can lose by accident.
func TestScenarioEscClearsTheSelectionBeforeTheDiff(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	s := newScenario(t, repoWithAPathOnBothSides(t))
	s.cursorTo("changes", "other.txt")
	s.key(' ')
	s.press(tea.KeyEsc)

	if row := s.sectionRow("changes", "other.txt"); strings.Contains(row, "[✓]") {
		t.Errorf("esc left the selection: %q", row)
	}
	if body := s.diffBody(); body == "" {
		t.Error("esc closed the diff as well as clearing the selection")
	}
}

// A file taller than the diff area is not read by moving past it. The cursor
// opens the diff on its way through, and recording a block on its head line
// alone marked a hundred lines the reader never saw.
func TestScenarioPassingOverALongFileLeavesItUnread(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := repoWithAPathOnBothSides(t)
	var long strings.Builder
	for i := range 200 {
		long.WriteString("line ")
		long.WriteString(string(rune('a' + i%26)))
		long.WriteByte('\n')
	}
	if err := os.WriteFile(filepath.Join(dir, "app/long.txt"), []byte(long.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	s := newScenario(t, dir)

	s.cursorTo("changes", "long.txt")
	s.cursorTo("changes", "other.txt")

	if row := s.sectionRow("changes", "long.txt"); !strings.Contains(row, "·") {
		t.Errorf("moving past long.txt marked it read: %q", row)
	}
}

// gitRunner is the isolated git the fixtures use, kept apart from the fixture
// bodies so each one reads as the repository state it builds.
func gitRunner(t *testing.T, dir string) (run func(...string), write func(string, string)) {
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
	run = func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write = func(name, body string) {
		t.Helper()
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return run, write
}

// A pane too short for a diff gives the whole body to the list, because the
// list is what the reader steers with and cutting it strands them.
func TestScenarioAShortPaneKeepsTheWholeListAndClosesTheDiff(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	s := newScenario(t, repoWithAPathOnBothSides(t))
	s.cursorTo("changes", "other.txt")
	if s.diffBody() == "" {
		t.Fatal("the diff never opened at full height")
	}

	s.send(tea.WindowSizeMsg{Width: 77, Height: 14})
	if body := s.diffBody(); body != "" {
		t.Errorf("the diff stayed open in a 14-row pane:\n%s", body)
	}
	for _, name := range []string{"both.txt", "other.txt"} {
		if s.sectionRow("changes", name) == "" {
			t.Errorf("%s left the list when the pane shrank\n%s", name, strings.Join(s.screen(), "\n"))
		}
	}
	if s.sectionRow("staged", "both.txt") == "" {
		t.Errorf("the staged row left the list\n%s", strings.Join(s.screen(), "\n"))
	}
}

// A deleted file has no content to show and still needs a row, because the
// reader decides whether to keep the deletion.
func TestScenarioADeletedFileGetsARowAndADiff(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := t.TempDir()
	run, write := gitRunner(t, dir)
	run("init", "-q", "-b", "main")
	write("app/gone.txt", "one\ntwo\n")
	run("add", ".")
	run("commit", "-q", "-m", "base")
	if err := os.Remove(filepath.Join(dir, "app/gone.txt")); err != nil {
		t.Fatal(err)
	}
	s := newScenario(t, dir)

	row := s.sectionRow("changes", "gone.txt")
	if row == "" {
		t.Fatalf("no row for the deleted file\n%s", strings.Join(s.screen(), "\n"))
	}
	if !strings.Contains(row, "D") {
		t.Errorf("the row does not say deleted: %q", row)
	}
	s.cursorTo("changes", "gone.txt")
	if body := s.diffBody(); !strings.Contains(body, "-one") {
		t.Errorf("the deleted lines are not shown:\n%s", body)
	}
}

// A rename keeps the reader oriented by naming both sides, which is the only
// way to tell a rename from a delete plus an add.
func TestScenarioARenamedFileNamesBothSides(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := t.TempDir()
	run, write := gitRunner(t, dir)
	run("init", "-q", "-b", "main")
	write("app/before.txt", "one\ntwo\nthree\n")
	run("add", ".")
	run("commit", "-q", "-m", "base")
	run("mv", "app/before.txt", "app/after.txt")
	s := newScenario(t, dir)

	row := s.sectionRow("staged", "after.txt")
	if row == "" {
		t.Fatalf("no row for the renamed file\n%s", strings.Join(s.screen(), "\n"))
	}
	if !strings.Contains(row, "before.txt") {
		t.Errorf("the row does not name the old path: %q", row)
	}
}

// An untracked directory arrives as one entry from git status. Every file in
// it needs its own row, or the reader cannot see what the agent wrote.
func TestScenarioAnUntrackedDirectoryShowsEveryFile(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := t.TempDir()
	run, write := gitRunner(t, dir)
	run("init", "-q", "-b", "main")
	write("keep.txt", "x\n")
	run("add", ".")
	run("commit", "-q", "-m", "base")
	write("fresh/one.txt", "a\nb\n")
	write("fresh/two.txt", "c\nd\n")
	s := newScenario(t, dir)

	for _, name := range []string{"one.txt", "two.txt"} {
		if s.sectionRow("changes", name) == "" {
			t.Errorf("%s has no row\n%s", name, strings.Join(s.screen(), "\n"))
		}
	}
}

// A repository with no commits is where an agent starts, and git rev-parse HEAD
// fails there. The pane has to open anyway.
func TestScenarioARepositoryWithNoCommitsOpens(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := t.TempDir()
	run, write := gitRunner(t, dir)
	run("init", "-q", "-b", "main")
	write("first.txt", "a\nb\n")
	s := newScenario(t, dir)

	if s.sectionRow("changes", "first.txt") == "" {
		t.Errorf("no row in an empty repository\n%s", strings.Join(s.screen(), "\n"))
	}
	if got := s.m.state.Notice; got != "" {
		t.Errorf("notice = %q", got)
	}
	s.cursorTo("changes", "first.txt")
	if body := s.diffBody(); !strings.Contains(body, "+a") {
		t.Errorf("no diff for the first file:\n%s", body)
	}
}

// A list longer than the pane scrolls, and the cursor has to stay drawn or the
// reader cannot tell which row a verb will act on.
func TestScenarioALongListScrollsWithTheCursor(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := t.TempDir()
	run, write := gitRunner(t, dir)
	run("init", "-q", "-b", "main")
	for i := range 40 {
		write(fmt.Sprintf("app/f%02d.txt", i), "one\n")
	}
	run("add", ".")
	run("commit", "-q", "-m", "base")
	for i := range 40 {
		write(fmt.Sprintf("app/f%02d.txt", i), "one\nCHANGED\n")
	}
	s := newScenario(t, dir)

	for range len(s.m.state.Rows) {
		s.key('j')
	}
	last := s.m.state.Rows[s.m.state.Cursor]
	if last.Kind() != state.RowFile {
		t.Fatalf("the cursor stopped on a %v row", last.Kind())
	}
	row := s.sectionRow("changes", base(last.Path()))
	if row == "" {
		t.Fatalf("the cursor row is not drawn: %q\n%s", last.Path(), strings.Join(s.screen(), "\n"))
	}
	if !strings.HasPrefix(row, "▌") {
		t.Errorf("the cursor mark is missing from %q", row)
	}
}

func base(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[i+1:]
	}
	return p
}

// tab switches by number, the way the bar is labeled.
func (s *scenario) tab(n rune) {
	s.t.Helper()
	s.key(n)
}

// repoWithHistory has commits to look back at, and a working change so the
// changes tab is not empty when the reader comes back to it.
func repoWithHistory(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run, write := gitRunner(t, dir)
	run("init", "-q", "-b", "main")
	write("app/a.txt", "one\ntwo\n")
	run("add", ".")
	run("commit", "-q", "-m", "first commit")
	write("app/a.txt", "one\nSECOND\n")
	write("app/b.txt", "new\n")
	run("add", ".")
	run("commit", "-q", "-m", "second commit")
	write("app/a.txt", "one\nSECOND\nWORKING\n")
	return dir
}

func TestScenarioHistoryTabListsCommits(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	s := newScenario(t, repoWithHistory(t))
	s.tab('2')

	screen := strings.Join(s.screen(), "\n")
	for _, want := range []string{"first commit", "second commit"} {
		if !strings.Contains(screen, want) {
			t.Errorf("%q is missing from the history tab\n%s", want, screen)
		}
	}
}

func TestScenarioUndoOfTheOnlyCommitReturnsToChanges(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := t.TempDir()
	run, write := gitRunner(t, dir)
	run("init", "-q", "-b", "main")
	write("a.txt", "one\n")
	run("add", ".")
	run("commit", "-q", "-m", "only")

	s := newScenario(t, dir)
	s.tab('2')
	s.key('u')

	screen := strings.Join(s.screen(), "\n")
	if s.m.state.Tab != state.TabChanges {
		t.Errorf("after undoing the only commit: Tab = %v, want TabChanges", s.m.state.Tab)
	}
	if !strings.Contains(screen, "undid commit · reset") {
		t.Errorf("success notice missing from screen\n%s", screen)
	}
	if strings.Contains(screen, "exit 128") {
		t.Errorf("raw git error on screen\n%s", screen)
	}
}

// Opening a commit's file shows that commit's diff. The history rows share the
// path of a working-copy row, and the read record has to keep them apart.
func TestScenarioHistoryFileShowsTheCommitDiff(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	s := newScenario(t, repoWithHistory(t))
	s.tab('2')
	s.key('d')
	for range len(s.m.state.Rows) + 2 {
		if strings.Contains(s.diffBody(), "SECOND") {
			return
		}
		s.key('j')
	}
	t.Fatalf("never opened a commit's file diff\n%s", strings.Join(s.screen(), "\n"))
}

// The history tab offers only diff. r is a changes-tab key, so pressing it on
// a commit's file must record nothing rather than marking the working copy the
// reader never opened.
func TestScenarioHistoryOffersOnlyDiffAndRecordsNoReads(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	s := newScenario(t, repoWithHistory(t))
	s.tab('2')
	s.key('d')
	for range len(s.m.state.Rows) + 2 {
		if strings.HasSuffix(s.m.state.Open.Path, "a.txt") {
			break
		}
		s.key('j')
	}
	if s.m.state.Open.Path == "" {
		t.Fatalf("never reached a.txt in history\n%s", strings.Join(s.screen(), "\n"))
	}
	footer := s.screen()[len(s.screen())-1]
	if strings.Contains(footer, "r read") {
		t.Errorf("the history footer offers read: %q", footer)
	}
	s.key('r')
	if len(s.m.read.Finished) != 0 || len(s.m.read.Blocks) != 0 {
		t.Errorf("r recorded a read on the history tab: %v / %v",
			s.m.read.Finished, s.m.read.Blocks)
	}

	s.tab('1')
	if row := s.sectionRow("changes", "a.txt"); !strings.Contains(row, "·") {
		t.Errorf("the working copy is no longer unread: %q", row)
	}
}

// Switching tabs and coming back leaves the changes list intact, because the
// reader uses the bar to look aside and expects to find their place again.
func TestScenarioLeavingAndReturningKeepsTheChangesList(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	s := newScenario(t, repoWithHistory(t))
	before := s.sectionRow("changes", "a.txt")
	if before == "" {
		t.Fatal("no a.txt row to begin with")
	}
	s.tab('2')
	s.tab('1')

	if got := s.sectionRow("changes", "a.txt"); got == "" {
		t.Errorf("a.txt left the list after a round trip\n%s", strings.Join(s.screen(), "\n"))
	}
}

// A conflicted path offers diff and discard and nothing else, because staging
// half a conflict is not a thing the reader can mean.
func TestScenarioAConflictedFileOffersOnlyDiffAndDiscard(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := t.TempDir()
	run, write := gitRunner(t, dir)
	run("init", "-q", "-b", "main")
	write("app/c.txt", "base\n")
	run("add", ".")
	run("commit", "-q", "-m", "base")
	run("checkout", "-q", "-b", "other")
	write("app/c.txt", "other side\n")
	run("commit", "-q", "-am", "other")
	run("checkout", "-q", "main")
	write("app/c.txt", "main side\n")
	run("commit", "-q", "-am", "main")
	cmd := exec.Command("git", "merge", "other")
	cmd.Dir = dir
	cmd.Env = []string{
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
	// The merge is meant to fail; a clean merge means the fixture built no
	// conflict and the test below would pass without testing anything.
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("the merge did not conflict: %s", out)
	}
	s := newScenario(t, dir)
	s.cursorTo("changes", "c.txt")

	row := s.sectionRow("changes", "c.txt")
	if !strings.Contains(row, "U ") {
		t.Errorf("the row does not say unmerged: %q", row)
	}
	// The row bar carries the verbs that change the index. diff and read are
	// footer keys, so the row shows only discard.
	for _, verb := range []state.VerbName{state.VerbNameStage, state.VerbNameUnstage, state.VerbNameStash} {
		if strings.Contains(row, verb.String()) {
			t.Errorf("a conflicted row offers %q: %q", verb, row)
		}
	}
	if !strings.Contains(row, "discard") {
		t.Errorf("a conflicted row does not offer discard: %q", row)
	}
	footer := s.screen()[len(s.screen())-1]
	for _, want := range []string{"x discard", "d diff", "resolve in your editor"} {
		if !strings.Contains(footer, want) {
			t.Errorf("the footer is missing %q: %q", want, footer)
		}
	}
	if strings.Contains(footer, "s stage") {
		t.Errorf("the footer offers stage on a conflict: %q", footer)
	}
	if body := s.diffBody(); !strings.Contains(body, "<<<<<<<") {
		t.Errorf("the conflict markers are not shown:\n%s", body)
	}

	s.key('d')
	s.key('s')
	screen := strings.Join(s.screen(), "\n")
	if strings.Contains(screen, "git apply") {
		t.Errorf("stage on a conflicted file ran git apply:\n%s", screen)
	}
	if s.sectionRow("changes", "c.txt") == "" {
		t.Errorf("c.txt disappeared from changes after stage")
	}
	if s.sectionRow("staged", "c.txt") != "" {
		t.Errorf("c.txt was staged: %q", s.sectionRow("staged", "c.txt"))
	}
}

func TestScenarioStashedTabListsAndReadsAStash(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := t.TempDir()
	run, write := gitRunner(t, dir)
	run("init", "-q", "-b", "main")
	write("app/a.txt", "one\n")
	run("add", ".")
	run("commit", "-q", "-m", "base")
	write("app/a.txt", "one\nSTASHED\n")
	run("stash", "push", "-m", "work in progress")
	write("app/a.txt", "one\nAFTER\n")
	s := newScenario(t, dir)

	s.tab('3')
	screen := strings.Join(s.screen(), "\n")
	if !strings.Contains(screen, "work in progress") {
		t.Fatalf("the stash is missing\n%s", screen)
	}
	s.key('r')
	if strings.Contains(s.screen()[headingRow(s, "work in progress")], "·") {
		t.Errorf("r did not mark the stash read: %q", s.screen()[headingRow(s, "work in progress")])
	}
}

func headingRow(s *scenario, want string) int {
	s.t.Helper()
	for i, line := range s.screen() {
		if strings.Contains(line, want) {
			return i
		}
	}
	s.t.Fatalf("no line holds %q", want)
	return -1
}

// The worktrees tab hides itself when the repository has only the main tree,
// because that is every repository and the tab would never say anything.
func TestScenarioWorktreesTabAppearsOnlyWithASecondTree(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := t.TempDir()
	run, write := gitRunner(t, dir)
	run("init", "-q", "-b", "main")
	write("a.txt", "one\n")
	run("add", ".")
	run("commit", "-q", "-m", "base")
	s := newScenario(t, dir)
	if strings.Contains(strings.Join(s.screen(), "\n"), "worktrees") {
		t.Errorf("the worktrees tab is drawn for a lone tree\n%s", strings.Join(s.screen(), "\n"))
	}

	run("worktree", "add", "-q", "-b", "side", dir+"/../side-tree")
	s2 := newScenario(t, dir)
	if !strings.Contains(strings.Join(s2.screen(), "\n"), "worktrees") {
		t.Fatalf("the worktrees tab is missing with two trees\n%s", strings.Join(s2.screen(), "\n"))
	}
	s2.tab('4')
	if !strings.Contains(strings.Join(s2.screen(), "\n"), "side") {
		t.Errorf("the second tree is not listed\n%s", strings.Join(s2.screen(), "\n"))
	}
}

// Shift+j extends the selection from the cursor without clearing what was
// already ticked, because a reader building a set loses it otherwise.
func TestScenarioRangeSelectAddsRowsAndKeepsTheOldOnes(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := t.TempDir()
	run, write := gitRunner(t, dir)
	run("init", "-q", "-b", "main")
	for _, n := range []string{"a", "b", "c", "d"} {
		write("app/"+n+".txt", "one\n")
	}
	run("add", ".")
	run("commit", "-q", "-m", "base")
	for _, n := range []string{"a", "b", "c", "d"} {
		write("app/"+n+".txt", "one\nEDITED\n")
	}
	s := newScenario(t, dir)

	s.cursorTo("changes", "d.txt")
	s.key(' ')
	s.cursorTo("changes", "a.txt")
	s.shift('j')
	s.shift('j')

	for _, n := range []string{"a.txt", "b.txt", "c.txt", "d.txt"} {
		if row := s.sectionRow("changes", n); !strings.Contains(row, "[✓]") {
			t.Errorf("%s is not ticked: %q", n, row)
		}
	}
}

// A binary file has no lines to show, and the pane still has to say what it is
// rather than drawing an empty diff.
func TestScenarioABinaryFileSaysSoInsteadOfShowingLines(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := t.TempDir()
	run, write := gitRunner(t, dir)
	run("init", "-q", "-b", "main")
	write("keep.txt", "x\n")
	run("add", ".")
	run("commit", "-q", "-m", "base")
	if err := os.WriteFile(filepath.Join(dir, "app.bin"), []byte{0, 1, 2, 0, 255, 0}, 0o644); err != nil {
		t.Fatal(err)
	}
	s := newScenario(t, dir)

	if s.sectionRow("changes", "app.bin") == "" {
		t.Fatalf("no row for the binary file\n%s", strings.Join(s.screen(), "\n"))
	}
	s.cursorTo("changes", "app.bin")
	if body := s.diffBody(); !strings.Contains(body, "bytes") {
		t.Errorf("the binary diff does not say its size:\n%s", body)
	}
}

// A mode change carries no lines either, and the reader still decides whether
// to keep it.
func TestScenarioAModeChangeSaysSo(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := t.TempDir()
	run, write := gitRunner(t, dir)
	run("init", "-q", "-b", "main")
	write("app/run.sh", "echo hi\n")
	run("add", ".")
	run("commit", "-q", "-m", "base")
	if err := os.Chmod(filepath.Join(dir, "app/run.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	s := newScenario(t, dir)

	if s.sectionRow("changes", "run.sh") == "" {
		t.Fatalf("no row for the mode change\n%s", strings.Join(s.screen(), "\n"))
	}
	s.cursorTo("changes", "run.sh")
	if body := s.diffBody(); !strings.Contains(body, "mode") {
		t.Errorf("the diff does not say the mode changed:\n%s", body)
	}
}

// The agent writes while the reader is looking. A reload has to pick the change
// up rather than leaving the pane on content that no longer exists.
func TestScenarioAWriteWhileOpenRedrawsTheDiff(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := t.TempDir()
	run, write := gitRunner(t, dir)
	run("init", "-q", "-b", "main")
	write("app/a.txt", "one\ntwo\n")
	run("add", ".")
	run("commit", "-q", "-m", "base")
	write("app/a.txt", "one\nFIRST\n")
	s := newScenario(t, dir)
	s.cursorTo("changes", "a.txt")
	if body := s.diffBody(); !strings.Contains(body, "+FIRST") {
		t.Fatalf("the first edit is not shown:\n%s", body)
	}

	write("app/a.txt", "one\nSECOND\n")
	s.send(tea.KeyPressMsg{Code: 'r', Text: "R", Mod: tea.ModShift})

	body := s.diffBody()
	if !strings.Contains(body, "+SECOND") {
		t.Errorf("the new content is not shown:\n%s", body)
	}
	if strings.Contains(body, "+FIRST") {
		t.Errorf("the old content is still shown:\n%s", body)
	}
}

// A verb that git refuses has to say so on the summary line. A silent failure
// leaves the reader pressing a key that never moves the tree.
func TestScenarioAFailedVerbShowsANotice(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := t.TempDir()
	run, write := gitRunner(t, dir)
	run("init", "-q", "-b", "main")
	write("app/a.txt", "one\n")
	run("add", ".")
	run("commit", "-q", "-m", "base")
	write("app/a.txt", "one\nEDITED\n")
	s := newScenario(t, dir)
	s.cursorTo("changes", "a.txt")

	// An index git cannot write is the shortest way to make a real verb fail.
	if err := os.WriteFile(filepath.Join(dir, ".git/index.lock"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	s.key('s')

	if s.m.state.Notice == "" {
		t.Errorf("staging failed silently\n%s", strings.Join(s.screen()[:6], "\n"))
	}
}

// clickText clicks the first screen line holding want, at the cell where sub
// starts on it.
func (s *scenario) clickText(want, sub string) {
	s.t.Helper()
	for y, line := range s.screen() {
		if !strings.Contains(line, want) {
			continue
		}
		at := strings.Index(line, sub)
		if at < 0 {
			continue
		}
		s.send(tea.MouseClickMsg{X: layout.Renderer{}.Of(line[:at]), Y: y, Button: tea.MouseLeft})
		return
	}
	s.t.Fatalf("no line holds %q and %q\n%s", want, sub, strings.Join(s.screen(), "\n"))
}

// Staging one block leaves the rest of the file unstaged, which is the whole
// reason the offer sits on the block rather than on the row.
func TestScenarioStagingOneBlockLeavesTheOthersUnstaged(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := t.TempDir()
	run, write := gitRunner(t, dir)
	run("init", "-q", "-b", "main")
	var base strings.Builder
	for i := range 40 {
		fmt.Fprintf(&base, "line %d\n", i)
	}
	write("app/a.txt", base.String())
	run("add", ".")
	run("commit", "-q", "-m", "base")
	edited := strings.Replace(base.String(), "line 0\n", "TOP\n", 1)
	edited = strings.Replace(edited, "line 39\n", "BOTTOM\n", 1)
	write("app/a.txt", edited)
	s := newScenario(t, dir)
	s.cursorTo("changes", "a.txt")
	if n := len(s.m.state.Open.Diff.Blocks); n != 2 {
		t.Fatalf("the fixture made %d blocks, want 2", n)
	}

	s.clickText("block 1/2", "s stage")
	s.reload()

	if s.sectionRow("staged", "a.txt") == "" {
		t.Fatalf("nothing was staged\n%s", strings.Join(s.screen(), "\n"))
	}
	if s.sectionRow("changes", "a.txt") == "" {
		t.Errorf("the whole file was staged, not one block\n%s", strings.Join(s.screen(), "\n"))
	}
	// The drawn counts go quiet once the row is stale, so the index itself is
	// what says which block landed.
	cached := gitOutput(t, dir, "diff", "--cached")
	if !strings.Contains(cached, "+TOP") {
		t.Errorf("the clicked block is not in the index:\n%s", cached)
	}
	if strings.Contains(cached, "+BOTTOM") {
		t.Errorf("the other block went in too:\n%s", cached)
	}
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = []string{
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
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// A commit's file rows have to say how big each change was and what happened
// to the file. Without them the reader has to open every file to find the one
// the commit is actually about.
func TestScenarioHistoryFileRowsCarryCountsAndStatus(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	s := newScenario(t, repoWithHistory(t))
	s.tab('2')
	s.key('d')

	screen := s.screen()
	var aRow, bRow string
	for _, line := range screen {
		if strings.Contains(line, "a.txt") {
			aRow = line
		}
		if strings.Contains(line, "b.txt") {
			bRow = line
		}
	}
	// git show --numstat says 1 1 for a.txt and 1 0 for b.txt.
	if !strings.Contains(aRow, "+1") || !strings.Contains(aRow, "−1") {
		t.Errorf("a.txt does not carry its counts: %q", aRow)
	}
	if !strings.Contains(bRow, "+1") || !strings.Contains(bRow, "−0") {
		t.Errorf("b.txt does not carry its counts: %q", bRow)
	}
	if !strings.Contains(bRow, "A ") {
		t.Errorf("b.txt was added and the row does not say so: %q", bRow)
	}
}

// Nothing on the history tab acts on a selection. A commit's file rows have no
// select verb, and the footer offers only diff. Ticking one hid
// the row's own verb and left a count the reader could not spend.
func TestScenarioSpaceOnAHistoryRowDoesNothing(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	s := newScenario(t, repoWithHistory(t))
	s.tab('2')
	s.key('d')
	for range len(s.m.state.Rows) + 2 {
		if strings.HasSuffix(s.m.state.Open.Path, "a.txt") {
			break
		}
		s.key('j')
	}
	var before string
	for _, line := range s.screen() {
		if strings.HasPrefix(line, "▌") {
			before = line
			break
		}
	}
	if before == "" {
		t.Fatal("no row carries the cursor")
	}
	s.key(' ')

	if n := len(s.m.state.Changes.Selected); n != 0 {
		t.Errorf("space selected %d rows on the history tab: %v", n, s.m.state.Changes.Selected)
	}
	var after string
	for _, line := range s.screen() {
		if strings.HasPrefix(line, "▌") {
			after = line
			break
		}
	}
	if after == "" {
		t.Fatal("no row carries the cursor after space")
	}
	if after != before {
		t.Errorf("the row changed:\n before %q\n after  %q", before, after)
	}
	if footer := s.screen()[len(s.screen())-1]; strings.Contains(footer, "selected") {
		t.Errorf("the footer counts a selection that nothing can spend: %q", footer)
	}
}

// The box only appears where something can fill it. Drawing one on a tab with
// no selection verb offers the reader a control that never changes.
func TestScenarioHistoryRowsDrawNoCheckbox(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	s := newScenario(t, repoWithHistory(t))
	s.tab('2')
	s.key('d')
	for range len(s.m.state.Rows) + 2 {
		if strings.HasSuffix(s.m.state.Open.Path, "a.txt") {
			break
		}
		s.key('j')
	}
	var row string
	for _, line := range s.screen() {
		if strings.HasPrefix(line, "▌") {
			row = line
			break
		}
	}
	if row == "" {
		t.Fatal("no row carries the cursor")
	}
	if strings.Contains(row, "[ ]") {
		t.Errorf("a history row draws a box nothing can fill: %q", row)
	}
	if got := (layout.Renderer{}).Of(row); got != 77 {
		t.Errorf("the row is %d cells, want 77: %q", got, row)
	}
}

func repoBasic(t *testing.T) string {
	t.Helper()
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
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, path)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, path), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "-b", "main")
	write("a.txt", "one\ntwo\nthree\n")
	write("src/c.go", "package main\n")
	write("docs/d.md", "# doc\n")
	run("add", "-A")
	run("-c", "user.email=t@example.com", "-c", "user.name=t", "commit", "-m", "init")
	write("a.txt", "one\nTWO\nthree\n")
	write("b.txt", "new\n")
	write("src/c.go", "package main\n\nfunc main() {}\n")
	run("add", "src/c.go")
	if err := os.Remove(filepath.Join(dir, "docs/d.md")); err != nil {
		t.Fatal(err)
	}
	return dir
}

func gitShortStatus(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "status", "--short")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git status --short: %v\n%s", err, out)
	}
	return string(out)
}

// Standing on a section heading must not stage the whole repository.
func TestScenarioSectionHeadingStageStaysInSection(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := repoBasic(t)
	before := gitShortStatus(t, dir)
	s := newScenario(t, dir)
	s.cursorToHeading("changes")
	footer := s.screen()[len(s.screen())-1]
	if strings.Contains(footer, "s stage") {
		t.Fatalf("footer should not offer stage on heading: %q", footer)
	}
	s.key('s')
	if after := gitShortStatus(t, dir); after != before {
		t.Fatalf("s on the changes heading changed the index\nbefore:\n%s\nafter:\n%s", before, after)
	}

	s.cursorToHeading("staged")
	before = gitShortStatus(t, dir)
	s.key('u')
	if after := gitShortStatus(t, dir); after != before {
		t.Fatalf("u on the staged heading changed the index\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// U after a post-discard edit asks before overwriting. y puts the worktree back
// to what discard removed, not to the edit the reader made afterward.
func TestScenarioUndoConfirmYRestoresWorktree(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	s := newScenario(t, repoWithAPathOnBothSides(t))
	s.cursorTo("changes", "other.txt")
	s.key('x')
	s.key('y')
	s.reload()
	if row := s.sectionRow("changes", "other.txt"); row != "" {
		t.Fatalf("the discard did not run: %q", row)
	}
	if err := os.WriteFile(filepath.Join(s.dir, "app/other.txt"), []byte("alpha\nEDITED-AFTER\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.shift('u')
	if s.m.state.Changes.UndiscardConfirm == nil {
		t.Fatal("U should ask before overwriting post-discard edits")
	}
	s.key('y')
	s.reload()
	if s.m.state.Changes.UndiscardConfirm != nil {
		t.Fatal("the confirmation should be cleared after y")
	}
	body, err := os.ReadFile(filepath.Join(s.dir, "app/other.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "EDITED") || strings.Contains(string(body), "EDITED-AFTER") {
		t.Errorf("y did not restore the pre-discard edit: %q", body)
	}
}

// esc closes the undo confirmation and leaves the post-discard worktree alone.
func TestScenarioUndoConfirmEscKeepsTheDiscard(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	s := newScenario(t, repoWithAPathOnBothSides(t))
	s.cursorTo("changes", "other.txt")
	s.key('x')
	s.key('y')
	s.reload()
	if err := os.WriteFile(filepath.Join(s.dir, "app/other.txt"), []byte("alpha\nEDITED-AFTER\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.shift('u')
	if s.m.state.Changes.UndiscardConfirm == nil {
		t.Fatal("U should ask before overwriting post-discard edits")
	}
	s.press(tea.KeyEsc)
	if s.m.state.Changes.UndiscardConfirm != nil {
		t.Fatal("the confirmation should be cleared after esc")
	}
	body, err := os.ReadFile(filepath.Join(s.dir, "app/other.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "EDITED-AFTER") {
		t.Errorf("esc should keep the post-discard edit: %q", body)
	}
}

func repoWithConflictingStash(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run, write := gitRunner(t, dir)
	run("init", "-q", "-b", "main")
	write("a.txt", "base\n")
	run("add", "a.txt")
	run("commit", "-q", "-m", "base")
	write("a.txt", "stashed\n")
	run("stash", "push", "-q", "-m", "hold")
	write("a.txt", "local\n")
	run("add", "a.txt")
	run("commit", "-q", "-m", "diverge")
	return dir
}

// b on a conflicting stash runs git stash branch; the new branch name has to
// show up in git branch --list, not only in a notice line.
func TestScenarioStashBranchAppearsInGitBranchList(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	s := newScenario(t, repoWithConflictingStash(t))
	s.tab('3')
	s.key('b')
	s.reload()
	branches := gitOutput(t, s.dir, "branch", "--list")
	if !strings.Contains(branches, "main-stash") {
		t.Errorf("git branch --list is missing main-stash:\n%s", branches)
	}
}

func repoWithLinkedWorktree(t *testing.T) (mainDir, sidePath string) {
	t.Helper()
	dir := t.TempDir()
	run, write := gitRunner(t, dir)
	run("init", "-q", "-b", "main")
	write("a.txt", "one\n")
	run("add", ".")
	run("commit", "-q", "-m", "base")
	side := filepath.Join(filepath.Dir(dir), "side-wt")
	run("worktree", "add", "-q", "-b", "side", side)
	return dir, side
}

// x on a linked worktree calls git worktree remove without a confirmation step.
func TestScenarioWorktreeRemoveLeavesGitWorktreeList(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	mainDir, sidePath := repoWithLinkedWorktree(t)
	s := newScenario(t, mainDir)
	s.tab('4')
	for range len(s.m.state.Rows) + 2 {
		row := s.m.state.Rows[s.m.state.Cursor]
		if row.Kind() == state.RowWorktree && !row.Worktree().Main {
			break
		}
		s.key('j')
	}
	s.key('x')
	s.key('y')
	s.reload()
	list := gitOutput(t, mainDir, "worktree", "list")
	if strings.Contains(list, sidePath) {
		t.Errorf("the linked worktree is still listed:\n%s", list)
	}
}
