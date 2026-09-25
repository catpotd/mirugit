package layout

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// hostile carries what a repository can put in any string git reports: a screen
// clear, a window-title write, a carriage return that redraws over the line,
// and a tab that the terminal advances on while the width math counts zero.
const hostile = "a\x1b[2Jb\x1b]0;title\x07c\rd\te\x7ff"

// sgr matches the color sequences this package writes itself. Everything left
// after removing them came from the repository.
var sgr = regexp.MustCompile("\x1b\\[[0-9;]*m")

func hostileState(tab state.Tab) state.State {
	s := state.State{Width: 77, Height: 30, Tab: state.TabChanges}
	s = state.Apply(s, state.StatusLoaded{
		Rows: []git.Entry{
			{Path: "dir" + hostile + "/a" + hostile + ".txt", Worktree: git.Modified},
			{Path: "plain.txt", Index: git.Modified},
			// A renamed file draws both of its names, by arithmetic no other
			// row runs. Without one here, the row that read its paths straight
			// out of the entry passed this test.
			{Path: "new" + hostile + ".txt", OldPath: "old" + hostile + ".txt",
				Index: git.Renamed},
			{Path: "moved" + hostile + ".txt", OldPath: "elsewhere/was" + hostile + ".txt",
				Index: git.Renamed},
		},
		Head: git.Head{Branch: "branch" + hostile},
	})
	s = state.Apply(s, state.HistoryLoaded{Commits: []git.CommitInfo{
		{SHA: "c1", ShortSHA: "c1", Subject: "subject" + hostile, Age: "1m"},
	}})
	s = state.Apply(s, state.StashedLoaded{Stashes: []git.StashRow{
		{Ref: "stash@{0}", SHA: "s0", Message: "message" + hostile, Branch: "on" + hostile},
	}})
	s = state.Apply(s, state.WorktreesLoaded{
		Worktrees: []git.WorktreeRow{
			{Path: "/w" + hostile, Name: "name" + hostile, Main: true, Branch: "b" + hostile},
		},
		Base: "main",
	})
	s = state.Apply(s, state.CommitExpanded{
		SHA:   "c1",
		Files: []git.Entry{{Path: "commit" + hostile + ".txt", Index: git.Modified}},
	})
	s = state.Apply(s, state.StashFilesLoaded{
		Ref:   "stash@{0}",
		Files: []git.Entry{{Path: "stash" + hostile + ".txt", Worktree: git.Modified}},
	})
	s = state.Apply(s, state.WorktreeFilesLoaded{
		Path:  "/w" + hostile,
		Files: []git.Entry{{Path: "tree" + hostile + ".txt", Worktree: git.Modified}},
	})
	// A notice carries git's stderr, and git quotes the branch or path it
	// failed on, so the repository reaches this line too.
	s = state.Apply(s, state.Failed{Line: "git said " + hostile})
	s = state.Apply(s, state.TabChanged{Tab: tab})
	return s
}

func withHostileDiff(s state.State) state.State {
	// The first row is a heading and has no path, so opening on it opened
	// nothing: the diff area and its header were never drawn by this test.
	path := ""
	for _, row := range s.Rows {
		if p := row.Path(); strings.Contains(p, hostile) {
			path = p
			break
		}
	}
	s = state.Apply(s, state.DiffOpened{Path: path, Origin: state.WorkingTree(state.SectionUnstaged)})
	return state.Apply(s, state.DiffLoaded{Diff: git.FileDiff{
		Path: path,
		Blocks: []git.Block{{
			Header: "@@ -1 +1 @@ " + hostile,
			Lines:  []string{"+" + hostile, "-plain"},
			Hash:   "h1",
		}},
	}})
}

// A repository is data, not a program. Its file names, branches, commit
// subjects and diff lines used to reach the terminal byte for byte, so opening
// a cloned repository could clear the screen or rewrite the window title.
func TestRepositoryTextNeverReachesTheTerminalAsControl(t *testing.T) {
	t.Parallel()
	for _, c := range framesUnderTest(t) {
		for i, line := range c.frame.Lines {
			rest := sgr.ReplaceAllString(line, "")
			for _, r := range rest {
				if r < 0x20 || r == 0x7f {
					t.Errorf("%s: 行 %d に制御文字 %q が残っている\n%q",
						c.name, i, r, line)
					break
				}
			}
		}
	}
}

// Expanding a tab makes the string longer, so it has to happen before the cut
// rather than after it. A line of tabs is the case that tells the two apart:
// eight of them are eight cells before expansion and thirty-two after.
func TestExpandedTabsAreCutToThePaneWidth(t *testing.T) {
	t.Parallel()
	s := state.State{Width: 77, Height: 30, Tab: state.TabChanges}
	s = state.Apply(s, state.StatusLoaded{
		Rows: []git.Entry{{Path: "a.txt", Worktree: git.Modified}},
		Head: git.Head{Branch: "main"},
	})
	s = state.Apply(s, state.DiffOpened{Path: "a.txt", Origin: state.WorkingTree(state.SectionUnstaged)})
	s = state.Apply(s, state.DiffLoaded{Diff: git.FileDiff{
		Path: "a.txt",
		Blocks: []git.Block{{
			Header: "@@ -1 +1 @@",
			Lines:  []string{"+" + strings.Repeat("\t", 30) + "end"},
			Hash:   "h1",
		}},
	}})
	f := Pane(s, nil, Renderer{})
	for i, line := range f.Lines {
		if got := (Widths{}).Of(sgr.ReplaceAllString(line, "")); got != 77 {
			t.Errorf("行 %d は %d セル, want 77\n%q", i, got, line)
		}
	}
}

type namedFrame struct {
	name  string
	frame Frame
}

func framesUnderTest(t *testing.T) []namedFrame {
	t.Helper()
	var out []namedFrame
	for _, tab := range []state.Tab{
		state.TabChanges, state.TabHistory, state.TabStashed, state.TabWorktrees,
	} {
		s := hostileState(tab)
		name := state.Facts[tab].Name
		out = append(out, namedFrame{name, Pane(s, nil, Renderer{})})
		if tab == state.TabChanges {
			out = append(out, namedFrame{name + "+diff", Pane(withHostileDiff(s), nil, Renderer{})})
		}
		out = append(out, namedFrame{
			fmt.Sprintf("%s+help", name),
			Pane(state.Apply(s, state.HelpOpened{}), nil, Renderer{}),
		})
	}
	return out
}

// Printable has to leave ordinary text alone, or every row would be rewritten.
func TestPrintableLeavesOrdinaryTextAlone(t *testing.T) {
	t.Parallel()
	for _, s := range []string{
		"internal/layout/diff.go", "feature/tab-facts", "日本語のファイル名.txt",
		"emoji-👨‍👩‍👧‍👦.txt", "", "a b  c",
	} {
		if got := Printable(s); got != s {
			t.Errorf("Printable(%q) = %q", s, got)
		}
	}
}

func TestPrintableReplacesControlCharacters(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ in, want string }{
		{"a\x1bb", "a·b"},
		{"a\x07b", "a·b"},
		{"a\rb", "a·b"},
		{"a\x00b", "a·b"},
		{"a\x7fb", "a·b"},
		{"\ta", "    a"},
		{"ab\tc", "ab  c"},
		{"abcd\te", "abcd    e"},
		// A space is not a control character. It only reaches the rewriting
		// loop beside one, and a row whose spaces come back as marks is
		// unreadable where the name it holds was fine.
		{"a b\x1bc", "a b·c"},
		{"a b\tc", "a b c"},
		{" \x00 ", " · "},
		// A string of nothing but deletes: every other case here holds an
		// ordinary character too, and a test that decides on those characters
		// rather than on the delete passes without ever looking at it. Drawn
		// raw, the delete measures nothing and the row comes out a cell short.
		{"\x7f", "·"},
		{"\x7f\x7f", "··"},
	} {
		if got := Printable(c.in); got != c.want {
			t.Errorf("Printable(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// Newlines split one line into two, which no caller of this package expects.
func TestPrintableReplacesNewlines(t *testing.T) {
	t.Parallel()
	got := Printable("a\nb")
	if strings.Contains(got, "\n") {
		t.Errorf("Printable(%q) = %q, 改行が残っている", "a\nb", got)
	}
}

// A path git hands over can hold bytes that are not UTF-8: with
// core.quotePath=false it passes them through as they are. The terminal draws
// such a byte as one replacement character while the width count reads it as
// nothing, so the row comes out short and the column after it is wrong.
func TestPrintableRewritesBytesThatAreNotUTF8(t *testing.T) {
	t.Parallel()
	for _, in := range []string{
		string([]byte{0xff}),
		"dir/" + string([]byte{0xc3}) + ".txt",
		string([]byte{0x80, 0x80}),
	} {
		got := Printable(in)
		if got == in {
			t.Errorf("Printable(%q) handed the bytes back as they are", in)
			continue
		}
		if !utf8.ValidString(got) {
			t.Errorf("Printable(%q) = %q, which is still not UTF-8", in, got)
		}
	}
}
