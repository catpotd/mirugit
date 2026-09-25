package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
)

// While the commit box has the keyboard, every key goes into the message: U and
// R are ordinary capitals there. The keys already know this. The mouse did not,
// so a click on a row's discard opened a confirmation that the footer never
// drew — the footer names the commit box first — and the y that would have
// confirmed it went into the message instead. The confirmation stayed, and the
// first y after leaving the box discarded.
func TestAClickWhileTypingEndsTheTypingRatherThanRunningAVerb(t *testing.T) {
	dir := repoWithUnstagedChange(t)
	m, err := New(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	m.probe.settled = true
	m.state = state.Apply(m.state, state.Resized{Width: 90, Height: 30})
	next, cmd := m.Update(loadRepo(context.Background(), dir, mustGitDir(t, dir))())
	m = next.(*Model)
	if cmd != nil {
		_ = cmd()
	}
	cursorToFileRow(t, m)
	m.state = state.Apply(m.state, state.MessageFocused{})
	m.state = state.Apply(m.state, state.MessageEdited{Text: "wip"})

	after, _ := m.click(layout.Target{Kind: layout.TargetVerb, Verb: state.VerbNameDiscard}, 0)
	m = after.(*Model)

	if m.state.Changes.MessageFocused {
		t.Error("a click on a row left the commit box holding the keyboard")
	}
	if m.state.Changes.DiscardConfirm != nil {
		t.Error("the click ran the verb as well as leaving the box")
	}
	if m.state.Changes.Message != "wip" {
		t.Errorf("leaving the box lost what was typed: %q", m.state.Changes.Message)
	}
}

// A click on the box itself is how the reader asks for it, so it keeps the
// keyboard.
func TestAClickOnTheCommitBoxKeepsTheKeyboard(t *testing.T) {
	t.Parallel()
	m := newParentModel(t)
	m.state = state.Apply(m.state, state.MessageFocused{})

	after, _ := m.click(layout.Target{Kind: layout.TargetCommitBox}, 0)
	if !after.(*Model).state.Changes.MessageFocused {
		t.Error("clicking the commit box gave the keyboard away")
	}
}

// The footer names the commit box before it names a confirmation, so a
// confirmation opened while typing is one the reader cannot see or answer.
func TestTheFooterCannotShowAConfirmationAndTheCommitBoxAtOnce(t *testing.T) {
	t.Parallel()
	s := state.State{Width: 90, Height: 30}
	s.Changes.Folded = map[string]bool{}
	s.Changes.Selected = map[string]bool{}
	s.Changes.Stale = map[string]bool{}
	s = state.Apply(s, state.MessageFocused{})
	s = state.Apply(s, state.DiscardConfirmationShown{
		Confirm: state.DiscardConfirm{Targets: []string{"a.txt"}, Files: 1}})

	footer := layout.Renderer{}.Footer(s, 90)
	if strings.Contains(footer, "y discard") {
		t.Skip("the footer now draws the confirmation over the commit box; " +
			"the click guard is no longer the only thing keeping them apart")
	}
	if !strings.Contains(footer, "esc leave") {
		t.Fatalf("the footer names neither: %q", footer)
	}
}

func cursorToFileRow(t *testing.T, m *Model) {
	t.Helper()
	for i, r := range m.state.Rows {
		if r.Kind() == state.RowFile {
			m.state = state.Apply(m.state, state.CursorMoved{By: i - m.state.Cursor})
			return
		}
	}
	t.Fatal("no file row to put the cursor on")
}

var _ = tea.KeyPressMsg{}

func mustGitDir(t *testing.T, dir string) string {
	t.Helper()
	gitDir, err := git.GitDir(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	return gitDir
}

// Moving to another worktree has to take its state directory with it: each
// linked worktree keeps its own, and reading the old one reports the old
// worktree's unfinished merge over the new one's files.
func TestMovingToAnotherWorktreeTakesItsStateDirectory(t *testing.T) {
	dir := repoWithUnstagedChange(t)
	m, err := New(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	before := m.gitDir

	other := repoWithUnstagedChange(t)
	m = applyOnce(t, m, m.rebindTo(other))

	if m.gitDir == before {
		t.Error("the pane kept the first worktree's state directory")
	}
	if m.gitDir != mustGitDir(t, other) {
		t.Errorf("gitDir = %q, want %q", m.gitDir, mustGitDir(t, other))
	}
}
