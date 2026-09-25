package state

import (
	"testing"

	"github.com/catpotd/mirugit/internal/git"
)

func TestVerbAppliesRejectsSectionHeading(t *testing.T) {
	t.Parallel()
	row := SectionHeadingRow(SectionUnstaged)
	for _, verb := range []Verb{VerbStage, VerbUnstage, VerbDiscard, VerbStash} {
		if VerbApplies(verb, row) {
			t.Errorf("verb %v applies to section heading", verb)
		}
	}
}

func TestVerbAppliesRejectsDirectoryRow(t *testing.T) {
	t.Parallel()
	row := DirectoryRow("src", SectionUnstaged)
	if VerbApplies(VerbStage, row) {
		t.Error("stage applies to directory row")
	}
}

func TestSelectionVerbsShowsUnstageForStagedOnlySelection(t *testing.T) {
	t.Parallel()
	s := State{Rows: []Row{
		SectionHeadingRow(SectionStaged),
		FileRow(git.Entry{Path: "c.go", Index: git.Modified}, SectionStaged),
	}}
	s = Apply(s, SelectionToggled{Path: "c.go", Section: SectionStaged})
	got := SelectionVerbs(s)
	for _, v := range got {
		if v == VerbNameStage {
			t.Fatalf("SelectionVerbs = %v, want unstage not stage", got)
		}
	}
	found := false
	for _, v := range got {
		if v == VerbNameUnstage {
			found = true
		}
	}
	if !found {
		t.Fatalf("SelectionVerbs = %v, want unstage", got)
	}
}

func TestChangesRowVerbs(t *testing.T) {
	t.Parallel()
	conflict := FileRow(

		git.Entry{Path: "both.txt", Index: git.Unmerged, Worktree: git.Unmerged},
		SectionUnstaged)

	if got := ChangesRowVerbs(conflict); len(got) != 1 || got[0] != VerbNameDiscard {
		t.Fatalf("conflict = %v, want [discard]", got)
	}

	unstaged := FileRow(

		git.Entry{Path: "a.txt", Worktree: git.Modified},
		SectionUnstaged)

	if got := ChangesRowVerbs(unstaged); len(got) != 3 || got[0] != VerbNameStage || got[1] != VerbNameDiscard || got[2] != VerbNameStash {
		t.Fatalf("unstaged = %v, want [stage discard stash]", got)
	}

	stagedRename := FileRow(

		git.Entry{Path: "new.txt", OldPath: "old.txt", Index: git.Renamed},
		SectionStaged)

	if got := ChangesRowVerbs(stagedRename); len(got) != 2 || got[0] != VerbNameUnstage || got[1] != VerbNameDiscard {
		t.Fatalf("staged rename = %v, want [unstage discard]", got)
	}

	if got := ChangesRowVerbs(DirectoryRow("src", SectionUnstaged)); got != nil {
		t.Fatalf("directory = %v, want nil", got)
	}
}

func TestVerbAppliesWithholdsStageOnConflict(t *testing.T) {
	t.Parallel()
	row := FileRow(

		git.Entry{Path: "both.txt", Index: git.Unmerged, Worktree: git.Unmerged},
		SectionUnstaged)

	if VerbApplies(VerbStage, row) {
		t.Error("stage applies on conflict")
	}
	if !VerbApplies(VerbDiscard, row) {
		t.Error("discard should apply on conflict")
	}
}

func TestVerbAppliesWithholdsUnstageOnConflictedStaged(t *testing.T) {
	t.Parallel()
	row := FileRow(

		git.Entry{Path: "both.txt", Index: git.Unmerged, Worktree: git.Unmerged},
		SectionStaged)

	if VerbApplies(VerbUnstage, row) {
		t.Error("unstage applies on conflicted staged row")
	}
}

// The footer draws ChangesRowVerbs and the keys ask VerbApplies, so a verb the
// reader can see has to be one the key runs. The expectations here are written
// out rather than read from ChangesRowVerbs: taking them from the same call the
// answer comes from compares an expression with itself, which stays green
// however the wiring breaks.
func TestVerbAppliesMatchesChangesRowVerbs(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		row  Row
		want map[Verb]bool
	}{
		{"an unstaged edit",
			FileRow(git.Entry{Path: "a.txt", Worktree: git.Modified}, SectionUnstaged),
			map[Verb]bool{VerbStage: true, VerbUnstage: false, VerbDiscard: true, VerbStash: true, VerbUndo: false}},
		{"a staged edit",
			FileRow(git.Entry{Path: "a.txt", Index: git.Modified}, SectionStaged),
			map[Verb]bool{VerbStage: false, VerbUnstage: true, VerbDiscard: true, VerbStash: true, VerbUndo: false}},
		{"a conflict, which only discard resolves",
			FileRow(git.Entry{Path: "both.txt", Index: git.Unmerged, Worktree: git.Unmerged}, SectionUnstaged),
			map[Verb]bool{VerbStage: false, VerbUnstage: false, VerbDiscard: true, VerbStash: false, VerbUndo: false}},
		{"a staged rename, which stash cannot carry",
			FileRow(git.Entry{Path: "new.txt", OldPath: "old.txt", Index: git.Renamed}, SectionStaged),
			map[Verb]bool{VerbStage: false, VerbUnstage: true, VerbDiscard: true, VerbStash: false, VerbUndo: false}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			for verb, want := range c.want {
				if got := VerbApplies(verb, c.row); got != want {
					t.Errorf("VerbApplies(%v) = %v, want %v (the footer draws %v)",
						verb, got, want, ChangesRowVerbs(c.row))
				}
			}
			// The footer and the keys read one answer, so what is drawn is what
			// runs.
			for _, name := range ChangesRowVerbs(c.row) {
				spent := false
				for _, verb := range selectionVerbOrder {
					if verbNames[verb] == name && VerbApplies(verb, c.row) {
						spent = true
					}
				}
				if !spent && name != VerbNameUndo {
					t.Errorf("the footer draws %q but no key runs it", name)
				}
			}
		})
	}
}

func TestVerbAppliesRejectsStashOnStagedRename(t *testing.T) {
	t.Parallel()
	row := FileRow(

		git.Entry{Path: "new.txt", OldPath: "old.txt", Index: git.Renamed},
		SectionStaged)

	if VerbApplies(VerbStash, row) {
		t.Error("stash applies on staged rename")
	}
}
