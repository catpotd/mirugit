package layout

import (
	"strings"
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

func stashRow(message string, status git.StashStatus) git.StashRow {
	return git.StashRow{
		Ref:       "stash@{0}",
		SHA:       "abc123def456",
		Message:   message,
		Branch:    "main",
		FileCount: 1,
		Age:       "8h",
		Status:    status,
	}
}

func TestStashRowIsExactlyPaneWidth(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	cases := []stashRowLayout{
		{Info: stashRow("あとで戻す作業", git.StashApplies)},
		{Info: stashRow(strings.Repeat("long message ", 8), git.StashConflicts)},
		{Info: stashRow("chore: 集約検証の土台", git.StashApplies), State: stashRowState{Cursor: true}, Verbs: state.StashVerbs(git.StashApplies)},
	}
	for _, c := range cases {
		line, _ := w.StashRow(c, paneWidth)
		if got := w.Of(line); got != paneWidth {
			t.Errorf("%q is %d cells, want %d", line, got, paneWidth)
		}
	}
}

func TestStashRowSurvivesEveryPaneWidth(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	cases := []stashRowLayout{
		{Info: stashRow("chore: 集約検証の土台", git.StashApplies)},
		{Info: stashRow("wip", git.StashApplies),
			State: stashRowState{Cursor: true},
			Verbs: []state.VerbName{state.VerbNameRestore, state.VerbNameDrop}},
	}
	for _, c := range cases {
		for width := 1; width <= 80; width++ {
			line, _ := w.StashRow(c, width)
			if width >= 40 && w.Of(line) != width {
				t.Fatalf("width %d: %q is %d cells, want %d", width, line, w.Of(line), width)
			}
		}
	}
}

func TestStashFileRowSurvivesEveryPaneWidth(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	file := git.Entry{
		Path:          "internal/layout/very_long_name.go",
		Worktree:      git.Modified,
		WorktreeCount: git.Count{Added: 3, Deleted: 1},
	}
	// The collision label is the widest thing this row can hold, so it is what
	// the narrow widths have to survive.
	row := nestedFileRow(file)
	row.State.Conflicts = true
	for width := 1; width <= 80; width++ {
		line, _ := w.FileRow(row, width)
		if width >= 50 && w.Of(line) != width {
			t.Fatalf("width %d: %q is %d cells, want %d", width, line, w.Of(line), width)
		}
	}
}

func TestConflictingStashVerbsOmitRestore(t *testing.T) {
	t.Parallel()
	verbs := state.StashVerbs(git.StashConflicts)
	for _, v := range verbs {
		if v == state.VerbNameRestore {
			t.Fatalf("conflicts should not offer restore: %v", verbs)
		}
	}
}

func TestStashVerbRegionsSitOnTheDrawnTokens(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	row := stashRowLayout{
		Info:  stashRow("applies", git.StashApplies),
		State: stashRowState{Cursor: true},
		Verbs: state.StashVerbs(git.StashApplies),
	}
	line, regions := w.StashRow(row, paneWidth)
	for _, r := range regions {
		if r.Target.Verb != state.VerbNameDrop {
			continue
		}
		if got := cellSlice(line, r.ColStart, r.ColEnd); got != "x drop" {
			t.Fatalf("drop region = %q, want %q", got, "x drop")
		}
		return
	}
	t.Fatal("no region for drop")
}

func TestBranchSitsLeftOfWhereRestoreWouldBe(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	applies := stashRowLayout{
		Info:  stashRow("applies", git.StashApplies),
		State: stashRowState{Cursor: true},
		Verbs: state.StashVerbs(git.StashApplies),
	}
	conflicts := stashRowLayout{
		Info:  stashRow("conflicts", git.StashConflicts),
		State: stashRowState{Cursor: true},
		Verbs: state.StashVerbs(git.StashConflicts),
	}
	aLine, _ := w.StashRow(applies, paneWidth)
	cLine, _ := w.StashRow(conflicts, paneWidth)
	restoreAt := strings.Index(aLine, "restore")
	branchAt := strings.Index(cLine, "branch")
	if restoreAt < 0 || branchAt < 0 {
		t.Fatalf("verbs missing: %q / %q", aLine, cLine)
	}
	if branchAt != restoreAt {
		t.Errorf("branch at %d, restore at %d", branchAt, restoreAt)
	}
}

func TestStashVerbsPutBranchBeforeRestoreInOrder(t *testing.T) {
	t.Parallel()
	applies := state.StashVerbs(git.StashApplies)
	if indexOf(applies, state.VerbNameRestore) >= indexOf(applies, state.VerbNameDrop) {
		t.Fatalf("restore should precede drop: %v", applies)
	}
	conflicts := state.StashVerbs(git.StashConflicts)
	if indexOf(conflicts, state.VerbNameBranch) >= indexOf(conflicts, state.VerbNameDrop) {
		t.Fatalf("branch should precede drop: %v", conflicts)
	}
}

func indexOf(ss []state.VerbName, s state.VerbName) int {
	for i, v := range ss {
		if v == s {
			return i
		}
	}
	return -1
}

func TestStashRowNameDoesNotMoveWhenRead(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	info := stashRow("stash message", git.StashApplies)
	unread, _ := w.StashRow(stashRowLayout{Info: info, State: stashRowState{Unread: true}}, paneWidth)
	read, _ := w.StashRow(stashRowLayout{Info: info}, paneWidth)
	if got := cellsBefore(unread, info.Message); got != markColumn+2 {
		t.Fatalf("unread message starts at %d, want %d", got, markColumn+2)
	}
	if cellsBefore(unread, info.Message) != cellsBefore(read, info.Message) {
		t.Errorf("the message moved when read:\n%q\n%q", unread, read)
	}
}

func TestStashRowUnreadMarkSitsInTheReadColumn(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	line, _ := w.StashRow(stashRowLayout{
		Info:  stashRow("stash message", git.StashApplies),
		State: stashRowState{Unread: true},
	}, paneWidth)
	if got := string([]rune(line)[markColumn]); got != "·" {
		t.Fatalf("unread mark = %q, want · in %q", got, line[:12])
	}
}

// The collision label is what tells the reader that restoring this stash would
// land on a file they have changed. The stash names where it would land; the
// row asks it. Nothing checked the label was drawn, so the answer could stop
// arriving without a test noticing.
func TestAStashFileThatWouldCollideSaysSo(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	stash := git.StashRow{
		Ref: "stash@{0}", SHA: "s0", Message: "hold", Branch: "main",
		FileCount: 2, Age: "1h", Status: git.StashConflicts,
		Collides: []string{"app/clash.go"},
	}
	s := state.State{Width: paneWidth, Height: 20, Tab: state.TabStashed, Cursor: 0,
		Stashed: state.Stashed{Stashes: []git.StashRow{stash}, Expanded: stash.SHA, Files: []git.Entry{
			{Path: "app/clash.go", Worktree: git.Modified, WorktreeCount: git.Count{Added: 1}},
			{Path: "app/clear.go", Worktree: git.Modified, WorktreeCount: git.Count{Added: 1}},
		}}}
	s.Rows = state.StashRowsFor(s)

	lines, _ := stashListOf(s, emptyReadMarks{}, w, everyLine)
	var clash, clear string
	for _, line := range lines {
		switch {
		case strings.Contains(line, "clash.go"):
			clash = line
		case strings.Contains(line, "clear.go"):
			clear = line
		}
	}
	if clash == "" || clear == "" {
		t.Fatalf("both files should be drawn:\n%s", strings.Join(lines, "\n"))
	}
	if !strings.Contains(clash, "conflicts") {
		t.Errorf("the file the stash lands on does not say so: %q", clash)
	}
	if strings.Contains(clear, "conflicts") {
		t.Errorf("a file the stash does not land on says it collides: %q", clear)
	}
}

// The collision label belongs to the stash the file is printed under: that
// stash is what knows where the file would land. A file row with no stash
// above it — the list was cut, or the row is not where it was — belongs to no
// stash, and labeling it "conflicts" says a file collides with nothing.
func TestAStashFileWithNoStashAboveItCollidesWithNothing(t *testing.T) {
	t.Parallel()
	s := state.State{Width: 90, Height: 30, Tab: state.TabStashed}
	s.Changes.Folded = map[string]bool{}
	s.Changes.Selected = map[string]bool{}
	// A file row on its own, with no stash row above it.
	s.Rows = []state.Row{state.FileRow(git.Entry{Path: "held.txt"}, state.SectionNone)}
	row := stashLayoutFileRow(s, s.Rows[0], 0)
	if row.State.Conflicts {
		t.Error("a file under no stash is drawn as colliding")
	}

	// Under a stash that names it, it does collide: the answer above is not
	// "never". The rows are built the way the program builds them, because the
	// link from a file to the stash above it is what the label is read through.
	real := state.State{Width: 90, Height: 30}
	real.Changes.Folded = map[string]bool{}
	real.Changes.Selected = map[string]bool{}
	real = state.Apply(real, state.TabChanged{Tab: state.TabStashed})
	real = state.Apply(real, state.StashedLoaded{Stashes: []git.StashRow{{
		Ref: "stash@{0}", SHA: "s0", Message: "one", Status: git.StashConflicts,
		Collides: []string{"held.txt"}}}})
	real = state.Apply(real, state.StashFilesLoaded{Ref: "stash@{0}",
		Files: []git.Entry{{Path: "held.txt"}, {Path: "other.txt"}}})

	found := 0
	for i, row := range real.Rows {
		if row.Kind() != state.RowFile {
			continue
		}
		found++
		got := stashLayoutFileRow(real, row, i).State.Conflicts
		if want := row.Path() == "held.txt"; got != want {
			t.Errorf("%q is drawn as colliding = %v, want %v", row.Path(), got, want)
		}
	}
	if found != 2 {
		t.Fatalf("the stash drew %d file rows, want 2", found)
	}
}
