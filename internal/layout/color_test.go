package layout

import (
	"strings"
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
	"github.com/charmbracelet/x/ansi"
)

func TestDisabledPaletteReturnsPlainText(t *testing.T) {
	t.Parallel()
	p := Palette{Enabled: false}
	for _, fn := range []func(string) string{
		p.Add, p.Del, p.Mark, p.Verb, p.Faint, p.Dim, p.Rule, p.Tab, p.Cursor,
	} {
		const in = "plain"
		if got := fn(in); got != in {
			t.Errorf("disabled palette changed %q to %q", in, got)
		}
	}
}

func TestWithoutColorUnreadCursorSelectionAndStagedStayReadable(t *testing.T) {
	t.Parallel()
	w := Renderer{Palette: Palette{Enabled: false}}
	read := emptyReadMarks{}
	s := state.State{Width: paneWidth, Height: 53, Head: git.Head{Branch: "main"}, Rows: []state.Row{
		state.FileRow(git.Entry{Path: "app/a.swift", Worktree: git.Modified}, state.SectionUnstaged),
		state.FileRow(git.Entry{Path: "app/b.swift", Index: git.Modified}, state.SectionStaged),
	}, Cursor: 0, Changes: state.Changes{Folded: map[string]bool{}, Selected: map[string]bool{}}}
	s = state.Apply(s, state.SelectionToggled{Path: "app/a.swift", Section: state.SectionUnstaged})
	f := Pane(s, read, w)

	var unread, cursor, selected, stagedHeading bool
	for _, line := range f.Lines {
		if strings.Contains(line, "·") {
			unread = true
		}
		if strings.Contains(line, "▌") {
			cursor = true
		}
		if strings.Contains(line, "[✓]") {
			selected = true
		}
		if strings.Contains(line, "staged") {
			stagedHeading = true
		}
	}
	if !unread {
		t.Error("unread mark · is missing")
	}
	if !cursor {
		t.Error("cursor ▌ is missing")
	}
	if !selected {
		t.Error("selection [✓] is missing")
	}
	if !stagedHeading {
		t.Error("staged section heading is missing")
	}
}

func TestStatusWordsKeepColorWhenTheCursorIsElsewhere(t *testing.T) {
	t.Parallel()
	w := Renderer{Palette: Palette{Enabled: true}}

	mergeClean := worktreeRow("wt-merge", "feat/merge", 0, git.MergeClean)
	line, _ := w.WorktreeRow(worktreeRowLayout{Info: mergeClean}, paneWidth)
	if !strings.Contains(line, w.Add("merges")) {
		t.Errorf("non-cursor merges should use Add: %q", line)
	}

	mergeConflict := worktreeRow("wt-conf", "feat/conf", 1, git.MergeConflicts)
	mergeConflict.Conflicts = 2
	line, _ = w.WorktreeRow(worktreeRowLayout{Info: mergeConflict}, paneWidth)
	if !strings.Contains(line, w.Del("conflicts")) {
		t.Errorf("non-cursor conflicts should use Del: %q", line)
	}

	line, _ = w.StashRow(stashRowLayout{Info: stashRow("msg", git.StashApplies)}, paneWidth)
	if !strings.Contains(line, w.Add("applies")) {
		t.Errorf("non-cursor applies should use Add: %q", line)
	}

	line, _ = w.StashRow(stashRowLayout{Info: stashRow("msg", git.StashConflicts)}, paneWidth)
	if !strings.Contains(line, w.Del("conflicts")) {
		t.Errorf("non-cursor stash conflicts should use Del: %q", line)
	}

	line, _ = w.StashRow(stashRowLayout{Info: stashRow("msg", git.StashUnrelated)}, paneWidth)
	if !strings.Contains(line, w.Dim("unrelated")) {
		t.Errorf("non-cursor unrelated should use Dim: %q", line)
	}
}

func TestColorDiffEmptyContextLineKeepsFaintNumber(t *testing.T) {
	t.Parallel()
	w := Renderer{Palette: Palette{Enabled: true}}
	const (
		numPlain = "  40 "
		raw      = " "
	)
	body := numPlain + raw
	plainPadded := w.Pad(body, "", paneWidth)
	colored := w.colorDiffLineNumbered(plainPadded, numPlain, raw)

	want := w.Faint(numPlain) + w.Dim(" ")
	if !strings.Contains(colored, want) {
		t.Errorf("got %q, want to contain %q", colored, want)
	}

	plainWidth := w.Of(plainPadded)
	coloredWidth := w.Of(ansi.Strip(colored))
	if plainWidth != coloredWidth {
		t.Errorf("strip width %d != plain width %d", coloredWidth, plainWidth)
	}
}

func TestConflictsAndMergesUseDelAndAdd(t *testing.T) {
	t.Parallel()
	w := Renderer{Palette: Palette{Enabled: true}}

	mergeClean := worktreeRow("wt-merge", "feat/merge", 0, git.MergeClean)
	mergeConflict := worktreeRow("wt-conf", "feat/conf", 1, git.MergeConflicts)
	mergeConflict.Conflicts = 2

	mergeLine, _ := w.WorktreeRow(worktreeRowLayout{
		Info:  mergeClean,
		State: worktreeRowState{Cursor: true},
	}, paneWidth)
	conflictLine, _ := w.WorktreeRow(worktreeRowLayout{
		Info:  mergeConflict,
		State: worktreeRowState{Cursor: true},
	}, paneWidth)
	appliesLine, _ := w.StashRow(stashRowLayout{
		Info:  stashRow("msg", git.StashApplies),
		State: stashRowState{Cursor: true},
	}, paneWidth)
	conflictsLine, _ := w.StashRow(stashRowLayout{
		Info:  stashRow("msg", git.StashConflicts),
		State: stashRowState{Cursor: true},
	}, paneWidth)
	unrelatedLine, _ := w.StashRow(stashRowLayout{
		Info:  stashRow("msg", git.StashUnrelated),
		State: stashRowState{Cursor: true},
	}, paneWidth)

	if !strings.Contains(mergeLine, w.Add("merges")) {
		t.Errorf("merges should use Add: %q", mergeLine)
	}
	if strings.Contains(mergeLine, w.Dim("merges")) {
		t.Errorf("merges should not use Dim: %q", mergeLine)
	}
	if !strings.Contains(conflictLine, w.Del("conflicts")) {
		t.Errorf("conflicts should use Del: %q", conflictLine)
	}
	if strings.Contains(conflictLine, w.Dim("conflicts")) {
		t.Errorf("conflicts should not use Dim: %q", conflictLine)
	}
	if !strings.Contains(appliesLine, w.Add("applies")) {
		t.Errorf("applies should use Add: %q", appliesLine)
	}
	if strings.Contains(appliesLine, w.Dim("applies")) {
		t.Errorf("applies should not use Dim: %q", appliesLine)
	}
	if !strings.Contains(conflictsLine, w.Del("conflicts")) {
		t.Errorf("stash conflicts should use Del: %q", conflictsLine)
	}
	if strings.Contains(conflictsLine, w.Dim("conflicts")) {
		t.Errorf("stash conflicts should not use Dim: %q", conflictsLine)
	}
	if !strings.Contains(unrelatedLine, w.Dim("unrelated")) {
		t.Errorf("unrelated should use Dim: %q", unrelatedLine)
	}
	if strings.Contains(unrelatedLine, w.Add("unrelated")) || strings.Contains(unrelatedLine, w.Del("unrelated")) {
		t.Errorf("unrelated should not use Add or Del: %q", unrelatedLine)
	}
}

// The unread mark is one cell wide and sits in a column of its own, where color
// alone is a faint signal: the same dot separates words elsewhere on the screen.
// Weight is what makes the reader's eye find the rows they have not read.
func TestTheUnreadMarkIsDrawnBold(t *testing.T) {
	t.Parallel()
	p := Palette{Enabled: true}
	got := p.Mark("·")
	// The weight is written into the same escape as the color: "1" is the bold
	// parameter, and the color follows it.
	if !strings.Contains(got, "\x1b[1;") {
		t.Errorf("the unread mark carries no weight: %q", got)
	}
	if ansi.Strip(got) != "·" {
		t.Errorf("the mark itself came out as %q", ansi.Strip(got))
	}
}

// The mark the diff header paints is the one it drew itself: the separator
// between the block number and the unread count. A file name is the
// repository's text, and the same character in it is part of the name. Painted,
// the name reads as though it carried a mark of mirugit's own.
func TestTheDiffHeaderPaintsItsOwnMarkAndNotTheNames(t *testing.T) {
	t.Parallel()
	w := Renderer{Palette: Palette{Enabled: true, Theme: cozmic}}
	markColor := rgbEscape(cozmic.Mark)

	plain := w.DiffHeader("plain.txt", 1, 2, 6, paneWidth)
	if n := strings.Count(plain, markColor); n != 1 {
		t.Fatalf("the header paints %d marks for a plain name, want the one it "+
			"drew: %q", n, plain)
	}

	withMark := w.DiffHeader("a·b.txt", 1, 2, 6, paneWidth)
	if n := strings.Count(withMark, markColor); n != 1 {
		t.Errorf("the header paints %d marks for a name holding one, want 1: %q",
			n, withMark)
	}
	// The name comes out as it went in.
	if !strings.HasPrefix(ansi.Strip(withMark), "a·b.txt") {
		t.Errorf("the name is not drawn as it is: %q", ansi.Strip(withMark))
	}
	if !strings.HasPrefix(withMark, "a·b.txt") {
		t.Errorf("the name carries a color of ours: %q", withMark)
	}
}
