package git

import "testing"

// The worktrees tab shows how far each branch is from the base and which commit
// it points at, and this reads that out of for-each-ref. Nothing tested the
// reading: a mutation that dropped every branch with a name, keeping only the
// nameless ones, survived the suite.
func TestBranchRefsReadsTheDistanceAndTheCommit(t *testing.T) {
	t.Parallel()
	const out = "main\x00\x00aaa\x00/repo\n" +
		"feature/x\x002\t3\x00bbb\x00\n"

	refs := parseBranchRefs([]byte(out))
	if len(refs) != 2 {
		t.Fatalf("got %v, want two branches", refs)
	}
	if got := refs["main"]; got != (branchRef{sha: "aaa", ok: true}) {
		t.Errorf("main is %+v", got)
	}
	if got := refs["feature/x"]; got != (branchRef{sha: "bbb", ahead: 2, behind: 3, ok: true}) {
		t.Errorf("feature/x is %+v", got)
	}
}

// Three fields is the fewest a line can carry and still name a branch, a
// distance and a commit; the fourth says which worktree holds it and is what a
// truncated read loses first. Both tests around this one use four fields or two,
// so the bound between them is not drawn by either.
func TestABranchRefLineEndingAfterTheCommitIsStillRead(t *testing.T) {
	t.Parallel()
	refs := parseBranchRefs([]byte("main\x002\t3\x00aaa\n"))
	if len(refs) != 1 {
		t.Fatalf("got %v, want the one branch", refs)
	}
	if got := refs["main"]; got != (branchRef{sha: "aaa", ahead: 2, behind: 3, ok: true}) {
		t.Errorf("main is %+v", got)
	}
}

// Every line here was written by git, so a line this rejects is a truncated or
// unexpected read rather than a branch. None of them may reach the map.
func TestBranchRefsDropsALineItCannotRead(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		out  string
	}{
		{"a line with two fields has no commit", "main\x002\t3\n"},
		{"a line with no separator at all", "main\n"},
		{"a nameless branch", "\x002\t3\x00aaa\x00\n"},
		{"a blank line between records", "\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if refs := parseBranchRefs([]byte(c.out)); len(refs) != 0 {
				t.Errorf("got %v, want nothing", refs)
			}
		})
	}
}
