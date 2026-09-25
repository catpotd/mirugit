package layout

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// Every tab loads on its own, and a frame is drawn between each arrival. The
// half-loaded values are not hypothetical — the code names them: a stash whose
// contents have not been read carries FileCount -1, a worktree that has not
// been counted carries Dirty -1, and a row with no SHA yet has no age to show.
// A frame built from those has to be a rectangle like any other, and a row that
// does not know its count must not print one: "-1f" is a number a reader would
// read as a number.
//
// Each tab is given three rows so that two of them are not the cursor's. A
// single row is always the cursor's, and the cursor row is the one path that
// was already fitted to the pane before tonight.
//
// What it does not catch: the frames replay one order of arrivals. The tabs
// load concurrently, so a session sees orders this does not. What it covers is
// every prefix of one order, at every width, on every tab.
func TestEveryHalfLoadedFrameIsARectangle(t *testing.T) {
	t.Parallel()
	const height = 14
	events := halfLoadedArrivals()

	for _, w := range []Renderer{{}, {Palette: Palette{Enabled: true}}} {
		for prefix := 0; prefix <= len(events); prefix++ {
			for tab := range state.TabCount {
				for _, width := range sweptWidths() {
					s := state.State{Width: width, Height: height}
					s.Changes.Folded = map[string]bool{}
					s.Changes.Selected = map[string]bool{}
					s.Changes.Stale = map[string]bool{}
					for _, e := range events[:prefix] {
						s = state.Apply(s, e)
					}
					s = state.Apply(s, state.TabChanged{Tab: tab})
					f := Pane(s, nil, w)

					where := func() string {
						return state.Facts[tab].Name + " after " +
							string(rune('0'+prefix)) + " arrivals at width " +
							string(rune('0'+width/100)) + string(rune('0'+width/10%10)) +
							string(rune('0'+width%10))
					}
					if len(f.Lines) != height {
						t.Errorf("%s: %d lines", where(), len(f.Lines))
					}
					for i, line := range f.Lines {
						plain := ansi.Strip(line)
						if got := w.Of(plain); got != width {
							t.Errorf("%s line %d: %d columns: %q", where(), i, got, plain)
						}
						// The mark for "not read yet" is -1 in the value and
						// nothing on the screen. A row that prints it has told
						// the reader a count that is not one.
						if strings.Contains(plain, "-1") {
							t.Errorf("%s line %d shows a count it has not read: %q",
								where(), i, plain)
						}
					}
					for _, reg := range f.Regions {
						if reg.Target.Kind == TargetFile && reg.ColEnd <= reg.ColStart {
							t.Errorf("%s: a row is not clickable", where())
						}
						if reg.Row < 0 || reg.Row >= len(f.Lines) {
							t.Errorf("%s: a region points at line %d of %d",
								where(), reg.Row, len(f.Lines))
						}
					}
				}
			}
		}
	}
}

// halfLoadedArrivals is one tab's worth of rows per event, each tab holding the
// values it carries before the read that fills them in.
func halfLoadedArrivals() []state.Event {
	// Distinct keys, because rows are expanded and folded by SHA and by ref.
	stashes := make([]git.StashRow, 3)
	for i := range stashes {
		stashes[i] = git.StashRow{
			Ref: fmt.Sprintf("stash@{%d}", i), SHA: fmt.Sprintf("%07x", i+1),
			Branch: "a-branch", Message: "a stash message", FileCount: -1,
		}
	}
	// The half-read row is built by the same function the reading uses, so the
	// two cannot drift apart: it carries the age and the merge verdict the read
	// has not answered yet, and a hand-written copy of it left both blank.
	trees := make([]git.WorktreeRow, 3)
	for i := range trees {
		trees[i] = git.WorktreeRowSkeleton(
			fmt.Sprintf("/w%d", i), "a-worktree-name", "a-branch")
	}
	trees[0].Main = true
	entries := make([]git.Entry, 3)
	for i := range entries {
		entries[i] = git.Entry{Path: fmt.Sprintf("pkg/file%d.go", i), Worktree: git.Modified}
	}
	commits := make([]git.CommitInfo, 3)
	for i := range commits {
		sha := fmt.Sprintf("%07x", i+1)
		commits[i] = git.CommitInfo{SHA: sha, ShortSHA: sha, Subject: "a commit subject"}
	}
	return []state.Event{
		state.StatusLoaded{Rows: entries, Head: git.Head{Branch: "main"}},
		state.HistoryLoaded{Commits: commits},
		state.StashedLoaded{Stashes: stashes},
		state.WorktreesLoaded{Worktrees: trees, Base: "main", Here: "/w0"},
	}
}
