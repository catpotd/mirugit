package state

// VerbName is the word the footer prints and the key handler runs. It was a
// plain string in four separate vocabularies — the footer's key table, the
// tui's handler table, the per-tab lists, and the help rows — and a name that
// appeared in one but not another failed at run time with "unknown verb".
//
// The word is wrapped in a struct rather than declared as a named string type,
// because Go converts an untyped literal into a named string type without
// complaint: with `type VerbName string`, runVerb("quti") still compiles. A
// struct has no such conversion, so the only way to name a verb is to reference
// one of the values below.
type VerbName struct{ word string }

func (v VerbName) String() string { return v.word }

// IsZero reports the absence of a verb. A help row that is a blank separator
// and a Target that names no command both hold it.
func (v VerbName) IsZero() bool { return v.word == "" }

var (
	VerbNameDown = VerbName{"down"}
	// VerbNamePage and VerbNameEnds have no footer key of their own: they move
	// the cursor and the footer names what the cursor row can run. They are in
	// the vocabulary so the help can list them and a click on the help line can
	// run them.
	VerbNamePage      = VerbName{"page"}
	VerbNameEnds      = VerbName{"ends"}
	VerbNameSelect    = VerbName{"select"}
	VerbNameSelectAll = VerbName{"select all"}
	VerbNameExtend    = VerbName{"extend"}
	VerbNameNextTab   = VerbName{"next tab"}
	VerbNameFold      = VerbName{"fold"}
	// VerbNameUnfold is what the footer prints on a folded directory. It has no
	// handler of its own: the key runs VerbNameFold, which toggles.
	VerbNameUnfold  = VerbName{"unfold"}
	VerbNameQuit    = VerbName{"quit"}
	VerbNameCommit  = VerbName{"commit"}
	VerbNameClear   = VerbName{"clear"}
	VerbNameStage   = VerbName{"stage"}
	VerbNameUnstage = VerbName{"unstage"}
	VerbNameStash   = VerbName{"stash"}
	VerbNameDiscard = VerbName{"discard"}
	VerbNameRestore = VerbName{"restore"}
	VerbNameBranch  = VerbName{"branch"}
	VerbNameDrop    = VerbName{"drop"}
	VerbNameGo      = VerbName{"go"}
	VerbNameRemove  = VerbName{"remove"}
	// VerbNameUndo restores what the last discard removed. VerbNameUncommit
	// moves the last commit back to the staged area. They were one name, so the
	// help could describe only one of them and the other key went unexplained.
	VerbNameUndo      = VerbName{"undo"}
	VerbNameUncommit  = VerbName{"uncommit"}
	VerbNameDiff      = VerbName{"diff"}
	VerbNameNextBlock = VerbName{"next block"}
	VerbNameReload    = VerbName{"reload"}
	VerbNameSHA       = VerbName{"sha"}
	VerbNameOpen      = VerbName{"open"}
	VerbNameRead      = VerbName{"read"}
	VerbNameFetch     = VerbName{"fetch"}
	VerbNameSync      = VerbName{"sync"}
)

// AllVerbNames is the whole vocabulary. Tables keyed by VerbName are checked
// against it rather than against each other, so a name added here without a
// handler or a key is named by the test that fails.
var AllVerbNames = []VerbName{
	VerbNameDown, VerbNamePage, VerbNameEnds, VerbNameSelect, VerbNameSelectAll, VerbNameExtend,
	VerbNameNextTab, VerbNameFold, VerbNameUnfold, VerbNameQuit,
	VerbNameCommit, VerbNameClear, VerbNameStage, VerbNameUnstage,
	VerbNameStash, VerbNameDiscard, VerbNameRestore, VerbNameBranch,
	VerbNameDrop, VerbNameGo, VerbNameRemove, VerbNameUndo, VerbNameUncommit, VerbNameDiff,
	VerbNameNextBlock, VerbNameReload, VerbNameSHA, VerbNameOpen,
	VerbNameRead, VerbNameFetch, VerbNameSync,
}
