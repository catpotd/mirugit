package layout

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// A row that does not fit drops whole columns. Cutting the line instead leaves
// the column beside the name running into it: measured, a pane 20 to 32 columns
// wide drew a worktree called "here" as "hmerges", and the width check cannot
// see it because the row is still exactly as wide as the pane.
func TestAWorktreeRowNeverCutsTheNameIntoAnotherColumn(t *testing.T) {
	t.Parallel()
	const name = "a-long-worktree-name"
	// Two trees, and the cursor on each in turn. One tree is always the one the
	// reader is standing in, and that row is never offered verbs — so a single
	// tree left the cursor row's widest shape undrawn, and the band where the
	// verbs ran into the name went unmeasured.
	for _, cursorAt := range []int{0, 1} {
		for width := minPaneWidth; width <= 90; width++ {
			s := state.State{Width: width, Height: 8}
			s = state.Apply(s, state.WorktreesLoaded{
				Worktrees: []git.WorktreeRow{
					{Path: "/w0", Name: name, Branch: "a-branch", SHA: "a", Main: true,
						Dirty: 3, Ahead: 2, Behind: 1, WroteAge: "5m", Merge: git.MergeClean},
					{Path: "/w1", Name: name, Branch: "another-branch", SHA: "b",
						Dirty: 1, Ahead: 1, WroteAge: "9m", Merge: git.MergeClean},
				}, Base: "main", Here: "/w0"})
			s = state.Apply(s, state.TabChanged{Tab: state.TabWorktrees})
			s = state.Apply(s, state.CursorMoved{By: cursorAt})

			f := Pane(s, nil, Renderer{})
			for _, row := range f.Lines[3:5] {
				plain := ansi.Strip(row)
				if !holdsTheNameWholeOrCut(plain, name) {
					t.Errorf("width %d cursor %d drew the name as something else: %q",
						width, cursorAt, plain)
				}
			}
		}
	}
}

// holdsTheNameWholeOrCut answers whether the row shows the name, or the front of
// it followed by the mark that says text was dropped.
func holdsTheNameWholeOrCut(row, name string) bool {
	if strings.Contains(row, name) {
		return true
	}
	for kept := 1; kept < len(name); kept++ {
		if strings.Contains(row, name[:kept]+ellipsis) {
			return true
		}
	}
	return false
}

// joinToFit drops columns, never parts of them. Each column holds
// characters of three bytes — ↑ ↓ … — so a version that cut a fixed number of
// bytes off the front split one and lost the mark the merge column draws.
func TestJoiningColumnsKeepsEachOneWhole(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	columns := []string{
		w.column("↑2 ↓1", worktreeAheadWidth),
		w.column("3 files", worktreeDirtyWidth),
		w.column("5m", worktreeWroteWidth),
		w.column("…", worktreeMergeWidth),
	}
	whole := map[string]bool{"": true}
	for first := range columns {
		whole[strings.Join(columns[first:], "")] = true
	}

	for room := 0; room <= 60; room++ {
		got := joinToFit(w, columns, room)
		if !whole[got] {
			t.Errorf("room %d left a part of a column: %q", room, got)
		}
		if w.Of(got) > room {
			t.Errorf("room %d kept %d columns of text", room, w.Of(got))
		}
	}
}

// A value that fills its column runs into the next one unless the column keeps
// its last cell blank: "verify-wt-long" and "feature/x" drew as
// "verify-wt-longfeature/x".
func TestWorktreeColumnsKeepAGapBetweenThem(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	line, _ := w.WorktreeRow(worktreeRowLayout{
		Info: git.WorktreeRow{
			Path: "/wt", Name: strings.Repeat("n", worktreeNameWidth),
			Branch: strings.Repeat("b", worktreeBranchWidth),
			SHA:    "abc", Dirty: 3, WroteAge: strings.Repeat("w", worktreeWroteWidth),
		},
	}, 90)
	// Pad が右側を後ろへ詰めるので、列の位置は左からの固定オフセットで決まらない。
	// 各列の中身が隣の中身とくっついていないことを、値そのもので見る。
	name := strings.Repeat("n", worktreeNameWidth)
	branch := strings.Repeat("b", worktreeBranchWidth)
	wrote := strings.Repeat("w", worktreeWroteWidth)
	for _, v := range []string{name, branch, wrote} {
		cut := w.Truncate(v, len(v)-1)
		if !strings.Contains(line, cut+" ") {
			t.Errorf("%q の直後に空白が無い: %q", cut, line)
		}
	}
}

// The dirty column counts files a tree has changed and is left empty when it
// has none: "0 files" beside a clean tree is a number the reader has to work
// out means nothing. A count that has not arrived is a different case again,
// and says so with the waiting mark.
func TestTheDirtyColumnIsEmptyOnlyForACleanTree(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name  string
		dirty int
		want  string
	}{
		{"a tree with one file changed", 1, "1 file"},
		{"a tree with several changed", 3, "3 files"},
		{"a clean tree", 0, ""},
		{"a tree whose count has not arrived", -1, "…"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := formatWorktreeDirty(git.WorktreeRow{Path: "/wt", Name: "wt", Dirty: c.dirty})
			if got != c.want {
				t.Errorf("the dirty column is %q, want %q", got, c.want)
			}
		})
	}
}
