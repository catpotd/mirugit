package layout

import (
	"strings"
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// The cursor is the one mark that says where a key will act, so exactly one row
// carries it. The stashed and worktrees panes each decide this for their own
// rows, and a mutation of either put it on every row but the cursor's.
func TestOneRowCarriesTheCursorMark(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	for _, c := range []struct {
		name  string
		state func() state.State
	}{
		{"stashed", stashPaneStateWithFiles},
		{"worktrees", worktreePaneStateWithFiles},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := c.state()
			for cursor := range s.Rows {
				s.Cursor = cursor
				lines, _ := TabList(s, emptyReadMarks{}, w)
				marks := 0
				for _, line := range lines {
					if strings.Contains(line, "▌") {
						marks++
					}
				}
				if marks != 1 {
					t.Errorf("cursor %d: %d rows carry the mark\n%s",
						cursor, marks, strings.Join(lines, "\n"))
				}
			}
		})
	}
}

// A file listed under a stash or a worktree is printed one level in, so the
// kind letter lines up under the row that holds it rather than beside it.
func TestAFileUnderARowIsDrawnOneLevelIn(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := stashPaneStateWithFiles()
	s.Cursor = 0
	lines, _ := TabList(s, emptyReadMarks{}, w)
	if len(lines) < 2 {
		t.Fatalf("no file row was drawn: %d lines", len(lines))
	}

	// Indentation keeps nested files aligned across the three expanding tabs.
	changes, _ := Renderer{}.FileRow(Row{
		Entry: git.Entry{Path: "a.txt", Worktree: git.Modified},
		State: rowState{Read: state.Read},
	}, paneWidth)
	want := kindColumn(changes) + 2
	if got := kindColumn(lines[1]); got != want {
		t.Errorf("the file's kind letter is at column %d, want %d\n%s",
			got, want, strings.Join(lines, "\n"))
	}
}

// The diff area draws from the block the scroll names. A scroll past the last
// block leaves the pane blank while the file plainly has content, so the area
// answers for a scroll it cannot honor rather than drawing nothing.
func TestADiffScrolledPastItsLastBlockStillDraws(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	d := git.FileDiff{Path: "a.txt", Blocks: []git.Block{
		{Header: "@@ -1 +1 @@", Lines: []string{"+one"}, Hash: "h1"},
		{Header: "@@ -9 +9 @@", Lines: []string{"+two"}, Hash: "h2"},
	}}
	for _, c := range []struct {
		start int
		draws []string
	}{
		{0, []string{"one", "two"}},
		{1, []string{"two"}},
		{2, nil},
		{5, nil},
	} {
		lines, _ := w.DiffArea(d, 0, c.start, 0, false, paneWidth, 10)
		if len(lines) != 10 {
			t.Errorf("start %d: %d lines, want 10", c.start, len(lines))
		}
		body := strings.Join(lines, "\n")
		for _, want := range c.draws {
			if !strings.Contains(body, want) {
				t.Errorf("start %d does not draw %q:\n%s", c.start, want, body)
			}
		}
		if c.draws == nil && (strings.Contains(body, "one") || strings.Contains(body, "two")) {
			t.Errorf("start %d is past the last block but drew a line:\n%s", c.start, body)
		}
		for i, line := range lines {
			if w.Of(line) != paneWidth {
				t.Errorf("start %d line %d: %d cells, want %d", c.start, i, w.Of(line), paneWidth)
			}
		}
	}
}

// The list scrolls to keep the cursor row visible. A row exactly one line below
// the window is the case the bound is written for: reading it as visible leaves
// the cursor drawn off the pane.
func TestTheWindowFollowsACursorJustBelowIt(t *testing.T) {
	t.Parallel()
	const listRows = 5
	for _, c := range []struct {
		name      string
		top, line int
		want      int
	}{
		{"the row above the window pulls it up", 3, 2, 2},
		{"a row inside it leaves the window alone", 3, 5, 3},
		{"the last row inside it leaves the window alone", 3, 7, 3},
		{"the row just below it pushes the window down", 3, 8, 4},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := ScrollTopFor(c.top, 20, listRows, c.line); got != c.want {
				t.Errorf("top %d with the cursor on line %d becomes %d, want %d",
					c.top, c.line, got, c.want)
			}
		})
	}
}

// The commit rows keep the box out: only the changes tab has verbs that spend a
// selection, and a box the reader can aim at but never fill reads as broken.
func TestACommitRowDrawsNoSelectionBox(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	line, _ := w.CommitRow(commitRowLayout{
		Info:  git.CommitInfo{SHA: "aaa", ShortSHA: "aaa", Subject: "one", Age: "1m"},
		State: commitRowState{Cursor: true},
	}, paneWidth)
	// An absent box proves nothing about a row that was never drawn.
	if !strings.Contains(line, "one") || !strings.Contains(line, "aaa") {
		t.Fatalf("the commit row drew neither its subject nor its sha: %q", line)
	}
	if strings.Contains(line, "[ ]") || strings.Contains(line, "[✓]") {
		t.Errorf("a commit row offers a box nothing can spend: %q", line)
	}
}

// ShownBlockHashes says which blocks the reader could see, and the marks are
// recorded from it. Answering for a block the pane did not draw marks something
// read that nobody looked at, which is the one thing the read marks exist to
// get right.
func TestOnlyTheBlocksTheDiffDrewAreNamed(t *testing.T) {
	t.Parallel()
	d := git.FileDiff{Path: "a.txt", Blocks: []git.Block{
		{Header: "@@ -1 +1 @@", Lines: []string{"+one"}, Hash: "h1"},
		{Header: "@@ -9 +9 @@", Lines: []string{"+two"}, Hash: "h2"},
	}}
	for _, c := range []struct {
		name   string
		start  int
		height int
		want   []string
	}{
		{"the whole file fits", 0, 20, []string{"h1", "h2"}},
		{"scrolled to the second block", 1, 20, []string{"h2"}},
		{"scrolled past the last block", 2, 20, nil},
		{"scrolled far past the last block", 9, 20, nil},
		{"no room to draw anything", 0, 0, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := ShownBlockHashes(d, c.start, 0, c.height)
			if len(got) != len(c.want) {
				t.Fatalf("named %v, want %v", got, c.want)
			}
			for i := range c.want {
				if got[i] != c.want[i] {
					t.Errorf("named %v, want %v", got, c.want)
					return
				}
			}
		})
	}
}

// The four things that mean the diff drew no blocks, each on its own. The test
// above varies the start and the height and leaves all four false, so any two of
// them could be joined the wrong way and the reader would be told a block was
// seen on a screen that had none.
func TestNoBlockIsNamedForADiffThatDrawsNone(t *testing.T) {
	t.Parallel()
	blocks := []git.Block{{Header: "@@ -1 +1 @@", Lines: []string{"+one"}, Hash: "h1"}}

	for _, c := range []struct {
		name string
		diff git.FileDiff
		want int
	}{
		{"a diff with blocks to draw", git.FileDiff{Path: "a.txt", Blocks: blocks}, 1},
		{"bytes that are not text", git.FileDiff{Path: "a.txt", Blocks: blocks, Binary: true}, 0},
		{"nothing but a mode change", git.FileDiff{Path: "a.txt", Blocks: blocks, ModeOnly: true}, 0},
		{"a diff too long to hold", git.FileDiff{Path: "a.txt", Blocks: blocks, TooLong: true}, 0},
		{"no blocks at all", git.FileDiff{Path: "a.txt"}, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := ShownBlockHashes(c.diff, 0, 0, 20); len(got) != c.want {
				t.Errorf("named %v, want %d of them", got, c.want)
			}
		})
	}
}
