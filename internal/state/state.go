package state

import (
	"strconv"

	"github.com/catpotd/mirugit/internal/git"
)

type Section int

const (
	// SectionNone is the zero value so a struct field left unset cannot pass for
	// a real side. It is also what a file that is not in this working tree
	// carries: one held by a commit, a stash or another worktree sits on no
	// side of this index. Where such a file came from is its Origin.
	SectionNone Section = iota
	SectionStaged
	SectionUnstaged
)

type Tab int

const (
	TabChanges Tab = iota
	TabHistory
	TabStashed
	TabWorktrees
	TabCount
)

// OpenDiff holds the open diff overlay. Its zero value is a closed pane.
type OpenDiff struct {
	Path string
	// Origin is where this diff came from and which one. Section and Commit
	// said this in two fields, and a caller had to test both to tell a stash's
	// file from the working tree's: Section None with Commit empty meant the
	// row above, and nothing stopped a commit SHA sitting beside Section
	// Staged.
	Origin      Origin
	Diff        git.FileDiff
	BlockCursor int
	// Scroll is the first block index DiffArea draws when the diff outgrows
	// the pane, so the cursor block can be reached without losing j/k on files.
	Scroll    int
	BlockLine int
	Closed    bool
	ClosedFor string
	// Peek forces a six-row diff when the list would otherwise close it.
	Peek           bool
	PrioritizeDiff bool
}

// Changes holds the changes-tab payload: entries, selection, and commit field.
type Changes struct {
	Entries []git.Entry
	// MessageFocused is what turns every key into a character, so a verb key
	// typed into the field does not also run the verb.
	MessageFocused bool
	Message        string
	Folded         map[string]bool
	// Selected is keyed by section and path together, not by row index, because
	// one path can occupy a row in each section and running a verb twice against
	// it is not what the count says. selectionKey builds the key.
	Selected map[string]bool
	// Stale marks paths whose open diff no longer matches the repository, so
	// stage is withheld until the reader reloads.
	Stale            map[string]bool
	Pending          *VerbRequested
	DiscardConfirm   *DiscardConfirm
	UndiscardConfirm *UndiscardConfirm
}

// History holds the history-tab payload.
type History struct {
	Commits       []git.CommitInfo
	HasMore       bool
	LoadingMore   bool
	ExpandedSHA   string
	ExpandedFiles []git.Entry
	RemoteURL     string
}

// Stashed holds the stashed-tab payload.
type Stashed struct {
	Stashes []git.StashRow
	// Expanded names the stash whose files are shown, by ref. A row opens
	// because the reader opened it, not because the cursor is on it: reading
	// the cursor instead closed what they had open as they moved to look at
	// something else, and no other tab behaves that way.
	Expanded string
	// Files holds what the expanded stash contains. The tab exists to answer
	// what was put down, which a count cannot.
	Files       []git.Entry
	DropConfirm *StashDropConfirm
	// RefsStale records that a verb which takes a stash off the list was
	// answered and the new list has not arrived. A stash is named by its place
	// in the list — stash@{0}, stash@{1} — so drop, restore and branch each
	// rename every stash below the one they took, and in that window the refs
	// here name stashes that have moved. Reading one asked git for a place
	// that was gone, and the reader was shown "log for 'stash' only has 8
	// entries" over a verb that worked.
	RefsStale bool
}

// Worktrees holds the worktrees-tab payload.
type Worktrees struct {
	List []git.WorktreeRow
	Base string
	Here string
	// Expanded names the tree whose files are shown, by path. See
	// Stashed.Expanded for why the cursor does not decide this.
	Expanded string
	// Files holds what the expanded tree has changed, for the same reason
	// Stashed.Files does: a count does not say what is in there.
	Files []git.Entry
	// RemoveConfirm is the tree x is about to remove.
	RemoveConfirm *WorktreeRemoveConfirm
}

// State is plain data throughout, so that a transition can be asserted
// directly.
type State struct {
	Tab    Tab
	Cursor int
	// TabCursors remembers each tab's row index so returning lands on the
	// same line; only selection is cleared on tab change, not the cursor.
	TabCursors [TabCount]int
	Rows       []Row
	Width      int
	Height     int
	// Unfinished is the git operation the repository is part way through. It
	// is drawn because the file list of a merge whose conflicts are resolved
	// is empty, and an empty list reads as nothing left to do.
	Unfinished   git.InProgress
	Notice       string
	NoticeFailed bool
	// ScrollTop is the first visible list line when the list outgrows the pane.
	ScrollTop  int
	HelpOpen   bool
	Head       git.Head
	Fetched    string
	Changes    Changes
	History    History
	Stashed    Stashed
	Worktrees  Worktrees
	Open       OpenDiff
	HelpScroll int
	LastUndo   *git.Undo
}

// FoldedIn reports whether a directory is folded in one section. The same path
// shows in both when a file is staged and edited again, and each copy folds on
// its own.
func FoldedIn(s State, sec Section, dir string) bool {
	return s.Changes.Folded[foldKey(sec, dir)]
}

func IsHiddenByFoldedDirectory(s State, sec Section, path string) bool {
	if path == "" {
		return false
	}
	for directory := ParentDir(path); ; directory = ParentDir(directory) {
		if FoldedIn(s, sec, directory) {
			return true
		}
		if directory == "" {
			return false
		}
	}
}

func foldKey(sec Section, dir string) string {
	return strconv.Itoa(int(sec)) + "\x00" + dir
}

// clampIndex bounds a row or block index to [0, last]. An empty list has
// last == -1 and answers 0, which is the cursor position an empty pane holds.
func clampIndex(v, last int) int {
	if v < 0 || last < 0 {
		return 0
	}
	if v > last {
		return last
	}
	return v
}

// StagedFileCount lives on State so footer and commit agree on when a commit can run.
func (s State) StagedFileCount() int {
	seen := map[string]bool{}
	n := 0
	for _, row := range s.Rows {
		if row.Section() != SectionStaged || !row.Entry().IsStaged() {
			continue
		}
		if seen[row.Path()] {
			continue
		}
		seen[row.Path()] = true
		n++
	}
	return n
}
