package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
)

// The stashed and worktrees tabs read what a row holds in a command, and the
// answer comes back as a message. Nothing reached the two handlers that apply
// it: measured with make cover-zero, handleWorktreeFilesMsg and
// handleStashFilesMsg were both at 0.0% while every other handler in update.go
// was covered. A tab whose files never arrive draws a row that opens onto
// nothing, and no test would have said so.
func TestFilesArriveUnderTheRowThatAskedForThem(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		open func() *Model
		msg  tea.Msg
		want string
	}{
		{
			name: "a worktree",
			open: func() *Model {
				s := state.State{Width: 80, Height: 20}
				s = state.Apply(s, state.WorktreesLoaded{Base: "main", Here: "/a",
					Worktrees: []git.WorktreeRow{
						{Path: "/a", Name: "a", Branch: "ba"},
						{Path: "/b", Name: "b", Branch: "bb"},
					}})
				s = state.Apply(s, state.TabChanged{Tab: state.TabWorktrees})
				return modelHolding(s)
			},
			msg:  worktreeFilesMsg{path: "/a", files: []git.Entry{{Path: "x.txt"}}},
			want: "x.txt",
		},
		{
			name: "a stash",
			open: func() *Model {
				s := state.State{Width: 80, Height: 20}
				s = state.Apply(s, state.StashedLoaded{Stashes: []git.StashRow{
					{Ref: "stash@{0}", SHA: "s0", Message: "one"},
					{Ref: "stash@{1}", SHA: "s1", Message: "two"},
				}})
				s = state.Apply(s, state.TabChanged{Tab: state.TabStashed})
				return modelHolding(s)
			},
			msg:  stashFilesMsg{ref: "stash@{0}", files: []git.Entry{{Path: "y.txt"}}},
			want: "y.txt",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := c.open()
			if fileRowsOf(m) != 0 {
				t.Fatal("a row is open before the files arrived")
			}

			next, _ := m.Update(c.msg)
			m = next.(*Model)

			if n := fileRowsOf(m); n != 1 {
				t.Fatalf("%d file rows after the answer arrived, want 1", n)
			}
			if got := state.Facts[m.state.Tab].ExpandedKey(m.state); got == "" {
				t.Error("the files arrived and no row is named as open")
			}
			if fileLineOf(m, c.want) == "" {
				t.Errorf("%q is not on the screen", c.want)
			}
		})
	}
}

// A message naming a row the reader has since left is dropped rather than
// drawn under whatever is open now. The read is started by one row and answered
// after the cursor moved, which is the ordinary case on a repository slow
// enough for the read to take a frame.
func TestFilesForARowNobodyOpenedAreNotDrawn(t *testing.T) {
	t.Parallel()
	s := state.State{Width: 80, Height: 20}
	s = state.Apply(s, state.StashedLoaded{Stashes: []git.StashRow{
		{Ref: "stash@{0}", SHA: "s0", Message: "one"},
	}})
	s = state.Apply(s, state.TabChanged{Tab: state.TabStashed})
	m := modelHolding(s)

	next, _ := m.Update(stashFilesMsg{ref: "stash@{9}", files: []git.Entry{{Path: "z.txt"}}})
	m = next.(*Model)

	if n := fileRowsOf(m); n != 0 {
		t.Errorf("%d file rows for a stash that is not open", n)
	}
	if fileLineOf(m, "z.txt") != "" {
		t.Error("a file of a row nobody opened is on the screen")
	}
	// Held as well as drawn: rowsFor answers no rows once the key is unknown,
	// so an answer kept without being drawn costs the reader nothing on this
	// frame and everything on the next reload, when the key comes back.
	if n := len(m.state.Stashed.Files); n != 0 {
		t.Errorf("%d file(s) of a row nobody opened are held", n)
	}
}

// modelHolding is modelWith without a repository. These two handlers apply a
// message and rebuild the rows; neither starts git, so a Model built field by
// field is the whole of what they need and the test runs under -short.
func modelHolding(s state.State) *Model {
	m := &Model{state: s, read: emptyRead(), render: layout.Renderer{}}
	// View draws the width probe until this is set, and the probe has no rows
	// on it.
	m.probe.settled = true
	return m
}

func fileRowsOf(m *Model) int {
	n := 0
	for _, row := range m.state.Rows {
		if row.Kind() == state.RowFile {
			n++
		}
	}
	return n
}
