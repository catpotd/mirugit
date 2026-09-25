package state

import (
	"fmt"
)

// setNotice writes Notice and clears NoticeFailed together. Writing them
// separately lets a later non-failure message inherit NoticeFailed and vanish
// on tab change.
func setNotice(s *State, text string) {
	s.Notice, s.NoticeFailed = text, false
}

// setFailedNotice marks a Notice from a failed verb so tab change can clear it.
func setFailedNotice(s *State, text string) {
	s.Notice, s.NoticeFailed = text, true
}

// Apply returns s after e, copying maps that mutation would alias. Tab guards drop
// selection events on non-changes tabs before the handler chain runs.
func Apply(s State, e Event) State {
	s.Changes.Folded = copyBools(s.Changes.Folded)
	s.Changes.Selected = copyBools(s.Changes.Selected)
	s.Changes.Stale = copyBools(s.Changes.Stale)

	// Only the changes tab has verbs that act on a selection. Guarding the key
	// alone left the checkbox click, which put the reader in a state the
	// footer counted and nothing could spend.
	if !Facts[s.Tab].AllowsSelection && selectsRows(e) {
		return s
	}
	if out, ok := applyNavigation(s, e); ok {
		return out
	}
	if out, ok := applyLoaded(s, e); ok {
		return out
	}
	if out, ok := applySelection(s, e); ok {
		return out
	}
	if out, ok := applyVerb(s, e); ok {
		return out
	}
	if out, ok := applyCommitMessage(s, e); ok {
		return out
	}
	return s
}

func applyNavigation(s State, e Event) (State, bool) {
	switch e := e.(type) {
	case CursorMoved:
		return applyCursorMoved(s, e), true
	case CursorMovedTo:
		return applyCursorMovedTo(s, e), true
	case BlockCursorMoved:
		return applyBlockCursorMoved(s, e), true
	case TabChanged:
		return applyTabChanged(s, e), true
	case DiffOpened:
		return applyDiffOpened(s, e), true
	case DiffClosed:
		return applyDiffClosed(s, e), true
	case DirectoryFolded:
		return applyDirectoryFolded(s, e), true
	case WorktreeGo:
		return applyWorktreeGo(s, e), true
	case Resized:
		return applyResized(s, e), true
	case ScrollSynced:
		return applyScrollSynced(s, e), true
	case HelpScrollClamped:
		s.HelpScroll = e.Scroll
		return s, true
	case BlockCursorSet:
		return applyBlockCursorSet(s, e), true
	case FetchedAgoUpdated:
		return applyFetchedAgoUpdated(s, e), true
	case HelpOpened:
		return applyHelpOpened(s), true
	case HelpClosed:
		return applyHelpClosed(s), true
	case HelpScrolled:
		return applyHelpScrolled(s, e), true
	case StaleChanged:
		return applyStaleChanged(s, e), true
	case DiffBlocked:
		return applyDiffBlocked(s), true
	}
	return s, false
}

func applyCommitMessage(s State, e Event) (State, bool) {
	switch e := e.(type) {
	case MessageFocused:
		return applyMessageFocused(s), true
	case MessageBlurred:
		return applyMessageBlurred(s), true
	case MessageEdited:
		return applyMessageEdited(s, e), true
	case CommitFinished:
		return applyCommitFinished(s, e), true
	case CommitBlocked:
		return applyCommitBlocked(s), true
	}
	return s, false
}

func applyCursorMoved(s State, e CursorMoved) State {
	if len(s.Rows) == 0 {
		s.Cursor = 0
		return s
	}
	if !Facts[s.Tab].SkipsFoldedRows || e.By == 0 {
		s.Cursor = clampIndex(s.Cursor+e.By, len(s.Rows)-1)
		return s
	}
	steps := e.By
	direction := 1
	if steps < 0 {
		direction, steps = -1, -steps
	}
	for steps > 0 {
		next := clampIndex(s.Cursor+direction, len(s.Rows)-1)
		if next == s.Cursor {
			return s
		}
		s.Cursor = next
		if rowHiddenByFold(s, s.Rows[next]) {
			continue
		}
		steps--
	}
	return s
}

func applyCursorMovedTo(s State, e CursorMovedTo) State {
	if len(s.Rows) == 0 {
		s.Cursor = 0
		return s
	}
	s.Cursor = clampIndex(e.Row, len(s.Rows)-1)
	return s
}

func rowHiddenByFold(s State, row Row) bool {
	switch row.Kind() {
	case RowDirectory:
		return IsHiddenByFoldedDirectory(s, row.Section(), row.DirPath())
	case RowFile:
		return IsHiddenByFoldedDirectory(s, row.Section(), row.Path())
	case RowCommit, RowStash, RowWorktree, RowSectionHeading:
		return false
	}
	return false
}

func applyBlockCursorMoved(s State, e BlockCursorMoved) State {
	if len(s.Open.Diff.Blocks) == 0 {
		return s
	}
	next := clampIndex(s.Open.BlockCursor+e.By, len(s.Open.Diff.Blocks)-1)
	if next != s.Open.BlockCursor {
		s.Open.BlockCursor, s.Open.BlockLine = next, 0
	}
	return s
}

func applyTabChanged(s State, e TabChanged) State {
	if e.Tab == s.Tab {
		return s
	}
	if s.NoticeFailed {
		setNotice(&s, "")
	}
	clearSelectedWithNotice(&s)
	return goToTab(s, e.Tab)
}

// goToTab puts the cursor back where it was on the tab being entered, and
// remembers where it was on the tab being left.
//
// Two places changed the tab and only this one did that. A tab that emptied
// under the reader — the last stash restored, the last worktree removed — sent
// them to changes through the other one, which dropped both cursors: the tab
// they left forgot where they were, and changes started at the top rather than
// where they had been.
func goToTab(s State, tab Tab) State {
	s.TabCursors[s.Tab] = s.Cursor
	s.Tab, s.Open = tab, OpenDiff{}
	s.ScrollTop = 0
	s.Cursor = s.TabCursors[tab]
	return refreshRows(s, keepCursorIndex)
}

func applyDiffOpened(s State, e DiffOpened) State {
	s.Open = OpenDiff{
		Path:           e.Path,
		Origin:         e.Origin,
		BlockCursor:    0,
		Scroll:         0,
		Peek:           e.Peek,
		PrioritizeDiff: e.PrioritizeDiff,
	}
	return s
}

func applyDiffClosed(s State, e DiffClosed) State {
	s.Open = OpenDiff{Closed: true, ClosedFor: e.For}
	return s
}

func applyDirectoryFolded(s State, e DirectoryFolded) State {
	key := foldKey(e.Section, e.Path)
	s.Changes.Folded[key] = !s.Changes.Folded[key]
	return s
}

func applyWorktreeGo(s State, e WorktreeGo) State {
	s.Worktrees.Here = e.Here
	s.Cursor = 0
	s.Open = OpenDiff{}
	clearSelected(&s)
	return s
}

func applyResized(s State, e Resized) State {
	s.Width, s.Height = e.Width, e.Height
	return s
}

func applyHelpOpened(s State) State {
	s.HelpOpen = true
	s.HelpScroll = 0
	return s
}

func applyHelpClosed(s State) State {
	s.HelpOpen = false
	return s
}

func applyHelpScrolled(s State, e HelpScrolled) State {
	s.HelpScroll += e.By
	return s
}

func applyStaleChanged(s State, e StaleChanged) State {
	if e.Stale {
		s.Changes.Stale[e.Path] = true
	} else {
		delete(s.Changes.Stale, e.Path)
	}
	return s
}

func applyDiffBlocked(s State) State {
	setNotice(&s, "no room for a diff · make the pane taller")
	return s
}

func applyMessageFocused(s State) State {
	s.Changes.MessageFocused = true
	return s
}

func applyMessageBlurred(s State) State {
	s.Changes.MessageFocused = false
	return s
}

func applyMessageEdited(s State, e MessageEdited) State {
	s.Changes.Message = e.Text
	return s
}

func applyCommitFinished(s State, e CommitFinished) State {
	s.Changes.Message = ""
	s.Changes.MessageFocused = false
	setNotice(&s, fmt.Sprintf("committed %s · %d %s", e.SHA, e.Files, FileWord(e.Files)))
	return s
}

func applyCommitBlocked(s State) State {
	setNotice(&s, "stage a file first")
	return s
}

// Plus and Minus print a line count the way every line that carries one does.
// The notice, the summary and the file rows each had a copy, and the minus sign
// is U+2212 rather than a hyphen, which is the kind of detail that drifts when
// there are three of it.
func Plus(n int) string {
	if n == 0 {
		return "+0"
	}
	return fmt.Sprintf("+%d", n)
}

func Minus(n int) string {
	return fmt.Sprintf("−%d", n)
}

func applyScrollSynced(s State, e ScrollSynced) State {
	if e.SetList {
		s.ScrollTop = e.ListTop
	}
	if e.SetDiff {
		s.Open.Scroll = e.DiffTop
	}
	if e.SetDiffBlockLine {
		s.Open.BlockLine = max(e.DiffBlockLine, 0)
	}
	return s
}

func applyBlockCursorSet(s State, e BlockCursorSet) State {
	next := clampIndex(e.Block, len(s.Open.Diff.Blocks)-1)
	if next != s.Open.BlockCursor {
		s.Open.BlockCursor, s.Open.BlockLine = next, 0
	}
	return s
}

func applyFetchedAgoUpdated(s State, e FetchedAgoUpdated) State {
	s.Fetched = e.Label
	return s
}
