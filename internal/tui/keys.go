package tui

import (
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/state"
)

// A terminal reports a capital as the lowercase code with shift set, so the
// uppercase rune never arrives on its own.
func isUndoKey(k tea.KeyPressMsg) bool {
	return k.Code == 'u' && k.Mod&tea.ModShift != 0
}

func isReloadKey(k tea.KeyPressMsg) bool {
	return k.Code == 'r' && k.Mod&tea.ModShift != 0
}

func (m *Model) key(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// ctrl+c leaves from anywhere, including a focused text field. The
	// prototype could not be closed from it, and the keys meant to close it
	// were typed into the commit message instead.
	if k.Mod&tea.ModCtrl != 0 && k.Code == 'c' {
		return m, tea.Quit
	}
	if m.state.HelpOpen {
		return m.helpKey(k)
	}
	// The commit box takes every key it can type. U and R are ordinary capitals
	// there; reading them as verbs left the reader unable to write them and, with
	// a discard behind them, opened a confirmation the footer does not draw.
	if m.state.Changes.MessageFocused {
		return m.messageKey(k)
	}
	// A pending confirmation swallows the key rather than letting undo through:
	// the discard has not run, so there is nothing of its own to reverse, and
	// the previous snapshot would land on a tree the reader was about to change.
	if isUndoKey(k) {
		return m.undoKey()
	}
	if isReloadKey(k) {
		return m.reloadKey()
	}
	if _, pending := state.PendingConfirmation(m.state); pending {
		return m.confirmationKey(k)
	}
	switch k.Code {
	case tea.KeyTab, 'j', 'k', ' ', 'a', tea.KeyLeft, tea.KeyRight, tea.KeyEsc,
		tea.KeyUp, tea.KeyDown, tea.KeyPgUp, tea.KeyPgDown, tea.KeyHome, tea.KeyEnd:
		return m.navKey(k)
	case 's', 'u', 'x', 'z', 'p', 'b', 'g', 'r':
		return m.rowVerbKey(k)
	default:
		if _, ok := tabFromDigit(k.Code); ok {
			return m.navKey(k)
		}
		return m.paneKey(k)
	}
}

// writesWorkingTree reports whether a key changes files on disk. Those stay
// withheld while the help overlay is open, because their result would be drawn
// behind it and the reader would not see what happened.
func (m *Model) writesWorkingTree(k tea.KeyPressMsg) bool {
	if isUndoKey(k) || isReloadKey(k) {
		return true
	}
	switch k.Code {
	case 's', 'u', 'x', 'z', 'p', 'b', 'c':
		return true
	}
	return false
}

// helpMove answers how far a key moves the help, and false for a key that is
// not a way to move. The amounts are the list's: a row, a page, the whole
// thing.
//
// end and home are given the help's own length rather than a large number.
// HelpScrolled adds to the offset without bounds — the clamp is what holds the
// end — so a number near the top of the range wraps to negative and lands the
// reader at the first line instead of the last.
func helpMove(k tea.KeyPressMsg, page, whole int) (by int, moves bool) {
	switch k.Code {
	case 'j', tea.KeyDown:
		return 1, true
	case 'k', tea.KeyUp:
		return -1, true
	case tea.KeyPgDown:
		return page, true
	case tea.KeyPgUp:
		return -page, true
	case tea.KeyEnd:
		return whole, true
	case tea.KeyHome:
		return -whole, true
	}
	return 0, false
}

// helpRows is how far a page moves inside the help, which is what the help
// itself occupies: the header and the rule above it are the two rows it does
// not get, and HelpScrollClamp is told the same number. A pane too short to
// hold either still moves a row at a time rather than standing still or
// running backwards.
func (m *Model) helpRows() int {
	if rows := m.state.Height - 2; rows > 1 {
		return rows
	}
	return 1
}

// helpKey answers the keys the overlay uses, then lets the rest of the list
// through by closing the help first, so the reader sees what the key did. A
// clicked help line closes it the same way.
func (m *Model) helpKey(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// The help lists j, k and the arrows together, and a reader who has just
	// read that line is in the pane it describes. Every key that moves the list
	// moves the help by the same amount, so the line stays true where it is
	// read.
	if by, moves := helpMove(k, m.helpRows(), m.render.HelpRowCount(m.state.Tab)); moves {
		m.state = state.Apply(m.state, state.HelpScrolled{By: by})
		m.state = state.Apply(m.state, state.HelpScrollClamped{
			Scroll: m.render.HelpScrollClamp(m.state.HelpScroll, m.state.Tab, m.helpRows())})
		return m, nil
	}
	switch k.Code {
	case tea.KeyEsc, '?':
		m.state = state.Apply(m.state, state.HelpClosed{})
	default:
		if m.writesWorkingTree(k) {
			return m, nil
		}
		m.state = state.Apply(m.state, state.HelpClosed{})
		return m.key(k)
	}
	return m, nil
}

func (m *Model) undoKey() (tea.Model, tea.Cmd) {
	if _, pending := state.PendingConfirmation(m.state); pending {
		return m.cancelConfirmation()
	}
	return m.requestUndo()
}

func (m *Model) reloadKey() (tea.Model, tea.Cmd) {
	if _, pending := state.PendingConfirmation(m.state); pending {
		return m.cancelConfirmation()
	}
	return m.requestReload()
}

// confirmationKey answers the one confirmation that is open. y confirms and
// every other key cancels, on all four of them: a reader who learns the rule on
// one has learned it everywhere.
func (m *Model) confirmationKey(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if k.Code != 'y' {
		return m.cancelConfirmation()
	}
	switch {
	case m.state.Changes.DiscardConfirm != nil:
		return m.confirmDiscard()
	case m.state.Changes.UndiscardConfirm != nil:
		return m.confirmUndiscard()
	case m.state.Stashed.DropConfirm != nil:
		return m.confirmStashDrop()
	case m.state.Worktrees.RemoveConfirm != nil:
		return m.confirmWorktreeRemove()
	}
	return m, nil
}

func (m *Model) cancelConfirmation() (tea.Model, tea.Cmd) {
	switch {
	case m.state.Changes.DiscardConfirm != nil:
		return m.cancelDiscard()
	case m.state.Changes.UndiscardConfirm != nil:
		return m.cancelUndiscard()
	case m.state.Stashed.DropConfirm != nil:
		return m.cancelStashDrop()
	case m.state.Worktrees.RemoveConfirm != nil:
		return m.cancelWorktreeRemove()
	}
	return m, nil
}

func tabFromDigit(code rune) (state.Tab, bool) {
	if code < '1' || int(code-'1') >= int(state.TabCount) {
		return 0, false
	}
	return state.Tab(code - '1'), true
}

func (m *Model) navKey(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if tab, ok := tabFromDigit(k.Code); ok {
		if !m.tabShown(tab) || tab == m.state.Tab {
			return m, nil
		}
		return m.switchTab(tab)
	}
	switch k.Code {
	case tea.KeyTab:
		return m.runVerb(state.VerbNameNextTab)
	case 'j', tea.KeyDown:
		if k.Mod&tea.ModShift != 0 {
			return m.runVerb(state.VerbNameExtend)
		}
		return m.runVerb(state.VerbNameDown)
	case 'k', tea.KeyUp:
		return m.moveCursor(k, -1)
	// A reader who reaches for the wheel has a way through a long list; one who
	// does not had only j and k, a row at a time.
	case tea.KeyPgDown:
		return m.moveCursor(k, m.listRowsNow())
	case tea.KeyPgUp:
		return m.moveCursor(k, -m.listRowsNow())
	case tea.KeyHome:
		return m.moveCursor(k, -len(m.state.Rows))
	case tea.KeyEnd:
		return m.moveCursor(k, len(m.state.Rows))
	case ' ':
		return m.runVerb(state.VerbNameSelect)
	case 'a':
		return m.runVerb(state.VerbNameSelectAll)
	case tea.KeyLeft:
		return m.runVerb(state.VerbNameFold)
	case tea.KeyRight:
		return m.unfoldAtCursor()
	case tea.KeyEsc:
		return m.runVerb(state.VerbNameClear)
	}
	return m, nil
}

func (m *Model) clearSelectionOrCloseDiff() (tea.Model, tea.Cmd) {
	if len(m.state.Changes.Selected) > 0 {
		m.state = state.Apply(m.state, state.SelectionCleared{})
		return m, nil
	}
	save := m.finishReading()
	// The diff names itself. cursorPath answers a different question — which
	// path a working-tree verb acts on — and it is empty for a file a commit,
	// a stash or another worktree holds, which left those tabs closing the diff
	// under a name followCursor never matched, so the next cursor move reopened
	// it at once.
	m.state = state.Apply(m.state, state.DiffClosed{For: m.state.Open.Path})
	return m, save
}

// rowVerbKey answers the keys that act on the row under the cursor. Whether one
// of them writes to the repository is asked separately, by writesWorkingTree,
// because the help overlay needs that answer for keys in both this function and
// paneKey.
func (m *Model) rowVerbKey(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch k.Code {
	case 's':
		if k.Mod&tea.ModShift != 0 {
			return m.requestSync()
		}
		return m.requestVerb(state.VerbStage)
	case 'u':
		if state.Facts[m.state.Tab].LowerUResetsCommit {
			return m.requestHistoryUndo()
		}
		return m.requestVerb(state.VerbUnstage)
	case 'x':
		return m.discardForTab()
	case 'z':
		return m.requestVerb(state.VerbStash)
	case 'p':
		if state.Facts[m.state.Tab].AnswersKey(state.VerbNameRestore) {
			return m.requestStashRestore()
		}
	case 'b':
		if state.Facts[m.state.Tab].AnswersKey(state.VerbNameBranch) {
			return m.requestStashBranch()
		}
	case 'g':
		if state.Facts[m.state.Tab].AnswersKey(state.VerbNameGo) {
			return m.requestWorktreeGo()
		}
	case 'r':
		return m.requestRead()
	}
	return m, nil
}

// paneKey answers the keys that act on the pane rather than on one row: opening
// a diff, moving inside it, fetching, committing, quitting.
func (m *Model) paneKey(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch k.Code {
	case 'd':
		return m.diffForTab()
	case 'c':
		return m.runVerb(state.VerbNameCommit)
	case 'n':
		by := 1
		if k.Mod&tea.ModShift != 0 {
			by = -1
		}
		return m.moveBlockCursor(by)
	case 'f':
		return m.requestFetch()
	case 'y':
		// y also confirms a pending dialog, but key() reaches those first, so
		// the two never race for the same press.
		if state.Facts[m.state.Tab].AnswersKey(state.VerbNameSHA) {
			return m.requestCopySHA()
		}
	case 'o':
		if state.Facts[m.state.Tab].AnswersKey(state.VerbNameOpen) {
			return m.requestOpenCommit()
		}
	case tea.KeyEnter:
		return m.diffForTab()
	case 'q':
		return m.runVerb(state.VerbNameQuit)
	case '?':
		m.state = state.Apply(m.state, state.HelpOpened{})
		return m, nil
	}
	return m, nil
}

func (m *Model) messageKey(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch k.Code {
	case tea.KeyEsc:
		m.state = state.Apply(m.state, state.MessageBlurred{})
		return m, nil
	case tea.KeyEnter:
		return m.requestCommit()
	case tea.KeyBackspace:
		if m.state.Changes.Message == "" {
			return m, nil
		}
		_, size := utf8.DecodeLastRuneInString(m.state.Changes.Message)
		m.state = state.Apply(m.state, state.MessageEdited{
			Text: m.state.Changes.Message[:len(m.state.Changes.Message)-size],
		})
		return m, nil
	}
	if k.Text != "" {
		m.state = state.Apply(m.state, state.MessageEdited{
			Text: m.state.Changes.Message + k.Text,
		})
		return m, nil
	}
	if k.Code >= 32 && k.Code < utf8.MaxRune {
		m.state = state.Apply(m.state, state.MessageEdited{
			Text: m.state.Changes.Message + string(k.Code),
		})
	}
	return m, nil
}
