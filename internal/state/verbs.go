package state

// Verb names git file operations the TUI confirms and runs. Cross-layer
// commands such as quit or down stay as layout.Target.Name strings in
// verbByName so navigation does not mix into VerbRequested or commands.go.
type Verb int

const (
	VerbStage Verb = iota
	VerbUnstage
	VerbDiscard
	VerbStash
	VerbUndo
)

// ChangesRowVerbs lists git file verbs for a changes-tab row. Layout adds
// select, diff, and read around this list.
func ChangesRowVerbs(row Row) []VerbName {
	if row.Kind() != RowFile {
		return nil
	}
	if row.Entry().IsConflicted() {
		return []VerbName{VerbNameDiscard}
	}
	var verbs []VerbName
	switch row.Section() {
	case SectionUnstaged:
		verbs = []VerbName{VerbNameStage, VerbNameDiscard}
	case SectionStaged:
		verbs = []VerbName{VerbNameUnstage, VerbNameDiscard}
	case SectionNone:
		return nil
	}
	if row.Entry().CanStash() {
		verbs = append(verbs, VerbNameStash)
	}
	return verbs
}

// VerbApplies reports whether verb can run on row. TUI key handling and the
// footer share this predicate so shown verbs match pressable ones.
func VerbApplies(verb Verb, row Row) bool {
	if row.Kind() != RowFile {
		return false
	}
	// ChangesRowVerbs never offers undo, so a verb that is not a selection verb
	// answers false here without a membership test of its own.
	return verbIn(ChangesRowVerbs(row), verbNames[verb])
}

var selectionVerbOrder = []Verb{VerbStage, VerbUnstage, VerbDiscard, VerbStash}

// verbNames is the word each Verb prints. selectionVerbOrder, not this map,
// says which of them a selection can spend.
var verbNames = map[Verb]VerbName{
	VerbStage:   VerbNameStage,
	VerbUnstage: VerbNameUnstage,
	VerbDiscard: VerbNameDiscard,
	VerbStash:   VerbNameStash,
	VerbUndo:    VerbNameUndo,
}

// SelectionVerbs lists verbs that apply to the current selection. The summary
// bar and the footer share this so the words on screen match what keys do.
func SelectionVerbs(s State) []VerbName {
	var out []VerbName
	for _, verb := range selectionVerbOrder {
		for _, row := range s.Rows {
			if !SelectedIn(s, row.Section(), row.Path()) {
				continue
			}
			if VerbApplies(verb, row) {
				out = append(out, verbNames[verb])
				break
			}
		}
	}
	return out
}
