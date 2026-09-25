package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

func (m *Model) verbEntries(verb state.Verb) []git.Entry {
	if len(m.state.Changes.Selected) > 0 {
		var out []git.Entry
		seen := map[string]bool{}
		for _, row := range m.state.Rows {
			if !state.SelectedIn(m.state, row.Section(), row.Path()) {
				continue
			}
			if seen[row.Path()] {
				continue
			}
			if state.VerbApplies(verb, row) {
				seen[row.Path()] = true
				out = append(out, row.Entry())
			}
		}
		return out
	}
	row, ok := state.CursorRow(m.state)
	if !ok {
		return nil
	}
	if !state.VerbApplies(verb, row) {
		return nil
	}
	return []git.Entry{row.Entry()}
}

// requestVerb asks whether the open diff still matches the file before staging
// what it shows. The answer costs a git call, so it is read off the update loop
// and the staging waits for it; runVerbNow is what happens once it is back.
func (m *Model) requestVerb(verb state.Verb) (tea.Model, tea.Cmd) {
	if verb == state.VerbStage && len(m.state.Changes.Selected) == 0 {
		if cmd := m.checkStaleFirst(noBlockYet); cmd != nil {
			return m, cmd
		}
	}
	return m.runVerbNow(verb)
}

func (m *Model) runVerbNow(verb state.Verb) (tea.Model, tea.Cmd) {
	if verb == state.VerbStage && len(m.state.Changes.Selected) == 0 {
		if m.state.Changes.Stale[m.cursorPath()] {
			return m, nil
		}
		if m.blockStageReady() {
			path := m.state.Open.Path
			block := m.state.Open.BlockCursor
			m.state = state.Apply(m.state, state.VerbRequested{
				Verb: state.VerbStage, Targets: []string{path}, Block: block,
			})
			return m, runStageBlock(m.rootContext(), m.dir, m.state.Open.Diff, block)
		}
	}
	entries := m.verbEntries(verb)
	if len(entries) == 0 {
		return m, nil
	}
	paths := make([]string, len(entries))
	for i, e := range entries {
		paths[i] = e.Path
	}
	m.state = state.Apply(m.state, state.VerbRequested{Verb: verb, Targets: paths, Block: -1})
	return m, verbCmd(m.rootContext(), m.dir, verb, entries)
}

func (m *Model) requestDiscard() (tea.Model, tea.Cmd) {
	entries := m.verbEntries(state.VerbDiscard)
	if len(entries) == 0 {
		return m, nil
	}
	confirm := discardConfirmOf(m.state, entries)
	paths := make([]string, len(entries))
	for i, e := range entries {
		paths[i] = e.Path
	}
	confirm.Targets = paths
	return m, snapshotForDiscard(m.rootContext(), m.dir, entries, confirm)
}

func (m *Model) confirmDiscard() (tea.Model, tea.Cmd) {
	if m.state.Changes.DiscardConfirm == nil {
		return m, nil
	}
	confirm := *m.state.Changes.DiscardConfirm
	entries := m.entriesForPaths(confirm.Targets)
	if len(entries) == 0 {
		m.state = state.Apply(m.state, state.DiscardCancelled{
			Notice: "nothing to discard · the changes are gone",
		})
		return m, nil
	}
	m.state = state.Apply(m.state, state.DiscardConfirmed{})
	return m, runDiscard(m.rootContext(), m.dir, entries, confirm.Undo, confirm)
}

func (m *Model) cancelDiscard() (tea.Model, tea.Cmd) {
	m.state = state.Apply(m.state, state.DiscardCancelled{})
	return m, nil
}

func (m *Model) confirmUndiscard() (tea.Model, tea.Cmd) {
	if m.state.Changes.UndiscardConfirm == nil {
		return m, nil
	}
	undo := m.state.Changes.UndiscardConfirm.Undo
	m.state = state.Apply(m.state, state.UndiscardConfirmed{})
	return m, runUndiscard(m.rootContext(), m.dir, undo)
}

func (m *Model) cancelUndiscard() (tea.Model, tea.Cmd) {
	m.state = state.Apply(m.state, state.UndiscardCancelled{})
	return m, nil
}

// The count of files is the length of entries, so the entries the caller
// hands over are one per path: verbEntries is what makes them so. A second
// pass dropping repeats here guarded the line counts and not the file count,
// which is the one a reader is asked to confirm.
func discardConfirmOf(s state.State, entries []git.Entry) state.DiscardConfirm {
	confirm := state.DiscardConfirm{Files: len(entries)}
	for _, e := range entries {
		for _, row := range s.Rows {
			if row.Path() != e.Path {
				continue
			}
			count := row.Entry().WorktreeCount
			if row.Section() == state.SectionStaged {
				count = row.Entry().IndexCount
			}
			confirm.Added += count.Added
			confirm.Deleted += count.Deleted
		}
	}
	return confirm
}

func (m *Model) entriesForPaths(paths []string) []git.Entry {
	want := map[string]bool{}
	for _, p := range paths {
		want[p] = true
	}
	seen := map[string]bool{}
	var out []git.Entry
	for _, row := range m.state.Rows {
		if !want[row.Path()] || seen[row.Path()] {
			continue
		}
		seen[row.Path()] = true
		out = append(out, row.Entry())
	}
	return out
}

// A file whose change is only staged has an empty worktree diff, so asking the
// wrong side draws an empty pane for a file that plainly changed.
// sectionFor is which side the row under the cursor is on. A path staged and
// then edited again has a row on each, holding different diffs, so the row
// decides which one opens rather than the path.
func (m *Model) sectionFor(path string) state.Section {
	row, ok := state.CursorRow(m.state)
	if !ok {
		return state.SectionUnstaged
	}
	if row.Path() != path || row.Kind() == state.RowCommit {
		return state.SectionUnstaged
	}
	return row.Section()
}
