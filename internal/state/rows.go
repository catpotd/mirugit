package state

import (
	"strconv"

	"github.com/catpotd/mirugit/internal/git"
)

// RowKind is what a row stands for. The tabs draw different things and the
// verbs differ per kind, so one list cannot be a list of files.
type RowKind int

const (
	RowFile RowKind = iota
	RowCommit
	RowStash
	RowWorktree
	RowSectionHeading
	RowDirectory
)

// Row is one drawn line. A file changed on both sides is two rows, so a row
// cannot be named by its path alone.
//
// The fields are unexported and the payloads are values rather than pointers,
// so a Row whose Kind says commit and whose commit is missing cannot be built.
// That state used to be reachable, and Path answered "" for it — the same
// answer a heading gives, so an invalid row showed up as a row that did
// nothing rather than as an error.
type Row struct {
	kind     RowKind
	section  Section
	entry    git.Entry
	dirPath  string
	commit   git.CommitInfo
	stash    git.StashRow
	worktree git.WorktreeRow
	// parent is the row this one is printed under, or -1 at the top level. The
	// builder knows it while it is placing the row and used to drop it, so five
	// places walked backwards to find it again and three of them disagreed.
	parent int
}

// FileRow is a file on any tab. Section says which side of the index it is on,
// and is SectionNone for the files nested under a stash or a worktree.
func FileRow(e git.Entry, sec Section) Row {
	return Row{kind: RowFile, entry: e, section: sec, parent: noParent}
}

// noParent is the parent of a row at the top level of its list.
const noParent = -1

// under names the row this one is printed beneath. Only the builders call it,
// because a parent is a fact about where the row was placed rather than about
// what it holds.
func (r Row) under(parent int) Row {
	r.parent = parent
	return r
}

// Parent answers the index of the row this one is printed under, and false at
// the top level.
func (r Row) Parent() (int, bool) {
	return r.parent, r.parent >= 0
}

// CommitRow is one commit on the history tab.
func CommitRow(c git.CommitInfo) Row {
	return Row{kind: RowCommit, commit: c, parent: noParent}
}

// StashRowOf is one stash on the stashed tab. The name carries Of because
// git.StashRow is the value it holds.
func StashRowOf(st git.StashRow) Row {
	return Row{kind: RowStash, stash: st, parent: noParent}
}

// WorktreeRowOf is one linked worktree on the worktrees tab.
func WorktreeRowOf(w git.WorktreeRow) Row {
	return Row{kind: RowWorktree, worktree: w, parent: noParent}
}

// SectionHeadingRow is the "staged"/"unstaged" line on the changes tab.
func SectionHeadingRow(sec Section) Row {
	return Row{kind: RowSectionHeading, section: sec, parent: noParent}
}

// DirectoryRow is a foldable directory on the changes tab.
func DirectoryRow(path string, sec Section) Row {
	return Row{kind: RowDirectory, dirPath: path, section: sec, parent: noParent}
}

// Kind says what the row stands for. Every other accessor answers the zero
// value unless Kind names it.
func (r Row) Kind() RowKind { return r.kind }

// Section is the side of the index a file or a directory sits on, and the
// section a heading names.
func (r Row) Section() Section { return r.section }

func (r Row) Entry() git.Entry          { return r.entry }
func (r Row) DirPath() string           { return r.dirPath }
func (r Row) Commit() git.CommitInfo    { return r.commit }
func (r Row) Stash() git.StashRow       { return r.stash }
func (r Row) Worktree() git.WorktreeRow { return r.worktree }

// Path names what a row points at, whichever kind it is, so that selection
// and the cursor do not each need to know the kinds apart.
func (r Row) Path() string {
	switch r.kind {
	case RowFile:
		return r.entry.Path
	case RowCommit:
		return r.commit.SHA
	case RowStash:
		return r.stash.Ref
	case RowWorktree:
		return r.worktree.Path
	case RowSectionHeading, RowDirectory:
	}
	// A heading and a directory point at no single thing. Go wants a statement
	// after the switch even though every kind is named above.
	return ""
}

func contains(rows []Row, path string) bool {
	for _, r := range rows {
		if r.Path() == path {
			return true
		}
	}
	return false
}

func hasOpenRow(s State) bool {
	if !s.Open.Origin.IsWorkingTree() {
		// A commit's or a stash's file is listed under the row that holds it,
		// and that row's own path is what the list carries.
		return contains(s.Rows, s.Open.Path)
	}
	for _, r := range s.Rows {
		if r.Kind() == RowFile && r.Section() == s.Open.Origin.Side() && r.Path() == s.Open.Path {
			return true
		}
	}
	return false
}

// stillPresent drops paths the refresh no longer lists, because git restore
// refuses the whole call over one stale path and the count would name rows the
// verb could not reach.
func stillPresent(selected map[string]bool, rows []Row) map[string]bool {
	out := make(map[string]bool, len(selected))
	for _, r := range rows {
		if r.Kind() != RowFile {
			continue
		}
		if key := selectionKey(r.Section(), r.Path()); selected[key] {
			out[key] = true
		}
	}
	return out
}

// cursorKey names a row for reload. The cursor keeps its index, but new rows
// above can make that index point at a different file; DirPath holds directory
// rows because Path() is empty for them.
func cursorKey(r Row) string {
	p := r.Path()
	if r.Kind() == RowDirectory {
		p = r.DirPath()
	}
	return strconv.Itoa(int(r.Kind())) + "\x00" + strconv.Itoa(int(r.Section())) + "\x00" + p
}

func rowIndex(rows []Row, key string) int {
	if key == "" {
		return -1
	}
	for i, r := range rows {
		if cursorKey(r) == key {
			return i
		}
	}
	return -1
}

func cursorKeyAt(rows []Row, cursor int) string {
	if cursor >= 0 && cursor < len(rows) {
		return cursorKey(rows[cursor])
	}
	return ""
}

// Row lists are rebuilt from scratch; the same index can point at a different
// row after rows above shrink or grow, so the cursor is restored by key.
func restoredCursor(rows []Row, key string, cursor int) int {
	if i := rowIndex(rows, key); i >= 0 {
		return i
	}
	return clampIndex(cursor, len(rows)-1)
}

func rowsForTab(s State) []Row {
	return Facts[s.Tab].Rows(s)
}

// cursorPolicy says what becomes of the cursor when the list is rebuilt.
type cursorPolicy int

const (
	// keepCursorRow follows the row the cursor was on. The list is rebuilt from
	// scratch, so the same index points at a different row once rows above it
	// appear or go.
	keepCursorRow cursorPolicy = iota
	// keepCursorIndex holds the index and pulls it inside the new list. It is
	// for the moves that have already decided where the cursor belongs — a tab
	// change restores that tab's own index, and leaving for changes starts at
	// the top.
	keepCursorIndex
)

// refreshRows rebuilds the list this tab draws and settles the cursor.
//
// Rows is not state the program keeps; it is what the payloads project to. It
// is stored because a frame reads it many times, and eleven places used to
// write it, each picking a cursor policy of its own. One of them picked clamp
// where its neighbors picked the row, and the table gained a field to describe
// the difference — a field nothing read, because the difference was not a
// decision anybody made.
func refreshRows(s State, policy cursorPolicy) State {
	prevKey := cursorKeyAt(s.Rows, s.Cursor)
	s.Rows = rowsForTab(s)
	if policy == keepCursorRow {
		s.Cursor = restoredCursor(s.Rows, prevKey, s.Cursor)
		return s
	}
	s.Cursor = clampIndex(s.Cursor, len(s.Rows)-1)
	return s
}

// StashRowsFor builds the stashed tab list, expanding file rows under the
// stash the cursor sits on when their contents are already loaded.
func StashRowsFor(s State) []Row {
	return stashParents.rowsFor(s)
}

// WorktreeRowsFor builds the worktrees tab list the same way for one tree.
func WorktreeRowsFor(s State) []Row {
	return worktreeParents.rowsFor(s)
}

// parentTab describes one of the two tabs whose list is parents that expand
// into the files they hold. The two differ only in the key a parent is found
// by after the list is rebuilt — a stash by Ref, a worktree by Path — and every
// function below was written twice because of that one difference.
type parentTab struct {
	kind RowKind
	tab  Tab
	// keyOf answers "" for a row that is not a parent of this kind, so a caller
	// walking mixed rows does not have to check the pointer itself.
	keyOf    func(Row) string
	parents  func(State) []Row
	files    func(State) []git.Entry
	setFiles func(State, []git.Entry) State
	// identityOf names a parent by something that outlives a reload. A stash is
	// asked for by ref, and a ref is a place in the list: dropping one renames
	// every stash below it, so a reader's open stash would become whichever one
	// moved into its number.
	identityOf func(Row) string
	// expanded holds the identity of the parent the reader opened. History
	// answers this with a field of its own and these two used to answer it with
	// the cursor's position instead.
	expanded    func(State) string
	setExpanded func(State, string) State
}

var stashParents = parentTab{
	kind: RowStash,
	tab:  TabStashed,
	keyOf: func(r Row) string {
		if r.Kind() != RowStash {
			return ""
		}
		return r.Stash().Ref
	},
	parents: func(s State) []Row {
		rows := make([]Row, len(s.Stashed.Stashes))
		for i := range s.Stashed.Stashes {
			rows[i] = StashRowOf(s.Stashed.Stashes[i])
		}
		return rows
	},
	files: func(s State) []git.Entry { return s.Stashed.Files },
	setFiles: func(s State, files []git.Entry) State {
		s.Stashed.Files = files
		return s
	},
	identityOf: func(r Row) string {
		if r.Kind() != RowStash {
			return ""
		}
		return r.Stash().SHA
	},
	expanded: func(s State) string { return s.Stashed.Expanded },
	setExpanded: func(s State, key string) State {
		s.Stashed.Expanded = key
		return s
	},
}

var worktreeParents = parentTab{
	kind: RowWorktree,
	tab:  TabWorktrees,
	keyOf: func(r Row) string {
		if r.Kind() != RowWorktree {
			return ""
		}
		return r.Worktree().Path
	},
	parents: func(s State) []Row {
		rows := make([]Row, len(s.Worktrees.List))
		for i := range s.Worktrees.List {
			rows[i] = WorktreeRowOf(s.Worktrees.List[i])
		}
		return rows
	},
	files: func(s State) []git.Entry { return s.Worktrees.Files },
	setFiles: func(s State, files []git.Entry) State {
		s.Worktrees.Files = files
		return s
	},
	// A tree is asked for by the path it lives at, and that is what it is.
	identityOf: func(r Row) string {
		if r.Kind() != RowWorktree {
			return ""
		}
		return r.Worktree().Path
	},
	expanded: func(s State) string { return s.Worktrees.Expanded },
	setExpanded: func(s State, key string) State {
		s.Worktrees.Expanded = key
		return s
	},
}

func (p parentTab) rowsFor(s State) []Row {
	parents := p.parents(s)
	files := p.files(s)
	expand := p.expandIndex(s, parents)
	rows := make([]Row, 0, len(parents)+len(files))
	for i, parent := range parents {
		rows = append(rows, parent)
		if i != expand || len(files) == 0 {
			continue
		}
		at := len(rows) - 1
		for _, f := range files {
			rows = append(rows, expandedFileRow(f).under(at))
		}
	}
	return rows
}

// expandIndex names the parent whose files are shown. Before the first list is
// built there are no rows to walk, so the cursor is the index itself.
func (p parentTab) expandIndex(s State, parents []Row) int {
	identity := p.expanded(s)
	if identity == "" {
		return -1
	}
	for i, parent := range parents {
		if p.identityOf(parent) == identity {
			return i
		}
	}
	return -1
}

// ParentAbove answers the row of the given kind at or above i, and where it
// sits: the row itself when the cursor is on it, and the row it belongs to when
// the cursor is on one of the files printed under it.
//
// Five places answered this and three of them disagreed. The history tab did
// not walk at all, so a file inside an expanded commit had no commit: copying
// its SHA and undoing it did nothing, while the same position on the stashed
// and worktrees tabs worked.
//
// It follows the link the builder recorded rather than scanning backwards. A
// scan finds whatever row of that kind happens to sit above, which is the same
// answer only while the list has one level of nesting.
//
// The index comes back with the row because the caller needs both and computing
// the second from the cursor gives a different parent: on the file row of the
// tip commit, counting commit rows above the cursor answered one while the
// parent was commit zero, and undo is only offered at zero.
func ParentAbove(rows []Row, i int, kind RowKind) (Row, int, bool) {
	for i >= 0 && i < len(rows) {
		if rows[i].Kind() == kind {
			return rows[i], i, true
		}
		// A link that does not point upwards is one the list outgrew: rows are
		// dropped from the front when a commit leaves the log, and an index
		// that then names the row itself walks in a circle.
		parent, ok := rows[i].Parent()
		if !ok || parent >= i {
			break
		}
		i = parent
	}
	return Row{}, -1, false
}

// filesLoaded takes an answer only if the reader is still on the parent it was
// asked for. The answers do not arrive in the order they were asked for.
// filesLoaded opens the parent the files were read for. Which parent that is
// comes from the answer itself, not from where the cursor is now: the read runs
// while the reader keeps moving, and matching against the cursor dropped every
// answer that arrived after they had gone on.
func (p parentTab) filesLoaded(s State, key string, files []git.Entry) State {
	identity, ok := p.identityFor(s, key)
	if !ok {
		return s
	}
	s = p.setExpanded(s, identity)
	s = p.setFiles(s, files)
	if s.Tab != p.tab {
		return s
	}
	return refreshRows(s, keepCursorRow)
}

// identityFor turns the key a read was asked with into the one that outlives a
// reload. It answers false for a parent this tab no longer lists: a stash
// dropped or a tree removed while its files were being read leaves an answer
// for a row that is gone.
func (p parentTab) identityFor(s State, key string) (string, bool) {
	for _, parent := range p.parents(s) {
		if p.keyOf(parent) == key {
			return p.identityOf(parent), true
		}
	}
	return "", false
}

// A file listed under a stash or a worktree sits on no side of this index, so
// it carries SectionNone. Which side of its own repository holds the change is
// already on the entry.
func expandedFileRow(e git.Entry) Row {
	return FileRow(e, SectionNone)
}

func FirstFileRow(rows []Row) int {
	for i, r := range rows {
		if r.Kind() == RowFile {
			return i
		}
	}
	return -1
}

func historyRows(commits []git.CommitInfo, expandedSHA string, files []git.Entry) []Row {
	rows := make([]Row, 0, len(commits)+len(files))
	for i := range commits {
		rows = append(rows, CommitRow(commits[i]))
		if commits[i].SHA != expandedSHA {
			continue
		}
		at := len(rows) - 1
		for _, e := range files {
			rows = append(rows, FileRow(e, SectionNone).under(at))
		}
	}
	return rows
}

// ParentOfFile answers the row that owns the file row at path — the commit,
// stash or worktree printed above it. One copy of this walk existed per tab,
// differing only in which field the caller then read off the parent.
func ParentOfFile(rows []Row, path string, kind RowKind) (Row, bool) {
	for i, row := range rows {
		if row.Kind() == RowFile && row.Path() == path {
			parent, _, ok := ParentAbove(rows, i, kind)
			return parent, ok
		}
	}
	return Row{}, false
}

// RowAt answers the row at i and false when i is outside the list. The list
// moves under a click: a reload from the watcher can shorten Rows between the
// frame a click was drawn on and the click arriving, which is how a click on a
// row that had gone used to take the pane down.
//
// The bound is written here rather than at each caller because it was written
// at nineteen of them, and a mutation run showed fourteen could be broken
// without a test noticing.
func RowAt(s State, i int) (Row, bool) {
	if i < 0 || i >= len(s.Rows) {
		return Row{}, false
	}
	return s.Rows[i], true
}

// CursorRow answers the row the cursor is on, and false when it is on none.
func CursorRow(s State) (Row, bool) { return RowAt(s, s.Cursor) }

// collapse closes the open row and drops the files it was showing. Rebuilding
// the rows is the caller's, because doing it here would make this table's own
// entry depend on the table.
func (p parentTab) collapse(s State) State {
	s = p.setExpanded(s, "")
	return p.setFiles(s, nil)
}

// The three closers are named functions rather than the methods themselves so
// that the table can hold them: a var that referred to a method value of
// another var in the same file made the initialisation circular.
func collapseStash(s State) State    { return stashParents.collapse(s) }
func collapseWorktree(s State) State { return worktreeParents.collapse(s) }
func collapseCommit(s State) State {
	s.History.ExpandedSHA, s.History.ExpandedFiles = "", nil
	return s
}
