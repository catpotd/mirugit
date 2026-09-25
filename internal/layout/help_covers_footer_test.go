package layout

import (
	"strings"
	"testing"

	"github.com/catpotd/mirugit/internal/state"
)

// The help overlay is where a reader looks up a key they saw in the footer. A
// footer key with no help row leaves them with a word and no way to learn what
// it does.
//
// Comparing verb names alone is not enough: two operations that share one name
// satisfy the check while the single help row describes only one of them. The
// keys are compared here as well, because a row that names a different key is a
// row about a different operation. That is what "undo" was — the footer printed
// it for the reset of the last commit and the help explained the undo of the
// last discard.
func TestEveryFooterKeyHasAHelpRowForThatKey(t *testing.T) {
	t.Parallel()
	w := Renderer{Clipboard: true, Browser: true}
	for tab := range state.TabCount {
		t.Run(state.Facts[tab].Name, func(t *testing.T) {
			t.Parallel()
			shown := footerVerbUnionForTab(tab, w)
			rows := helpRowsForTab(tab, w)

			for verb := range shown {
				key, hasKey := FooterKeyFor(verb)
				if !hasKey {
					continue
				}
				// A verb with no row of its own is looked up in the row that
				// lists its key beside another verb's.
				wanted := verb
				if under, ok := helpRowFor[verb]; ok {
					wanted = under
				}
				explained := false
				for _, row := range rows {
					if row.verb != wanted {
						continue
					}
					// A row may list several keys for one verb ("j  k").
					for _, listed := range strings.Fields(row.keys) {
						if listed == key {
							explained = true
						}
					}
				}
				if !explained {
					t.Errorf("the footer prints %q for %q, and no help row on this tab "+
						"explains that key", key, verb)
				}
			}
		})
	}
}

// helpAlwaysVerbs names the verbs whose help row is drawn on every tab: moving
// about, quitting, and the three that reach the network.
//
// The list is written out here rather than read from the map. Reading it from
// the map makes the check disappear along with whatever is taken out of it: an
// entry removed leaves a sweep with one fewer verb in it, still green.
var alwaysListedVerbs = []state.VerbName{
	state.VerbNameDown, state.VerbNamePage, state.VerbNameEnds,
	state.VerbNameNextTab, state.VerbNameQuit, state.VerbNameReload,
	state.VerbNameFetch, state.VerbNameSync, state.VerbNameUndo,
}

func TestTheVerbsTheHelpAlwaysListsAreOnEveryTab(t *testing.T) {
	t.Parallel()
	w := Renderer{Clipboard: true, Browser: true}

	if len(helpAlwaysVerbs) != len(alwaysListedVerbs) {
		t.Errorf("helpAlwaysVerbs holds %d verbs and this test names %d",
			len(helpAlwaysVerbs), len(alwaysListedVerbs))
	}
	for _, verb := range alwaysListedVerbs {
		if !helpAlwaysVerbs[verb] {
			t.Errorf("%q is named here as always listed and helpAlwaysVerbs does not say so", verb)
		}
	}

	for tab := range state.TabCount {
		t.Run(state.Facts[tab].Name, func(t *testing.T) {
			t.Parallel()
			rows := helpRowsForTab(tab, w)
			for _, verb := range alwaysListedVerbs {
				found := false
				for _, row := range rows {
					if row.verb == verb {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("%q is always listed and this tab's help has no row for it", verb)
				}
			}
		})
	}
}

// Turning an entry off changes nothing for a verb the footer has no key for:
// the switch below the lookup answers the same. Undo is the one entry the
// lookup decides on its own.
//
// This says which of them are in that position, so the day one moves onto the
// footer the answer stops being the same and this fails rather than the map
// quietly starting to matter.
func TestTheAlwaysListedVerbsTheFooterAlsoOffers(t *testing.T) {
	t.Parallel()
	decidedHere := map[state.VerbName]bool{state.VerbNameUndo: true}
	for _, verb := range alwaysListedVerbs {
		_, onFooter := footerKeys[verb]
		if onFooter != decidedHere[verb] {
			t.Errorf("%q is on the footer = %v; the lookup in helpAlwaysVerbs decides "+
				"it = %v. Add it to decidedHere, and check that the help still lists "+
				"it on every tab", verb, onFooter, decidedHere[verb])
		}
	}
}

// HelpVerbs walks the base rows and then every tab's rows, so a verb shown on
// three tabs is reached four times. The list it hands back names each verb
// once. Its callers ask "is this verb in the help" with a Contains, which
// answers the same whether the verb is in it once or four times, so nothing
// else would notice the list growing.
func TestHelpVerbsNamesEachVerbOnce(t *testing.T) {
	t.Parallel()
	verbs := HelpVerbs()
	if len(verbs) == 0 {
		t.Fatal("the help names no verb, so this proves nothing")
	}
	seen := make(map[state.VerbName]int, len(verbs))
	for _, v := range verbs {
		seen[v]++
	}
	for v, n := range seen {
		if n != 1 {
			t.Errorf("%q is in the list %d times", v, n)
		}
	}
	// A verb drawn on more than one tab is what makes the count above mean
	// something: without one, walking the tabs could stop after the first and
	// this would still pass.
	shared := 0
	for tab := range state.TabCount {
		for _, row := range helpRowsForTab(tab, Renderer{Clipboard: true, Browser: true}) {
			if row.verb == state.VerbNameQuit {
				shared++
			}
		}
	}
	if shared < 2 {
		t.Errorf("quit is drawn on %d tabs; the count above needs a verb on several", shared)
	}
}
