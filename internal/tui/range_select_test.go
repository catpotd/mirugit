package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
)

// shift with a movement key extends the selection, which README says in those
// words: it adds the rows the cursor passes and takes nothing back. Moving up
// after moving down walks the cursor back over rows that stay selected.
//
// A reader who wants one of them out presses space on it. Making shift+k
// deselect would mean the same key adds in one direction and removes in the
// other, which is two answers to one question.
func TestShiftMovesExtendTheSelectionAndTakeNothingBack(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	m.state = state.Apply(m.state, state.StatusLoaded{
		Head: git.Head{Branch: "main"},
		Rows: []git.Entry{
			{Path: "f1.txt", Worktree: git.Modified},
			{Path: "f2.txt", Worktree: git.Modified},
			{Path: "f3.txt", Worktree: git.Modified},
			{Path: "f4.txt", Worktree: git.Modified},
		},
	})
	m.state.Cursor = state.FirstFileRow(m.state.Rows)

	down := tea.KeyPressMsg{Code: 'j', Mod: tea.ModShift}
	up := tea.KeyPressMsg{Code: 'k', Mod: tea.ModShift}

	for range 2 {
		next, _ := m.Update(down)
		m = next.(*Model)
	}
	if got := len(m.state.Changes.Selected); got != 3 {
		t.Fatalf("two shift+j from the first row selected %d rows, want 3", got)
	}

	next, _ := m.Update(up)
	m = next.(*Model)

	if got := len(m.state.Changes.Selected); got != 3 {
		t.Errorf("shift+k took a row back out of the selection: %d rows left, want 3",
			got)
	}
	// The cursor moved even though the selection did not; the reader has to be
	// able to walk back over what they selected.
	if row, ok := state.CursorRow(m.state); !ok || row.Path() != "f2.txt" {
		t.Errorf("the cursor is on %v, and two down then one up is f2.txt", row.Path())
	}
}

func TestShiftDownDoesNotSelectFilesHiddenByAFoldedDirectory(t *testing.T) {
	t.Parallel()
	m, foldedDirectory := foldedDirectoryModel(t)
	m.state.Cursor = foldedDirectory

	down := tea.KeyPressMsg{Code: 'j', Mod: tea.ModShift}
	next, _ := m.Update(down)
	m = next.(*Model)

	if state.SelectedIn(m.state, state.SectionUnstaged, "app/a.go") {
		t.Fatal("shift+j selected a file hidden under the folded directory")
	}

	next, _ = m.Update(down)
	m = next.(*Model)
	if !state.SelectedIn(m.state, state.SectionUnstaged, "other/b.go") {
		t.Error("shift+j did not select the next visible file")
	}
}

func TestShiftUpDoesNotSelectFilesHiddenByAFoldedDirectory(t *testing.T) {
	t.Parallel()
	m, foldedDirectory := foldedDirectoryModel(t)
	for i, row := range m.state.Rows {
		if row.Kind() == state.RowFile && row.Path() == "other/b.go" {
			m.state.Cursor = i
			break
		}
	}

	up := tea.KeyPressMsg{Code: 'k', Mod: tea.ModShift}
	next, _ := m.Update(up)
	m = next.(*Model)
	next, _ = m.Update(up)
	m = next.(*Model)

	if m.state.Cursor != foldedDirectory {
		t.Errorf("cursor = %d, want folded directory row %d", m.state.Cursor, foldedDirectory)
	}
	if state.SelectedIn(m.state, state.SectionUnstaged, "app/a.go") {
		t.Fatal("shift+k selected a file hidden under the folded directory")
	}
}

func foldedDirectoryModel(t *testing.T) (*Model, int) {
	t.Helper()
	m := fixed(t)
	m.state = state.Apply(m.state, state.StatusLoaded{
		Head: git.Head{Branch: "main"},
		Rows: []git.Entry{
			{Path: "app/a.go", Worktree: git.Modified},
			{Path: "other/b.go", Worktree: git.Modified},
			{Path: "z/after.go", Worktree: git.Modified},
		},
	})
	m.state = state.Apply(m.state, state.DirectoryFolded{
		Path: "app", Section: state.SectionUnstaged,
	})
	for i, row := range m.state.Rows {
		if row.Kind() == state.RowDirectory && row.DirPath() == "app" {
			return m, i
		}
	}
	t.Fatal("the changes rows have no app directory")
	return nil, -1
}

// Shift on a checkbox click extends the selection to the row clicked, the same
// answer shift+j gives. Without it the click toggles that one row, so a reader
// who shift-clicks the fourth row after selecting the first gets one row
// selected instead of four, and the three in between stay out.
func TestShiftClickingACheckboxExtendsToThatRow(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		mod  tea.KeyMod
		want int
	}{
		{"with shift", tea.ModShift, 4},
		{"without shift", 0, 2},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := fixed(t)
			m.state = state.Apply(m.state, state.StatusLoaded{
				Head: git.Head{Branch: "main"},
				Rows: []git.Entry{
					{Path: "f1.txt", Worktree: git.Modified},
					{Path: "f2.txt", Worktree: git.Modified},
					{Path: "f3.txt", Worktree: git.Modified},
					{Path: "f4.txt", Worktree: git.Modified},
				},
			})
			var files []int
			for i, r := range m.state.Rows {
				if r.Kind() == state.RowFile {
					files = append(files, i)
				}
			}
			if len(files) < 4 {
				t.Fatalf("%d file rows, want at least 4", len(files))
			}
			first := files[0]
			m.state.Cursor = first
			m.state = state.Apply(m.state, state.SelectionToggled{
				Path: m.state.Rows[first].Path(), Section: m.state.Rows[first].Section()})
			if len(m.state.Changes.Selected) != 1 {
				t.Fatalf("the first row is not selected, so this proves nothing")
			}

			fourth := files[3]
			// A row's checkbox carries no section; the one that does is the
			// section heading's, and that toggles every row under it.
			next, _ := m.checkboxClick(layout.Target{
				Kind: layout.TargetCheckbox,
				Row:  fourth,
				Path: m.state.Rows[fourth].Path(),
			}, c.mod)
			m = next.(*Model)

			if got := len(m.state.Changes.Selected); got != c.want {
				t.Errorf("%d rows are selected, want %d", got, c.want)
			}
		})
	}
}
