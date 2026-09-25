package state

import (
	"testing"

	"github.com/catpotd/mirugit/internal/git"
)

func TestRowsArriveInTheOrderTheTreeDrawsThem(t *testing.T) {
	t.Parallel()
	// git reports these in its own order. The tree groups by directory and
	// sorts the group names, so a cursor stepping through Rows would jump
	// around the screen unless Rows already carries the drawn order.
	s := Apply(State{}, StatusLoaded{Rows: []git.Entry{
		{Path: "app/a.txt"},
		{Path: "root.txt"},
		{Path: "app/b.txt"},
		{Path: "docs/c.md"},
	}})

	var got []string
	for _, r := range s.Rows {
		if r.Kind() != RowFile {
			continue
		}
		got = append(got, r.Entry().Path)
	}
	want := []string{"root.txt", "app/a.txt", "app/b.txt", "docs/c.md"}
	for i := range want {
		if i >= len(got) || got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestFilesInOneDirectoryKeepTheOrderGitReported(t *testing.T) {
	t.Parallel()
	s := Apply(State{}, StatusLoaded{Rows: []git.Entry{
		{Path: "app/z.txt"},
		{Path: "app/a.txt"},
	}})
	var firstFile int
	for i, r := range s.Rows {
		if r.Kind() == RowFile {
			firstFile = i
			break
		}
	}
	if s.Rows[firstFile].Entry().Path != "app/z.txt" {
		t.Errorf("sorting inside a directory would move a row under the pointer: %v", s.Rows)
	}
}

func TestChangesRowsIncludeEveryDirectoryBetweenRootAndFile(t *testing.T) {
	t.Parallel()
	s := Apply(State{}, StatusLoaded{Rows: []git.Entry{
		{Path: "AGENTS.md", Worktree: git.Modified},
		{Path: ".agents/rules/antigravity.md", Worktree: git.Modified},
		{Path: ".agents/skills/critique/SKILL.md", Worktree: git.Modified},
	}})

	var got []string
	for _, row := range s.Rows {
		switch row.Kind() {
		case RowDirectory:
			path := row.DirPath()
			if path == "" {
				path = "/"
			}
			got = append(got, "dir:"+path)
		case RowFile:
			got = append(got, "file:"+row.Path())
		case RowSectionHeading:
		case RowCommit, RowStash, RowWorktree:
			t.Fatalf("unexpected row kind %v", row.Kind())
		}
	}
	want := []string{
		"dir:/",
		"file:AGENTS.md",
		"dir:.agents/rules",
		"file:.agents/rules/antigravity.md",
		"dir:.agents/skills/critique",
		"file:.agents/skills/critique/SKILL.md",
	}
	if len(got) != len(want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("row %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestFileRowCountCountsARowPerSection(t *testing.T) {
	t.Parallel()
	entries := []git.Entry{
		{Path: "i.txt", Index: git.Modified, Worktree: git.Modified},
		{Path: "b.txt", Index: git.Untracked, Worktree: git.Untracked},
		{Path: "c.txt", Index: git.Unmerged, Worktree: git.Unmerged},
	}
	got := FileRowCount(entries)
	if got != 4 {
		t.Fatalf("FileRowCount = %d, want 4 (one row per section, not %d paths)", got, len(entries))
	}
}

func TestStagedRowsComeBeforeChangesRows(t *testing.T) {
	t.Parallel()
	s := Apply(State{}, StatusLoaded{Rows: []git.Entry{
		{Path: "a.txt", Worktree: git.Modified},
		{Path: "b.txt", Index: git.Modified, Worktree: git.Modified},
		{Path: "c.txt", Index: git.Modified},
	}})

	sawChanges := false
	for _, r := range s.Rows {
		if r.Section() == SectionUnstaged {
			sawChanges = true
		}
		if r.Section() == SectionStaged && sawChanges {
			t.Fatal("a staged row appeared after an unstaged row")
		}
	}
}

// A section heading is drawn only over the files it has. Drawn over none, it
// says a section is there with nothing in it, and the cursor can be put on a
// heading that names no change.
func TestASectionHeadingIsDrawnOnlyOverFilesItHas(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name     string
		entries  []git.Entry
		headings []Section
	}{
		{"only unstaged", []git.Entry{{Path: "a.txt", Worktree: git.Modified}},
			[]Section{SectionUnstaged}},
		{"only staged", []git.Entry{{Path: "a.txt", Index: git.Modified}},
			[]Section{SectionStaged}},
		{"both", []git.Entry{{Path: "a.txt", Index: git.Modified, Worktree: git.Modified}},
			[]Section{SectionStaged, SectionUnstaged}},
		{"nothing at all", nil, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			var got []Section
			for _, row := range displayOrder(c.entries) {
				if row.Kind() == RowSectionHeading {
					got = append(got, row.Section())
				}
			}
			if len(got) != len(c.headings) {
				t.Fatalf("headings = %v, want %v", got, c.headings)
			}
			for i := range got {
				if got[i] != c.headings[i] {
					t.Errorf("heading %d is %v, want %v", i, got[i], c.headings[i])
				}
			}
		})
	}
}
