package layout

import (
	"testing"

	"github.com/catpotd/mirugit/internal/state"
)

// A row outside the window keeps its place and its click target and skips only
// the text. The cursor and the scroll are placed from those targets, so a
// hidden row that carries a different kind than the drawn one moves the cursor
// somewhere the reader is not: the history tab's hidden rows once carried
// TargetCommit, which CursorLineInList reads as "not a row", and the cursor
// went off the pane.
func TestAHiddenRowCarriesWhatTheDrawnRowCarries(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	for tab := range state.TabCount {
		t.Run(state.Facts[tab].Name, func(t *testing.T) {
			t.Parallel()
			s := paneStateFor(t, tab)
			drawn := rowKindsByIndex(renderFor(tab).List(s, nil, w, everyLine))
			hidden := rowKindsByIndex(renderFor(tab).List(s, nil, w, noLines))

			if len(drawn) == 0 {
				t.Fatal("the tab drew no rows, so this proves nothing")
			}
			for line, kind := range drawn {
				got, ok := hidden[line]
				if !ok {
					t.Errorf("screen line %d has no target when the window leaves it out", line)
					continue
				}
				if got != kind {
					t.Errorf("screen line %d is %v when drawn and %v when hidden", line, kind, got)
				}
			}
			// A hidden row that appears where no drawn row is would put a click
			// on a row the reader cannot see.
			for line := range hidden {
				if _, ok := drawn[line]; !ok {
					t.Errorf("screen line %d has a target only when hidden", line)
				}
			}
		})
	}
}

// rowKindsByIndex answers, for each screen line, which state row
// CursorLineInList would say sits there. That is the question the cursor and
// the scroll are placed by, so it is the one the two windows have to agree on.
// Target.Row is the state row and several regions on one line share it, so the
// screen line is the key.
func rowKindsByIndex(_ []string, regions []Region) map[int]TargetKind {
	out := map[int]TargetKind{}
	for _, reg := range regions {
		switch reg.Target.Kind {
		case TargetFile, TargetDirectory, TargetNone:
			if _, seen := out[reg.Row]; !seen {
				out[reg.Row] = reg.Target.Kind
			}
		case TargetSectionHeading, TargetTab, TargetBlock, TargetButton,
			TargetCommitBox, TargetCommit, TargetCheckbox, TargetVerb,
			TargetHelp, TargetHelpLine:
		}
	}
	return out
}
