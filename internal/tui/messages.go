package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

type repoMsg struct {
	Repo    git.Repo
	Commits []git.CommitInfo
	HasMore bool
	Stashes []git.StashRow
	// Hashes travels with the rest because the marks it feeds have to be in
	// place for the same frame the rows are. Sent as a message of its own, a
	// frame would draw the new rows against the old marks.
	Hashes state.BlockHashes
	Err    error
}

type worktreeFilesMsg struct {
	path  string
	files []git.Entry
	err   error
}

type stashFilesMsg struct {
	ref   string
	files []git.Entry
	err   error
}

type stashedMsg struct {
	stashes []git.StashRow
	head    git.Head
	err     error
}

// stashStatusMsg carries what one stash holds. See worktreeStatusMsg.
//
// sha names the stash the read was issued for. A stash is named by its place in
// the list, so anything that drops one renames every stash below it and the
// index alone no longer says which stash this answers for.
type stashStatusMsg struct {
	index int
	sha   string
	row   git.StashRow
	err   error
}

type historyMsg struct {
	commits []git.CommitInfo
	hasMore bool
	err     error
}

type historyMoreMsg struct {
	offset    int
	newestSHA string
	page      git.HistoryPage
	err       error
}

type commitFilesMsg struct {
	sha   string
	files []git.Entry
	err   error
}

// reboundMsg carries everything pointing at another worktree needs, so the
// model swaps all of it or none.
type reboundMsg struct {
	dir    string
	gitDir string
	read   *state.ReadState
	err    error
}

type undoMsg struct {
	err error
}

type diffMsg struct {
	Diff git.FileDiff
	Err  error
}

type verbMsg struct {
	finished state.VerbFinished
	err      error
}

type snapshotMsg struct {
	confirm state.DiscardConfirm
	err     error
}

type discardMsg struct {
	finished state.DiscardFinished
	err      error
}

// undoCheckMsg says how many of the paths a discard removed have been written
// since. Nothing is put back until this arrives: the count is what separates
// putting them back from asking first.
type undoCheckMsg struct {
	undo   git.Undo
	edited int
	err    error
}

// staleCheckMsg says whether the diff on screen still matches the file. The
// staging it gates waits for it: staging a diff the pane no longer shows puts
// something in the index the reader never read.
type staleCheckMsg struct {
	path  string
	block int
	stale bool
	err   error
}

type undiscardMsg struct {
	finished state.DiscardFinished
	err      error
}

type commitMsg struct {
	finished state.CommitFinished
	err      error
}

type readMsg struct {
	path   string
	origin state.Origin
	hashes []string
	err    error
}

type stashDropMsg struct {
	err error
}

type stashVerbMsg struct {
	err    error
	branch string
	// restored is how many files a restore put back, and zero for the other
	// stash verbs.
	restored int
}

type worktreesMsg struct {
	trees []git.WorktreeRow
	base  string
	err   error
}

// worktreeStatusMsg carries one worktree's status. path names the tree the read
// was issued for, because the list can be replaced while it is in flight.
type worktreeStatusMsg struct {
	index int
	path  string
	row   git.WorktreeRow
	err   error
}

type worktreeRemoveMsg struct {
	err error
}

// readSavedMsg answers saveRead. Only the error matters: the marks are already
// in memory, and this says whether they reached disk.
type readSavedMsg struct{ err error }

// staleMsg answers checkStale. The path travels with the answer because the
// reader can move to another file while git is running.
type staleMsg struct {
	path  string
	stale bool
	err   error
}

type fetchedMsg struct {
	at  time.Time
	err error
}

type updateAvailableMsg struct {
	version string
}

// failing is a message that can carry a repository error. Every command that
// talks to git answers with one, and each handler tested it first: twenty-one
// copies of the same three lines, and a mutation of one of them survived the
// suite. updateResult asks once, before the handler runs.
//
// staleMsg is not one of these. It applies what it learned and then reports the
// error, because a read that failed still says the diff on screen can no longer
// be trusted.
type failing interface{ failed() error }

// handlesItsOwnFailure names the messages whose handler runs even when the read
// failed, because each has something to apply first. The shared path applies
// nothing and returns.
//
// staleMsg: what it learned matters even when the read failed — a diff that can
// no longer be trusted. The stash and worktree verbs: the window each opens is
// closed by the list that comes after it, and one that did not run still needs
// one. The two status reads: a read the list has moved past is not the reader's
// to see.
func handlesItsOwnFailure(msg tea.Msg) bool {
	switch msg.(type) {
	case staleMsg, stashDropMsg, stashVerbMsg, stashStatusMsg,
		worktreeStatusMsg, worktreeRemoveMsg, staleCheckMsg, historyMoreMsg:
		return true
	}
	return false
}

func (m repoMsg) failed() error           { return m.Err }
func (m worktreeFilesMsg) failed() error  { return m.err }
func (m stashFilesMsg) failed() error     { return m.err }
func (m stashedMsg) failed() error        { return m.err }
func (m historyMsg) failed() error        { return m.err }
func (m historyMoreMsg) failed() error    { return m.err }
func (m commitFilesMsg) failed() error    { return m.err }
func (m undoMsg) failed() error           { return m.err }
func (m reboundMsg) failed() error        { return m.err }
func (m diffMsg) failed() error           { return m.Err }
func (m verbMsg) failed() error           { return m.err }
func (m snapshotMsg) failed() error       { return m.err }
func (m discardMsg) failed() error        { return m.err }
func (m undiscardMsg) failed() error      { return m.err }
func (m staleCheckMsg) failed() error     { return m.err }
func (m undoCheckMsg) failed() error      { return m.err }
func (m commitMsg) failed() error         { return m.err }
func (m readMsg) failed() error           { return m.err }
func (m stashVerbMsg) failed() error      { return m.err }
func (m worktreesMsg) failed() error      { return m.err }
func (m worktreeStatusMsg) failed() error { return m.err }

func (m stashDropMsg) failed() error      { return m.err }
func (m stashStatusMsg) failed() error    { return m.err }
func (m worktreeRemoveMsg) failed() error { return m.err }
func (m staleMsg) failed() error          { return m.err }
func (m fetchedMsg) failed() error        { return m.err }
func (m readSavedMsg) failed() error      { return m.err }
