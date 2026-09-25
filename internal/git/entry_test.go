package git

import (
	"reflect"
	"testing"
)

func TestPathspecPrefixesLiteralSoMagicCharactersAreNotRead(t *testing.T) {
	t.Parallel()
	// A leading colon is pathspec magic. Without the prefix git rejects the
	// path even when the file exists.
	if got, want := pathspec(":magic.txt"), ":(literal):magic.txt"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := pathspec("plain.txt"), ":(literal)plain.txt"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenameEntryPassesBothPaths(t *testing.T) {
	t.Parallel()
	e := Entry{Path: "new.txt", OldPath: "old.txt", Index: Renamed}
	want := []string{":(literal)new.txt", ":(literal)old.txt"}
	if got := e.Pathspecs(); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestPlainEntryPassesOnePath(t *testing.T) {
	t.Parallel()
	e := Entry{Path: "a.txt", Worktree: Modified}
	want := []string{":(literal)a.txt"}
	if got := e.Pathspecs(); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestStagedRenameCannotBeStashed(t *testing.T) {
	t.Parallel()
	e := Entry{Path: "new.txt", OldPath: "old.txt", Index: Renamed}
	if e.CanStash() {
		t.Error("staged rename should not be stashable")
	}
	e = Entry{Path: "new.txt", OldPath: "old.txt", Index: Copied}
	if e.CanStash() {
		t.Error("staged copy should not be stashable")
	}
	e = Entry{Path: "new.txt", OldPath: "old.txt", Worktree: Renamed}
	if !e.CanStash() {
		t.Error("unstaged rename should be stashable")
	}
}

func TestStagedAndUnstagedReadTheirOwnColumn(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name             string
		e                Entry
		staged, unstaged bool
	}{
		{"index only", Entry{Index: Modified}, true, false},
		{"worktree only", Entry{Worktree: Modified}, false, true},
		{"both", Entry{Index: Modified, Worktree: Modified}, true, true},
		{"untracked", Entry{Worktree: Untracked}, false, true},
		{"unmerged", Entry{Index: Unmerged, Worktree: Unmerged}, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.e.IsStaged(); got != c.staged {
				t.Errorf("IsStaged() = %v, want %v", got, c.staged)
			}
			if got := c.e.IsUnstaged(); got != c.unstaged {
				t.Errorf("IsUnstaged() = %v, want %v", got, c.unstaged)
			}
		})
	}
}

// git marks an unmerged file with U on one side or on both: "AU" is added by
// us, "UD" is deleted by them, "UU" is both. A file the pane does not call
// conflicted is a file it offers to stage, and staging one side of a conflict
// records the conflict markers as the resolution.
func TestAFileIsConflictedWhenEitherSideIsUnmerged(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name           string
		index, worktre Kind
		want           bool
	}{
		{"UU · both sides", Unmerged, Unmerged, true},
		{"AU · added by us", Added, Unmerged, true},
		{"UD · deleted by them", Unmerged, Deleted, true},
		{"UA · added by them", Unmerged, Added, true},
		{"DU · deleted by us", Deleted, Unmerged, true},
		{"an ordinary change", Modified, Modified, false},
		{"nothing at all", Unchanged, Unchanged, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			e := Entry{Path: "a.txt", Index: c.index, Worktree: c.worktre}
			if got := e.IsConflicted(); got != c.want {
				t.Errorf("IsConflicted = %v, want %v", got, c.want)
			}
		})
	}
}
