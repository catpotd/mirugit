package layout

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// A conflicted row draws "conflicts" and, after it, whatever the right half
// holds: the verbs while the cursor is on the row and it has any, the added and
// removed counts otherwise. A cursor row with no verbs is the second case, and
// a bound that lets it into the first paints an empty verb field over the
// counts — the row then says a file conflicts and nothing about its size.
func TestAConflictedCursorRowWithNoVerbsStillShowsItsCounts(t *testing.T) {
	t.Parallel()
	w := Renderer{Palette: Palette{Enabled: true}}
	const width = 55

	for _, c := range []struct {
		name   string
		cursor bool
	}{
		{"with the cursor on it", true},
		{"with the cursor elsewhere", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r := Row{Entry: git.Entry{Path: "a.txt", Worktree: git.Modified}, Index: 1}
			r.State.Cursor = c.cursor
			r.State.Conflicts = true

			line, _ := w.FileRow(r, width)
			plain := ansi.Strip(line)
			if !strings.Contains(plain, "conflicts") {
				t.Fatalf("the row does not say it conflicts, so this proves nothing: %q", plain)
			}
			if !strings.Contains(plain, "+0") {
				t.Errorf("the row says it conflicts and nothing about its size: %q", plain)
			}
		})
	}
}

func TestAConflictedCursorRowSeparatesTheConflictLabelFromItsVerb(t *testing.T) {
	t.Parallel()
	r := Row{
		Entry: git.Entry{Path: "a.txt", Worktree: git.Modified},
		State: rowState{Cursor: true, Conflicts: true, NoSelect: true},
		Verbs: []state.VerbName{state.VerbNameDiff},
	}
	line, _ := (Renderer{}).FileRow(r, paneWidth)
	if strings.Contains(line, "conflictsd diff") {
		t.Fatalf("the conflict label and verb run together: %q", line)
	}
	if !strings.Contains(line, "conflicts d diff") {
		t.Errorf("the conflict label and verb have no visible gap: %q", line)
	}
}
