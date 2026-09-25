package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
)

// The four destructive verbs ask before they run, and the rule is the same on
// each: y confirms, anything else cancels, the footer names the verb. worktree
// remove had no question at all, so the same x key asked on two tabs out of
// three and a reader could not learn which.
//
// The four run from one table because that is the only shape in which adding a
// fifth is a compile error rather than a tab nobody checked.
func TestEveryDestructiveVerbAsksTheSameQuestion(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		open func(*Model)
		verb state.VerbName
		// pending reads the confirmation this verb stores.
		pending func(state.State) bool
	}{
		{
			name: "discard on the changes tab",
			open: func(m *Model) {
				m.state = state.Apply(m.state, state.DiscardConfirmationShown{
					Confirm: state.DiscardConfirm{Targets: []string{"a.txt"}, Files: 1}})
			},
			verb:    state.VerbNameDiscard,
			pending: func(s state.State) bool { return s.Changes.DiscardConfirm != nil },
		},
		{
			name: "restore over edits",
			open: func(m *Model) {
				m.state = state.Apply(m.state, state.UndiscardConfirmationShown{
					Confirm: state.UndiscardConfirm{Files: 1}})
			},
			verb:    state.VerbNameRestore,
			pending: func(s state.State) bool { return s.Changes.UndiscardConfirm != nil },
		},
		{
			name: "drop on the stashed tab",
			open: func(m *Model) {
				m.state = state.Apply(m.state, state.StashDropConfirmationShown{
					Confirm: state.StashDropConfirm{SHA: "s0", Message: "hold"}})
			},
			verb:    state.VerbNameDrop,
			pending: func(s state.State) bool { return s.Stashed.DropConfirm != nil },
		},
		{
			name: "remove on the worktrees tab",
			open: func(m *Model) {
				m.state = state.Apply(m.state, state.WorktreeRemoveConfirmationShown{
					Confirm: state.WorktreeRemoveConfirm{Path: "/wt", Name: "wt"}})
			},
			verb:    state.VerbNameRemove,
			pending: func(s state.State) bool { return s.Worktrees.RemoveConfirm != nil },
		},
	}

	// The table above is this package's own list of the prompts, and a prompt
	// added to state and to nothing else would leave it a case short. Counting
	// it is what makes the omission a failure here rather than a tab whose
	// question nobody asks.
	if len(cases) != state.ConfirmationCount {
		t.Fatalf("this test checks %d destructive prompts and state has %d",
			len(cases), state.ConfirmationCount)
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			t.Run("the footer names the verb and the way out", func(t *testing.T) {
				t.Parallel()
				m := confirmModel(t)
				c.open(m)
				footer := layout.Renderer{}.Footer(m.state, 90)
				for _, want := range []string{"y " + c.verb.String(), "cancels"} {
					if !strings.Contains(footer, want) {
						t.Errorf("the footer does not say %q: %q", want, footer)
					}
				}
			})

			t.Run("a key that is not y cancels", func(t *testing.T) {
				t.Parallel()
				for _, key := range []rune{'j', 'x', 'q', 'U', 'R'} {
					m := confirmModel(t)
					c.open(m)
					next, _ := m.Update(tea.KeyPressMsg{Code: key})
					if c.pending(next.(*Model).state) {
						t.Errorf("%q left the question open", string(key))
					}
				}
			})

			t.Run("a click cancels", func(t *testing.T) {
				t.Parallel()
				m := confirmModel(t)
				c.open(m)
				next, _ := m.click(layout.Target{Kind: layout.TargetFile, Path: "a.txt"}, 0)
				if c.pending(next.(*Model).state) {
					t.Error("a click left the question open")
				}
			})

			t.Run("canceling says what was kept", func(t *testing.T) {
				t.Parallel()
				m := confirmModel(t)
				c.open(m)
				next, _ := m.Update(tea.KeyPressMsg{Code: 'j'})
				if notice := next.(*Model).state.Notice; !strings.HasPrefix(notice, "canceled") {
					t.Errorf("the notice after canceling is %q", notice)
				}
			})
		})
	}
}

func confirmModel(t *testing.T) *Model {
	t.Helper()
	m := newParentModel(t)
	m.dir = t.TempDir()
	m.state = state.Apply(m.state, state.StatusLoaded{
		Rows: []git.Entry{{Path: "a.txt", Worktree: git.Modified}},
		Head: git.Head{Branch: "main"}})
	return m
}

// The x key runs a different verb per tab, and on every tab it has to ask
// before it runs. Opening the confirmation by hand checks what happens after
// the question; this checks that the key asks it at all.
func TestTheXKeyAsksOnEveryTabItRunsOn(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name    string
		open    func(*Model)
		pending func(state.State) bool
	}{
		{
			name: "worktrees",
			open: func(m *Model) {
				m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabWorktrees})
				m.state = state.Apply(m.state, state.WorktreesLoaded{Base: "main", Here: "/repo",
					Worktrees: []git.WorktreeRow{
						{Path: "/repo", Name: "repo", Main: true, SHA: "aaa"},
						{Path: "/wt", Name: "wt", SHA: "bbb"}}})
				m.state = state.Apply(m.state, state.CursorMoved{By: 1})
			},
			pending: func(s state.State) bool { return s.Worktrees.RemoveConfirm != nil },
		},
		{
			name: "stashed",
			open: func(m *Model) {
				m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabStashed})
				m.state = state.Apply(m.state, state.StashedLoaded{Stashes: []git.StashRow{
					{Ref: "stash@{0}", SHA: "s0", Message: "hold", Status: git.StashApplies}}})
			},
			pending: func(s state.State) bool { return s.Stashed.DropConfirm != nil },
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := confirmModel(t)
			c.open(m)
			// The handler is called directly: Update batches a reload with
			// whatever the key answered, and a batch says nothing about
			// whether the verb ran.
			_, cmd := m.rowVerbKey(tea.KeyPressMsg{Code: 'x'})
			if !c.pending(m.state) {
				t.Error("x ran without asking")
			}
			if cmd != nil {
				t.Errorf("x issued a command before the question was answered: %T", cmd())
			}
		})
	}
}
