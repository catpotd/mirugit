package layout

import (
	"fmt"
	"strings"

	"github.com/catpotd/mirugit/internal/state"
)

// helpGap keeps the help mark clear of the last verb, so a click aimed at one
// cannot land on the other.
const helpGap = 3

var footerKeys = map[state.VerbName]string{
	state.VerbNameRestore:   "p",
	state.VerbNameBranch:    "b",
	state.VerbNameDrop:      "x",
	state.VerbNameGo:        "g",
	state.VerbNameRemove:    "x",
	state.VerbNameSelect:    "space",
	state.VerbNameFold:      "←",
	state.VerbNameUnfold:    "→",
	state.VerbNameRead:      "r",
	state.VerbNameStage:     "s",
	state.VerbNameUnstage:   "u",
	state.VerbNameDiscard:   "x",
	state.VerbNameDiff:      "d",
	state.VerbNameStash:     "z",
	state.VerbNameUndo:      "U",
	state.VerbNameUncommit:  "u",
	state.VerbNameSHA:       "y",
	state.VerbNameOpen:      "o",
	state.VerbNameNextBlock: "n",
}

// FooterKeyFor names the key the footer prints for a verb. The keyboard has to
// answer that key on the tabs where the footer prints it, so a test outside
// this package needs the pairing.
func FooterKeyFor(verb state.VerbName) (string, bool) {
	k, ok := footerKeys[verb]
	return k, ok
}

// Footer draws the verb contract for the cursor row. Only verbs that run in
// this build appear, because what is shown must be pressable.
func (w Renderer) Footer(s state.State, width int) string {
	return w.colorFooter(s, w.footerPlain(s, width))
}

func (w Renderer) footerPlain(s state.State, width int) string {
	// Each of these lines is right-aligned against the pane, and Pad lets an
	// overlong pair overflow rather than squeezing it. They are the only footer
	// text built from a fixed phrase rather than from fields fitted to the
	// pane: measured, "y discard · any other key cancels" is 33 cells and drew
	// into a pane of 20, which wrapped and pushed the row above it off screen.
	//
	// The end is what gives way. The key is at the front, and a reader who
	// cannot see the whole sentence still has to see which key answers it.
	if s.Changes.MessageFocused {
		if s.StagedFileCount() == 0 {
			return w.Pad("", w.Truncate("esc leave · stage a file first", width), width)
		}
		return w.Pad("", w.Truncate("esc leave · enter commit", width), width)
	}
	if c, ok := state.PendingConfirmation(s); ok {
		text := "y " + c.Verb.String() + " · any other key cancels"
		if c.Warning != "" {
			text += " · " + c.Warning
		}
		return w.Pad("", w.Truncate(text, width), width)
	}

	// The footer names its target only when the target is not the cursor row.
	// The design drawing puts the open file's name here instead, and pays for
	// it: the same drawing's footer has to drop "x discard" while the row above
	// it offers discard, which is the one rule this line has to keep.
	left := ""
	if n := len(s.Changes.Selected); n > 0 {
		left = fmt.Sprintf("%d selected", n)
	}

	verbs := footerVerbs(s, w)
	note := conflictFooterNote(s)
	// The target and the verbs are separate statements; without a gap they read
	// as one phrase.
	if left != "" {
		left += "  "
	}
	room := width - w.Of(left)
	right := w.footerRight(verbs, room, note)
	// A pane too narrow even for the note alone drops it: the note says where
	// to go next, and the pane it is drawn on is the way there. Keeping it
	// makes the row wider than the pane, and Pad lets an overlong pair
	// overflow, so the row wraps and pushes every row under it down.
	if w.Of(right) > room {
		right = w.footerRight(verbs, room, "")
	}
	return w.Pad(left, right, width)
}

// footerRight builds the half that holds the verbs, the conflict note and the
// mark that opens the help, fitted into the room that is left.
func (w Renderer) footerRight(verbs []state.VerbName, room int, note string) string {
	fitted := w.fitFooterVerbs(verbs, room, note)
	parts := make([]string, 0, len(fitted))
	for _, v := range fitted {
		parts = append(parts, footerKeys[v]+" "+v.String())
	}
	right := strings.Join(parts, " · ")
	if note != "" {
		if right != "" {
			right += "      "
		}
		right += note
	}
	return right + strings.Repeat(" ", helpGap) + "?"
}

func conflictFooterNote(s state.State) string {
	row, ok := state.CursorRow(s)
	if s.Tab != state.TabChanges || !ok {
		return ""
	}
	if row.Kind() != state.RowFile || !row.Entry().IsConflicted() {
		return ""
	}
	return "resolve in your editor"
}

func (w Renderer) fitFooterVerbs(verbs []state.VerbName, room int, note string) []state.VerbName {
	suffix := strings.Repeat(" ", helpGap) + "?"
	if note != "" {
		suffix = "      " + note + suffix
	}
	for n := len(verbs); n >= 0; n-- {
		parts := make([]string, 0, n)
		for _, v := range verbs[:n] {
			parts = append(parts, footerKeys[v]+" "+v.String())
		}
		line := strings.Join(parts, " · ") + suffix
		if n == 0 || w.Of(line) <= room {
			return verbs[:n]
		}
	}
	return nil
}

func (w Renderer) colorFooter(s state.State, line string) string {
	if s.Changes.MessageFocused {
		if s.StagedFileCount() == 0 {
			return w.colorFooterLine(line, nil)
		}
		return w.colorFooterLine(line, []state.VerbName{state.VerbNameCommit})
	}
	if c, ok := state.PendingConfirmation(s); ok {
		return w.colorFooterLine(line, []state.VerbName{c.Verb}, "cancels")
	}
	return w.colorFooterLine(line, footerVerbs(s, w))
}

func footerVerbs(s state.State, w Renderer) []state.VerbName {
	if state.ConfirmationOpen(s) {
		return nil
	}
	verbs := renderFor(s.Tab).FooterVerbs(s, w)
	if s.Open.Path != "" && len(s.Open.Diff.Blocks) > 1 {
		verbs = append(verbs, state.VerbNameNextBlock)
	}
	return verbs
}

func changesFooterVerbs(s state.State) []state.VerbName {
	selected := len(s.Changes.Selected) > 0
	row, ok := state.CursorRow(s)
	if !ok {
		if selected {
			return append([]state.VerbName{state.VerbNameSelect}, state.SelectionVerbs(s)...)
		}
		return []state.VerbName{state.VerbNameSelect, state.VerbNameDiff, state.VerbNameRead}
	}
	var verbs []state.VerbName
	if selected {
		verbs = append(verbs, state.VerbNameSelect)
	}
	verbs = append(verbs, changesKindVerbs(s, row, selected)...)
	if selected {
		verbs = append(verbs, state.SelectionVerbs(s)...)
	}
	return verbs
}

func changesKindVerbs(s state.State, row state.Row, selected bool) []state.VerbName {
	switch row.Kind() {
	case state.RowSectionHeading:
		if selected {
			return nil
		}
		return []state.VerbName{state.VerbNameSelect}
	case state.RowDirectory:
		if selected {
			return []state.VerbName{directoryFoldVerb(s, row)}
		}
		return []state.VerbName{state.VerbNameSelect, directoryFoldVerb(s, row)}
	case state.RowFile:
		if selected {
			return []state.VerbName{state.VerbNameDiff, state.VerbNameRead}
		}
		return changesFileVerbs(row)
	case state.RowCommit, state.RowStash, state.RowWorktree:
		// These three do not appear on the changes tab. The answer below is
		// what the old default arm gave them, so a row that somehow arrives
		// still gets a footer it can print.
	}
	if selected {
		return nil
	}
	return []state.VerbName{state.VerbNameSelect, state.VerbNameDiff, state.VerbNameRead}
}

func changesFileVerbs(row state.Row) []state.VerbName {
	core := state.ChangesRowVerbs(row)
	if row.Entry().IsConflicted() {
		return append(core, state.VerbNameDiff, state.VerbNameRead)
	}
	return append(append([]state.VerbName{state.VerbNameSelect}, core...), state.VerbNameDiff, state.VerbNameRead)
}

func directoryFoldVerb(s state.State, row state.Row) state.VerbName {
	if state.FoldedIn(s, row.Section(), row.DirPath()) {
		return state.VerbNameUnfold
	}
	return state.VerbNameFold
}

// The three tabs whose rows expand into files put the file's own verb first and
// the parent's after it. The keys act on the parent from a file row, and a key
// that runs without a word for it on this line is how a stash was dropped from a
// row whose footer said only "d diff".
func historyFooterVerbs(s state.State, w Renderer) []state.VerbName {
	return parentFooterVerbs(s, func(parent state.Row, at int) []state.VerbName {
		return commitVerbsForRemote(parent.Commit(), state.CommitIndexAt(s, at),
			s.History.RemoteURL, w.Clipboard, w.Browser)
	})
}

func stashFooterVerbs(s state.State) []state.VerbName {
	return parentFooterVerbs(s, func(parent state.Row, _ int) []state.VerbName {
		return state.StashVerbs(parent.Stash().Status)
	})
}

func worktreeFooterVerbs(s state.State) []state.VerbName {
	return parentFooterVerbs(s, func(parent state.Row, _ int) []state.VerbName {
		return state.WorktreeVerbs(parent.Worktree(), parent.Worktree().Path == s.Worktrees.Here)
	})
}

func parentFooterVerbs(s state.State, verbsOf func(parent state.Row, at int) []state.VerbName) []state.VerbName {
	var verbs []state.VerbName
	if row, ok := state.CursorRow(s); ok && row.Kind() == state.RowFile {
		verbs = append(verbs, state.VerbNameDiff)
	}
	parent, at, ok := state.Facts[s.Tab].ParentRow(s)
	if !ok {
		return verbs
	}
	return appendNewVerbs(verbs, verbsOf(parent, at))
}

// appendNewVerbs drops a verb the line already names. A file row joins its own
// keys with the parent's, and d is on both: the file's diff and the commit's
// are the same key, and the footer offered it as two words.
func appendNewVerbs(have, add []state.VerbName) []state.VerbName {
	for _, verb := range add {
		if !verbIn(have, verb) {
			have = append(have, verb)
		}
	}
	return have
}

func verbIn(verbs []state.VerbName, want state.VerbName) bool {
	for _, verb := range verbs {
		if verb == want {
			return true
		}
	}
	return false
}
