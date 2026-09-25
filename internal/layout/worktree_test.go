package layout

import (
	"slices"
	"strings"
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

func worktreeRow(name, branch string, dirty int, merge git.MergeStatus) git.WorktreeRow {
	return git.WorktreeRow{
		Path:     "/repo/" + name,
		Name:     name,
		Branch:   branch,
		SHA:      "abc123",
		Ahead:    2,
		Dirty:    dirty,
		WroteAge: "wrote 2m",
		Merge:    merge,
	}
}

func TestWorktreeRowIsExactlyPaneWidth(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	cases := []worktreeRowLayout{
		{Info: worktreeRow("wt-feature", "feat/feature-x", 3, git.MergeClean)},
		{Info: worktreeRow("wt-fix", "fix/fix-y", 1, git.MergeConflicts), State: worktreeRowState{Cursor: true}},
		{Info: worktreeRow("wt-here", "feat/here", 0, git.MergeClean), State: worktreeRowState{Here: true}},
		{Info: git.WorktreeRowSkeleton("/repo/wt", "wt", "feat")},
	}
	for _, c := range cases {
		line, _ := w.WorktreeRow(c, paneWidth)
		if got := w.Of(line); got != paneWidth {
			t.Errorf("%q is %d cells, want %d", line, got, paneWidth)
		}
	}
}

func TestWorktreeRowShowsEllipsisWhileLoading(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	row := git.WorktreeRowSkeleton("/repo/wt", "wt", "feat")
	line, _ := w.WorktreeRow(worktreeRowLayout{Info: row}, paneWidth)
	for _, want := range []string{"…"} {
		if !strings.Contains(line, want) {
			t.Fatalf("line %q should contain %q while loading", line, want)
		}
	}
}

func TestWorktreePaneNeverShowsAgentLabel(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	trees := []git.WorktreeRow{
		worktreeRow("wt-feature", "feat/feature-x", 3, git.MergeClean),
		worktreeRow("wt-fix", "fix/fix-y", 1, git.MergeConflicts),
	}
	s := state.State{Width: paneWidth, Height: 20, Tab: state.TabWorktrees, Rows: []state.Row{
		state.WorktreeRowOf(trees[0]),
		state.WorktreeRowOf(trees[1]),
	}, Worktrees: state.Worktrees{List: trees, Base: "main"}}
	f := Pane(s, nil, w)
	for i, line := range f.Lines {
		if strings.Contains(line, "agent") {
			t.Fatalf("line %d contains agent label: %q", i, line)
		}
	}
}

func TestWorktreesHeadingShowsBaseOnce(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	line, _ := w.Heading("worktrees", 2, 0, 0, state.SectionNone, "", "vs main", paneWidth)
	if !strings.Contains(line, "worktrees · 2") {
		t.Fatalf("heading %q", line)
	}
	if !strings.Contains(line, "vs main") {
		t.Fatalf("heading %q", line)
	}
}

// Unpushed commits survive removal, measured in
// TestWorktreeRemoveKeepsUnpushedCommits, so they are not a reason to withhold
// the offer. What git refuses is the main tree and a tree holding uncommitted
// work.
func TestWorktreeVerbsOfferRemoveOnACleanLinkedTree(t *testing.T) {
	t.Parallel()
	row := worktreeRow("wt", "feat", 0, git.MergeClean)
	if !slices.Contains(state.WorktreeVerbs(row, false), state.VerbNameRemove) {
		t.Errorf("remove withheld from a clean linked tree: %v", state.WorktreeVerbs(row, false))
	}
}

func TestWorktreeVerbsWithholdRemoveForTheMainTree(t *testing.T) {
	t.Parallel()
	row := worktreeRow("wt", "feat", 0, git.MergeClean)
	row.Main = true
	if slices.Contains(state.WorktreeVerbs(row, false), state.VerbNameRemove) {
		t.Errorf("remove offered for the main tree: %v", state.WorktreeVerbs(row, false))
	}
}

func TestWorktreeVerbsWithholdRemoveWhileUncommittedWorkIsThere(t *testing.T) {
	t.Parallel()
	row := worktreeRow("wt", "feat", 3, git.MergeClean)
	if slices.Contains(state.WorktreeVerbs(row, false), state.VerbNameRemove) {
		t.Errorf("remove offered for a dirty tree: %v", state.WorktreeVerbs(row, false))
	}
}

func TestWorktreeRowMarksHere(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	row := worktreeRow("wt", "feat", 0, git.MergeClean)
	line, _ := w.WorktreeRow(worktreeRowLayout{
		Info:  row,
		State: worktreeRowState{Here: true},
	}, paneWidth)
	if !strings.HasPrefix(line, "    *") {
		t.Fatalf("line %q should start with here marker", line)
	}
	line, _ = w.WorktreeRow(worktreeRowLayout{Info: row}, paneWidth)
	if !strings.HasPrefix(line, "     ") {
		t.Fatalf("line %q should start with blank prefix", line)
	}
}

func TestWorktreeRowMarksHereUnderCursor(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	row := worktreeRow("wt", "feat", 0, git.MergeClean)
	line, _ := w.WorktreeRow(worktreeRowLayout{
		Info:  row,
		State: worktreeRowState{Cursor: true, Here: true},
	}, paneWidth)
	if !strings.HasPrefix(line, "▌   *") {
		t.Fatalf("line %q should start with cursor and here marker", line)
	}
}

func TestWorktreeHereMarkSitsInTheReadColumn(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	line, _ := w.WorktreeRow(worktreeRowLayout{
		Info:  worktreeRow("wt", "feat", 0, git.MergeClean),
		State: worktreeRowState{Here: true},
	}, paneWidth)
	if got := string([]rune(line)[markColumn]); got != "*" {
		t.Fatalf("here mark = %q, want * in %q", got, line[:12])
	}
}

func TestWorktreeVerbsOmitGoOnHere(t *testing.T) {
	t.Parallel()
	row := worktreeRow("wt", "feat", 0, git.MergeClean)
	if slices.Contains(state.WorktreeVerbs(row, true), state.VerbNameGo) {
		t.Errorf("go offered on current tree: %v", state.WorktreeVerbs(row, true))
	}
	if got := state.WorktreeVerbs(row, true); row.CanRemove() && !slices.Equal(got, []state.VerbName{state.VerbNameRemove}) {
		t.Errorf("got %v, want [remove]", got)
	}
	if got := state.WorktreeVerbs(row, false); len(got) == 0 || got[0] != state.VerbNameGo {
		t.Errorf("got %v, want go first", got)
	}
}

func TestWorktreeCursorRowKeepsMerge(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	row := worktreeRow("wt", "feat", 0, git.MergeClean)

	plain, _ := w.WorktreeRow(worktreeRowLayout{Info: row}, paneWidth)
	cursor, _ := w.WorktreeRow(worktreeRowLayout{
		Info:  row,
		State: worktreeRowState{Cursor: true},
		Verbs: []state.VerbName{state.VerbNameGo, state.VerbNameRemove},
	}, paneWidth)
	if got := w.Of(cursor); got != paneWidth {
		t.Fatalf("cursor line is %d cells, want %d: %q", got, paneWidth, cursor)
	}
	if !strings.Contains(cursor, "merges") {
		t.Fatalf("cursor line = %q, want merges", cursor)
	}
	mergeCol := cellsBefore(plain, "merges")
	cursorMergeCol := cellsBefore(cursor, "merges")
	if mergeCol < 0 || cursorMergeCol != mergeCol {
		t.Fatalf("merges at column %d, non-cursor at %d: %q", cursorMergeCol, mergeCol, cursor)
	}
	if !strings.Contains(cursor, "g go") || !strings.Contains(cursor, "x remove") {
		t.Fatalf("cursor line = %q, want g go and x remove", cursor)
	}

	conflict := worktreeRow("wt-conf", "feat/conf", 1, git.MergeConflicts)
	conflict.Conflicts = 2
	conflictPlain, _ := w.WorktreeRow(worktreeRowLayout{Info: conflict}, paneWidth)
	conflictCursor, _ := w.WorktreeRow(worktreeRowLayout{
		Info:  conflict,
		State: worktreeRowState{Cursor: true},
		Verbs: []state.VerbName{state.VerbNameGo, state.VerbNameRemove},
	}, paneWidth)
	if !strings.Contains(conflictCursor, "conflicts 2") {
		t.Fatalf("cursor line = %q, want conflicts 2", conflictCursor)
	}
	conflictCol := cellsBefore(conflictPlain, "conflicts")
	conflictCursorCol := cellsBefore(conflictCursor, "conflicts")
	if conflictCol < 0 || conflictCursorCol != conflictCol {
		t.Fatalf("conflicts at column %d, non-cursor at %d: %q", conflictCursorCol, conflictCol, conflictCursor)
	}

	colored := Renderer{Palette: Palette{Enabled: true}}
	mergeLine, _ := colored.WorktreeRow(worktreeRowLayout{
		Info:  row,
		State: worktreeRowState{Cursor: true},
		Verbs: []state.VerbName{state.VerbNameGo, state.VerbNameRemove},
	}, paneWidth)
	if !strings.Contains(mergeLine, colored.Add("merges")) {
		t.Errorf("merges should use Add: %q", mergeLine)
	}
	conflictLine, _ := colored.WorktreeRow(worktreeRowLayout{
		Info:  conflict,
		State: worktreeRowState{Cursor: true},
		Verbs: []state.VerbName{state.VerbNameGo, state.VerbNameRemove},
	}, paneWidth)
	if !strings.Contains(conflictLine, colored.Del("conflicts")) {
		t.Errorf("conflicts should use Del: %q", conflictLine)
	}
}

func TestWorktreeFileRowSurvivesEveryPaneWidth(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	file := git.Entry{
		Path:          "internal/layout/very_long_name.go",
		Worktree:      git.Modified,
		WorktreeCount: git.Count{Added: 3, Deleted: 1},
	}
	for width := 1; width <= 80; width++ {
		line, _ := w.FileRow(nestedFileRow(file), width)
		if width >= 40 && w.Of(line) != width {
			t.Fatalf("width %d: %q is %d cells, want %d", width, line, w.Of(line), width)
		}
	}
}

// A branch called feat/merges pads to the same trailing cells as the merge
// field, so a line-wide search for that text paints the branch instead.
func TestWorktreeCursorRowPaintsTheMergeColumnNotTheBranch(t *testing.T) {
	t.Parallel()
	w := Renderer{Palette: Palette{Enabled: true, Theme: slate}}
	for _, branch := range []string{"feat", "feat/merges", "merges", "very-long-branch-name"} {
		for _, cursor := range []bool{false, true} {
			var verbs []state.VerbName
			if cursor {
				verbs = []state.VerbName{state.VerbNameGo, state.VerbNameRemove}
			}
			line, _ := w.WorktreeRow(worktreeRowLayout{
				Info: git.WorktreeRow{Path: "/w", Name: "wt", Branch: branch,
					SHA: "abc", Merge: git.MergeClean},
				State: worktreeRowState{Cursor: cursor},
				Verbs: verbs,
			}, paneWidth)
			plain := stripSGR(line)
			if got := w.Of(plain); got != paneWidth {
				t.Errorf("branch %q cursor %v: row is %d cells, want %d",
					branch, cursor, got, paneWidth)
			}
			at := strings.LastIndex(plain, "merges")
			if at < 0 {
				t.Errorf("branch %q cursor %v: the merge column is gone: %q",
					branch, cursor, plain)
				continue
			}
			want := w.Of(plain[:at])
			i := strings.Index(line, w.Add("merges"))
			if i < 0 {
				t.Errorf("branch %q cursor %v: merges is not painted", branch, cursor)
				continue
			}
			if got := w.Of(stripSGR(line[:i])); got != want {
				t.Errorf("branch %q cursor %v: painted at column %d, want %d",
					branch, cursor, got, want)
			}
		}
	}
}
