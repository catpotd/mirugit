package layout

import "github.com/catpotd/mirugit/internal/state"

// TargetKind is typed rather than packed into a string, because the one class
// of bug this program keeps producing is a click resolving to something other
// than what was drawn.
type TargetKind int

const (
	// TargetNone marks a screen line that only carries the cursor, not a click target.
	TargetNone TargetKind = iota
	// TargetFile opens or selects the file row at Row in s.Rows.
	TargetFile
	// TargetDirectory folds the directory row at Row in s.Rows.
	TargetDirectory
	// TargetSectionHeading moves the cursor onto the section heading named by Section.
	TargetSectionHeading
	// TargetTab switches to Tab when the tab bar label is clicked.
	TargetTab
	// TargetBlock moves the diff block cursor to Block.
	TargetBlock
	// TargetButton runs the named header button, such as sync.
	TargetButton
	// TargetCommitBox focuses the commit message field.
	TargetCommitBox
	// TargetCommit requests a commit when the commit button is clicked.
	TargetCommit
	// TargetCheckbox toggles file, directory, or section selection.
	TargetCheckbox
	// TargetVerb names which verb a click landed on, so the row and the action bar
	// can offer the same words without the caller re-deriving them from the text.
	TargetVerb
	// TargetHelp opens the list; TargetHelpLine runs the command a line names.
	TargetHelp
	TargetHelpLine
)

type Target struct {
	Kind   TargetKind
	Path   string
	Commit string
	Block  int
	Tab    state.Tab
	Name   string
	// Verb names the command a verb region, a help line, or a header button
	// runs. Name stays for labels that no key runs, such as a tab's own word.
	Verb state.VerbName
	// Row names which s.Rows entry was hit when the target is a list row.
	Row int
	// Section names which section heading or section checkbox was hit.
	Section state.Section
}

// Region is half-open in columns: ColStart is inside, ColEnd is not. The field
// names alone leave a reader guessing which end is included.
type Region struct {
	Target   Target
	Row      int
	ColStart int
	ColEnd   int
}

// Frame carries the lines and the regions together, because the bug this
// program keeps producing is the two disagreeing, and one value can be asserted
// as a whole.
type Frame struct {
	Lines   []string
	Regions []Region
}

// Hit names the region under a coordinate and, separately, the file or
// directory row that screen line belongs to. A click arms the row it lands on,
// and a file's name covers only a few of the seventy-seven cells the reader can
// be clicking.
func (f Frame) Hit(row, col int) (region, rowTarget Target, hasRow bool) {
	for _, r := range f.Regions {
		if r.Row == row && col >= r.ColStart && col < r.ColEnd {
			region = r.Target
			break
		}
	}
	for _, r := range f.Regions {
		if r.Row != row {
			continue
		}
		switch r.Target.Kind {
		case TargetFile, TargetDirectory:
			if r.Target.Row >= 0 {
				return region, r.Target, true
			}
		case TargetNone, TargetSectionHeading, TargetTab, TargetBlock, TargetButton, TargetCommitBox, TargetCommit, TargetCheckbox, TargetVerb, TargetHelp, TargetHelpLine:
			// Only files and directories are list rows. A verb, a tab or the
			// help mark sits on a row but is not one.
		}
	}
	return region, Target{}, false
}
