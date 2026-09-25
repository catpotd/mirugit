package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// A read is issued for one row and can land after the list has moved past it.
// The rule is the same on both tabs whose rows are read one at a time: report a
// failure the list still holds, and say nothing about one it does not.
//
// The two ran the rule from separate code and drifted: stash got it and
// worktrees did not, and a reader with the worktrees tab open was shown git's
// complaint about a tree another terminal had removed. They are checked
// together so a third tab of this shape cannot be added to one and not the
// other.
func TestAStaleReadIsSilentOnEveryTabThatReadsRowsOneAtATime(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		// load puts two rows on the tab.
		load func(*Model)
		// stale is a read for a row the list does not hold.
		stale func() any
		// live is a read for a row it does.
		live func() any
	}{
		{
			name: "stashed",
			load: func(m *Model) {
				m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabStashed})
				m.state = state.Apply(m.state, state.StashedLoaded{Stashes: []git.StashRow{
					{Ref: "stash@{0}", SHA: "s0", Branch: "main", FileCount: -1},
					{Ref: "stash@{1}", SHA: "s1", Branch: "main", FileCount: -1},
				}})
			},
			stale: func() any {
				return stashStatusMsg{index: 1, sha: "gone", err: errors.New("the list moved")}
			},
			live: func() any {
				return stashStatusMsg{index: 1, sha: "s1", err: errors.New("permission denied")}
			},
		},
		{
			name: "worktrees",
			load: func(m *Model) {
				m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabWorktrees})
				m.state = state.Apply(m.state, state.WorktreesLoaded{Base: "main", Here: "/repo",
					Worktrees: []git.WorktreeRow{
						{Path: "/repo", Name: "repo", Main: true, SHA: "aaa"},
						{Path: "/wt", Name: "wt", SHA: "bbb"},
					}})
			},
			stale: func() any {
				return worktreeStatusMsg{index: 1, path: "/gone", err: errors.New("the list moved")}
			},
			live: func() any {
				return worktreeStatusMsg{index: 1, path: "/wt", err: errors.New("permission denied")}
			},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			t.Run("a read the list moved past says nothing", func(t *testing.T) {
				t.Parallel()
				m := fixed(t)
				c.load(m)
				next, _ := m.Update(c.stale())
				if notice := next.(*Model).state.Notice; notice != "" {
					t.Errorf("got %q", notice)
				}
			})

			t.Run("a read the list still holds is reported", func(t *testing.T) {
				t.Parallel()
				m := fixed(t)
				c.load(m)
				next, _ := m.Update(c.live())
				if notice := next.(*Model).state.Notice; notice != "permission denied" {
					t.Errorf("got %q", notice)
				}
			})
		})
	}
}

// A list names its rows; what each one holds is read separately, one git per
// row, and does not change because the list was read again. Replacing every row
// with an unread one blanked the counts and the verbs on every five-second
// poll, and they came back a few seconds later when the reads landed.
//
// The two tabs kept this in separate code and drifted: stash was fixed and
// worktrees was not, and the blanking stayed on the tab nobody had looked at.
func TestAReloadKeepsWhatWasReadOnEveryTabThatReadsRowsOneAtATime(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		// read loads two rows whose contents are known.
		read func(*Model)
		// again loads the same two rows as a list alone would: named, unread.
		again func(*Model)
		// held answers what the pane would draw for the row at index.
		held func(state.State, int) string
	}{
		{
			name: "stashed",
			read: func(m *Model) {
				m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabStashed})
				m.state = state.Apply(m.state, state.StashedLoaded{Stashes: []git.StashRow{
					{Ref: "stash@{0}", SHA: "s0", Branch: "main", FileCount: 2, Status: git.StashApplies},
					{Ref: "stash@{1}", SHA: "s1", Branch: "main", FileCount: 5, Status: git.StashConflicts},
				}})
			},
			again: func(m *Model) {
				m.state = state.Apply(m.state, state.StashedLoaded{Stashes: []git.StashRow{
					{Ref: "stash@{0}", SHA: "s0", Branch: "main", FileCount: -1},
					{Ref: "stash@{1}", SHA: "s1", Branch: "main", FileCount: -1},
				}})
			},
			held: func(s state.State, i int) string {
				row := s.Stashed.Stashes[i]
				return fmt.Sprintf("%d files, status %v", row.FileCount, row.Status)
			},
		},
		{
			name: "worktrees",
			read: func(m *Model) {
				m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabWorktrees})
				m.state = state.Apply(m.state, state.WorktreesLoaded{Base: "main", Here: "/repo",
					Worktrees: []git.WorktreeRow{
						{Path: "/repo", Name: "repo", Main: true, SHA: "aaa", Dirty: 3, WroteAge: "1h"},
						{Path: "/wt", Name: "wt", SHA: "bbb", Dirty: 0},
					}})
			},
			again: func(m *Model) {
				m.state = state.Apply(m.state, state.WorktreesLoaded{Base: "main", Here: "/repo",
					Worktrees: []git.WorktreeRow{
						{Path: "/repo", Name: "repo", Main: true, SHA: "aaa", Dirty: -1},
						{Path: "/wt", Name: "wt", SHA: "bbb", Dirty: -1},
					}})
			},
			held: func(s state.State, i int) string {
				row := s.Worktrees.List[i]
				return fmt.Sprintf("%d files, wrote %q", row.Dirty, row.WroteAge)
			},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := fixed(t)
			c.read(m)
			want := []string{c.held(m.state, 0), c.held(m.state, 1)}

			c.again(m)

			for i, w := range want {
				if got := c.held(m.state, i); got != w {
					t.Errorf("row %d came back as %q after a reload, and held %q", i, got, w)
				}
			}
		})
	}
}

// A read lands on the row it was issued for, not on the place that row held.
// Anything that removes a row shifts the ones below it, and a read still in
// flight would otherwise write one row's contents onto another's.
func TestAReadLandsOnItsOwnRowOnEveryTabThatReadsRowsOneAtATime(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		// load puts one row on the tab, unread.
		load func(*Model)
		// overtaken is a read for a row this list does not hold, aimed at index 0.
		overtaken func() any
		// held answers what the pane would draw for the row at index 0.
		held func(state.State) string
	}{
		{
			name: "stashed",
			load: func(m *Model) {
				m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabStashed})
				m.state = state.Apply(m.state, state.StashedLoaded{Stashes: []git.StashRow{
					{Ref: "stash@{0}", SHA: "s1", Branch: "main", FileCount: -1},
				}})
			},
			overtaken: func() any {
				return stashStatusMsg{index: 0, sha: "s0",
					row: git.StashRow{Ref: "stash@{0}", SHA: "s0", FileCount: 9,
						Status: git.StashApplies}}
			},
			held: func(s state.State) string {
				row := s.Stashed.Stashes[0]
				return fmt.Sprintf("%s holding %d", row.SHA, row.FileCount)
			},
		},
		{
			name: "worktrees",
			load: func(m *Model) {
				m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabWorktrees})
				m.state = state.Apply(m.state, state.WorktreesLoaded{Base: "main", Here: "/repo",
					Worktrees: []git.WorktreeRow{
						{Path: "/repo", Name: "repo", Main: true, SHA: "aaa"},
						{Path: "/other", Name: "other", SHA: "ccc", Dirty: -1},
					}})
			},
			overtaken: func() any {
				return worktreeStatusMsg{index: 1, path: "/gone",
					row: git.WorktreeRow{Path: "/gone", Name: "gone", SHA: "bbb", Dirty: 7}}
			},
			held: func(s state.State) string {
				row := s.Worktrees.List[1]
				return fmt.Sprintf("%s holding %d", row.Path, row.Dirty)
			},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := fixed(t)
			c.load(m)
			before := c.held(m.state)

			next, _ := m.Update(c.overtaken())
			m = next.(*Model)

			if got := c.held(m.state); got != before {
				t.Errorf("a read for a row this list does not hold landed on %q, "+
					"which held %q", got, before)
			}
		})
	}
}

// drop is not the only verb that moves this list's refs: pop and branch take a
// stash off the list too, and every stash below the one they took is renamed.
// A read issued before the new list arrives then names a stash that has moved.
//
// The three ran from separate code and only drop was guarded, so the same
// "log for 'stash' only has N entries" waited behind p and b.
func TestEveryVerbThatMovesTheStashRefsWaitsForTheNewList(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		// answered is the message that says the verb ran.
		answered func() any
		// refused is the same verb reporting that it could not run.
		refused func() any
	}{
		{"drop",
			func() any { return stashDropMsg{} },
			func() any { return stashDropMsg{err: errors.New("git refused")} }},
		{"restore",
			func() any { return stashVerbMsg{restored: 1} },
			func() any { return stashVerbMsg{err: errors.New("git refused")} }},
		{"branch",
			func() any { return stashVerbMsg{branch: "main-stash"} },
			func() any { return stashVerbMsg{err: errors.New("git refused")} }},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := fixed(t)
			m.dir = t.TempDir()
			m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabStashed})
			m.state = state.Apply(m.state, state.StashedLoaded{Stashes: []git.StashRow{
				{Ref: "stash@{0}", SHA: "s0", Branch: "main", FileCount: 1},
				{Ref: "stash@{1}", SHA: "s1", Branch: "main", FileCount: 1},
			}})

			next, cmd := m.Update(c.answered())
			m = next.(*Model)

			read := 0
			walkCmds(cmd, func(msg tea.Msg) {
				if _, ok := msg.(stashFilesMsg); ok {
					read++
				}
			})
			if read > 0 {
				t.Errorf("%s read the list it had just moved", c.name)
			}

			// A read still in flight for a stash the verb moved says nothing.
			next, _ = m.Update(stashStatusMsg{index: 1, sha: "s1",
				err: errors.New("log for 'stash' only has 1 entries")})
			if notice := next.(*Model).state.Notice; strings.Contains(notice, "log for") {
				t.Errorf("%s reported a read it had invalidated: %q", c.name, notice)
			}

			// A verb that could not run says so and asks for the list again:
			// the shared failure path applies nothing and would leave the
			// window it opened shut for good.
			m2 := fixed(t)
			m2.dir = t.TempDir()
			m2.state = state.Apply(m2.state, state.TabChanged{Tab: state.TabStashed})
			next, cmd = m2.Update(c.refused())
			m2 = next.(*Model)
			if m2.state.Notice != "git refused" {
				t.Errorf("%s could not run and said %q", c.name, m2.state.Notice)
			}
			asked := false
			walkCmds(cmd, func(msg tea.Msg) {
				switch msg.(type) {
				case stashedMsg, repoMsg:
					asked = true
				}
			})
			if !asked {
				t.Errorf("%s could not run and did not ask for the list again", c.name)
			}
		})
	}
}

// The window opens when the verb is issued, not when it answers. git has moved
// the refs by the time the answer arrives, and a read issued in between — the
// five-second poll is enough — asks for a stash that has moved.
//
// drop opened it at the question; restore and branch waited for the answer, so
// the same "log for 'stash' only has N entries" waited in a smaller window
// behind p and b.
func TestTheWindowOpensWhenTheVerbIsIssued(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		// status decides which verbs the row offers: a stash that would
		// conflict offers branch instead of restore.
		status git.StashStatus
		issue  func(*Model) (tea.Model, tea.Cmd)
	}{
		{"drop", git.StashApplies, func(m *Model) (tea.Model, tea.Cmd) {
			next, _ := m.requestStashDrop()
			return next.(*Model).Update(tea.KeyPressMsg{Code: 'y'})
		}},
		{"restore", git.StashApplies, (*Model).requestStashRestore},
		{"branch", git.StashConflicts, (*Model).requestStashBranch},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := fixed(t)
			m.dir = t.TempDir()
			m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabStashed})
			m.state = state.Apply(m.state, state.StashedLoaded{Stashes: []git.StashRow{
				{Ref: "stash@{0}", SHA: "s0", Branch: "main", FileCount: 1,
					Status: c.status},
			}})
			m.state.Cursor = 0

			next, _ := c.issue(m)
			m = next.(*Model)

			if !m.state.Stashed.RefsStale {
				t.Errorf("%s was issued and the refs were still called current", c.name)
			}
		})
	}
}
