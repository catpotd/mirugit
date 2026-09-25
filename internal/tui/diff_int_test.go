package tui

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

func repoWithAStagedChange(t *testing.T) string {
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
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	if err := os.WriteFile(dir+"/a.txt", []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "a.txt")
	run("commit", "-q", "-m", "base")
	if err := os.WriteFile(dir+"/a.txt", []byte("one\nCHANGED\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "a.txt")
	return dir
}

// A file whose only change is staged has nothing on the worktree side, so
// asking git for the worktree diff returns an empty one and the pane draws
// nothing when the file is opened.
func TestOpeningAStagedFileAsksGitForTheStagedSide(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := repoWithAStagedChange(t)
	m, err := New(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	repo, err := git.Load(context.Background(), dir, mustGitDir(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	m.Update(repoMsgFor(t, dir, repo))

	_, cmd := m.open("a.txt")
	if cmd == nil {
		t.Fatal("opening a file issued no command")
	}
	msg, ok := cmd().(diffMsg)
	if !ok {
		t.Fatalf("want a diffMsg, got %T", cmd())
	}
	if msg.Err != nil {
		t.Fatal(msg.Err)
	}
	if len(msg.Diff.Blocks) == 0 {
		t.Error("the staged change produced no blocks, so the pane shows an empty diff")
	}
}

func repoWithUnstagedChange(t *testing.T) string {
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
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	if err := os.WriteFile(dir+"/a.txt", []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "a.txt")
	run("commit", "-q", "-m", "base")
	if err := os.WriteFile(dir+"/a.txt", []byte("one\nCHANGED\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestStartupShowsTheCursorFileDiff(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
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

	m.View()
	found := false
	for _, line := range m.frame.Lines {
		if strings.Contains(line, "block") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("startup should draw the cursor file's diff blocks")
	}
}

func repoWithTwoChangedFiles(t *testing.T) string {
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
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	for _, name := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(dir+"/"+name, []byte("one\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("add", "a.txt", "b.txt")
	run("commit", "-q", "-m", "base")
	if err := os.WriteFile(dir+"/a.txt", []byte("one\nA\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/b.txt", []byte("one\nB\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestClosedFileShowsChangedAfterRewrite(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := repoWithTwoChangedFiles(t)
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

	// Read b.txt without keeping it open.
	next, _ = m.Update(tea.KeyPressMsg{Code: 'j'})
	m = next.(*Model)
	next, cmd := m.Update(tea.KeyPressMsg{Code: 'r'})
	m = applyReadCmd(t, next.(*Model), cmd)

	if err := os.WriteFile(dir+"/b.txt", []byte("one\nB\nrewritten\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo, err = git.Load(context.Background(), dir, mustGitDir(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	next, cmd = m.Update(repoMsgFor(t, dir, repo))
	m = next.(*Model)
	_ = cmd

	line := fileLineOf(m, "b.txt")
	if !strings.Contains(line, "●") {
		t.Fatalf("want ● on a read-then-changed closed file, got %q", line)
	}
}

func TestRenameKeepsReadState(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
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
	}
	cmdGit := exec.Command("git", "mv", "a.txt", "renamed.txt")
	cmdGit.Dir = dir
	cmdGit.Env = env
	if out, err := cmdGit.CombinedOutput(); err != nil {
		t.Fatalf("git mv: %v\n%s", err, out)
	}
	repo, err = git.Load(context.Background(), dir, mustGitDir(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	next, cmd = m.Update(repoMsgFor(t, dir, repo))
	m = next.(*Model)
	_ = cmd

	// git mv stages the rename and leaves the edit unstaged, so the path now has
	// a row on each side. The read mark belongs to the row that holds the diff
	// the reader read, which is the unstaged one; the staged rename is new.
	line := lastFileLineOf(m, "renamed.txt")
	if strings.Contains(line, "·") {
		t.Fatalf("want read state after rename, got unread row %q", line)
	}
}

// The summary counts unread files and the rows draw a dot on each one, both
// from markOf. A frame where they disagree tells the reader two different
// things about the same tree.
func TestSummaryUnreadCountMatchesTheDottedRows(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := repoWithTwoChangedFiles(t)
	m, err := New(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.probe.settled = true
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 53})

	reload := func() {
		t.Helper()
		repo, err := git.Load(context.Background(), dir, mustGitDir(t, dir))
		if err != nil {
			t.Fatal(err)
		}
		next, cmd := m.Update(repoMsgFor(t, dir, repo))
		m = applyDiffCmd(t, next.(*Model), cmd)
	}
	press := func(c rune) {
		t.Helper()
		next, cmd := m.Update(tea.KeyPressMsg{Code: c})
		m = next.(*Model)
		// A key can produce a read, a diff, or nothing; feeding each message
		// back is what the runtime does and what the marks depend on.
		for range 12 {
			if cmd == nil {
				return
			}
			msg := cmd()
			if msg == nil {
				return
			}
			next, cmd = m.Update(msg)
			m = next.(*Model)
		}
	}

	reload()
	for _, keys := range [][]rune{{'r'}, {'j'}, {'r'}} {
		for _, c := range keys {
			press(c)
		}
		m.View()

		var counted int
		for _, line := range m.frame.Lines {
			if i := strings.Index(line, " unread ·"); i > 0 {
				n, err := strconv.Atoi(strings.TrimSpace(line[:i]))
				if err != nil {
					t.Fatalf("unread count in %q: %v", line, err)
				}
				counted = n
				break
			}
		}
		var dotted int
		for _, line := range m.frame.Lines {
			mark := strings.Index(line, "·")
			if mark < 0 {
				continue
			}
			rowKind := strings.TrimLeft(line[mark+len("·"):], " ")
			if strings.HasPrefix(rowKind, "M  ") {
				dotted++
			}
		}
		if counted != dotted {
			t.Errorf("after %q: summary says %d unread, %d rows carry a dot",
				string(keys), counted, dotted)
			for _, line := range m.frame.Lines[:10] {
				t.Logf("  %s", line)
			}
		}
	}
}

func repoWithFiveDistantBlocks(t *testing.T) string {
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
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	var lines []string
	for i := 1; i <= 400; i++ {
		lines = append(lines, strconv.Itoa(i))
	}
	if err := os.WriteFile(dir+"/long.txt", []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "long.txt")
	run("commit", "-q", "-m", "long")
	content, err := os.ReadFile(dir + "/long.txt")
	if err != nil {
		t.Fatal(err)
	}
	rows := strings.Split(string(content), "\n")
	for _, lineNum := range []int{10, 100, 200, 300, 390} {
		rows[lineNum-1] = "changed" + strconv.Itoa(lineNum)
	}
	if err := os.WriteFile(dir+"/long.txt", []byte(strings.Join(rows, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func frameHasBlockLabel(m *Model, label string) bool {
	m.View()
	for _, line := range m.frame.Lines {
		if strings.Contains(line, label) {
			return true
		}
	}
	return false
}

func applyKeys(t *testing.T, m *Model, key tea.KeyPressMsg) *Model {
	t.Helper()
	next, cmd := m.Update(key)
	m = next.(*Model)
	for range 12 {
		if cmd == nil {
			return m
		}
		msg := cmd()
		if msg == nil {
			return m
		}
		next, cmd = m.Update(msg)
		m = next.(*Model)
	}
	return m
}

func TestHiddenBlocksAreReachableAndReadableByKeyboard(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := repoWithFiveDistantBlocks(t)
	m, err := New(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.probe.settled = true
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 28})

	repo, err := git.Load(context.Background(), dir, mustGitDir(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	next, cmd := m.Update(repoMsgFor(t, dir, repo))
	m = next.(*Model)
	m = applyDiffCmd(t, m, cmd)

	if len(m.state.Open.Diff.Blocks) < 5 {
		t.Fatalf("want at least 5 diff blocks, got %d", len(m.state.Open.Diff.Blocks))
	}
	if frameHasBlockLabel(m, "block 5/5") {
		t.Fatal("all blocks should not fit before scrolling")
	}

	for range 4 {
		m = applyKeys(t, m, tea.KeyPressMsg{Code: 'n'})
	}
	if !frameHasBlockLabel(m, "block 5/5") {
		t.Fatal("want block 5/5 after four next-block presses")
	}

	m = applyKeys(t, m, tea.KeyPressMsg{Code: 'n', Mod: tea.ModShift})
	if !frameHasBlockLabel(m, "block 4/5") {
		t.Fatal("want block 4/5 after shift+n")
	}

	block := m.state.Open.BlockCursor
	next, cmd = m.Update(tea.KeyPressMsg{Code: 's'})
	// Staging asks whether the diff on screen still matches the file first,
	// so what it stages is read after that answer is back.
	m = applyOnce(t, next.(*Model), cmd)
	if m.state.Changes.Pending == nil || m.state.Changes.Pending.Block != block {
		t.Fatalf("want block stage for block %d, got pending %+v", block, m.state.Changes.Pending)
	}
	if cmd == nil {
		t.Fatal("stage should issue a command")
	}
	for m.state.Open.BlockCursor < len(m.state.Open.Diff.Blocks)-1 {
		m = applyKeys(t, m, tea.KeyPressMsg{Code: 'n'})
	}
	hashes := make([]string, len(m.state.Open.Diff.Blocks))
	for i, b := range m.state.Open.Diff.Blocks {
		hashes[i] = b.Hash
	}
	m = applyKeys(t, m, tea.KeyPressMsg{Code: tea.KeyEsc})
	for _, h := range hashes {
		if !m.read.BlockRead("long.txt", h) {
			t.Fatalf("block %s was not marked read", h)
		}
	}
}
