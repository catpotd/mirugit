package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/state"
)

type verbFn func(*Model) (tea.Model, tea.Cmd)

var verbByName = map[state.VerbName]verbFn{
	state.VerbNameDown: func(m *Model) (tea.Model, tea.Cmd) {
		return m.moveCursor(tea.KeyPressMsg{}, 1)
	},
	state.VerbNameSelect: func(m *Model) (tea.Model, tea.Cmd) {
		return m.selectAtCursor()
	},
	state.VerbNameSelectAll: func(m *Model) (tea.Model, tea.Cmd) {
		m.state = state.Apply(m.state, state.TabSelectionToggled{})
		return m, nil
	},
	state.VerbNameExtend: func(m *Model) (tea.Model, tea.Cmd) {
		return m.moveCursor(tea.KeyPressMsg{Mod: tea.ModShift}, 1)
	},
	state.VerbNameNextTab: func(m *Model) (tea.Model, tea.Cmd) {
		next := m.nextShownTab()
		if next == m.state.Tab {
			return m, nil
		}
		return m.switchTab(next)
	},
	state.VerbNameFold: func(m *Model) (tea.Model, tea.Cmd) {
		return m.foldAtCursor()
	},
	// The footer prints unfold on a folded directory and that word is
	// clickable, so it needs a handler even though the key is the same one.
	state.VerbNameUnfold: func(m *Model) (tea.Model, tea.Cmd) {
		return m.foldAtCursor()
	},
	state.VerbNameQuit: func(m *Model) (tea.Model, tea.Cmd) {
		return m, tea.Quit
	},
	state.VerbNameCommit: func(m *Model) (tea.Model, tea.Cmd) {
		// A focused field takes every key it can type, so focusing one the pane
		// is not drawing leaves j, k and the tab numbers going into a box the
		// reader cannot see. The key and the help line both arrive here.
		if !state.Facts[m.state.Tab].HasCommitBox {
			return m, nil
		}
		m.state = state.Apply(m.state, state.MessageFocused{})
		return m, nil
	},
	state.VerbNameClear: func(m *Model) (tea.Model, tea.Cmd) {
		return m.clearSelectionOrCloseDiff()
	},
	state.VerbNameStage: func(m *Model) (tea.Model, tea.Cmd) {
		return m.requestVerb(state.VerbStage)
	},
	state.VerbNameUnstage: func(m *Model) (tea.Model, tea.Cmd) {
		return m.requestVerb(state.VerbUnstage)
	},
	state.VerbNameStash: func(m *Model) (tea.Model, tea.Cmd) {
		return m.requestVerb(state.VerbStash)
	},
	state.VerbNameDiscard: func(m *Model) (tea.Model, tea.Cmd) {
		return m.discardForTab()
	},
	state.VerbNameRestore: func(m *Model) (tea.Model, tea.Cmd) {
		return m.requestStashRestore()
	},
	state.VerbNameBranch: func(m *Model) (tea.Model, tea.Cmd) {
		return m.requestStashBranch()
	},
	state.VerbNameDrop: func(m *Model) (tea.Model, tea.Cmd) {
		return m.requestStashDrop()
	},
	state.VerbNameGo: func(m *Model) (tea.Model, tea.Cmd) {
		return m.requestWorktreeGo()
	},
	state.VerbNameRemove: func(m *Model) (tea.Model, tea.Cmd) {
		return m.requestWorktreeRemove()
	},
	state.VerbNameUndo: func(m *Model) (tea.Model, tea.Cmd) {
		return m.requestUndo()
	},
	state.VerbNameUncommit: func(m *Model) (tea.Model, tea.Cmd) {
		return m.requestHistoryUndo()
	},
	state.VerbNamePage: func(m *Model) (tea.Model, tea.Cmd) {
		return m.moveCursor(tea.KeyPressMsg{}, m.listRowsNow())
	},
	state.VerbNameEnds: func(m *Model) (tea.Model, tea.Cmd) {
		return m.moveCursor(tea.KeyPressMsg{}, len(m.state.Rows))
	},
	state.VerbNameDiff: func(m *Model) (tea.Model, tea.Cmd) {
		return m.diffForTab()
	},
	state.VerbNameNextBlock: func(m *Model) (tea.Model, tea.Cmd) {
		return m.moveBlockCursor(1)
	},
	state.VerbNameReload: func(m *Model) (tea.Model, tea.Cmd) {
		return m.requestReload()
	},
	state.VerbNameSHA: func(m *Model) (tea.Model, tea.Cmd) {
		return m.requestCopySHA()
	},
	state.VerbNameOpen: func(m *Model) (tea.Model, tea.Cmd) {
		return m.requestOpenCommit()
	},
	state.VerbNameRead: func(m *Model) (tea.Model, tea.Cmd) {
		return m.requestRead()
	},
	state.VerbNameFetch: func(m *Model) (tea.Model, tea.Cmd) {
		return m.requestFetch()
	},
	state.VerbNameSync: func(m *Model) (tea.Model, tea.Cmd) {
		return m.requestSync()
	},
}

func (m *Model) runVerb(name state.VerbName) (tea.Model, tea.Cmd) {
	if name.IsZero() {
		return m, nil
	}
	// Every VerbName has a handler and no other value can be built outside
	// state, so this arm is unreachable today. It stays because the map lookup
	// has to answer something, and a nil call would take the pane down.
	fn, ok := verbByName[name]
	if !ok {
		m.state = state.Apply(m.state, state.Failed{
			Line: fmt.Sprintf("unknown verb %q", name),
		})
		return m, nil
	}
	return fn(m)
}
