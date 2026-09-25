package layout

import (
	"strings"
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

func TestFooterIsExactlyPaneWidth(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	cases := []state.State{{Width: paneWidth, Height: 53, Changes: state.Changes{Selected: map[string]bool{}}, Rows: []state.Row{
		state.FileRow(git.Entry{Path: "a.txt", Worktree: git.Modified}, state.SectionUnstaged),
	}}, {Width: paneWidth, Height: 53, Cursor: 0, Changes: state.Changes{Selected: map[string]bool{"a.txt": true}}, Rows: []state.Row{
		state.FileRow(git.Entry{Path: "a.txt", Index: git.Modified}, state.SectionStaged),
	}}}
	for i, s := range cases {
		line := w.Footer(s, paneWidth)
		if got := w.Of(line); got != paneWidth {
			t.Errorf("[%d] got %d cells, want %d: %q", i, got, paneWidth, line)
		}
		if !strings.HasSuffix(line, "?") {
			t.Errorf("[%d] want ? on the right: %q", i, line)
		}
	}
}

func TestChangesFooterIncludesSelectAndReadWithinPaneWidth(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := state.State{Width: paneWidth, Cursor: 0, Rows: []state.Row{
		state.FileRow(git.Entry{Path: "a.txt", Worktree: git.Modified}, state.SectionUnstaged),
	}, Changes: state.Changes{Selected: map[string]bool{}}}
	line := w.Footer(s, paneWidth)
	if got := w.Of(line); got != paneWidth {
		t.Errorf("got %d cells, want %d: %q", got, paneWidth, line)
	}
	if !strings.Contains(line, "space select") || !strings.Contains(line, "r read") {
		t.Errorf("got %q", line)
	}
}

func TestFooterShowsStageOnUnstagedCursor(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := state.State{Width: paneWidth, Cursor: 0, Rows: []state.Row{
		state.FileRow(git.Entry{Path: "a.txt", Worktree: git.Modified}, state.SectionUnstaged),
	}, Changes: state.Changes{Selected: map[string]bool{}}}
	line := w.Footer(s, paneWidth)
	if !strings.Contains(line, "s stage") || !strings.Contains(line, "d diff") {
		t.Errorf("got %q", line)
	}
}

func TestFooterShowsStashOnUnstagedCursor(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := state.State{Width: paneWidth, Cursor: 0, Rows: []state.Row{
		state.FileRow(git.Entry{Path: "a.txt", Worktree: git.Modified}, state.SectionUnstaged),
	}, Changes: state.Changes{Selected: map[string]bool{}}}
	line := w.Footer(s, paneWidth)
	if !strings.Contains(line, "z stash") {
		t.Errorf("got %q", line)
	}
}

func TestFooterOmitsStashOnStagedRename(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := state.State{Width: paneWidth, Cursor: 0, Rows: []state.Row{state.FileRow(
		git.Entry{Path: "new.txt", OldPath: "old.txt", Index: git.Renamed},
		state.SectionStaged)}, Changes: state.Changes{Selected: map[string]bool{}}}
	line := w.Footer(s, paneWidth)
	if strings.Contains(line, "stash") {
		t.Errorf("staged rename should not offer stash: %q", line)
	}
}

func TestFooterShowsUnstageOnStagedCursor(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := state.State{Width: paneWidth, Cursor: 0, Rows: []state.Row{
		state.FileRow(git.Entry{Path: "a.txt", Index: git.Modified}, state.SectionStaged),
	}, Changes: state.Changes{Selected: map[string]bool{}}}
	line := w.Footer(s, paneWidth)
	if !strings.Contains(line, "u unstage") {
		t.Errorf("got %q", line)
	}
}

func TestFooterShowsDiscardConfirmation(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := state.State{Width: paneWidth, Changes: state.Changes{DiscardConfirm: &state.DiscardConfirm{Files: 2}}}
	line := w.Footer(s, paneWidth)
	if !strings.Contains(line, "y discard") || !strings.Contains(line, "any other key cancels") {
		t.Errorf("got %q", line)
	}
}

func TestFooterShowsStashDropConfirmation(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := state.State{Width: paneWidth, Stashed: state.Stashed{DropConfirm: &state.StashDropConfirm{
		SHA: "abc", Message: "hold",
	}}}
	line := w.Footer(s, paneWidth)
	if !strings.Contains(line, "y drop") || !strings.Contains(line, "any other key cancels") ||
		!strings.Contains(line, "stash cannot be undone") {
		t.Errorf("got %q", line)
	}
}

func TestFooterSeparatesTheVerbsFromTheHelpMark(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	line := w.Footer(state.State{Width: paneWidth, Height: 53, Cursor: 0, Rows: []state.Row{state.FileRow(
		git.Entry{Path: "a.txt", Worktree: git.Modified},
		state.SectionUnstaged)}, Changes: state.Changes{Selected: map[string]bool{}}}, paneWidth)

	if !strings.HasSuffix(line, " ?") {
		t.Errorf("the help mark is flush against the last verb: %q", line)
	}
}

func TestFooterWhileTypingACommitMessage(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	want := w.Pad("", "esc leave · enter commit", paneWidth)
	line := w.Footer(state.State{Width: paneWidth, Rows: []state.Row{state.FileRow(

		git.Entry{Path: "a.txt", Index: git.Modified}, state.SectionStaged)}, Changes: state.Changes{MessageFocused: true}}, paneWidth)
	if got := w.Of(line); got != paneWidth {
		t.Errorf("got %d cells, want %d: %q", got, paneWidth, line)
	}
	if line != want {
		t.Errorf("got %q, want %q", line, want)
	}
}

func TestFooterWhileTypingWithNothingStaged(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	want := w.Pad("", "esc leave · stage a file first", paneWidth)
	line := w.Footer(state.State{Width: paneWidth, Changes: state.Changes{MessageFocused: true}}, paneWidth)
	if got := w.Of(line); got != paneWidth {
		t.Errorf("got %d cells, want %d: %q", got, paneWidth, line)
	}
	if line != want {
		t.Errorf("got %q, want %q", line, want)
	}
	if strings.Contains(line, "enter commit") {
		t.Errorf("footer must not offer commit with nothing staged: %q", line)
	}
}

func TestHistoryFooterIsExactlyPaneWidth(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := state.State{Width: paneWidth, Height: 53, Tab: state.TabHistory, Cursor: 0, Rows: []state.Row{state.CommitRow(git.CommitInfo{SHA: "abc", ShortSHA: "abc", Subject: "one", Unpushed: true, Age: "1m"})}, History: state.History{Commits: []git.CommitInfo{{SHA: "abc", ShortSHA: "abc", Subject: "one", Unpushed: true, Age: "1m"}}}}
	line := w.Footer(s, paneWidth)
	if got := w.Of(line); got != paneWidth {
		t.Errorf("got %d cells, want %d: %q", got, paneWidth, line)
	}
}

func TestHistoryFooterOmitsOpenForNonURLRemote(t *testing.T) {
	t.Parallel()
	w := Renderer{Browser: true}
	base := state.State{Width: paneWidth, Height: 53, Tab: state.TabHistory, Cursor: 0, Rows: []state.Row{state.CommitRow(git.CommitInfo{SHA: "abc", ShortSHA: "abc", Subject: "one", Unpushed: false, Age: "1m"})}, History: state.History{Commits: []git.CommitInfo{{SHA: "abc", ShortSHA: "abc", Subject: "one", Unpushed: false, Age: "1m"}}}}

	s := base
	s.History.RemoteURL = "/private/tmp/origin.git"
	line := w.Footer(s, paneWidth)
	if strings.Contains(line, "open") {
		t.Fatalf("filesystem remote should not show open: %q", line)
	}

	s = base
	s.History.RemoteURL = "git@github.com:owner/repo.git"
	line = w.Footer(s, paneWidth)
	if !strings.Contains(line, "open") {
		t.Fatalf("ssh remote should show open: %q", line)
	}
}

// The target and the verbs are two different statements, so they need air
// between them; run together they read as one phrase.
func TestTheFooterSeparatesItsTargetFromTheVerbs(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	line := w.Footer(state.State{Width: paneWidth, Height: 53, Rows: []state.Row{state.FileRow(
		git.Entry{Path: "a.txt", Worktree: git.Modified}, state.SectionUnstaged)}, Changes: state.Changes{Selected: map[string]bool{"a.txt": true}}}, paneWidth)
	if strings.Contains(line, "selected space") {
		t.Errorf("the target runs into the verbs: %q", line)
	}
	if w.Of(line) != paneWidth {
		t.Errorf("footer is %d cells", w.Of(line))
	}
}

func TestConflictedFooterShowsResolveInYourEditor(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := state.State{Width: paneWidth, Height: 53, Tab: state.TabChanges, Cursor: 0, Rows: []state.Row{state.FileRow(

		git.Entry{Path: "both.txt", Index: git.Unmerged, Worktree: git.Unmerged},
		state.SectionUnstaged)}, Changes: state.Changes{Selected: map[string]bool{}}}
	line := w.Footer(s, paneWidth)
	if !strings.Contains(line, "resolve in your editor") {
		t.Errorf("got %q", line)
	}
}

func TestConflictedFooterOmitsStage(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := state.State{Width: paneWidth, Tab: state.TabChanges, Cursor: 0, Rows: []state.Row{state.FileRow(

		git.Entry{Path: "both.txt", Index: git.Unmerged, Worktree: git.Unmerged},
		state.SectionUnstaged)}, Changes: state.Changes{Selected: map[string]bool{}}}
	line := w.Footer(s, paneWidth)
	if strings.Contains(line, "s stage") {
		t.Errorf("conflicted footer should omit stage: %q", line)
	}
}

func TestConflictedFooterIsExactly77Cells(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := state.State{Width: paneWidth, Height: 53, Tab: state.TabChanges, Cursor: 0, Rows: []state.Row{state.FileRow(

		git.Entry{Path: "both.txt", Index: git.Unmerged, Worktree: git.Unmerged},
		state.SectionUnstaged)}, Changes: state.Changes{Selected: map[string]bool{}}}
	line := w.Footer(s, paneWidth)
	if got := w.Of(line); got != paneWidth {
		t.Errorf("got %d cells, want %d: %q", got, paneWidth, line)
	}
}

// The footer names its target only when the target is not the cursor row. The cursor row's own name is one line up, and repeating it costs the
// width the verbs need.
func TestFooterNamesTheSelectionAndNotTheCursorRow(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := state.State{Tab: state.TabChanges, Cursor: 0, Rows: []state.Row{state.FileRow(

		git.Entry{
			Path:     "app/Tests/AppTests/SyncOwnershipTests.swift",
			Worktree: git.Modified,
		}, state.SectionUnstaged)}, Open: state.OpenDiff{Path: "app/Tests/AppTests/SyncOwnershipTests.swift"}}
	line := w.footerPlain(s, paneWidth)
	if strings.Contains(line, "SyncOwnershipTests") {
		t.Errorf("the cursor row's name is repeated in the footer: %q", line)
	}
	for _, verb := range []state.VerbName{state.VerbNameSelect, state.VerbNameStage, state.VerbNameDiscard, state.VerbNameStash, state.VerbNameDiff, state.VerbNameRead} {
		if !strings.Contains(line, verb.String()) {
			t.Errorf("%q missing from %q", verb, line)
		}
	}

	s.Changes.Selected = map[string]bool{"a.go": true, "b.go": true}
	if got := w.footerPlain(s, paneWidth); !strings.HasPrefix(got, "2 selected") {
		t.Errorf("got %q, want it to start with the selection count", got)
	}
}

func TestFooterLeftIsEmptyWhenDiffIsClosed(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := state.State{Width: paneWidth, Height: 53, Cursor: 0, Rows: []state.Row{state.FileRow(
		git.Entry{Path: "app/SyncOwnershipTests.swift", Worktree: git.Modified},
		state.SectionUnstaged)}, Changes: state.Changes{Selected: map[string]bool{}}}
	line := w.footerPlain(s, paneWidth)
	if strings.HasPrefix(strings.TrimLeft(line, " "), "SyncOwnershipTests") {
		t.Errorf("got %q, want an empty left side", line)
	}
}

func TestFooterWithOpenDiffIsExactly77Cells(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := state.State{Width: paneWidth, Height: 53, Cursor: 0, Rows: []state.Row{state.FileRow(
		git.Entry{Path: "app/SyncOwnershipTests.swift", Worktree: git.Modified},
		state.SectionUnstaged)}, Changes: state.Changes{Selected: map[string]bool{}}, Open: state.OpenDiff{Path: "app/SyncOwnershipTests.swift"}}
	line := w.footerPlain(s, paneWidth)
	if got := w.Of(line); got != paneWidth {
		t.Errorf("got %d cells, want %d: %q", got, paneWidth, line)
	}
}

func TestFooterWithClosedDiffIsExactly77Cells(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := state.State{Width: paneWidth, Height: 53, Cursor: 0, Rows: []state.Row{state.FileRow(
		git.Entry{Path: "app/SyncOwnershipTests.swift", Worktree: git.Modified},
		state.SectionUnstaged)}, Changes: state.Changes{Selected: map[string]bool{}}}
	line := w.footerPlain(s, paneWidth)
	if got := w.Of(line); got != paneWidth {
		t.Errorf("got %d cells, want %d: %q", got, paneWidth, line)
	}
}

func TestNonConflictedFooterOmitsResolveNote(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := state.State{Width: paneWidth, Height: 53, Tab: state.TabChanges, Cursor: 0, Rows: []state.Row{state.FileRow(
		git.Entry{Path: "a.txt", Worktree: git.Modified},
		state.SectionUnstaged)}, Changes: state.Changes{Selected: map[string]bool{}}}
	line := w.Footer(s, paneWidth)
	if strings.Contains(line, "resolve in your editor") {
		t.Errorf("non-conflicted row should not show resolve note: %q", line)
	}
}

func TestFooterShowsSelectionVerbsOnDirectoryRow(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := state.State{Width: paneWidth, Tab: state.TabChanges, Cursor: 0, Rows: []state.Row{
		state.DirectoryRow("src", state.SectionUnstaged),
		state.FileRow(git.Entry{Path: "a.txt", Worktree: git.Modified}, state.SectionUnstaged),
		state.FileRow(git.Entry{Path: "b.txt", Worktree: git.Untracked}, state.SectionUnstaged),
	}}
	s = state.Apply(s, state.SelectionToggled{Path: "a.txt", Section: state.SectionUnstaged})
	s = state.Apply(s, state.SelectionToggled{Path: "b.txt", Section: state.SectionUnstaged})
	line := w.Footer(s, paneWidth)
	for _, want := range []string{"space select", "← fold", "s stage", "x discard", "z stash"} {
		if !strings.Contains(line, want) {
			t.Errorf("missing %q in %q", want, line)
		}
	}
	if strings.Contains(line, "u unstage") {
		t.Errorf("staged selection verbs should not appear: %q", line)
	}
}

func TestFooterShowsUnfoldOnFoldedDirectoryRow(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := state.State{Width: paneWidth, Tab: state.TabChanges, Cursor: 0, Rows: []state.Row{
		state.DirectoryRow("src", state.SectionUnstaged),
	}}
	s = state.Apply(s, state.DirectoryFolded{Path: "src", Section: state.SectionUnstaged})
	line := w.Footer(s, paneWidth)
	if !strings.Contains(line, "→ unfold") {
		t.Errorf("missing → unfold in %q", line)
	}
	if strings.Contains(line, "← fold") {
		t.Errorf("folded row should not show ← fold: %q", line)
	}
}

func TestFooterShowsSelectionVerbsOnSectionHeading(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := state.State{Width: paneWidth, Tab: state.TabChanges, Cursor: 0, Rows: []state.Row{
		state.SectionHeadingRow(state.SectionStaged),
		state.FileRow(git.Entry{Path: "c.go", Index: git.Modified}, state.SectionStaged),
	}}
	s = state.Apply(s, state.SelectionToggled{Path: "c.go", Section: state.SectionStaged})
	line := w.Footer(s, paneWidth)
	for _, want := range []string{"space select", "u unstage", "x discard"} {
		if !strings.Contains(line, want) {
			t.Errorf("missing %q in %q", want, line)
		}
	}
	if strings.Contains(line, "s stage") {
		t.Errorf("stage should not appear for staged-only selection: %q", line)
	}
}

func TestFooterFileRowUnderSelectionShowsSelectionVerbs(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := state.State{Width: paneWidth, Tab: state.TabChanges, Cursor: 0, Rows: []state.Row{
		state.FileRow(git.Entry{Path: "c.go", Index: git.Modified}, state.SectionStaged),
		state.FileRow(git.Entry{Path: "a.txt", Worktree: git.Modified}, state.SectionUnstaged),
	}}
	s = state.Apply(s, state.SelectionToggled{Path: "a.txt", Section: state.SectionUnstaged})
	line := w.Footer(s, 120)
	for _, want := range []string{"s stage", "d diff", "r read"} {
		if !strings.Contains(line, want) {
			t.Errorf("missing %q in %q", want, line)
		}
	}
	if strings.Contains(line, "u unstage") {
		t.Errorf("cursor-row unstage should not appear when only unstaged file is selected: %q", line)
	}
}

func TestFooterOmitsGoOnHereWorktree(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	here := "/repo"
	other := "/repo/wt"
	trees := []git.WorktreeRow{
		{Path: here, Name: "repo", Branch: "main", Main: true},
		{Path: other, Name: "wt", Branch: "feat", SHA: "abc"},
	}
	hereState := state.State{Width: paneWidth, Height: 53, Tab: state.TabWorktrees, Cursor: 0, Rows: []state.Row{
		state.WorktreeRowOf(trees[0]),
		state.WorktreeRowOf(trees[1]),
	}, Worktrees: state.Worktrees{Here: here}, Changes: state.Changes{Selected: map[string]bool{}}}
	line := w.Footer(hereState, paneWidth)
	if strings.Contains(line, "go") {
		t.Errorf("footer on current tree should omit go: %q", line)
	}
	awayState := hereState
	awayState.Cursor = 1
	line = w.Footer(awayState, paneWidth)
	if !strings.Contains(line, "go") {
		t.Errorf("footer on other tree while viewing here should include go: %q", line)
	}
}

func TestFooterOffersNextBlockWhileADiffWithHiddenBlocksIsOpen(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	blocks := []git.Block{{Hash: "a"}, {Hash: "b"}}
	s := state.State{Width: 120, Height: 53, Tab: state.TabChanges, Rows: []state.Row{state.FileRow(
		git.Entry{Path: "a.txt", Worktree: git.Modified},
		state.SectionUnstaged)}, Open: state.OpenDiff{Path: "a.txt", Diff: git.FileDiff{Blocks: blocks}}, Changes: state.Changes{Selected: map[string]bool{}}}
	line := w.footerPlain(s, 120)
	if !strings.Contains(line, "n next block") {
		t.Errorf("got %q", line)
	}
	s.Open.Path = ""
	if strings.Contains(w.footerPlain(s, 120), "n next block") {
		t.Error("should not offer next block without an open diff")
	}
	s.Open.Path = "a.txt"
	s.Open.Diff = git.FileDiff{Blocks: []git.Block{{Hash: "a"}}}
	if strings.Contains(w.footerPlain(s, 120), "n next block") {
		t.Error("should not offer next block when only one block exists")
	}
}

func TestFooterOmitsSelectOffTheChangesTab(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	commit := git.CommitInfo{SHA: "abc", ShortSHA: "abc", Subject: "one", Age: "1m"}
	stash := git.StashRow{
		Ref: "stash@{0}", SHA: "s", Message: "hold", Branch: "main",
		Age: "1h", Status: git.StashApplies,
	}
	here := "/repo"
	wt := git.WorktreeRow{Path: here, Name: "repo", Branch: "main", Main: true}
	cases := []state.State{{
		Width: paneWidth, Height: 53, Tab: state.TabHistory, Cursor: 0,
		History: state.History{Commits: []git.CommitInfo{commit}},
		Rows:    []state.Row{state.CommitRow(commit)},
	}, {
		Width: paneWidth, Height: 53, Tab: state.TabStashed, Cursor: 0,
		Stashed: state.Stashed{Stashes: []git.StashRow{stash}},
		Rows:    []state.Row{state.StashRowOf(stash)},
	}, {
		Width: paneWidth, Height: 53, Tab: state.TabWorktrees, Cursor: 0,
		Worktrees: state.Worktrees{Here: here}, Changes: state.Changes{Selected: map[string]bool{}},
		Rows: []state.Row{state.WorktreeRowOf(wt)},
	}}
	for i, s := range cases {
		line := w.Footer(s, paneWidth)
		if strings.Contains(line, "space select") {
			t.Errorf("[%d] footer should not offer space select: %q", i, line)
		}
	}
}

func TestFooterDoesNotWrapAVerbTwiceWhenTheOpenFileIsThatVerb(t *testing.T) {
	t.Parallel()
	w := Renderer{Palette: Palette{Enabled: true}}
	s := state.State{Width: paneWidth, Cursor: 1, Rows: []state.Row{
		state.FileRow(git.Entry{Path: "other.go", Worktree: git.Modified}, state.SectionUnstaged),
		state.FileRow(git.Entry{Path: "stage", Worktree: git.Modified}, state.SectionUnstaged),
	}, Open: state.OpenDiff{Path: "stage"}, Changes: state.Changes{Selected: map[string]bool{}}}
	line := w.Footer(s, paneWidth)
	double := w.Verb(w.Verb("stage"))
	if strings.Contains(line, double) {
		t.Errorf("footer double-wrapped stage: %q", line)
	}
}

func TestFooterOffersDiffOnAStashFileRow(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := stashPaneStateWithFiles()
	if strings.Contains(w.Footer(s, paneWidth), "d diff") {
		t.Errorf("stash row footer should not offer diff: %q", w.Footer(s, paneWidth))
	}
	s.Cursor = 1
	if !strings.Contains(w.Footer(s, paneWidth), "d diff") {
		t.Errorf("stash file row footer = %q, want d diff", w.Footer(s, paneWidth))
	}
}

// The note is for the changes tab, where the verbs that would stage a file are.
// Both halves of its guard were open: every test that reaches it passes a
// changes tab with a row under the cursor, so the tab test and the row test
// could be joined the wrong way and nothing would say so.
func TestTheResolveNoteIsOnlyOnTheChangesTab(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	conflicted := state.FileRow(
		git.Entry{Path: "both.txt", Index: git.Unmerged, Worktree: git.Unmerged},
		state.SectionUnstaged)

	for _, c := range []struct {
		name string
		tab  state.Tab
		rows []state.Row
		want bool
	}{
		{"a conflicted row on the changes tab", state.TabChanges, []state.Row{conflicted}, true},
		{"the same row on the stashed tab", state.TabStashed, []state.Row{conflicted}, false},
		{"the same row on the worktrees tab", state.TabWorktrees, []state.Row{conflicted}, false},
		{"the changes tab with nothing under the cursor", state.TabChanges, nil, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := state.State{Width: paneWidth, Height: 53, Tab: c.tab, Cursor: 0,
				Rows: c.rows, Changes: state.Changes{Selected: map[string]bool{}}}
			line := w.Footer(s, paneWidth)
			if got := strings.Contains(line, "resolve in your editor"); got != c.want {
				t.Errorf("the note is %v on the footer, want %v: %q", got, c.want, line)
			}
		})
	}
}

// The note sits apart from the verbs, and the gap between them is what keeps
// "d diff" from reading as part of "resolve in your editor". The gap belongs
// only where there are verbs to be apart from: put before a note that stands
// alone, it pushes the note off its column.
func TestTheResolveNoteIsHeldApartFromTheVerbsBesideIt(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := state.State{Width: paneWidth, Height: 53, Tab: state.TabChanges, Cursor: 0,
		Rows: []state.Row{state.FileRow(
			git.Entry{Path: "both.txt", Index: git.Unmerged, Worktree: git.Unmerged},
			state.SectionUnstaged)},
		Changes: state.Changes{Selected: map[string]bool{}}}
	line := w.Footer(s, paneWidth)

	const note = "resolve in your editor"
	at := strings.Index(line, note)
	if at < 0 {
		t.Fatalf("the note is not on the footer, so this proves nothing: %q", line)
	}
	before := strings.TrimRight(line[:at], " ")
	if before == "" {
		t.Fatalf("no verbs are drawn beside the note, so this proves nothing: %q", line)
	}
	if gap := at - len(before); gap < 2 {
		t.Errorf("%d spaces between %q and the note; they read as one line: %q",
			gap, before, line)
	}
}

// The count of what is selected and the keys that act on it are two statements,
// and a gap is what keeps them from reading as one phrase. The gap belongs to
// the count: a footer with nothing selected has no count to separate, and two
// blanks at the head of the line push every key one place along.
func TestTheSelectionCountIsSeparatedFromTheKeysBesideIt(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	rows := []state.Row{
		state.FileRow(git.Entry{Path: "a.go", Worktree: git.Modified}, state.SectionUnstaged),
	}
	s := state.State{Width: paneWidth, Height: 53, Cursor: 0, Rows: rows,
		Changes: state.Changes{Selected: map[string]bool{"a.go": true, "b.go": true}}}

	// A wide pane leaves slack that hides a missing gap, so the widths where
	// the line is full are the ones that show it: measured, the count and the
	// first key ran together at 44 columns and were one space apart at 36.
	for width := 20; width <= 90; width++ {
		line := w.footerPlain(s, width)
		at := strings.Index(line, "selected")
		if at < 0 {
			continue
		}
		rest := line[at+len("selected"):]
		gap := len(rest) - len(strings.TrimLeft(rest, " "))
		if strings.TrimSpace(rest) != "" && gap < 2 {
			t.Errorf("at %d columns the count is %d cells from what follows it: %q",
				width, gap, line)
		}
	}

	// With nothing selected there is no count, and the keys are right-aligned
	// as before: the gap belongs to the count and goes with it.
	s.Changes.Selected = map[string]bool{}
	none := w.footerPlain(s, paneWidth)
	if strings.Contains(none, "selected") {
		t.Errorf("a footer with nothing selected still counts one: %q", none)
	}
	if got := w.Of(none); got != paneWidth {
		t.Errorf("the footer is %d cells, want %d: %q", got, paneWidth, none)
	}
}

// While the commit box has the focus, the key that commits is the one the
// footer paints — and it is painted only when there is something to commit.
// Painting it with an empty index offers a key that does nothing; leaving it
// plain when the index is full hides the one key the reader is there for.
func TestTheCommitKeyIsPaintedOnlyWhenSomethingIsStaged(t *testing.T) {
	t.Parallel()
	w := Renderer{Palette: Palette{Enabled: true, Theme: slate}}
	build := func(staged bool) state.State {
		s := state.State{Width: paneWidth, Height: 53, Tab: state.TabChanges}
		s.Changes.Folded = map[string]bool{}
		s.Changes.Selected = map[string]bool{}
		entry := git.Entry{Path: "a.txt", Worktree: git.Modified}
		if staged {
			entry = git.Entry{Path: "a.txt", Index: git.Modified}
		}
		s = state.Apply(s, state.StatusLoaded{Rows: []git.Entry{entry},
			Head: git.Head{Branch: "main"}})
		s.Changes.MessageFocused = true
		return s
	}
	// With the box focused the footer reads "esc leave · enter commit", so the
	// word is what the paint lands on.
	const word = "commit"
	for _, c := range []struct {
		name   string
		staged bool
		want   bool
	}{
		{"nothing staged", false, false},
		{"one file staged", true, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := build(c.staged)
			line := w.colorFooter(s, w.footerPlain(s, paneWidth))
			// With nothing staged the footer says so instead of offering the
			// key, and there is nothing to paint.
			if got := strings.Contains(stripSGR(line), "enter commit"); got != c.staged {
				t.Fatalf("the footer offers the commit key = %v, want %v: %q",
					got, c.staged, stripSGR(line))
			}
			painted := strings.Contains(line, w.Verb(word))
			if painted != c.want {
				t.Errorf("the commit key is painted = %v, want %v: %q",
					painted, c.want, line)
			}
		})
	}
}

// The note about a conflict is drawn after the verbs, so the room the verbs
// are fitted into has to account for it. Fitting them as if the note were not
// there keeps more verbs than the row can hold, and the line then runs past
// the pane: Pad lets an overlong pair overflow on purpose, so the row wraps
// and every row below it moves down.
func TestTheFooterFitsItsVerbsAroundTheConflictNote(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	conflicted := state.FileRow(git.Entry{Path: "a.txt",
		Index: git.Unmerged, Worktree: git.Unmerged}, state.SectionUnstaged)
	s := state.State{Width: paneWidth, Height: 53, Tab: state.TabChanges, Cursor: 0,
		Rows: []state.Row{conflicted}, Changes: state.Changes{Selected: map[string]bool{}}}

	withNote := 0
	for width := 20; width <= 100; width++ {
		line := w.Footer(s, width)
		if got := w.Of(line); got != width {
			t.Errorf("at %d columns the footer is %d cells: %q", width, got, line)
		}
		if strings.Contains(line, "resolve in your editor") {
			withNote++
		}
	}
	if withNote == 0 {
		t.Fatal("the note was never drawn, so this proves nothing")
	}
}

// The note is dropped only when there is no room for it. A pane exactly as wide
// as the note and the help mark has that room: dropping it there leaves the
// reader of a conflicted file with no word about what to do, on the one pane
// where the file is named.
func TestTheNoteIsDrawnOnTheNarrowestPaneThatHoldsIt(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	const note = "resolve in your editor"
	fits := w.Of(note) + helpGap + w.Of("?")

	for _, c := range []struct {
		name  string
		width int
		want  bool
	}{
		{"the narrowest pane that holds it", fits, true},
		{"one cell narrower", fits - 1, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := state.State{Width: c.width, Height: 53, Tab: state.TabChanges, Cursor: 0,
				Rows: []state.Row{state.FileRow(
					git.Entry{Path: "both.txt", Index: git.Unmerged, Worktree: git.Unmerged},
					state.SectionUnstaged)},
				Changes: state.Changes{Selected: map[string]bool{}}}
			line := w.footerPlain(s, c.width)
			if got := strings.Contains(line, note); got != c.want {
				t.Errorf("the note is %v on a pane of %d cells, want %v: %q",
					got, c.width, c.want, line)
			}
			// A row wider than the pane wraps and pushes every row under it down.
			if got := w.Of(line); got != c.width {
				t.Errorf("the footer is %d cells on a pane of %d: %q", got, c.width, line)
			}
		})
	}
}

// The verbs are dropped from the end, one at a time, until what is left fits.
// A pane the whole list fills exactly is a pane it fits: dropping the last verb
// there hides a key the pane had room to print.
func TestTheLastVerbIsKeptOnAPaneItFillsExactly(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	s := state.State{Height: 53, Tab: state.TabChanges, Cursor: 0,
		Rows: []state.Row{state.FileRow(
			git.Entry{Path: "a.txt", Worktree: git.Modified}, state.SectionUnstaged)},
		Changes: state.Changes{Selected: map[string]bool{}}}
	verbs := footerVerbs(s, w)
	if len(verbs) < 2 {
		t.Fatalf("the footer offers %v, so dropping one proves nothing", verbs)
	}
	last := verbs[len(verbs)-1]
	key, ok := FooterKeyFor(last)
	if !ok {
		t.Fatalf("the footer prints no key for %q", last)
	}
	fits := w.Of(w.footerRight(verbs, 1000, ""))

	for _, c := range []struct {
		name  string
		width int
		want  bool
	}{
		{"the pane the list fills exactly", fits, true},
		{"one cell narrower", fits - 1, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s.Width = c.width
			line := w.footerPlain(s, c.width)
			if got := strings.Contains(line, key+" "+last.String()); got != c.want {
				t.Errorf("%q is %v on a pane of %d cells, want %v: %q",
					last, got, c.width, c.want, line)
			}
			if got := w.Of(line); got != c.width {
				t.Errorf("the footer is %d cells on a pane of %d: %q", got, c.width, line)
			}
		})
	}
}
