package layout

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// A frame is written straight to the terminal. One line too many scrolls the
// screen and leaves the row above it drawn twice; one column too wide wraps and
// does the same. Neither is visible in a test that reads the frame as a list of
// strings, and both are what a reader sees.
//
// The width is asserted exactly rather than as a ceiling, which is what makes
// this reach past wrapping. A row that came out short does not wrap, but it is
// short because something replaced a field with one of a different size, and
// that is a field showing the wrong thing. Measured: a row drawn with the verbs
// it was not offered came out four columns short at every width.
//
// It sweeps the shapes a pane is actually given — the four tabs, a pane too
// short to hold a list, one wide enough for every verb and one at the narrowest
// width mirugit agrees to draw — because the arithmetic that splits rows between
// the list and the diff is where a row goes missing or is counted twice.
//
// What it does not catch: a line of the right width holding the wrong words. Two
// fields of equal width swapped for each other pass here, and the tests that
// read row contents are what stop that. Nor does it reach the terminal — it
// measures the frame the way mirugit believes the terminal will draw it, so a
// terminal that disagrees about a character's width is a separate check.
func TestAFrameNeverOutgrowsTheTerminal(t *testing.T) {
	t.Parallel()
	// Both renderers, because the color path rebuilds the tail of a row and can
	// change its printable width. Escapes are stripped before measuring; Of
	// counts them as text.
	// Clipboard and Browser add verbs, and a verb list that is longer is one
	// more shape the fitting has to settle. A renderer with neither left y sha
	// and o open out of every row the sweep drew.
	for _, w := range []Renderer{
		{},
		{Palette: Palette{Enabled: true}},
		{Clipboard: true, Browser: true},
		{Palette: Palette{Enabled: true}, Clipboard: true, Browser: true},
		// The terminal decides whether an ambiguous character draws in one cell
		// or two, and mirugit asks it at startup. Every row it draws has to come
		// out the same width under either answer: measured, the two-cell answer
		// drew the rule lines twice the width of the pane.
		{Widths: Widths{EastAsian: true}},
		{Widths: Widths{EastAsian: true}, Palette: Palette{Enabled: true},
			Clipboard: true, Browser: true},
		// The other theme draws the bar with partial blocks, which is its own
		// arithmetic and its own set of ambiguous-width characters.
		{Palette: Palette{Enabled: true, Theme: ThemeByName("cozmic")}},
		{Widths: Widths{EastAsian: true},
			Palette: Palette{Enabled: true, Theme: ThemeByName("cozmic")}},
	} {
		for _, tab := range []state.Tab{
			state.TabChanges, state.TabHistory, state.TabStashed, state.TabWorktrees,
		} {
			// Two heights, not a sweep: TestEveryHeightDrawsARectangle already
			// runs height against width. What this one adds is the renderers and
			// the cursor positions, so it needs a pane that scrolls and one that
			// does not, and nothing between.
			for _, height := range []int{minPaneHeightForTest, 53} {
				for _, width := range sweptWidths() {
					// The cursor is put on each kind of row rather than at a
					// few indexes. Every index the sweep used to try landed on
					// a file row, and the heading drew its cursor mark by a
					// different path: measured, a heading under the cursor came
					// out one cell wider than the pane wherever the mark is
					// drawn in two cells.
					for _, at := range cursorRowKinds(fittingState(tab, width, height)) {
						// The diff is opened with peek. A list longer than the
						// pane keeps the pane, and this one is: without peek the
						// diff area was never drawn here, so its own arithmetic
						// met neither the renderers nor the non-ASCII names.
						s := state.Apply(fittingState(tab, width, height),
							state.CursorMoved{By: at.by})
						s = openDiffOnCursorWith(s, func(d git.FileDiff) git.FileDiff { return d })
						checkFrameFits(t, w, s, fmt.Sprintf("tab %s at %dx%d on a %s row color=%v",
							state.Facts[tab].Name, width, height, at.what, w.Enabled))
					}
				}
			}
		}
	}
}

// checkFrameFits is what every sweep asserts, so a sweep along a new axis is a
// loop and a call rather than a copy of these four checks.
func checkFrameFits(t *testing.T, w Renderer, s state.State, where string) {
	t.Helper()
	f := Pane(s, nil, w)

	if len(f.Lines) != s.Height {
		t.Errorf("%s: drew %d lines, want %d", where, len(f.Lines), s.Height)
	}
	// Color is measured with the escapes stripped, so this pair of checks would
	// pass a frame whose color had eaten a character. Every line drawn with
	// color has to strip back to the line the same renderer draws without it:
	// the coloring works by finding text in a line already laid out, and a
	// search that matched the wrong run would take text with it.
	plainRenderer := w
	plainRenderer.Enabled = false
	plain := Pane(s, nil, plainRenderer)

	for i, line := range f.Lines {
		stripped := ansi.Strip(line)
		if got := w.Of(stripped); got != s.Width {
			t.Errorf("%s: line %d is %d columns wide", where, i, got)
		}
		if w.Enabled && i < len(plain.Lines) {
			if stripped != plain.Lines[i] {
				t.Errorf("%s: line %d reads differently once colored:\n  with    %q\n  without %q",
					where, i, stripped, plain.Lines[i])
			}
		}
	}
	for _, reg := range f.Regions {
		if reg.Row < 0 || reg.Row >= len(f.Lines) {
			t.Errorf("%s: a %s region names line %d of %d",
				where, kindName(reg.Target.Kind), reg.Row, len(f.Lines))
			continue
		}
		line := ansi.Strip(f.Lines[reg.Row])
		// No region may name a column the row does not have, and none may be
		// empty. A click on an empty one cannot land; a click past the end
		// reports a column the pane never drew.
		if reg.ColEnd <= reg.ColStart || reg.ColEnd > w.Of(line) {
			t.Errorf("%s: a %s region is [%d,%d) of %d columns",
				where, kindName(reg.Target.Kind), reg.ColStart, reg.ColEnd, w.Of(line))
			continue
		}
		if holds, ok := regionHolds[reg.Target.Kind]; ok {
			drawn := drawnCells(w, line, reg.ColStart, reg.ColEnd)
			if !holds(reg.Target, drawn) {
				t.Errorf("%s: the %s region sits on %q",
					where, kindName(reg.Target.Kind), drawn)
			}
		}
	}
}

// regionHolds says what each kind of region must be drawn over. A region and
// the text it names are worked out in different places — the text where the row
// is laid out, the region where it is recorded — and every defect this table
// has caught was the two disagreeing while the row itself came out the right
// width.
//
// Every kind has an entry so that a new one cannot be added without an answer
// here. Some cover repository text, which can be anything: a file's name, a
// directory's, a tab's, the commit message. Those say so rather than being
// left out, and their bounds are checked whether or not they have a rule.
var regionHolds = map[TargetKind]func(Target, string) bool{
	TargetCheckbox: func(_ Target, drawn string) bool {
		return drawn == "[ ]" || drawn == "[✓]" || drawn == "[-]"
	},
	TargetButton: func(_ Target, drawn string) bool {
		return strings.HasPrefix(drawn, "[") && strings.HasSuffix(drawn, "]")
	},
	TargetCommit: func(_ Target, drawn string) bool {
		return strings.HasPrefix(drawn, "[commit ")
	},
	TargetHelp: func(_ Target, drawn string) bool { return drawn == "?" },
	// Equality, not containment: the verb go also reads inside every path
	// ending .go, so a region that slid onto a file name would pass a test
	// that only looked for the word.
	TargetVerb: func(target Target, drawn string) bool {
		return drawn == keyedVerb(target.Verb) || drawn == target.Verb.String()
	},
	TargetBlock: func(_ Target, drawn string) bool {
		return strings.Contains(drawn, "block ")
	},
	// A help line is the whole row, and its description is cut in a narrow
	// pane, so the key is what stays: it is at the front and names the verb the
	// line runs.
	TargetHelpLine: func(target Target, drawn string) bool {
		// A line with no verb is a blank or a note; only the ones that run
		// something have a key, and the key is at the front where the cut
		// cannot reach it.
		if target.Verb.IsZero() {
			return true
		}
		key, ok := footerKeys[target.Verb]
		if !ok {
			return true
		}
		return strings.HasPrefix(strings.TrimSpace(drawn), key)
	},
	TargetSectionHeading: func(_ Target, drawn string) bool {
		return strings.Contains(drawn, "·")
	},

	// Whatever the repository called it, or the reader typed.
	TargetFile:      anyText,
	TargetDirectory: anyText,
	TargetTab:       anyText,
	TargetCommitBox: anyText,
	// A line that carries the cursor and no target.
	TargetNone: anyText,
}

func anyText(Target, string) bool { return true }

// kindName names a target kind for a failure message. The kinds are a number in
// the frame, and a number does not say which click stopped working.
func kindName(k TargetKind) string {
	switch k {
	case TargetNone:
		return "none"
	case TargetFile:
		return "file"
	case TargetDirectory:
		return "directory"
	case TargetSectionHeading:
		return "section heading"
	case TargetTab:
		return "tab"
	case TargetBlock:
		return "diff block"
	case TargetButton:
		return "button"
	case TargetCommitBox:
		return "commit box"
	case TargetCommit:
		return "commit button"
	case TargetCheckbox:
		return "checkbox"
	case TargetVerb:
		return "verb"
	case TargetHelp:
		return "help mark"
	case TargetHelpLine:
		return "help line"
	}
	return "unnamed"
}

// minPaneHeightForTest is the shortest pane the drawing is asked for. One row
// is what a terminal can shrink to.
const minPaneHeightForTest = 1

// fittingState fills every tab with more rows than the pane can hold, which is
// where the split between the list and the diff has to give something up.
func fittingState(tab state.Tab, width, height int) state.State {
	s := state.State{Width: width, Height: height}
	s.Changes.Folded = map[string]bool{}
	s.Changes.Selected = map[string]bool{}
	s.Changes.Stale = map[string]bool{}

	// One row of each kind carries characters the terminal may draw in two
	// cells. fmt pads a string to a count of runes, Of measures cells, and the
	// two only agree while every name is ASCII: a fixture of nothing but ASCII
	// cannot see the difference anywhere.
	entries := make([]git.Entry, 40)
	for i := range entries {
		entries[i] = git.Entry{
			Path:     "pkg/dir/file" + string(rune('a'+i%26)) + ".go",
			Worktree: git.Modified,
			// Counts, because the bar is drawn from them and a row with none
			// draws no bar: the block characters it is made of are ambiguous
			// width, and neither theme's bar met that until now.
			WorktreeCount: git.Count{Added: i * 7, Deleted: i * 3},
		}
	}
	entries[2].Path = "pkg/dir/日本語のファイル名.go"
	entries[3].Path = "pkg/dir/ふりがな付きの長い名前をもつファイル.go"
	// A renamed file draws both names with an arrow between them, which is its
	// own arithmetic and no other row's. The arrow is ambiguous width.
	entries[4].OldPath = "pkg/other/the-name-it-had-before.go"
	entries[4].Index = git.Renamed
	entries[5].Path = "pkg/dir/名前を変えた.go"
	entries[5].OldPath = "pkg/dir/前の名前.go"
	entries[5].Index = git.Renamed
	// Ahead and behind together are what puts the sync button in the header,
	// and the button is a field of its own that the pane has to fit. Two of the
	// load events carry the head, so both have to say the same thing or the
	// later one takes the button back off the screen.
	head := git.Head{Branch: "a-branch-name-of-some-length", HasUpstream: true,
		Ahead: 3, Behind: 12}
	s = state.Apply(s, state.StatusLoaded{Rows: entries, Head: head})

	// Distinct keys. Rows are expanded and folded by SHA and by ref, so forty
	// rows carrying one key each meant the expanding was working on a heap of
	// duplicates and never on the row the cursor was pointing at.
	commits := make([]git.CommitInfo, 40)
	for i := range commits {
		sha := fmt.Sprintf("%07x", i+1)
		commits[i] = git.CommitInfo{SHA: sha, ShortSHA: sha, Age: "2d",
			Unpushed: i < 3, HasUpstream: true,
			Subject: "a commit subject long enough to need truncating somewhere"}
	}
	s = state.Apply(s, state.HistoryLoaded{Commits: commits})

	stashes := make([]git.StashRow, 40)
	for i := range stashes {
		stashes[i] = git.StashRow{
			Ref: fmt.Sprintf("stash@{%d}", i), SHA: fmt.Sprintf("%07x", i+1),
			Branch: "main", Age: "2d", Message: "a stash message",
			FileCount: i, Status: git.StashApplies,
		}
	}
	stashes[1].Status = git.StashConflicts
	// A file the stash would land on top of carries a label of its own beside
	// the figures, and it is another fixed-width field for the fitting to
	// settle.
	stashes[0].Collides = []string{"pkg/dir/collides.go"}
	stashes[2].Branch = "枝の名前"
	stashes[2].Message = "日本語のメッセージを持つ stash"
	s = state.Apply(s, state.StashedLoaded{Stashes: stashes, Head: head})

	trees := make([]git.WorktreeRow, 40)
	for i := range trees {
		// Distinct paths so that only one row is the one the reader is standing
		// in. Every row being "here" left g go out of every verb list, and the
		// cursor row's widest shape was never drawn.
		trees[i] = git.WorktreeRow{Path: fmt.Sprintf("/w%d", i), Name: "a-worktree-name",
			Branch: "a-branch", SHA: fmt.Sprintf("%07x", i+1), Dirty: i,
			Ahead: 2, Behind: 1, WroteAge: "5m", Merge: git.MergeClean}
	}
	trees[0].Main = true
	trees[1].Merge = git.MergeConflicts
	trees[1].Conflicts = 2
	trees[2].Name = "日本語作業木"
	trees[2].Branch = "枝の名前"
	// A tree with nothing uncommitted in it is the one x remove is offered on,
	// so without it the cursor row's two-verb shape was never drawn.
	trees[3].Dirty = 0
	s = state.Apply(s, state.WorktreesLoaded{Worktrees: trees, Base: "main", Here: "/w0"})

	s = state.Apply(s, state.TabChanged{Tab: tab})

	// A row opened to show the files inside it. History, stashed and worktrees
	// all draw nested file rows, and none of them appeared in this sweep while
	// every row stayed closed. One of the files carries a name the terminal may
	// draw wide, and one collides, which is a label of its own.
	nested := []git.Entry{
		{Path: "pkg/dir/nested.go", Index: git.Added},
		{Path: "pkg/dir/入れ子になった日本語の名前.go", Index: git.Modified},
		{Path: "pkg/dir/collides.go", Index: git.Modified},
	}
	switch tab {
	case state.TabHistory:
		s = state.Apply(s, state.CommitExpanded{SHA: commits[0].SHA, Files: nested})
	case state.TabStashed:
		s = state.Apply(s, state.StashFilesLoaded{Ref: stashes[0].Ref, Files: nested})
	case state.TabWorktrees:
		s = state.Apply(s, state.WorktreeFilesLoaded{Path: trees[0].Path, Files: nested})
	case state.TabChanges:
		// Its rows are the files themselves; there is nothing to open.
	}
	return s
}

// sweptWidths runs the narrow end one column at a time. fitRowVerbs drops one
// verb at a time, so the drawing changes shape at single-column steps, and a
// list of round numbers stepped over the whole band where it went wrong:
// measured, history wrapped at every width from 21 to 38 and stashed from 24 to
// 38, while 20, 60, 77, 110 and 200 all passed. Past the width where every
// field fits, the shape stops changing and round numbers are enough.
func sweptWidths() []int {
	// From one column, not from the width the pane agrees to draw at. Below
	// that it says "too narrow" instead, and that answer is a frame like any
	// other: as many lines as the height, each as wide as the pane.
	widths := make([]int, 0, 90+3)
	for width := 1; width <= 90; width++ {
		widths = append(widths, width)
	}
	return append(widths, 110, 160, 200)
}

// drawnCells cuts a line by drawn columns. Slicing by bytes reads the wrong
// text: … ▌ ↑ ↓ are three bytes each and one column wide.
func drawnCells(w Renderer, line string, start, end int) string {
	var out strings.Builder
	col := 0
	w.eachCluster(line, func(cluster string, cells, _ int) bool {
		if col >= start && col < end {
			out.WriteString(cluster)
		}
		col += cells
		return col < end
	})
	return out.String()
}

// openDiffOnCursorWith opens the diff on whatever row the cursor is on, letting
// the caller change it first. Three shapes exist only while a diff is open: the
// row that says its file changed under it, the diff area itself, and the two
// diffs that have no blocks to draw — a binary file, and a file whose only
// change is its mode.
//
// "stale · R reload" is offered only for the cursor row of the open file, so
// forty rows marked stale drew nothing at all before this, and the mark is
// applied after the diff loads because loading one clears it.
//
// It opens with peek because that is what makes the diff area appear here at
// all. A list longer than the pane keeps the pane, and this fixture's list is;
// peek is the one path that holds rows for the diff whatever the list is doing.
// Without it the diff met neither the renderers nor the non-ASCII names.
func openDiffOnCursorWith(s state.State, shape func(git.FileDiff) git.FileDiff) state.State {
	row, ok := state.CursorRow(s)
	if !ok {
		return s
	}
	path := row.Path()
	if path == "" {
		return s
	}
	s = state.Apply(s, state.DiffOpened{Path: path, Peek: true,
		Origin: state.WorkingTree(row.Section())})
	s = state.Apply(s, state.DiffLoaded{Diff: shape(git.FileDiff{Path: path,
		Blocks: []git.Block{{
			Header: "@@ -1,40 +1,42 @@ func aFunctionWithARatherLongName(argument int) error {",
			Lines: []string{
				"+a line of added code long enough to need cutting in a narrow pane",
				"-日本語を含む行も、狭い pane では切らなければならない",
				" a context line long enough to need cutting in a narrow pane",
			},
			Hash: "h1",
		}}})})
	// The file changed again after its diff was read. Loading a diff clears the
	// mark, so marking the row before that drew nothing: the label is offered
	// only on the cursor row of the file whose diff is open.
	return state.Apply(s, state.StaleChanged{Path: path, Stale: true})
}

// cursorRowKinds finds one cursor position per kind of row the tab holds, so a
// sweep covers the headings and the rows that open as well as the files.
func cursorRowKinds(s state.State) []struct {
	by   int
	what string
} {
	names := map[state.RowKind]string{
		state.RowFile: "file", state.RowCommit: "commit", state.RowStash: "stash",
		state.RowWorktree: "worktree", state.RowSectionHeading: "section heading",
		state.RowDirectory: "directory",
	}
	var found []struct {
		by   int
		what string
	}
	seen := map[state.RowKind]bool{}
	for by := -len(s.Rows); by < len(s.Rows); by++ {
		moved := state.Apply(s, state.CursorMoved{By: by})
		row, ok := state.CursorRow(moved)
		if !ok || seen[row.Kind()] {
			continue
		}
		seen[row.Kind()] = true
		found = append(found, struct {
			by   int
			what string
		}{by, names[row.Kind()]})
	}
	return found
}
