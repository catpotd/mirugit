package state

import (
	"testing"
)

// A file staged and then edited again has two diffs: HEAD against the index and
// the index against the tree. Reading one says nothing about the other, and the
// two rows carry their own mark.
func TestReadingOneSideLeavesTheOtherUnread(t *testing.T) {
	t.Parallel()
	r := &ReadState{
		Finished:       map[string]bool{},
		Blocks:         map[string]bool{},
		stagedHashes:   map[string][]string{"a.go": {"staged-1"}},
		worktreeHashes: map[string][]string{"a.go": {"worktree-1"}},
	}
	// The hashes come from one git diff of one side, the way loadReadMarks
	// supplies them, so a merged HashesIn cannot pass by marking both.
	r.MarkFileIn(WorkingTree(SectionStaged), "a.go", []string{"staged-1"})

	if got := r.MarkIn(WorkingTree(SectionStaged), "a.go", r.HashesIn(WorkingTree(SectionStaged), "a.go")); got != Read {
		t.Errorf("staged side = %v, want Read", got)
	}
	if got := r.MarkIn(WorkingTree(SectionUnstaged), "a.go", r.HashesIn(WorkingTree(SectionUnstaged), "a.go")); got != Unread {
		t.Errorf("unstaged side = %v, want Unread", got)
	}
}

// The zero value must name no side. When it was SectionStaged, a DiffOpened
// that forgot the field silently loaded and marked the staged diff.
func TestTheZeroSectionHasNoHashes(t *testing.T) {
	t.Parallel()
	r := &ReadState{
		Finished:       map[string]bool{},
		Blocks:         map[string]bool{},
		stagedHashes:   map[string][]string{"a.go": {"staged-1"}},
		worktreeHashes: map[string][]string{"a.go": {"worktree-1"}},
	}
	if got := r.HashesIn(WorkingTree(SectionNone), "a.go"); len(got) != 0 {
		t.Errorf("got %v, want none", got)
	}
}

// git add moves the same lines from one diff to the other. The block hash is
// the content, so the row the edit lands on is already read and the reader is
// not asked to read it twice.
func TestStagingWhatWasReadKeepsItRead(t *testing.T) {
	t.Parallel()
	r := &ReadState{Finished: map[string]bool{}, Blocks: map[string]bool{}}
	r.worktreeHashes = map[string][]string{"a.go": {"h1"}}
	r.MarkFileIn(WorkingTree(SectionUnstaged), "a.go", r.HashesIn(WorkingTree(SectionUnstaged), "a.go"))

	r.worktreeHashes = map[string][]string{}
	r.stagedHashes = map[string][]string{"a.go": {"h1"}}
	if got := r.MarkIn(WorkingTree(SectionStaged), "a.go", r.HashesIn(WorkingTree(SectionStaged), "a.go")); got != Read {
		t.Errorf("got %v, want Read", got)
	}
}

// ● says the reader read this file and it changed since. A side they never
// opened has no block of theirs in it, so it is unread rather than changed.
func TestASideWithNoSeenBlockIsUnreadNotChanged(t *testing.T) {
	t.Parallel()
	r := &ReadState{Finished: map[string]bool{}, Blocks: map[string]bool{}}
	r.MarkFileIn(WorkingTree(SectionStaged), "a.go", []string{"staged-1"})

	if got := r.MarkIn(WorkingTree(SectionUnstaged), "a.go", []string{"worktree-1"}); got != Unread {
		t.Errorf("got %v, want Unread", got)
	}
	if got := r.MarkIn(WorkingTree(SectionStaged), "a.go", []string{"staged-1", "new-1"}); got != Changed {
		t.Errorf("a new block on the row that was read = %v, want Changed", got)
	}
}

func TestZeroValueMarkIsUnread(t *testing.T) {
	t.Parallel()
	var mark Mark
	if mark != Unread {
		t.Errorf("zero-value Mark = %v, want Unread", mark)
	}
}

// nil の ReadState は「まだ何も読んでいない」を答える。layout は interface 越しに
// 受け取るので、型付き nil でも map を参照して落ちてはいけない。
func TestNilReadStateAnswersUnread(t *testing.T) {
	t.Parallel()
	var r *ReadState
	if got := r.MarkIn(WorkingTree(SectionUnstaged), "a.go", []string{"h"}); got != Unread {
		t.Errorf("MarkIn = %v, want Unread", got)
	}
	if got := r.StashMark("abc"); got != Unread {
		t.Errorf("StashMark = %v, want Unread", got)
	}
	if got := r.WorktreeMark("/wt", "abc"); got != Unread {
		t.Errorf("WorktreeMark = %v, want Unread", got)
	}
	if r.BlockRead("a.go", "h") {
		t.Error("BlockRead = true, want false")
	}
	if got := r.HashesIn(WorkingTree(SectionUnstaged), "a.go"); got != nil {
		t.Errorf("HashesIn = %v, want nil", got)
	}
}

// Prune keeps a read mark whose path is still in the repository and drops the
// rest. It finds the path by undoing the key it was written under, and a key
// written by an older version carries a shape this no longer knows. Answering
// "found" with an empty path for those hands Prune a path to look up instead of
// the answer it asked for, which is that the key belongs to no row.
func TestAKeyThisDoesNotKnowNamesNoPath(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name     string
		key      string
		wantPath string
		wantOK   bool
	}{
		{"a staged file", "staged:a.txt", "a.txt", true},
		{"an unstaged file", "changes:a.txt", "a.txt", true},
		{"a file with no section", "none:a.txt", "a.txt", true},
		{"a file inside a commit", "commit:abc\x00a.txt", "a.txt", true},
		{"a file inside a commit's parent", "parent:abc\x00a.txt", "a.txt", true},
		{"a commit key with no path after the sha", "commit:abc", "", false},
		{"a key from before the sections were separated", "a.txt", "", false},
		{"a stash key, which Prune skips before it gets here", "stash:abc", "", false},
		{"nothing at all", "", "", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			path, ok := finishedPath(c.key)
			if path != c.wantPath || ok != c.wantOK {
				t.Errorf("finishedPath(%q) = %q, %v; want %q, %v",
					c.key, path, ok, c.wantPath, c.wantOK)
			}
		})
	}
}
