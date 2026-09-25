package layout

import (
	"strings"
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// "select" を素で探すと "1 selected" の中に先に当たる。押せる "space select" は
// 無色のまま、語の途中でリセットが落ちる。
func TestSelectVerbIsColoredWhereItCanBePressed(t *testing.T) {
	t.Parallel()
	entry := git.Entry{Path: "a.txt", Worktree: git.Modified}
	s := state.State{
		Tab: state.TabChanges, Width: 77, Height: 24,
		Changes: state.Changes{Entries: []git.Entry{entry}, Selected: map[string]bool{"unstaged:a.txt": true}},
		Rows:    []state.Row{state.FileRow(entry, state.SectionUnstaged)},
	}
	w := Renderer{Palette: Palette{Enabled: true}}
	colored := w.Footer(s, 77)
	plain := w.footerPlain(s, 77)
	if !strings.Contains(plain, "selected") || !strings.Contains(plain, "space select") {
		t.Fatalf("フッターの形が想定と違う: %q", plain)
	}
	verb := w.Verb("select")
	if !strings.Contains(colored, "space "+verb) {
		t.Errorf("押せる 'space select' に色が付いていない: %q", colored)
	}
	if strings.Contains(colored, verb+"ed") {
		t.Errorf("'selected' の中の select に色が付いた: %q", colored)
	}
}
