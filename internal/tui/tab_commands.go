package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/state"
)

// tabCommands is what this package does differently per tab. Every field is a
// command the tab issues, so none of them can live in state.TabFacts: tea.Cmd
// belongs to this layer. Adding a tab means adding one arm here rather than
// finding the four switches that used to choose between them.
type tabCommands struct {
	// Reload rereads what this tab draws beyond the worktree list, which every
	// reload reads for the bar's fourth count. The worktrees tab therefore has
	// nothing of its own to name: naming it read the list twice, and its rows
	// are the ones that cost a git each.
	Reload func(m *Model) tea.Cmd
	// NestedDiff reads the diff of a file listed under a stash or a worktree
	// row. It answers "" for a tab whose files sit at the top level.
	NestedDiff func(m *Model, path string) (dir string, cmd tea.Cmd)
	// ExpandRow reads what the row the reader opened holds. It answers nil for
	// a tab whose rows do not expand, and for a key this tab does not list.
	//
	// The key is a parameter rather than read off the cursor inside, because
	// the row that opens is the one the reader asked for. Two copies of this
	// took the cursor instead, and arriving at a neighbor closed what was open.
	ExpandRow func(m *Model, key string) tea.Cmd
	// KeyAtCursor names the row under the cursor, in the key ExpandRow takes.
	KeyAtCursor func(m *Model) string
	// MarkCursorRead records that the reader has looked at the row under the
	// cursor. This one does follow the cursor: looking is what reading is.
	MarkCursorRead func(m *Model) tea.Cmd
}

func commandsFor(tab state.Tab) tabCommands {
	switch tab {
	case state.TabHistory:
		return tabCommands{
			Reload:         func(m *Model) tea.Cmd { return loadHistory(m.readsForTab(), m.dir) },
			NestedDiff:     noNestedDiff,
			ExpandRow:      (*Model).expandCommit,
			KeyAtCursor:    (*Model).commitSHAAtCursor,
			MarkCursorRead: noMarkRead,
		}
	case state.TabStashed:
		return tabCommands{
			Reload:         func(m *Model) tea.Cmd { return loadStashes(m.readsForTab(), m.dir) },
			NestedDiff:     (*Model).stashNestedDiff,
			ExpandRow:      (*Model).expandStash,
			KeyAtCursor:    (*Model).stashRefAtCursor,
			MarkCursorRead: (*Model).markStashCursorRead,
		}
	case state.TabWorktrees:
		return tabCommands{
			Reload:         noReload,
			NestedDiff:     (*Model).worktreeNestedDiff,
			ExpandRow:      (*Model).expandWorktree,
			KeyAtCursor:    (*Model).worktreePathAtCursor,
			MarkCursorRead: (*Model).markWorktreeCursorRead,
		}
	case state.TabChanges:
	}
	return tabCommands{
		Reload:         func(m *Model) tea.Cmd { return m.reloadRepo() },
		NestedDiff:     noNestedDiff,
		ExpandRow:      noExpandRow,
		KeyAtCursor:    noKeyAtCursor,
		MarkCursorRead: noMarkRead,
	}
}

// noNestedDiff is NestedDiff for a tab whose files are in this repository.
func noNestedDiff(*Model, string) (string, tea.Cmd) { return "", nil }

// noExpandRow is ExpandRow for a tab whose rows do not expand.
func noExpandRow(*Model, string) tea.Cmd { return nil }

// noKeyAtCursor is KeyAtCursor for the same tabs.
func noKeyAtCursor(*Model) string { return "" }

// noMarkRead is MarkCursorRead for a tab that keeps no read marks.
func noMarkRead(*Model) tea.Cmd { return nil }

// noReload is Reload for the worktrees tab: reloadCurrentTab already reads the
// list this tab draws.
func noReload(*Model) tea.Cmd { return nil }
