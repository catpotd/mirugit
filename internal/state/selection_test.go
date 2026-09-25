package state

import (
	"testing"

	"github.com/catpotd/mirugit/internal/git"
)

func bothSidesEntry() git.Entry {
	return git.Entry{
		Path:          "a.txt",
		Index:         git.Modified,
		Worktree:      git.Modified,
		IndexCount:    git.Count{Added: 2, Deleted: 1},
		WorktreeCount: git.Count{Added: 5, Deleted: 0},
	}
}

func TestRowPathReturnsEntryPathForRowFile(t *testing.T) {
	t.Parallel()
	row := FileRow(git.Entry{Path: "a.txt"}, SectionNone)
	if row.Path() != "a.txt" {
		t.Errorf("got %q", row.Path())
	}
}

func TestSelectionRangeExtendedKeepsExistingPaths(t *testing.T) {
	t.Parallel()
	s := State{Rows: []Row{
		FileRow(git.Entry{Path: "a.txt", Worktree: git.Modified}, SectionUnstaged),
		FileRow(git.Entry{Path: "b.txt", Worktree: git.Modified}, SectionUnstaged),
		FileRow(git.Entry{Path: "c.txt", Worktree: git.Modified}, SectionUnstaged),
	}, Cursor: 1, Changes: Changes{Selected: map[string]bool{}}}
	s = Apply(s, SelectionToggled{Path: "a.txt", Section: SectionUnstaged})
	s = Apply(s, SelectionRangeExtended{From: 1, To: 2})
	for _, p := range []string{"a.txt", "b.txt", "c.txt"} {
		if !SelectedIn(s, SectionUnstaged, p) {
			t.Fatalf("%s is not ticked: %+v", p, s.Changes.Selected)
		}
	}
}

func TestApplyDoesNotMutateTheSelectedMapItWasGiven(t *testing.T) {
	t.Parallel()
	orig := map[string]bool{"a.txt": true}
	before := State{Changes: Changes{Selected: orig}}
	_ = Apply(before, SelectionToggled{Path: "b.txt"})
	if orig["b.txt"] {
		t.Error("Apply mutated the caller's Selected map")
	}
}

func TestChangingTabClearsTheSelection(t *testing.T) {
	t.Parallel()
	s := State{Tab: TabChanges, Changes: Changes{Selected: map[string]bool{"a.txt": true}}}
	s = Apply(s, TabChanged{Tab: TabHistory})
	if len(s.Changes.Selected) != 0 {
		t.Fatalf("got %+v", s.Changes.Selected)
	}
}

func TestSectionToggleSelectsEveryPathInTheSection(t *testing.T) {
	t.Parallel()
	s := State{Rows: []Row{
		FileRow(git.Entry{Path: "a.txt", Index: git.Modified}, SectionStaged),
		FileRow(git.Entry{Path: "b.txt", Worktree: git.Modified}, SectionUnstaged),
	}, Changes: Changes{Selected: map[string]bool{}}}
	s = Apply(s, SectionToggled{Section: SectionStaged})
	if !SelectedIn(s, SectionStaged, "a.txt") || SelectedIn(s, SectionUnstaged, "b.txt") {
		t.Fatalf("got %+v", s.Changes.Selected)
	}
}

// The count the summary shows is rows, not paths, so a path on both sides
// contributes two. Selecting the tab ticks every row the reader can see.
func TestTabSelectionTicksEveryRowIncludingBothSidesOfAPath(t *testing.T) {
	t.Parallel()
	s := State{Rows: []Row{
		FileRow(bothSidesEntry(), SectionStaged),
		FileRow(bothSidesEntry(), SectionUnstaged),
		FileRow(git.Entry{Path: "b.txt", Worktree: git.Modified}, SectionUnstaged),
	}, Changes: Changes{Selected: map[string]bool{}}}
	s = Apply(s, TabSelectionToggled{})
	if got := SelectionCount(s); got != 3 {
		t.Fatalf("got %d ticked rows, want 3: %+v", got, s.Changes.Selected)
	}
	if !SelectedIn(s, SectionStaged, "a.txt") ||
		!SelectedIn(s, SectionUnstaged, "a.txt") ||
		!SelectedIn(s, SectionUnstaged, "b.txt") {
		t.Fatalf("got %+v", s.Changes.Selected)
	}
}

func TestTabSelectionToggledAgainClearsEverything(t *testing.T) {
	t.Parallel()
	s := State{Rows: []Row{
		FileRow(bothSidesEntry(), SectionStaged),
		FileRow(bothSidesEntry(), SectionUnstaged),
	}, Changes: Changes{Selected: map[string]bool{}}}
	s = Apply(s, TabSelectionToggled{})
	s = Apply(s, TabSelectionToggled{})
	if len(s.Changes.Selected) != 0 {
		t.Fatalf("got %+v, want empty", s.Changes.Selected)
	}
}

func TestSelectionClearedEmptiesTheSet(t *testing.T) {
	t.Parallel()
	s := State{Changes: Changes{Selected: map[string]bool{"a.txt": true}}}
	s = Apply(s, SelectionCleared{})
	if len(s.Changes.Selected) != 0 {
		t.Fatalf("got %+v", s.Changes.Selected)
	}
}

// The agent finishing a file is the normal case here, and git restore refuses
// the whole call over one stale path, so a vanished path cannot stay selected.
func TestSelectedPathsReturnsTickedPathsOfOneSection(t *testing.T) {
	t.Parallel()
	s := State{
		Rows: []Row{
			FileRow(git.Entry{Path: "a.txt", Worktree: git.Modified}, SectionUnstaged),
			FileRow(git.Entry{Path: "b.txt", Worktree: git.Modified}, SectionUnstaged),
			FileRow(git.Entry{Path: "c.txt", Index: git.Modified}, SectionStaged),
		},
		Changes: Changes{Selected: map[string]bool{}},
	}
	s = Apply(s, SelectionToggled{Path: "a.txt", Section: SectionUnstaged})
	s = Apply(s, SelectionToggled{Path: "b.txt", Section: SectionUnstaged})

	got := selectedPaths(s, SectionUnstaged)
	if len(got) != 2 {
		t.Fatalf("got %d paths, want 2: %v", len(got), got)
	}
	for _, want := range []string{"a.txt", "b.txt"} {
		found := false
		for _, p := range got {
			if p == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s is missing from %v", want, got)
		}
	}
	if paths := selectedPaths(s, SectionStaged); len(paths) != 0 {
		t.Errorf("staged section should be empty, got %v", paths)
	}
}

func TestAPathThatLeavesTheListLeavesTheSelection(t *testing.T) {
	t.Parallel()
	s := Apply(State{Changes: Changes{Selected: map[string]bool{}}}, StatusLoaded{Rows: []git.Entry{
		{Path: "a.txt", Worktree: git.Modified},
		{Path: "b.txt", Worktree: git.Modified},
	}})
	s = Apply(s, SelectionToggled{Path: "a.txt", Section: SectionUnstaged})
	s = Apply(s, SelectionToggled{Path: "b.txt", Section: SectionUnstaged})

	s = Apply(s, StatusLoaded{Rows: []git.Entry{{Path: "b.txt", Worktree: git.Modified}}})

	if SelectedIn(s, SectionUnstaged, "a.txt") {
		t.Error("a.txt is gone from the list but still selected")
	}
	// The refresh runs on every write to the repository, so a tick that does not
	// survive it cannot be spent: the reader ticks files and the count empties
	// itself. Checking only the path that left passes while every path is
	// dropped, because a key kept with a false value still counts in len.
	if !SelectedIn(s, SectionUnstaged, "b.txt") {
		t.Error("b.txt is still listed and lost its tick to the refresh")
	}
	if len(s.Changes.Selected) != 1 {
		t.Errorf("selection = %v, want only b.txt", s.Changes.Selected)
	}
}

// Selecting a whole section takes the files in that section. A heading or a
// directory row has no file to act on, and a file in the other section belongs
// to a verb the reader did not press: staging the unstaged section would then
// also unstage what was staged.
func TestSelectingASectionTakesItsOwnFileRowsOnly(t *testing.T) {
	t.Parallel()
	s := State{Tab: TabChanges}
	s = Apply(s, StatusLoaded{Rows: []git.Entry{
		{Path: "app/a.txt", Worktree: git.Modified},
		{Path: "app/b.txt", Worktree: git.Modified},
		{Path: "c.txt", Index: git.Modified},
	}})
	var headings, directories int
	for _, row := range s.Rows {
		if row.Kind() == RowSectionHeading {
			headings++
		}
		if row.Kind() == RowDirectory {
			directories++
		}
	}
	if headings == 0 || directories == 0 {
		t.Fatalf("the list holds %d headings and %d directories, so this proves nothing",
			headings, directories)
	}

	got := sectionPaths(s, SectionUnstaged)
	want := []string{"app/a.txt", "app/b.txt"}
	if len(got) != len(want) {
		t.Fatalf("the section holds %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("path %d is %q, want %q", i, got[i], want[i])
		}
	}
}

// Folding a directory selects the files in it, and a directory belongs to one
// section: the same folder can hold a staged file and an unstaged one, and the
// verb the reader pressed acts on one of them. A heading or another directory
// row has no file to act on at all.
func TestSelectingADirectoryTakesItsOwnFileRowsOnly(t *testing.T) {
	t.Parallel()
	s := State{Tab: TabChanges}
	s = Apply(s, StatusLoaded{Rows: []git.Entry{
		{Path: "app/a.txt", Worktree: git.Modified},
		{Path: "app/b.txt", Index: git.Modified},
		{Path: "other/c.txt", Worktree: git.Modified},
	}})

	got := directoryPaths(s, "app", SectionUnstaged)
	if len(got) != 1 || got[0] != "app/a.txt" {
		t.Errorf("the directory holds %v, want only the unstaged file in it", got)
	}
	if staged := directoryPaths(s, "app", SectionStaged); len(staged) != 1 ||
		staged[0] != "app/b.txt" {
		t.Errorf("the staged side holds %v, want only app/b.txt", staged)
	}
}

func TestSelectingADirectoryTakesEveryFileBelowIt(t *testing.T) {
	t.Parallel()
	s := Apply(State{}, StatusLoaded{Rows: []git.Entry{
		{Path: "app/top.go", Worktree: git.Modified},
		{Path: "app/internal/child.go", Worktree: git.Modified},
		{Path: "other/keep.go", Worktree: git.Modified},
	}})

	s = Apply(s, DirectorySelectionToggled{Path: "app", Section: SectionUnstaged})
	for _, path := range []string{"app/top.go", "app/internal/child.go"} {
		if !SelectedIn(s, SectionUnstaged, path) {
			t.Errorf("%q below the selected directory is not selected", path)
		}
	}
	if SelectedIn(s, SectionUnstaged, "other/keep.go") {
		t.Error("a file outside the selected directory is selected")
	}
}
