package layout

import (
	"fmt"
	"strings"

	"github.com/catpotd/mirugit/internal/git"

	"github.com/catpotd/mirugit/internal/state"
)

const (
	worktreeNameWidth   = 14
	worktreeBranchWidth = 18
	worktreeAheadWidth  = 8
	worktreeDirtyWidth  = 8
	worktreeWroteWidth  = 10
	worktreeMergeWidth  = 13
)

// column lays a value out in its column and always leaves the last cell blank,
// so a value that fills the column does not run into the one beside it.
//
// The padding is counted in cells. fmt pads a string to a count of runes, and
// the mark that says text was dropped is one rune of two cells wherever the
// terminal draws ambiguous characters wide — so a column holding a cut value
// came out one cell too wide, and the row with it.
func (w Renderer) column(value string, width int) string {
	return w.padTo(w.Truncate(Printable(value), width-1), width)
}

// padTo widens value to cells columns. fmt pads a string to a count of runes,
// and a value holding a wide character or the mark that says text was dropped
// is fewer runes than cells — so every field that pads with fmt and then sits
// beside another comes out short by the difference.
func (w Renderer) padTo(value string, cells int) string {
	return value + strings.Repeat(" ", max(cells-w.Of(value), 0))
}

type worktreeRowState struct {
	Cursor bool
	Unread bool
	Here   bool
}

type worktreeRowLayout struct {
	Info  git.WorktreeRow
	State worktreeRowState
	Verbs []state.VerbName
	Index int
}

// WorktreeRow draws one tree. Its name and branch are two columns rather than
// one subject, which is why it hands subjectRow a width for the pair: the rows
// have to line up down the list, and a name that took whatever was left would
// put every branch in a different column.
//
// It used to lay itself out. That copy is where the two defects lived that the
// other rows did not have — a name cut off the front to make the row fit, and a
// verb whose click region sat on the column beside it.
func (w Renderer) WorktreeRow(r worktreeRowLayout, width int) (string, []Region) {
	mark := ' '
	if r.State.Here {
		mark = '*'
	} else if r.State.Unread {
		mark = '·'
	}
	left := w.assembleGutter(r.State.Cursor, false, true, mark)
	merge := w.column(formatWorktreeMerge(r.Info), worktreeMergeWidth)

	// Merge is last so that it is the column joinToFit keeps longest: a tree
	// that is merging is the one thing the row's own name cannot say.
	columns := []string{
		w.column(formatWorktreeAhead(r.Info), worktreeAheadWidth),
		w.column(formatWorktreeDirty(r.Info), worktreeDirtyWidth),
		w.column(formatWorktreeWrote(r.Info), worktreeWroteWidth),
		merge,
	}
	return w.subjectRow(subjectRowInput{
		Left:         left,
		RightColumns: columns,
		// Both halves go through column, which leaves the last cell of each
		// blank: a branch that filled its column ran into what sits beside it.
		Subject: w.column(r.Info.Name, worktreeNameWidth) +
			w.column(r.Info.Branch, worktreeBranchWidth),
		SubjectWidth:    worktreeNameWidth + worktreeBranchWidth,
		Target:          Target{Kind: TargetFile, Path: r.Info.Path, Row: r.Index},
		Cursor:          r.State.Cursor,
		Verbs:           r.Verbs,
		ColorLastColumn: func() string { return w.colorWorktreeMeta(merge) },
	}, width)
}

// paintTail swaps the last cells of a plain string for an already-colored one of
// the same width. Widths.Of counts escape bytes as cells, so Pad cannot be given
// colored text and the swap has to happen after the columns are laid out.
func (w Renderer) paintTail(plain string, width int, colored string, cells int) string {
	keep := width - cells
	col := 0
	cut := -1
	w.eachCluster(plain, func(_ string, cells, offset int) bool {
		if col >= keep {
			cut = offset
			return false
		}
		col += cells
		return true
	})
	if cut >= 0 {
		return plain[:cut] + colored
	}
	return plain + colored
}

func formatWorktreeAhead(worktree git.WorktreeRow) string {
	if worktree.SHA == "" {
		return "…"
	}
	return git.FormatWorktreeAheadBehind(worktree.Ahead, worktree.Behind)
}

func formatWorktreeDirty(worktree git.WorktreeRow) string {
	if worktree.Dirty < 0 {
		return "…"
	}
	if worktree.Dirty == 0 {
		return ""
	}
	return fmt.Sprintf("%d %s", worktree.Dirty, state.FileWord(worktree.Dirty))
}

func formatWorktreeWrote(worktree git.WorktreeRow) string {
	return worktree.WroteAge
}

func formatWorktreeMerge(worktree git.WorktreeRow) string {
	switch worktree.Merge {
	case git.MergeUnknown:
		return "…"
	case git.MergeConflicts:
		if worktree.Conflicts == 1 {
			return "conflicts 1"
		}
		return fmt.Sprintf("conflicts %d", worktree.Conflicts)
	case git.MergeClean:
		if worktree.SHA == "" {
			return "…"
		}
		return "merges"
	default:
		return ""
	}
}

func (w Renderer) colorWorktreeMeta(plain string) string {
	if !w.Enabled {
		return plain
	}
	return w.colorStatusWords(plain)
}
