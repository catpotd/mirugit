package tui

import (
	"testing"

	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
)

// The vocabulary lived in four tables that only agreed because tests said so,
// and a name in one but not another failed at run time with "unknown verb".
// AllVerbNames is now the one list; these check the tables against it rather
// than against each other, so a missing entry names itself.
func TestEveryVerbNameHasAHandler(t *testing.T) {
	t.Parallel()
	for _, name := range state.AllVerbNames {
		if _, ok := verbByName[name]; !ok {
			t.Errorf("%q に verbByName の handler が無い", name)
		}
	}
}

func TestEveryHandlerIsInTheVocabulary(t *testing.T) {
	t.Parallel()
	known := make(map[state.VerbName]bool, len(state.AllVerbNames))
	for _, name := range state.AllVerbNames {
		known[name] = true
	}
	for name := range verbByName {
		if !known[name] {
			t.Errorf("verbByName の %q が AllVerbNames に無い", name)
		}
	}
}

func TestEveryFooterKeyIsInTheVocabulary(t *testing.T) {
	t.Parallel()
	known := make(map[state.VerbName]bool, len(state.AllVerbNames))
	for _, name := range state.AllVerbNames {
		known[name] = true
	}
	for _, name := range state.AllVerbNames {
		if _, ok := layout.FooterKeyFor(name); ok {
			continue
		}
		// Verbs the footer never prints: they are reached by a key or a click
		// on something other than a footer word.
		switch name {
		case state.VerbNameDown, state.VerbNamePage, state.VerbNameEnds,
			state.VerbNameSelectAll, state.VerbNameExtend,
			state.VerbNameNextTab, state.VerbNameQuit, state.VerbNameCommit,
			state.VerbNameClear, state.VerbNameReload, state.VerbNameFetch,
			state.VerbNameSync:
			continue
		}
		t.Errorf("%q に footer のキーが無い", name)
	}
}

func TestEveryHelpRowVerbIsInTheVocabulary(t *testing.T) {
	t.Parallel()
	known := make(map[state.VerbName]bool, len(state.AllVerbNames))
	for _, name := range state.AllVerbNames {
		known[name] = true
	}
	for _, name := range layout.HelpVerbs() {
		if !known[name] {
			t.Errorf("help の %q が AllVerbNames に無い", name)
		}
	}
}
