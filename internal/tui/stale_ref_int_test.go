package tui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// A stash is named by its place in the list — stash@{0}, stash@{1} — so dropping
// one renames every stash below it. Asking for the cursor's files with the list
// the drop made stale asks for a place that is no longer there, and git says so:
// the reader who had just dropped a stash was shown
// "fatal: log for 'stash' only has 8 entries".
//
// The reload brings the new list and reads the cursor's files from it. Nothing
// has to be read in between.
func TestDroppingAStashDoesNotReadTheOldList(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to git")
	}
	t.Parallel()
	dir := testRepo(t)
	write(t, dir, "a.txt", "base\n")
	runGitIn(t, dir, "add", ".")
	runGitIn(t, dir, "commit", "-m", "base")
	for i := range 3 {
		write(t, dir, "a.txt", "edit\n"+string(rune('a'+i))+"\n")
		runGitIn(t, dir, "stash", "push", "-m", "held")
	}

	m, err := New(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.probe.settled = true
	m.state = state.Apply(m.state, state.Resized{Width: 90, Height: 40})
	m = pumpInit(t, m)
	m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabStashed})
	m = pumpCmd(t, m, m.reloadCurrentTab())

	// The last stash in the list: dropping it leaves a place nothing fills, so
	// a read of the old list asks git for a stash that is gone.
	last := -1
	for i, row := range m.state.Rows {
		if row.Kind() == state.RowStash {
			last = i
		}
	}
	if last < 0 {
		t.Fatalf("no stash rows to drop: %d rows", len(m.state.Rows))
	}
	m.state.Cursor = last

	next, _ := m.requestStashDrop()
	m = next.(*Model)
	if _, pending := state.PendingConfirmation(m.state); !pending {
		t.Fatal("dropping a stash is supposed to ask first")
	}
	next, cmd := m.Update(tea.KeyPressMsg{Code: 'y'})
	m = pumpCmd(t, next.(*Model), cmd)

	if strings.Contains(m.state.Notice, "fatal") ||
		strings.Contains(m.state.Notice, "only has") {
		t.Errorf("dropping a stash reported git's complaint about the old list: %q",
			m.state.Notice)
	}
	if got := stashRowCount(m.state); got != 2 {
		t.Errorf("two stashes should be left, the pane draws %d", got)
	}
}

func stashRowCount(s state.State) int {
	n := 0
	for _, row := range s.Rows {
		if row.Kind() == state.RowStash {
			n++
		}
	}
	return n
}

// write puts a file in the working tree.
func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A drop that fails leaves the list as it was, so the window it opened has to
// close: the reader is still on the stash they tried to drop, and the pane owes
// them its contents. Leaving the marker up meant that stash's files, and every
// stash they moved to after it, were never read again.
func TestADropThatFailsReopensTheStashList(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabStashed})
	m.state = state.Apply(m.state, state.StashedLoaded{Stashes: []git.StashRow{
		{Ref: "stash@{0}", SHA: "s0", Branch: "main", Status: git.StashApplies},
	}})
	m.state = state.Apply(m.state, state.StashDropConfirmationShown{
		Confirm: state.StashDropConfirm{SHA: "s0", Message: "held"}})
	m.state = state.Apply(m.state, state.StashDropConfirmed{})
	if !m.state.Stashed.RefsStale {
		t.Fatal("answering the question is supposed to open the window")
	}

	next, cmd := m.Update(stashDropMsg{err: errors.New("stash drop refused")})
	m = next.(*Model)
	if m.state.Notice == "" {
		t.Error("a drop that did not run said nothing")
	}

	// The list that comes back is what closes the window, on both ways out.
	if !hasStashedCmd(cmd) {
		t.Fatal("a drop that failed did not ask for the list again")
	}
	m.state = state.Apply(m.state, state.StashedLoaded{Stashes: []git.StashRow{
		{Ref: "stash@{0}", SHA: "s0", Branch: "main", Status: git.StashApplies},
	}})
	if m.state.Stashed.RefsStale {
		t.Error("the list came back and the window stayed shut")
	}
}

// hasStashedCmd reports whether the batch asks for the stash list.
func hasStashedCmd(cmd tea.Cmd) bool {
	found := false
	walkCmds(cmd, func(msg tea.Msg) {
		switch msg.(type) {
		case stashedMsg, repoMsg:
			found = true
		}
	})
	return found
}

// The reads a drop invalidated are the reader's own doing, and they land after
// the drop. Reporting them put "log for 'stash' only has N entries" on top of a
// drop that worked, and closed the window before the new list arrived — which
// let the next read reach for a stash that had moved.
func TestAReadTheDropInvalidatedIsNotReportedAtTheReader(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabStashed})
	m.state = state.Apply(m.state, state.StashedLoaded{Stashes: []git.StashRow{
		{Ref: "stash@{0}", SHA: "s0", Branch: "main", FileCount: -1},
	}})
	m.state = state.Apply(m.state, state.StashDropConfirmationShown{
		Confirm: state.StashDropConfirm{SHA: "s0", Message: "held"}})
	m.state = state.Apply(m.state, state.StashDropConfirmed{})

	gone := errors.New("log for 'stash' only has 1 entries")
	next, _ := m.Update(stashStatusMsg{index: 0, sha: "s0", err: gone})
	m = next.(*Model)

	if m.state.Notice != "" {
		t.Errorf("a read the drop invalidated was reported: %q", m.state.Notice)
	}
	if !m.state.Stashed.RefsStale {
		t.Error("that read closed the window the drop opened")
	}
}

// A read that fails outside a drop is a real failure and is reported.
func TestAReadThatFailsOnItsOwnIsReported(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabStashed})
	m.state = state.Apply(m.state, state.StashedLoaded{Stashes: []git.StashRow{
		{Ref: "stash@{0}", SHA: "s0", Branch: "main", FileCount: -1},
	}})

	next, _ := m.Update(stashStatusMsg{index: 0, sha: "s0", err: errors.New("permission denied")})
	m = next.(*Model)

	if m.state.Notice != "permission denied" {
		t.Errorf("a read that failed on its own said %q", m.state.Notice)
	}
}

// A read is stale when the list no longer holds the stash it was read for,
// whoever moved it. Watching the reader's own drop covered their key and not
// another terminal's: the pane reported git's complaint at someone who had
// done nothing.
func TestAReadOvertakenByAnotherTerminalIsNotReported(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabStashed})
	m.state = state.Apply(m.state, state.StashedLoaded{Stashes: []git.StashRow{
		{Ref: "stash@{0}", SHA: "s0", Branch: "main", FileCount: -1},
	}})

	// The read was issued for s0. Another terminal dropped it, and the list
	// that came back holds s1 in its place.
	m.state = state.Apply(m.state, state.StashedLoaded{Stashes: []git.StashRow{
		{Ref: "stash@{0}", SHA: "s1", Branch: "main", FileCount: -1},
	}})

	gone := errors.New("log for 'stash' only has 1 entries")
	next, _ := m.Update(stashStatusMsg{index: 0, sha: "s0", err: gone})
	m = next.(*Model)

	if m.state.Notice != "" {
		t.Errorf("a read the list had moved past was reported: %q", m.state.Notice)
	}
}
