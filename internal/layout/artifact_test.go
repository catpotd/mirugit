package layout

import (
	"strings"
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// artifactWant is the frozen appearance of the whole changes pane, taken from
// Pane itself. Every column, glyph, and gap below is the answer; a change to any
// of them is a change to what the reader sees, so it has to be made here on
// purpose.
//
// Trailing spaces are stripped on both sides before comparing, because a
// terminal shows nothing there either. The cursor row spells its verbs as
// "s stage" rather than a colored field so the offer survives NO_COLOR. The
// diff header says "esc close" because esc is the key that closes it.
func artifactWant() []string {
	return []string{
		" changes 4 ·                                                          main ·",
		"─━━━━━━━━━━━─────────────────────────────────────────────────────────────────",
		"[ commit message                                                            ]",
		"3 unread · 4 files · +176 −3                                  [commit 1 file]",
		"",
		"      staged · 1 file · +7 −1",
		"            ▾ app/Sources/Infrastructure                                    1",
		"              M  Dependencies.swift                                 +7     −1",
		"",
		"      changes · 3 files · +169 −2",
		"            ▾ app/Sources/Infrastructure                                    3",
		"    ·         M  SettingsView.swift                                +45     −1",
		"▌[ ]·         M  SyncOwnershipTests.swift       s stage  x discard  z stash",
		"    ·         ?  Localizable.xcstrings                            +119     −0",
		"─────────────────────────────────────────────────────────────────────────────",
		"SyncOwnershipTests.swift                   block 1/6 · 6 unread     esc close",
		"",
		"    ▌  block 1/6                                         s stage        +2 −0",
		"   1    1  a",
		"        2 +b",
		"        3 +c",
		"",
		"       block 2/6                                                        +5 −1",
		"   9   10  c",
		"       11 +d",
		"",
		"       block 3/6                                                        +0 −0",
		"",
		"       block 4/6                                                        +0 −0",
		"",
		"       block 5/6                                                        +0 −0",
		"",
		"       block 6/6                                                        +0 −0",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"─────────────────────────────────────────────────────────────────────────────",
		"           space select · s stage · x discard · z stash · d diff · r read   ?",
	}
}

func artifactState() state.State {
	entries := []git.Entry{
		{Path: "app/Sources/Infrastructure/Dependencies.swift", Index: git.Modified, IndexCount: git.Count{Added: 7, Deleted: 1}},
		{Path: "app/Sources/Infrastructure/SettingsView.swift", Worktree: git.Modified, WorktreeCount: git.Count{Added: 45, Deleted: 1}},
		{Path: "app/Sources/Infrastructure/SyncOwnershipTests.swift", Worktree: git.Modified, WorktreeCount: git.Count{Added: 5, Deleted: 1}},
		{Path: "app/Sources/Infrastructure/Localizable.xcstrings", Worktree: git.Untracked, WorktreeCount: git.Count{Added: 119}},
	}
	s := state.Apply(state.State{Width: paneWidth, Height: 53}, state.StatusLoaded{Rows: entries, Head: git.Head{Branch: "main"}})
	for i, row := range s.Rows {
		if row.Kind() == state.RowFile && row.Path() == "app/Sources/Infrastructure/SyncOwnershipTests.swift" {
			s.Cursor = i
			break
		}
	}
	s.Open = state.OpenDiff{Path: "app/Sources/Infrastructure/SyncOwnershipTests.swift", Diff: artifactDiff()}
	return s
}
func artifactDiff() git.FileDiff {
	return git.FileDiff{
		Path: "app/Sources/Infrastructure/SyncOwnershipTests.swift",
		Blocks: []git.Block{
			{Header: "@@ -1,1 +1,3 @@", OldStart: 1, NewStart: 1, Lines: []string{" a", "+b", "+c"}, Added: 2, Hash: "h1"},
			{Header: "@@ -9,1 +10,2 @@", OldStart: 9, NewStart: 10, Lines: []string{" c", "+d"}, Added: 5, Deleted: 1, Hash: "h2"},
			{Header: "@@ -20,1 +21,1 @@", Hash: "h3"},
			{Header: "@@ -30,1 +31,1 @@", Hash: "h4"},
			{Header: "@@ -40,1 +41,1 @@", Hash: "h5"},
			{Header: "@@ -50,1 +51,1 @@", Hash: "h6"},
		},
	}
}

func TestArtifactMainDrawing(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	got := Pane(artifactState(), artifactMarks{}, w).Lines
	want := artifactWant()
	if len(got) != len(want) {
		t.Fatalf("got %d lines, want %d:\n%q", len(got), len(want), got)
	}
	for i := range want {
		if w.Of(got[i]) != paneWidth {
			t.Errorf("line %d: %d cells, want %d: %q", i, w.Of(got[i]), paneWidth, got[i])
		}
		if strings.TrimRight(got[i], " ") != strings.TrimRight(want[i], " ") {
			t.Errorf("line %d:\n got %q\nwant %q", i, got[i], want[i])
		}
	}
}

// TestArtifactStashedDrawing is the frozen appearance of the stashed tab.
// A conflicting stash offers no restore: restoring it would leave an unmerged
// tree this program cannot finish, and the rule for a verb line is that what is
// shown can be pressed. StashVerbs withholds it and a separate test covers that.
func TestArtifactStashedDrawing(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	stashedHeading, _ := w.Heading("stashed", 2, 0, 0, state.SectionNone, "", "", paneWidth)
	got := make([]string, 0, 3)
	got = append(got, stashedHeading)
	// The drawing is what this pins, so the rows are built here rather than
	// through the pane: whether a file collides is the stash's answer and has
	// its own test.
	for _, r := range []Row{
		{
			Entry: git.Entry{Path: "internal/collect/aggregate.go", Worktree: git.Modified},
			Count: git.Count{Added: 38, Deleted: 4},
			State: rowState{Indent: 1, Read: state.Read, NoSelect: true,
				Conflicts: true, FullPath: true},
		},
		{
			Entry: git.Entry{Path: "internal/collect/aggregate_test.go", Worktree: git.Modified},
			Count: git.Count{Added: 112},
			State: rowState{Indent: 1, Read: state.Read, NoSelect: true, FullPath: true},
		},
	} {
		line, _ := w.FileRow(r, paneWidth)
		got = append(got, line)
	}
	// The bar divides changed lines by fifteen. No divisor fits every screen:
	// searching 10 to 29 leaves only fifteen, which matches all four bars on the
	// changes pane and neither of the two here (42 lines drawn as 2 cells wants
	// 17 or more, 112 drawn as 8 wants 15 or less). Four matches beat two.
	//
	// "conflicts" sits right-aligned just left of the figures block so neither
	// the label nor +N moves when the name or the bar grows.
	want := []string{
		"      stashed · 2",
		"        M  internal/collect/aggregate.go               conflicts   +38     −4",
		"        M  internal/collect/aggregate_test.go                     +112     −0",
	}
	for i := range want {
		if w.Of(got[i]) != paneWidth {
			t.Errorf("line %d: %d cells, want %d: %q", i, w.Of(got[i]), paneWidth, got[i])
		}
		if strings.TrimRight(got[i], " ") != strings.TrimRight(want[i], " ") {
			t.Errorf("line %d:\n got %q\nwant %q", i, got[i], want[i])
		}
	}
}

// The worktrees tab as the design drawing shows it.
func TestArtifactWorktreesDrawing(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	worktreesHeading, _ := w.Heading("worktrees", 2, 0, 0, state.SectionNone, "", "vs main", paneWidth)
	got := make([]string, 0, 2)
	got = append(got, worktreesHeading)
	line, _ := w.FileRow(nestedFileRow(git.Entry{
		Path: "internal/sync/engine.go", Worktree: git.Modified,
		WorktreeCount: git.Count{Added: 7, Deleted: 2},
	}), paneWidth)
	got = append(got, line)
	want := []string{
		"      worktrees · 2                                                   vs main",
		"        M  internal/sync/engine.go                                  +7     −2",
	}
	for i := range want {
		if w.Of(got[i]) != paneWidth {
			t.Errorf("line %d: %d cells, want %d: %q", i, w.Of(got[i]), paneWidth, got[i])
		}
		if strings.TrimRight(got[i], " ") != strings.TrimRight(want[i], " ") {
			t.Errorf("line %d:\n got %q\nwant %q", i, got[i], want[i])
		}
	}
}
