package layout

import (
	"fmt"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

type stashRowState struct {
	Cursor bool
	Unread bool
}

type stashRowLayout struct {
	Info  git.StashRow
	State stashRowState
	Verbs []state.VerbName
	Index int
}

func (w Renderer) StashRow(r stashRowLayout, width int) (string, []Region) {
	mark := ' '
	if r.State.Unread {
		mark = '·'
	}
	return w.subjectRow(subjectRowInput{
		Left:      w.assembleGutter(r.State.Cursor, false, true, mark),
		Right:     w.stashMeta(r.Info),
		Subject:   r.Info.Message,
		Target:    Target{Kind: TargetFile, Path: r.Info.Ref, Row: r.Index},
		Cursor:    r.State.Cursor,
		Verbs:     r.Verbs,
		ColorMeta: func() string { return w.colorStashMeta(r.Info) },
	}, width)
}

func (w Renderer) stashMeta(stash git.StashRow) string {
	branch := w.Truncate(Printable(stash.Branch), 4)
	status := ""
	switch stash.Status {
	case git.StashApplies:
		status = "applies"
	case git.StashConflicts:
		status = "conflicts"
	case git.StashUnknown:
		// The verdict is still being worked out; the column stays blank rather
		// than claiming an answer.
	}
	// Column widths read off the design drawing: the branch left-aligned, then
	// the file count, the age and the verdict each right-aligned so the verdict
	// lands on the last cell.
	//
	// The branch is padded in cells rather than by fmt, which counts runes: a
	// branch called 枝の名前 cut to four cells is three runes, and fmt made the
	// column five cells wide, moving every column after it one cell left. The
	// three that follow are numbers and one of a short set of words, all ASCII.
	return w.padTo(branch, 4) +
		w.padFront(formatStashFiles(stash.FileCount), 5) +
		w.padFront(stash.Age, 6) +
		w.padFront(status, 13)
}

// formatStashFiles draws nothing for a stash whose contents have not been read.
// The list a reload takes leaves the count at -1; the stashed tab fills it when
// it is opened, and a row drawn in between would otherwise say "-1f".
func formatStashFiles(n int) string {
	if n < 0 {
		return ""
	}
	return fmt.Sprintf("%df", n)
}

func (w Renderer) colorStashMeta(stash git.StashRow) string {
	plain := w.stashMeta(stash)
	if !w.Enabled {
		return plain
	}
	return w.colorStatusWords(plain)
}
