package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// A confirmation asks the reader about one named target. The cursor must not
// move under it: the footer says "any other key cancels", and a cursor that
// walks while the prompt is up leaves the reader answering about a row they can
// no longer see.
//
// The three prompts are driven from one table. The hover named two of the three
// fields and left the undo prompt out, which is the shape this table exists to
// stop: three prompts, three places asking, and nobody comparing them.
func TestNoConfirmationLetsThePointerMoveTheCursor(t *testing.T) {
	t.Parallel()
	for _, c := range everyConfirmation() {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := newParentModel(t)
			m.state = state.Apply(m.state, state.StatusLoaded{
				Rows: []git.Entry{
					{Path: "a.txt", Worktree: git.Modified},
					{Path: "b.txt", Worktree: git.Modified},
				},
				Head: git.Head{Branch: "main"},
			})
			m.View()

			otherRow := -1
			for _, r := range m.frame.Regions {
				if r.Target.Kind == 1 && r.Target.Row != m.state.Cursor && r.Target.Row >= 0 {
					otherRow = r.Row
					break
				}
			}
			if otherRow < 0 {
				t.Fatal("no second row to point at, so this proves nothing")
			}

			c.show(m)
			if !state.ConfirmationOpen(m.state) {
				t.Fatal("the prompt is not showing, so this proves nothing")
			}
			before := m.state.Cursor

			next, _ := m.Update(tea.MouseMotionMsg{X: 20, Y: otherRow})
			m = next.(*Model)

			if m.state.Cursor != before {
				t.Errorf("the pointer moved the cursor from %d to %d while the %s prompt was up",
					before, m.state.Cursor, c.name)
			}
		})
	}
}

// Every prompt also takes the row verbs off the screen, for the same reason: a
// verb offered on a row is one the next key runs, and the next key answers the
// prompt instead.
func TestNoConfirmationLeavesRowVerbsOnScreen(t *testing.T) {
	t.Parallel()
	for _, c := range everyConfirmation() {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := state.State{Width: 90, Height: 30}
			s.Changes.Folded = map[string]bool{}
			s.Changes.Selected = map[string]bool{}
			s.Changes.Stale = map[string]bool{}
			if !state.CursorRowVerbsAllowed(s) {
				t.Fatal("the verbs are already off, so this proves nothing")
			}
			if state.CursorRowVerbsAllowed(c.showIn(s)) {
				t.Errorf("the row verbs stay on screen under the %s prompt", c.name)
			}
		})
	}
}

// confirmation is one destructive prompt, in the two shapes the tests need.
type confirmation struct {
	name string
	show func(*Model)
	// showIn is the same prompt applied to a bare state, for the checks that do
	// not need a model.
	showIn func(state.State) state.State
}

// everyConfirmation is the list both checks run. It is one list because it was
// two, and the second one was written out by hand: worktree remove was added to
// the pane and to neither table, so the hover moved the cursor under a prompt
// that holds it still everywhere else.
//
// TestTheListHoldsEveryConfirmationTheProgramCanShow keeps the list whole.
func everyConfirmation() []confirmation {
	shown := []state.Event{
		state.DiscardConfirmationShown{
			Confirm: state.DiscardConfirm{Targets: []string{"a.txt"}, Files: 1}},
		state.UndiscardConfirmationShown{
			Confirm: state.UndiscardConfirm{Files: 1}},
		state.StashDropConfirmationShown{
			Confirm: state.StashDropConfirm{SHA: "s0", Message: "one"}},
		state.WorktreeRemoveConfirmationShown{
			Confirm: state.WorktreeRemoveConfirm{Path: "/wt", Name: "wt"}},
	}
	out := make([]confirmation, 0, len(shown))
	for _, event := range shown {
		out = append(out, confirmation{
			name:   confirmationName(event),
			show:   func(m *Model) { m.state = state.Apply(m.state, event) },
			showIn: func(s state.State) state.State { return state.Apply(s, event) },
		})
	}
	return out
}

func confirmationName(event state.Event) string {
	s := state.State{}
	s.Changes.Folded = map[string]bool{}
	s.Changes.Selected = map[string]bool{}
	s.Changes.Stale = map[string]bool{}
	c, ok := state.PendingConfirmation(state.Apply(s, event))
	if !ok {
		return "an event that opens no prompt"
	}
	return c.Verb.String()
}

// A prompt the pane can show and the list does not hold is a prompt neither
// check sees. The count is the one thing that cannot be kept in step by hand.
func TestTheListHoldsEveryConfirmationTheProgramCanShow(t *testing.T) {
	t.Parallel()
	if got, want := len(everyConfirmation()), state.ConfirmationCount; got != want {
		t.Errorf("the list holds %d prompts and the program can show %d", got, want)
	}
	for _, c := range everyConfirmation() {
		if c.name == "an event that opens no prompt" {
			t.Errorf("an entry in the list opens no prompt")
		}
	}
}
