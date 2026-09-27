package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
	"time"
)

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if model, cmd, handled := m.updateEvent(msg); handled {
		return model, cmd
	}
	return m.updateResult(msg)
}

func (m *Model) finishUpdate(cmd tea.Cmd) (tea.Model, tea.Cmd) {
	follow := m.followCursor()
	read := m.markCursorRead()
	m.syncScroll()
	return m, tea.Batch(cmd, follow, read)
}

func (m *Model) failUpdate(err error) (tea.Model, tea.Cmd) {
	m.state = state.Apply(m.state, failed(err))
	return m, nil
}

func (m *Model) updateEvent(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case tea.FocusMsg:
		model, cmd := m.handleFocus()
		return model, cmd, true
	case tea.BlurMsg:
		model, cmd := m.handleBlur()
		return model, cmd, true
	case watchReadyMsg:
		model, cmd := m.handleWatchReady(msg)
		return model, cmd, true
	case watchEventMsg:
		model, cmd := m.handleWatchEvent()
		return model, cmd, true
	case watchDebounceDoneMsg:
		model, cmd := m.handleWatchDebounceDone(msg)
		return model, cmd, true
	case watchPollTickMsg:
		model, cmd := m.handleWatchPollTick()
		return model, cmd, true
	case tea.WindowSizeMsg:
		return m.handleWindowSize(msg)
	case tea.CursorPositionMsg:
		model, cmd := m.handleCursorPosition(msg)
		return model, cmd, true
	case widthProbeTimeoutMsg:
		model, cmd := m.handleWidthProbeTimeout()
		return model, cmd, true
	case widthProbeAfterDrawMsg:
		return m, requestCursorPosition(), true
	case tea.KeyPressMsg:
		return m.handleKeyPress(msg)
	case tea.MouseMotionMsg:
		// The pointer crossing the pane is not the reader choosing a row. It
		// used to move the cursor, which took the verbs to wherever the pointer
		// went and replaced the open diff with the diff of whatever it passed
		// over. The cursor moves on a key or a click.
		return m, nil, true
	case tea.MouseClickMsg:
		return m.handleMouseClick(msg)
	case tea.MouseWheelMsg:
		return m.handleMouseWheel(msg)
	}
	return nil, nil, false
}

func (m *Model) updateResult(msg tea.Msg) (tea.Model, tea.Cmd) {
	// A command that could not read the repository has nothing for its handler
	// to apply, and the reader has to be told. The ones that do have something
	// to apply say so and are left to their handlers.
	if f, ok := msg.(failing); ok && !handlesItsOwnFailure(msg) {
		if err := f.failed(); err != nil {
			return m.failUpdate(err)
		}
	}
	switch msg := msg.(type) {
	case readSavedMsg:
		// The write either worked or was reported above; either way there is
		// nothing to draw from it.
		return m, nil
	case staleMsg:
		model, cmd := m.handleStaleMsg(msg)
		return model, cmd
	case repoMsg:
		return m.handleRepoMsg(msg)
	case stashedMsg:
		return m.handleStashedMsg(msg)
	case worktreeFilesMsg:
		return m.handleWorktreeFilesMsg(msg)
	case stashFilesMsg:
		return m.handleStashFilesMsg(msg)
	case historyMsg:
		return m.handleHistoryMsg(msg)
	case historyMoreMsg:
		return m.handleHistoryMoreMsg(msg)
	case commitFilesMsg:
		return m.handleCommitFilesMsg(msg)
	case diffMsg:
		return m.handleDiffMsg(msg)
	case verbMsg:
		return m.handleVerbMsg(msg)
	case snapshotMsg:
		return m.handleSnapshotMsg(msg)
	case discardMsg:
		return m.handleDiscardMsg(msg)
	case stashDropMsg:
		return m.handleStashDropMsg(msg)
	case stashVerbMsg:
		return m.handleStashVerbMsg(msg)
	case worktreesMsg:
		return m.handleWorktreesMsg(msg)
	case stashStatusMsg:
		return m.handleStashStatusMsg(msg)
	case worktreeStatusMsg:
		return m.handleWorktreeStatusMsg(msg)
	case worktreeRemoveMsg:
		return m.handleWorktreeRemoveMsg(msg)
	case undiscardMsg:
		return m.handleUndiscardMsg(msg)
	case commitMsg:
		return m.handleCommitMsg(msg)
	case readMsg:
		return m.handleReadMsg(msg)
	case staleCheckMsg:
		return m.handleStaleCheckMsg(msg)
	case undoCheckMsg:
		return m.handleUndoCheckMsg(msg)
	case undoMsg:
		return m.handleUndoMsg()
	case reboundMsg:
		return m.handleReboundMsg(msg)
	case fetchedMsg:
		return m.handleFetchedMsg(msg)
	case updateAvailableMsg:
		if msg.version != "" {
			m.state = state.Apply(m.state, state.UpdateAvailable{Version: msg.version})
		}
		return m.finishUpdate(nil)
	}
	return m.finishUpdate(nil)
}

func (m *Model) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd, bool) {
	m.state = state.Apply(m.state, state.Resized{Width: msg.Width, Height: msg.Height})
	model, cmd := m.finishUpdate(nil)
	return model, cmd, true
}

func (m *Model) handleKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	next, cmd := m.key(msg)
	model, finishCmd := next.(*Model).finishUpdate(cmd)
	return model, finishCmd, true
}

func (m *Model) handleMouseClick(msg tea.MouseClickMsg) (tea.Model, tea.Cmd, bool) {
	region, row, hasRow := m.frame.Hit(msg.Y, msg.X)
	t := region
	if t.Kind == layout.TargetNone && hasRow {
		t = row
	}
	next, cmd := m.click(t, msg.Mod)
	m = next.(*Model)
	if t.Kind == layout.TargetVerb {
		m.syncScroll()
		return m, cmd, true
	}
	model, finishCmd := m.finishUpdate(cmd)
	return model, finishCmd, true
}

// handleStaleMsg takes the answer only if the reader is still on that file. The
// check ran as a command, so they can have moved on while git was running.
func (m *Model) handleStaleMsg(msg staleMsg) (tea.Model, tea.Cmd) {
	if msg.path != m.state.Open.Path {
		return m, nil
	}
	m.state = state.Apply(m.state, state.StaleChanged{Path: msg.path, Stale: msg.stale})
	if msg.err != nil {
		m.state = state.Apply(m.state, failed(msg.err))
	}
	return m, nil
}

// handleStaleCheckMsg stages what the check allowed. A stale diff stops here:
// the row says so and the reader reloads.
func (m *Model) handleStaleCheckMsg(msg staleCheckMsg) (tea.Model, tea.Cmd) {
	m.state = state.Apply(m.state, state.StaleChanged{Path: msg.path, Stale: msg.stale})
	// A check that could not run leaves the row stale rather than staging what
	// it could not confirm: diffStale answers true beside its error, and the
	// reader is told why.
	if msg.err != nil {
		m.state = state.Apply(m.state, failed(msg.err))
		return m.finishUpdate(nil)
	}
	if msg.stale {
		return m.finishUpdate(nil)
	}
	next, cmd := m.stagingAfterStaleCheck(msg.block)
	return next.(*Model).finishUpdate(cmd)
}

func (m *Model) stagingAfterStaleCheck(block int) (tea.Model, tea.Cmd) {
	if block == noBlockYet {
		return m.runVerbNow(state.VerbStage)
	}
	return m.stageBlockNow(block)
}

// handleUndoCheckMsg puts the discarded paths back, or asks first when some of
// them have been written since. Reaching here means the count arrived.
func (m *Model) handleUndoCheckMsg(msg undoCheckMsg) (tea.Model, tea.Cmd) {
	if msg.edited > 0 {
		m.state = state.Apply(m.state, state.UndiscardConfirmationShown{
			Confirm: state.UndiscardConfirm{Undo: msg.undo, Files: msg.edited},
		})
		return m.finishUpdate(nil)
	}
	return m.finishUpdate(runUndiscard(m.rootContext(), m.dir, msg.undo))
}

// handleReboundMsg swaps the directory, its git directory and its read marks
// together. Reaching here means all three arrived: a failure was answered
// before this by the path every message carrying an err takes.
func (m *Model) handleReboundMsg(msg reboundMsg) (tea.Model, tea.Cmd) {
	m.dir, m.gitDir, m.read = msg.dir, msg.gitDir, msg.read
	// The reads in flight are for the worktree being left, and their answers
	// name paths this pane is no longer pointed at.
	m.startTabReads()
	m.asked.expandRow = ""
	m.closeWatcher()
	m.state = state.Apply(m.state, state.WorktreeGo{Here: msg.dir})
	return m.finishUpdate(tea.Batch(beginWatch(m.rootContext(), msg.dir),
		m.reloadRepo(), loadWorktrees(m.readsForTab(), msg.dir)))
}

func (m *Model) handleRepoMsg(msg repoMsg) (tea.Model, tea.Cmd) {
	m.read.SetHashes(msg.Hashes)
	for _, e := range msg.Repo.Entries {
		if e.OldPath != "" {
			m.read.Rename(e.OldPath, e.Path)
		}
	}
	m.state = state.Apply(m.state, state.StatusLoaded{
		Rows: msg.Repo.Entries, Head: msg.Repo.Head, RemoteURL: msg.Repo.RemoteURL,
		Unfinished: msg.Repo.Unfinished})
	m.replaceHistory(msg.Commits, msg.HasMore)
	m.state = state.Apply(m.state, state.StashedLoaded{
		Stashes: msg.Stashes, Head: msg.Repo.Head})
	if msg.Repo.CountsErr != nil {
		m.state = state.Apply(m.state, countsIncomplete(msg.Repo.CountsErr))
	}
	return m.finishUpdate(nil)
}

func (m *Model) handleStashedMsg(msg stashedMsg) (tea.Model, tea.Cmd) {
	m.state = state.Apply(m.state, state.StashedLoaded{Stashes: msg.stashes, Head: msg.head})
	save := m.markStashCursorRead()
	return m.finishUpdate(tea.Batch(save, m.reopenExpandedRow(),
		stashStatusCmds(m.readsForTab(), m.dir, msg.stashes, msg.head)))
}

// handleStashStatusMsg takes one stash's contents. It handles its own failure
// so a read the list has moved past is not reported: anything that drops a
// stash renames every stash below it, and the reads already in flight then name
// stashes that have moved. Reporting those put "log for 'stash' only has N
// entries" on top of a drop that worked, and at a reader whose only part in it
// was having the tab open while another terminal dropped one.
//
// A read the list still holds failed on its own and is the reader's to see.
func (m *Model) handleStashStatusMsg(msg stashStatusMsg) (tea.Model, tea.Cmd) {
	return m.handleRowStatus(msg.failed(), m.stashStillAt(msg.index, msg.sha),
		state.StashRowUpdated{Index: msg.index, Row: msg.row})
}

// handleRowStatus takes one row's status. A read that failed is reported only
// while the list still holds the row it was issued for: a stash dropped or a
// tree removed while its status was being read fails on a row nobody is looking
// at any more, and saying so would put an error over a verb that worked.
func (m *Model) handleRowStatus(err error, stillThere bool, updated state.Event) (tea.Model, tea.Cmd) {
	if err != nil {
		if stillThere {
			m.state = state.Apply(m.state, failed(err))
		}
		return m, nil
	}
	m.state = state.Apply(m.state, updated)
	return m.finishUpdate(nil)
}

func (m *Model) handleWorktreeFilesMsg(msg worktreeFilesMsg) (tea.Model, tea.Cmd) {
	m.state = state.Apply(m.state,
		state.WorktreeFilesLoaded{Path: msg.path, Files: msg.files})
	return m.finishUpdate(nil)
}

func (m *Model) handleStashFilesMsg(msg stashFilesMsg) (tea.Model, tea.Cmd) {
	m.state = state.Apply(m.state,
		state.StashFilesLoaded{Ref: msg.ref, Files: msg.files})
	return m.finishUpdate(nil)
}

func (m *Model) handleHistoryMsg(msg historyMsg) (tea.Model, tea.Cmd) {
	m.replaceHistory(msg.commits, msg.hasMore)
	return m.finishUpdate(nil)
}

func (m *Model) handleHistoryMoreMsg(msg historyMoreMsg) (tea.Model, tea.Cmd) {
	if err := msg.failed(); err != nil {
		m.state = state.Apply(m.state, state.HistoryMoreFailed{})
		m.state = state.Apply(m.state, failed(err))
		return m, nil
	}
	if !m.historyStartsWith(msg.newestSHA) {
		m.state = state.Apply(m.state, state.HistoryMoreFailed{})
		return m, nil
	}
	m.state = state.Apply(m.state, state.HistoryMoreLoaded{
		Offset: msg.offset, Commits: msg.page.Commits, HasMore: msg.page.HasMore})
	return m, nil
}

func (m *Model) replaceHistory(commits []git.CommitInfo, hasMore bool) {
	if len(m.state.History.Commits) > len(commits) && sameCommitPrefix(m.state.History.Commits, commits) {
		return
	}
	m.state = state.Apply(m.state, state.HistoryLoaded{Commits: commits, HasMore: hasMore})
}

func sameCommitPrefix(current, replacement []git.CommitInfo) bool {
	if len(replacement) == 0 || len(current) < len(replacement) {
		return false
	}
	for i := range replacement {
		if current[i].SHA != replacement[i].SHA {
			return false
		}
	}
	return true
}

func (m *Model) historyStartsWith(sha string) bool {
	return sha != "" && len(m.state.History.Commits) > 0 && m.state.History.Commits[0].SHA == sha
}

func (m *Model) handleCommitFilesMsg(msg commitFilesMsg) (tea.Model, tea.Cmd) {
	m.state = state.Apply(m.state, state.CommitExpanded{SHA: msg.sha, Files: msg.files})
	return m.finishUpdate(nil)
}

func (m *Model) handleDiffMsg(msg diffMsg) (tea.Model, tea.Cmd) {
	m.state = state.Apply(m.state, state.DiffLoaded{Diff: msg.Diff})
	m.recordShownBlocks()
	return m.finishUpdate(nil)
}

func (m *Model) handleVerbMsg(msg verbMsg) (tea.Model, tea.Cmd) {
	wasBlock := m.state.Changes.Pending != nil &&
		m.state.Changes.Pending.Verb == state.VerbStage && m.state.Changes.Pending.Block >= 0
	openPath := m.state.Open.Path
	m.state = state.Apply(m.state, msg.finished)
	cmd := m.reloadCurrentTab()
	if wasBlock && openPath != "" {
		cmd = tea.Batch(cmd, loadDiff(m.readsForDiff(), m.dir, openPath, m.state.Open.Origin.Side() == state.SectionStaged))
	}
	return m.finishUpdate(cmd)
}

func (m *Model) handleSnapshotMsg(msg snapshotMsg) (tea.Model, tea.Cmd) {
	m.state = state.Apply(m.state, state.DiscardConfirmationShown{Confirm: msg.confirm})
	return m.finishUpdate(nil)
}

func (m *Model) handleDiscardMsg(msg discardMsg) (tea.Model, tea.Cmd) {
	m.state = state.Apply(m.state, msg.finished)
	return m.finishUpdate(m.reloadRepo())
}

// handleStashDropMsg reloads and reads nothing else. A stash is named by its
// place in the list, so a drop renames every stash below it: asking for the
// cursor's files here asks for a place that is no longer there, and git says
// "log for 'stash' only has N entries" over a drop that worked. The reload
// brings the new list and reads the cursor's files from it.
//
// It handles its own failure rather than going through the shared one, because
// the shared one applies nothing and returns: the window a drop opens is closed
// by the new list, and a drop that did not run still needs one.
func (m *Model) handleStashDropMsg(msg stashDropMsg) (tea.Model, tea.Cmd) {
	m.state = state.Apply(m.state, state.StashRefsMoved{})
	if err := msg.failed(); err != nil {
		m.state = state.Apply(m.state, failed(err))
	}
	m.syncScroll()
	return m, m.reloadCurrentTab()
}

// handleStashVerbMsg takes a restore or a branch. Both take a stash off the
// list and rename every stash below it, so like a drop they read nothing until
// the new list arrives, and report their own failure because the shared path
// applies nothing and returns.
func (m *Model) handleStashVerbMsg(msg stashVerbMsg) (tea.Model, tea.Cmd) {
	m.state = state.Apply(m.state, state.StashRefsMoved{})
	if err := msg.failed(); err != nil {
		m.state = state.Apply(m.state, failed(err))
		m.syncScroll()
		return m, m.reloadCurrentTab()
	}
	if msg.branch != "" {
		m.state = state.Apply(m.state, state.StashBranchFinished{Branch: msg.branch})
	}
	if msg.restored > 0 {
		m.state = state.Apply(m.state, state.StashRestoreFinished{Files: msg.restored})
	}
	m.syncScroll()
	return m, m.reloadCurrentTab()
}

func (m *Model) handleWorktreesMsg(msg worktreesMsg) (tea.Model, tea.Cmd) {
	m.state = state.Apply(m.state, state.WorktreesLoaded{
		Worktrees: msg.trees,
		Base:      msg.base,
		Here:      m.dir,
	})
	save := m.markWorktreeCursorRead()
	// The bar's count comes off the list itself. What each tree holds — how
	// many files it has, how far its branch is from the base — costs a git per
	// tree and is drawn only by the worktrees tab, so a reload from any other
	// tab reads the list and stops there.
	if !state.Facts[m.state.Tab].DrawsWorktreeStatus {
		return m.finishUpdate(save)
	}
	cmd := tea.Batch(save, worktreeStatusCmds(m.readsForTab(), msg.trees, msg.base),
		m.reopenExpandedRow())
	return m.finishUpdate(cmd)
}

// handleWorktreeStatusMsg takes one worktree's status. It handles its own
// failure for the reason handleStashStatusMsg does: a tree removed while its
// read was in flight is gone by the time git is asked, and reporting that puts
// git's complaint on top of a remove that worked.
func (m *Model) handleWorktreeStatusMsg(msg worktreeStatusMsg) (tea.Model, tea.Cmd) {
	return m.handleRowStatus(msg.failed(), m.worktreeStillAt(msg.index, msg.path),
		state.WorktreeRowUpdated{Index: msg.index, Row: msg.row})
}

// worktreeStillAt reports whether the list still holds that tree where the read
// was issued for it. See stashStillAt.
//
// It has no RefsStale to consult, because a worktree is named by its path and a
// removal renames nothing. What that leaves is a narrow window the stash side
// closes and this one does not: between a remove landing and the new list
// arriving, the old list still holds the removed path at its old place, so a
// read that was in flight for it is reported rather than dropped. It costs one
// notice line that the next list clears.
func (m *Model) worktreeStillAt(index int, path string) bool {
	rows := m.state.Worktrees.List
	return index >= 0 && index < len(rows) && rows[index].Path == path
}

// handleWorktreeRemoveMsg reloads and reads nothing else. The cursor is still
// on the row for a directory that is gone, and reading its files there asks git
// about a tree it no longer has. The new list moves the cursor off it and the
// read follows from there — handleStashDropMsg for the same reason.
//
// It reports its own failure because the shared path applies nothing and
// returns, and a remove that did not run still needs the list back.
func (m *Model) handleWorktreeRemoveMsg(msg worktreeRemoveMsg) (tea.Model, tea.Cmd) {
	if err := msg.failed(); err != nil {
		m.state = state.Apply(m.state, failed(err))
	}
	m.syncScroll()
	return m, tea.Batch(m.reloadRepo(), loadWorktrees(m.readsForTab(), m.dir))
}

func (m *Model) handleUndiscardMsg(msg undiscardMsg) (tea.Model, tea.Cmd) {
	m.state = state.Apply(m.state, msg.finished)
	return m.finishUpdate(m.reloadRepo())
}

func (m *Model) handleCommitMsg(msg commitMsg) (tea.Model, tea.Cmd) {
	m.state = state.Apply(m.state, msg.finished)
	return m.finishUpdate(m.reloadRepo())
}

func (m *Model) handleReadMsg(msg readMsg) (tea.Model, tea.Cmd) {
	m.read.MarkFileIn(msg.origin, msg.path, msg.hashes)
	return m.finishUpdate(m.saveRead())
}

func (m *Model) handleUndoMsg() (tea.Model, tea.Cmd) {
	m.state = state.Apply(m.state, state.UndoFinished{})
	return m.finishUpdate(m.reloadRepo())
}

func (m *Model) handleFetchedMsg(msg fetchedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		// Keeping the last known time: a failed read does not mean the fetch
		// never happened, and zeroing it drew "never fetched" three minutes
		// after a successful one.
		m.state = state.Apply(m.state, failed(msg.err))
		return m, nil
	}
	m.fetchedAt = msg.at
	// The label is a function of the clock, and View no longer reads it.
	m.state = state.Apply(m.state, state.FetchedAgoUpdated{
		Label: layout.FormatFetchedAgo(m.fetchedAt, time.Now())})
	return m, nil
}
