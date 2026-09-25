package tui

import (
	"context"
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// Every verb that changes the repository says what it did, and restore said
// nothing: the notice from the stash the reader had just taken stayed on the
// row above the list, so the screen read "stashed 1 file" about the moment they
// put it back.
//
// A notice nobody replaced is worse than no notice, because it names the
// opposite of what happened.
func TestRestoringAStashSaysSo(t *testing.T) {
	t.Parallel()
	m, err := New(context.Background(), testRepo(t), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 53})
	m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabStashed})
	m.state = state.Apply(m.state, state.StashedLoaded{Stashes: []git.StashRow{{
		Ref: "stash@{0}", SHA: "abc", Branch: "main", FileCount: 2,
	}}})
	m.state.Cursor = 0
	m.state = state.Apply(m.state, state.VerbFinished{Verb: state.VerbStash, Applied: 1})
	if m.state.Notice == "" {
		t.Fatal("this test needs a stale notice to be worth running")
	}

	next, _ := m.Update(stashVerbMsg{restored: 2})
	m = next.(*Model)
	if want := "restored 2 files from the stash"; m.state.Notice != want {
		t.Errorf("got %q, want %q", m.state.Notice, want)
	}
}

// One file is one file. The count reads off the stash row, which is what the
// list was already drawing.
func TestRestoringOneFileSaysFile(t *testing.T) {
	t.Parallel()
	m, err := New(context.Background(), testRepo(t), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 53})
	next, _ := m.Update(stashVerbMsg{restored: 1})
	m = next.(*Model)
	if want := "restored 1 file from the stash"; m.state.Notice != want {
		t.Errorf("got %q, want %q", m.state.Notice, want)
	}
}
