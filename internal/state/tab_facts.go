package state

// TabFacts holds the per-tab constants that used to diverge across layout and tui.
type TabFacts struct {
	Name                 string
	HeaderRows           int
	BlankLineAfterNotice bool
	AlwaysShowInBar      bool
	LeaveWhenCountAtMost int
	AllowsSelection      bool
	HelpClearClosesDiff  bool
	RequestReadOnTab     bool
	DiffFollowsCursor    bool
	LowerUResetsCommit   bool
	DiffPeekWhenClosed   bool
	HistoryPaging        bool
	SkipsFoldedRows      bool

	// HasCommitBox says the pane draws the message field on this tab. The c key
	// focused the field on every tab, and a focused field takes every key it can
	// type: pressing c on the stashed tab left j and k and the tab numbers going
	// into a box nothing was drawing.
	HasCommitBox bool

	// OpenFile says where a file row's diff comes from. It replaced a Section
	// and a bool that could disagree: a tab could claim both that its files
	// read from a commit and that they read from the row above.
	OpenFile OpenFileWay

	// DiscardVerb is what the x key runs here. Two bools used to say this, and
	// nothing stopped both from being true at once.
	DiscardVerb VerbName

	// DrawsWorktreeStatus says the rows show what each worktree holds: how many
	// files it has and how far its branch is from the base. Filling that costs a
	// git per tree, and every reload reads the worktree list so the bar's fourth
	// count follows the repository like the other three.
	DrawsWorktreeStatus bool

	// RowKeys are the verbs this tab answers beyond the ones every tab has.
	// Five bools used to say this, one per verb.
	RowKeys []VerbName

	// Rows builds the list this tab draws. It sits here so that adding a tab
	// touches this table rather than a switch in another file.
	Rows func(State) []Row

	// Collapse closes the row this tab has open. A tab whose rows do not expand
	// returns the state unchanged.
	Collapse func(State) State

	// ExpandedKey names the row this tab has open, in the key its reader asks
	// for files with: a stash by ref, a worktree by path, a commit by SHA. It
	// answers "" when nothing is open.
	ExpandedKey func(State) string

	// ParentRow answers the row whose files are shown under the cursor and where
	// it sits, and false on a tab whose rows do not expand. Every question about
	// "the row the reader is on" goes through it: which stash a key acts on,
	// which commit a file belongs to, and what the footer offers there. The
	// expanded files are rows too, so a cursor index alone cannot answer any of
	// them.
	ParentRow func(State) (Row, int, bool)
}

// OpenFileWay says where a file row reads its diff from.
type OpenFileWay int

const (
	// OpenFileFromWorkingTree is the changes tab: the file is in this
	// repository and git diff answers for it.
	OpenFileFromWorkingTree OpenFileWay = iota
	// OpenFileFromCommit is the history tab: the diff belongs to the commit
	// printed above the file.
	OpenFileFromCommit
	// OpenFileFromParent is the stashed and worktrees tabs: the diff belongs to
	// the stash or the tree printed above the file, neither of which the
	// working tree holds.
	OpenFileFromParent
)

// AnswersKey reports whether this tab runs verb when its key is pressed.
func (f TabFacts) AnswersKey(verb VerbName) bool {
	for _, v := range f.RowKeys {
		if v == verb {
			return true
		}
	}
	return false
}

// Facts is one row per tab. The rows are built rather than written out,
// because nine of the eighteen fields held nothing but "which tab is this":
// six were true on the changes tab alone, one false there alone, and two named
// history. A per-tab value that says which tab it is can be set to the wrong
// answer, and a wrong entry looks exactly like a right one.
//
// changesFacts holds those nine as constants. parentFacts does not take them,
// so a tab whose rows expand into files cannot be given a commit box.
var Facts = [TabCount]TabFacts{
	changesFacts(),
	parentFacts(parentTabFacts{
		Name:                 "history",
		LeaveWhenCountAtMost: 0,
		DiffFollowsCursor:    true,
		LowerUResetsCommit:   true,
		HistoryPaging:        true,
		OpenFile:             OpenFileFromCommit,
		DiscardVerb:          VerbNameDiscard,
		RowKeys:              []VerbName{VerbNameSHA, VerbNameOpen},
		ParentKind:           RowCommit,
		Rows: func(s State) []Row {
			return historyRows(s.History.Commits, s.History.ExpandedSHA, s.History.ExpandedFiles)
		},
		Collapse:    collapseCommit,
		ExpandedKey: func(s State) string { return s.History.ExpandedSHA },
	}),
	parentFacts(parentTabFacts{
		Name:                 "stashed",
		LeaveWhenCountAtMost: 0,
		OpenFile:             OpenFileFromParent,
		DiscardVerb:          VerbNameDrop,
		RowKeys:              []VerbName{VerbNameRestore, VerbNameBranch},
		ParentKind:           RowStash,
		Rows:                 StashRowsFor,
		Collapse:             collapseStash,
		// The ref, because that is what a read is asked with; Expanded holds
		// the SHA, which is what survives the list being renumbered.
		ExpandedKey: func(s State) string {
			for _, stash := range s.Stashed.Stashes {
				if stash.SHA == s.Stashed.Expanded {
					return stash.Ref
				}
			}
			return ""
		},
	}),
	parentFacts(parentTabFacts{
		Name:                 "worktrees",
		LeaveWhenCountAtMost: 1,
		OpenFile:             OpenFileFromParent,
		DiscardVerb:          VerbNameRemove,
		RowKeys:              []VerbName{VerbNameGo},
		DrawsWorktreeStatus:  true,
		ParentKind:           RowWorktree,
		Rows:                 WorktreeRowsFor,
		Collapse:             collapseWorktree,
		ExpandedKey:          func(s State) string { return s.Worktrees.Expanded },
	}),
}

// changesFacts is the tab that draws the working tree itself. It is the only
// one with a commit box and the only one whose verbs spend a selection.
func changesFacts() TabFacts {
	return TabFacts{
		Name:                 "changes",
		HeaderRows:           5,
		HasCommitBox:         true,
		BlankLineAfterNotice: true,
		AlwaysShowInBar:      true,
		LeaveWhenCountAtMost: -1,
		AllowsSelection:      true,
		HelpClearClosesDiff:  false,
		RequestReadOnTab:     true,
		DiffFollowsCursor:    true,
		LowerUResetsCommit:   false,
		DiffPeekWhenClosed:   true,
		HistoryPaging:        false,
		SkipsFoldedRows:      true,
		OpenFile:             OpenFileFromWorkingTree,
		DiscardVerb:          VerbNameDiscard,
		RowKeys:              nil,
		DrawsWorktreeStatus:  false,
		Rows:                 func(s State) []Row { return displayOrder(s.Changes.Entries) },
		Collapse:             keepExpanded,
		ExpandedKey:          nothingExpanded,
		ParentRow:            noParentRow,
	}
}

// parentTabFacts is what a tab of rows that expand into files still has to say
// for itself. Every field the changes tab alone answers is absent.
type parentTabFacts struct {
	Name                 string
	LeaveWhenCountAtMost int
	DiffFollowsCursor    bool
	LowerUResetsCommit   bool
	HistoryPaging        bool
	SkipsFoldedRows      bool
	OpenFile             OpenFileWay
	DiscardVerb          VerbName
	RowKeys              []VerbName
	DrawsWorktreeStatus  bool
	ParentKind           RowKind
	Rows                 func(State) []Row
	Collapse             func(State) State
	ExpandedKey          func(State) string
}

func parentFacts(p parentTabFacts) TabFacts {
	return TabFacts{
		Name:                 p.Name,
		HeaderRows:           3,
		HasCommitBox:         false,
		BlankLineAfterNotice: false,
		AlwaysShowInBar:      false,
		LeaveWhenCountAtMost: p.LeaveWhenCountAtMost,
		AllowsSelection:      false,
		HelpClearClosesDiff:  true,
		RequestReadOnTab:     false,
		DiffFollowsCursor:    p.DiffFollowsCursor,
		LowerUResetsCommit:   p.LowerUResetsCommit,
		HistoryPaging:        p.HistoryPaging,
		SkipsFoldedRows:      p.SkipsFoldedRows,
		DiffPeekWhenClosed:   false,
		OpenFile:             p.OpenFile,
		DiscardVerb:          p.DiscardVerb,
		RowKeys:              p.RowKeys,
		DrawsWorktreeStatus:  p.DrawsWorktreeStatus,
		Rows:                 p.Rows,
		Collapse:             p.Collapse,
		ExpandedKey:          p.ExpandedKey,
		ParentRow:            parentAbove(p.ParentKind),
	}
}

// nothingExpanded is ExpandedKey for a tab whose rows do not expand into files.
func nothingExpanded(State) string { return "" }

// keepExpanded is Collapse for a tab whose rows do not expand into files.
func keepExpanded(s State) State { return s }

// noParentRow is ParentRow for the same tabs.
func noParentRow(State) (Row, int, bool) { return Row{}, -1, false }

func parentAbove(kind RowKind) func(State) (Row, int, bool) {
	return func(s State) (Row, int, bool) { return ParentAbove(s.Rows, s.Cursor, kind) }
}

// TabVisible reports whether the bar should draw a tab at the given count.
func TabVisible(tab Tab, count int) bool {
	f := Facts[tab]
	if f.AlwaysShowInBar {
		return true
	}
	return count > f.LeaveWhenCountAtMost
}
