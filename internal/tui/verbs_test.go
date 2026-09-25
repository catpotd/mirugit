package tui

import (
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
)

// helpVerbsWithoutHandler names the help verbs the registry cannot run. Both the
// real check and the test that proves the check works call it, so a change to
// one cannot leave the other passing on its own copy of the rule.
func helpVerbsWithoutHandler(registry map[state.VerbName]verbFn) []state.VerbName {
	var missing []state.VerbName
	for _, name := range layout.HelpVerbs() {
		if _, ok := registry[name]; !ok {
			missing = append(missing, name)
		}
	}
	return missing
}

func TestHelpAndRowVerbsShareOneHandlerMap(t *testing.T) {
	t.Parallel()
	for _, name := range helpVerbsWithoutHandler(verbByName) {
		t.Errorf("help verb %q has no handler in verbByName", name)
	}
	for name := range verbByName {
		found := slices.Contains(layout.HelpVerbs(), name)
		switch name {
		case state.VerbNameDrop, state.VerbNameRemove:
			found = slices.Contains(layout.HelpVerbs(), state.VerbNameDiscard)
		case state.VerbNameUnfold:
			// The help lists one row for the pair; the footer prints whichever
			// word applies to the directory under the cursor.
			found = slices.Contains(layout.HelpVerbs(), state.VerbNameFold)
		}
		if !found {
			t.Errorf("verbByName has %q but help does not list it", name)
		}
	}
}

func TestMissingHelpVerbIsDetected(t *testing.T) {
	t.Parallel()
	removed := layout.HelpVerbs()[0]
	shrunk := make(map[state.VerbName]verbFn, len(verbByName)-1)
	for name, fn := range verbByName {
		if name != removed {
			shrunk[name] = fn
		}
	}
	if !slices.Contains(helpVerbsWithoutHandler(shrunk), removed) {
		t.Fatalf("did not detect missing help verb %q", removed)
	}
}

func TestHelpLineClickAndKeyReachVerbByName(t *testing.T) {
	// backup := verbByName は参照コピーで、差し込んだスタブが後続テストに残る。
	// 複製を差し替えてから書き換える
	backup := verbByName
	stubbed := make(map[state.VerbName]verbFn, len(verbByName))
	for name, fn := range verbByName {
		stubbed[name] = fn
	}
	verbByName = stubbed
	t.Cleanup(func() { verbByName = backup })

	type counts struct {
		help int
		key  int
	}
	called := map[state.VerbName]*counts{
		state.VerbNameDown:   {},
		state.VerbNameSelect: {},
		state.VerbNameFold:   {},
		state.VerbNameQuit:   {},
	}
	for name := range called {
		name := name
		verbByName[name] = func(m *Model) (tea.Model, tea.Cmd) {
			called[name].help++
			return m, nil
		}
	}

	m := &Model{state: state.State{Tab: state.TabChanges, Height: 24, Width: 80}}

	_, _ = m.helpLineClick(layout.Target{Verb: state.VerbNameDown})
	if called[state.VerbNameDown].help != 1 {
		t.Errorf("helpLineClick down: got %d calls, want 1", called[state.VerbNameDown].help)
	}

	called[state.VerbNameDown].help = 0
	verbByName[state.VerbNameDown] = func(m *Model) (tea.Model, tea.Cmd) {
		called[state.VerbNameDown].key++
		return m, nil
	}
	_, _ = m.navKey(tea.KeyPressMsg{Code: 'j'})
	if called[state.VerbNameDown].key != 1 {
		t.Errorf("key j/down: got %d calls, want 1", called[state.VerbNameDown].key)
	}

	_, _ = m.helpLineClick(layout.Target{Verb: state.VerbNameSelect})
	if called[state.VerbNameSelect].help != 1 {
		t.Errorf("helpLineClick select: got %d calls, want 1", called[state.VerbNameSelect].help)
	}

	called[state.VerbNameSelect].help = 0
	verbByName[state.VerbNameSelect] = func(m *Model) (tea.Model, tea.Cmd) {
		called[state.VerbNameSelect].key++
		return m, nil
	}
	_, _ = m.navKey(tea.KeyPressMsg{Code: ' '})
	if called[state.VerbNameSelect].key != 1 {
		t.Errorf("key space/select: got %d calls, want 1", called[state.VerbNameSelect].key)
	}

	_, _ = m.helpLineClick(layout.Target{Verb: state.VerbNameFold})
	if called[state.VerbNameFold].help != 1 {
		t.Errorf("helpLineClick fold: got %d calls, want 1", called[state.VerbNameFold].help)
	}

	called[state.VerbNameFold].help = 0
	verbByName[state.VerbNameFold] = func(m *Model) (tea.Model, tea.Cmd) {
		called[state.VerbNameFold].key++
		return m, nil
	}
	_, _ = m.navKey(tea.KeyPressMsg{Code: tea.KeyLeft})
	if called[state.VerbNameFold].key != 1 {
		t.Errorf("key left/fold: got %d calls, want 1", called[state.VerbNameFold].key)
	}

	_, _ = m.helpLineClick(layout.Target{Verb: state.VerbNameQuit})
	if called[state.VerbNameQuit].help != 1 {
		t.Errorf("helpLineClick quit: got %d calls, want 1", called[state.VerbNameQuit].help)
	}

	called[state.VerbNameQuit].help = 0
	verbByName[state.VerbNameQuit] = func(m *Model) (tea.Model, tea.Cmd) {
		called[state.VerbNameQuit].key++
		return m, tea.Quit
	}
	_, _ = m.paneKey(tea.KeyPressMsg{Code: 'q'})
	if called[state.VerbNameQuit].key != 1 {
		t.Errorf("key q/quit: got %d calls, want 1", called[state.VerbNameQuit].key)
	}
}

// The test that fed runVerb an unknown name is gone: state.VerbName carries an
// unexported field, so no name outside state's own list can be written. What
// used to be a run-time notice is now a value that cannot be constructed.

func TestRunVerbEmptyNameDoesNothing(t *testing.T) {
	t.Parallel()
	m := &Model{state: state.State{}}
	_, _ = m.runVerb(state.VerbName{})
	if m.state.Notice != "" {
		t.Errorf("empty verb should not set Notice, got %q", m.state.Notice)
	}
	if m.state.NoticeFailed {
		t.Fatal("empty verb should not set NoticeFailed")
	}
}
