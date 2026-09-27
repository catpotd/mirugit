package state

import "github.com/catpotd/mirugit/internal/git"

// VerbRequested stores Pending from a footer or row verb. Block names a diff
// block index, or −1 when the verb acts on whole files.
type VerbRequested struct {
	Verb    Verb
	Targets []string
	Block   int
}

// VerbFinished clears Pending, consumes selection for applied verbs, and sets
// Notice from the outcome.
type VerbFinished struct {
	Verb    Verb
	Applied int
	Ignored []string
	Notice  string
}

// DiscardConfirm holds a snapshot taken before the reader confirms, because
// discard is destructive and the undo ref must exist before the prompt.
type DiscardConfirm struct {
	Targets []string
	Undo    git.Undo
	Files   int
	Added   int
	Deleted int
}

// StashDropConfirm holds the stash about to be dropped, because drop is
// destructive and stash is outside refs/mirugit/undo/. SHA is stored instead of
// a position spec because stash@{N} can point at a different stash after reload.
type StashDropConfirm struct {
	SHA     string
	Message string
}

// WorktreeRemoveConfirm holds the worktree about to be removed. The same x key
// asks first on the changes and stashed tabs, and asking on two of three tabs
// is a rule the reader cannot learn.
type WorktreeRemoveConfirm struct {
	Path string
	Name string
}

// UndiscardConfirm holds a discard undo about to overwrite post-discard edits.
type UndiscardConfirm struct {
	Undo  git.Undo
	Files int
}

// DiscardFinished records how many paths discard or undiscard touched, stores
// LastUndo when a discard ran, and sets Notice.
type DiscardFinished struct {
	Applied int
	Added   int
	Deleted int
	Undo    git.Undo
	// Restored marks the undo of a discard rather than a discard. The line
	// has to say which one happened, and a snapshot already put back has
	// nothing left to offer.
	Restored bool
}

// Event is the closed set of state transitions the TUI applies through Apply.
type Event interface{ event() }

// CursorMoved shifts Cursor by By within s.Rows, clamped to the list bounds.
type CursorMoved struct{ By int }

// CursorMovedTo places Cursor on Row, clamped to the row list.
type CursorMovedTo struct{ Row int }

// BlockCursorMoved shifts BlockCursor by By within the open diff's blocks.
type BlockCursorMoved struct{ By int }

// TabChanged switches Tab, saves the tab's cursor, reloads Rows for the new tab,
// and clears the open diff and selection.
type TabChanged struct{ Tab Tab }

// DiffOpened opens Path in the diff pane. It always writes OpenSection,
// OpenCommit, and DiffPeek, and resets the block cursor and diff scroll.
type DiffOpened struct {
	Path           string
	Peek           bool
	PrioritizeDiff bool
	// Origin is where the diff came from and which one. A path staged and
	// edited again has a row on each side of the index, and they hold different
	// diffs; a commit and a stash each hold their own.
	Origin Origin
}

// DiffClosed clears the open diff and sets DiffClosedFor to For so follow does not
// reopen the same file's diff.
type DiffClosed struct{ For string }

// DirectoryFolded toggles Folded for Path in Section.
type DirectoryFolded struct {
	Path    string
	Section Section
}

// StatusLoaded replaces Entries, Head, and RemoteURL, rebuilds Rows on the changes
// tab, and prunes selection and open diff state that no longer applies.
type StatusLoaded struct {
	Rows      []git.Entry
	Head      git.Head
	RemoteURL string
	// Unfinished is what git is part way through, which the status alone does
	// not say.
	Unfinished git.InProgress
}

// HistoryLoaded replaces Commits and rebuilds Rows on the history tab.
type HistoryLoaded struct {
	Commits []git.CommitInfo
	HasMore bool
}

// HistoryMoreRequested prevents scrolling from starting the same page read repeatedly.
type HistoryMoreRequested struct{}

// HistoryMoreLoaded carries the requested offset so stale pages cannot append after a reload.
type HistoryMoreLoaded struct {
	Offset  int
	Commits []git.CommitInfo
	HasMore bool
}

// HistoryMoreFailed lets scrolling retry a page after its read fails.
type HistoryMoreFailed struct{}

// WorktreeFilesLoaded replaces WorktreeFiles for the expanded worktree and
// rebuilds Rows on the worktrees tab.
type WorktreeFilesLoaded struct {
	Path  string
	Files []git.Entry
}

// StashFilesLoaded replaces the stash file list for the expanded stash ref and
// rebuilds Rows on the stashed tab.
type StashFilesLoaded struct {
	Ref   string
	Files []git.Entry
}

// StashedLoaded replaces Stashes and Head, rebuilds Rows on the stashed tab, and
// leaves for changes when the last stash goes away.
type StashedLoaded struct {
	Stashes []git.StashRow
	Head    git.Head
}

// StashBranchFinished sets Notice to the branch name after branching from a stash.
type StashBranchFinished struct {
	Branch string
}

// StashRestoreFinished sets Notice to how many files the stash put back.
type StashRestoreFinished struct {
	Files int
}

// StashRefsMoved says a verb took a stash off the list, so the refs on hand no
// longer name the stashes they did. Every verb that does it says so: drop,
// restore and branch each rename every stash below the one they took.
type StashRefsMoved struct{}

// WorktreesLoaded replaces Worktrees, WorktreeBase, and WorktreeHere, rebuilds
// Rows on the worktrees tab, and leaves for changes when only one worktree remains.
type WorktreesLoaded struct {
	Worktrees []git.WorktreeRow
	Base      string
	Here      string
}

// WorktreeRowUpdated replaces one Worktrees entry and rebuilds Rows when the
// worktrees tab is active.
type WorktreeRowUpdated struct {
	Index int
	Row   git.WorktreeRow
}

// StashRowUpdated replaces one Stashes entry and rebuilds Rows when the stashed
// tab is active. What a stash holds is read one stash at a time, so a read that
// fails takes one row's answer rather than the list.
type StashRowUpdated struct {
	Index int
	Row   git.StashRow
}

// WorktreeGo records WorktreeHere, resets cursor and open diff, and clears selection.
type WorktreeGo struct {
	Here string
}

// CommitExpanded records ExpandedSHA and ExpandedFiles and rebuilds history Rows.
type CommitExpanded struct {
	SHA   string
	Files []git.Entry
}

// RowCollapsed closes whatever row the current tab has open. One event for the
// three tabs whose rows expand: which field it clears is a fact about the tab,
// and a caller that had to know would be the fourth place that knows.
type RowCollapsed struct{}

// UndoFinished sets Notice after an undo commit reset.
type UndoFinished struct{}

// DiffLoaded replaces Diff and clears stale on the open path.
type DiffLoaded struct{ Diff git.FileDiff }

// ScrollSynced sets the first visible line of the list and of the open diff.
// The values are computed in layout, which knows the geometry, but they land
// here so Apply stays the one place state changes.
type ScrollSynced struct {
	ListTop          int
	DiffTop          int
	DiffBlockLine    int
	SetList          bool
	SetDiff          bool
	SetDiffBlockLine bool
}

// BlockCursorSet moves the block cursor to an index the caller already clamped.
type BlockCursorSet struct{ Block int }

// FetchedAgoUpdated sets the label that ages on its own.
type FetchedAgoUpdated struct{ Label string }

// UpdateAvailable carries a stable release version for the informational notice.
type UpdateAvailable struct{ Version string }

// HelpScrollClamped sets the help offset to a value layout already bounded.
type HelpScrollClamped struct{ Scroll int }

// Resized sets Width and Height.
type Resized struct{ Width, Height int }

// SelectionToggled toggles Selected for Path in Section.
type SelectionToggled struct {
	Path    string
	Section Section
}

// SelectionRangeExtended adds visible file rows between From and To into Selected.
type SelectionRangeExtended struct{ From, To int }

// SectionToggled toggles Selected for every file in Section.
type SectionToggled struct{ Section Section }

// DirectorySelectionToggled toggles Selected for every file under Path in Section.
type DirectorySelectionToggled struct {
	Path    string
	Section Section
}

// TabSelectionToggled toggles Selected for every selectable file on the changes tab.
type TabSelectionToggled struct{}

// SelectionCleared empties Selected. It leaves Notice alone; the tab switch is
// what posts the deselected notice.
type SelectionCleared struct{}

// DiscardConfirmationShown stores DiscardConfirm and clears Notice.
type DiscardConfirmationShown struct {
	Confirm DiscardConfirm
}

// DiscardCancelled clears DiscardConfirm and sets Notice.
type DiscardCancelled struct{ Notice string }

// DiscardConfirmed arms Pending with VerbDiscard from DiscardConfirm.
type DiscardConfirmed struct{}

// StashDropConfirmationShown stores StashDropConfirm and clears Notice.
type StashDropConfirmationShown struct {
	Confirm StashDropConfirm
}

// StashDropCancelled clears StashDropConfirm and sets Notice.
type StashDropCancelled struct{}

// StashDropConfirmed clears StashDropConfirm so the drop can run.
type StashDropConfirmed struct{}

// WorktreeRemoveConfirmationShown stores WorktreeRemoveConfirm and clears Notice.
type WorktreeRemoveConfirmationShown struct {
	Confirm WorktreeRemoveConfirm
}

// WorktreeRemoveCancelled clears WorktreeRemoveConfirm and sets Notice.
type WorktreeRemoveCancelled struct{}

// WorktreeRemoveConfirmed clears WorktreeRemoveConfirm so the removal can run.
type WorktreeRemoveConfirmed struct{}

// UndiscardConfirmationShown stores UndiscardConfirm and clears Notice.
type UndiscardConfirmationShown struct {
	Confirm UndiscardConfirm
}

// UndiscardCancelled clears UndiscardConfirm and sets Notice.
type UndiscardCancelled struct{}

// UndiscardConfirmed clears UndiscardConfirm so the undiscard can run.
type UndiscardConfirmed struct{}

// MessageFocused sets MessageFocused.
type MessageFocused struct{}

// MessageBlurred clears MessageFocused.
type MessageBlurred struct{}

// MessageEdited replaces Message.
type MessageEdited struct{ Text string }

// CommitFinished clears Message and MessageFocused and sets Notice with the new SHA.
type CommitFinished struct {
	SHA   string
	Files int
}

// CommitBlocked sets Notice when commit was requested with nothing staged.
type CommitBlocked struct{}

// DiffBlocked sets Notice when the pane is too short to show a diff.
type DiffBlocked struct{}

// HelpOpened sets HelpOpen and resets HelpScroll.
type HelpOpened struct{}

// HelpClosed clears HelpOpen.
type HelpClosed struct{}

// HelpScrolled shifts HelpScroll by By.
type HelpScrolled struct{ By int }

// StaleChanged sets or clears Stale for Path.
type StaleChanged struct {
	Path  string
	Stale bool
}

// Failed carries a repository error to the screen, because dropping one would
// turn a missing binary or a deleted repository into a silent blank pane.
type Failed struct {
	// Line is what the reader sees. It is a sentence rather than an error
	// because deciding which sentence an error deserves means knowing the
	// error's type, and the types belong to the packages that produce them:
	// this package would be reading Error() and cutting it up, which is what
	// it used to do.
	Line string
}

func (CursorMoved) event()                     {}
func (CursorMovedTo) event()                   {}
func (BlockCursorMoved) event()                {}
func (TabChanged) event()                      {}
func (DiffOpened) event()                      {}
func (DiffClosed) event()                      {}
func (DirectoryFolded) event()                 {}
func (StatusLoaded) event()                    {}
func (HistoryLoaded) event()                   {}
func (HistoryMoreRequested) event()            {}
func (HistoryMoreLoaded) event()               {}
func (HistoryMoreFailed) event()               {}
func (WorktreeFilesLoaded) event()             {}
func (StashFilesLoaded) event()                {}
func (StashedLoaded) event()                   {}
func (StashBranchFinished) event()             {}
func (StashRestoreFinished) event()            {}
func (WorktreesLoaded) event()                 {}
func (WorktreeRowUpdated) event()              {}
func (StashRowUpdated) event()                 {}
func (StashRefsMoved) event()                  {}
func (WorktreeGo) event()                      {}
func (CommitExpanded) event()                  {}
func (RowCollapsed) event()                    {}
func (UndoFinished) event()                    {}
func (DiffLoaded) event()                      {}
func (HelpScrollClamped) event()               {}
func (ScrollSynced) event()                    {}
func (BlockCursorSet) event()                  {}
func (FetchedAgoUpdated) event()               {}
func (UpdateAvailable) event()                 {}
func (Resized) event()                         {}
func (SelectionToggled) event()                {}
func (SelectionRangeExtended) event()          {}
func (SectionToggled) event()                  {}
func (DirectorySelectionToggled) event()       {}
func (TabSelectionToggled) event()             {}
func (SelectionCleared) event()                {}
func (VerbRequested) event()                   {}
func (VerbFinished) event()                    {}
func (DiscardConfirmationShown) event()        {}
func (DiscardCancelled) event()                {}
func (DiscardConfirmed) event()                {}
func (StashDropConfirmationShown) event()      {}
func (WorktreeRemoveConfirmationShown) event() {}
func (WorktreeRemoveCancelled) event()         {}
func (WorktreeRemoveConfirmed) event()         {}
func (StashDropCancelled) event()              {}
func (StashDropConfirmed) event()              {}
func (UndiscardConfirmationShown) event()      {}
func (UndiscardCancelled) event()              {}
func (UndiscardConfirmed) event()              {}
func (MessageFocused) event()                  {}
func (MessageBlurred) event()                  {}
func (MessageEdited) event()                   {}
func (CommitFinished) event()                  {}
func (CommitBlocked) event()                   {}
func (DiffBlocked) event()                     {}
func (DiscardFinished) event()                 {}
func (HelpOpened) event()                      {}
func (HelpClosed) event()                      {}
func (HelpScrolled) event()                    {}
func (StaleChanged) event()                    {}
func (Failed) event()                          {}

func selectsRows(e Event) bool {
	switch e.(type) {
	case SelectionToggled, SelectionRangeExtended, SectionToggled,
		DirectorySelectionToggled, TabSelectionToggled:
		return true
	}
	return false
}
