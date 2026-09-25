package git

import "testing"

// worktree list --porcelain writes a block per tree and separates them with a
// blank line, and the last block has no blank after it. Both endings have to
// close the block: a mutation that turned the blank-line case into its opposite
// survived the suite, which means the separator could have stopped working and
// no test would say so.
func TestEveryTreeInTheListIsRead(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		out  string
		want []linkedWorktree
	}{
		{
			name: "one tree, no trailing blank",
			out:  "worktree /repo\nHEAD abc\nbranch refs/heads/main\n",
			want: []linkedWorktree{{path: "/repo", branch: "main"}},
		},
		{
			name: "two trees separated by a blank",
			out: "worktree /repo\nHEAD abc\nbranch refs/heads/main\n\n" +
				"worktree /side\nHEAD def\nbranch refs/heads/topic\n",
			want: []linkedWorktree{
				{path: "/repo", branch: "main"},
				{path: "/side", branch: "topic"},
			},
		},
		{
			name: "a detached tree",
			out:  "worktree /repo\nHEAD abc\ndetached\n",
			want: []linkedWorktree{{path: "/repo", branch: "(detached)"}},
		},
		{
			name: "blocks that follow each other with no blank between",
			out:  "worktree /a\nbranch refs/heads/x\nworktree /b\nbranch refs/heads/y\n",
			want: []linkedWorktree{{path: "/a", branch: "x"}, {path: "/b", branch: "y"}},
		},
		{
			// A read cut short ends mid-block with no newline, and the loop's
			// blank-line case never fires. Every other case here ends with a
			// newline, so the close after the loop is only reached by this one.
			name: "a block that ends without a newline",
			out:  "worktree /repo\nbranch refs/heads/main",
			want: []linkedWorktree{{path: "/repo", branch: "main"}},
		},
		{
			name: "nothing at all",
			out:  "",
			want: nil,
		},
		{
			name: "blank lines and nothing else",
			out:  "\n\n\n",
			want: nil,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := parseWorktreeList([]byte(c.out))
			if len(got) != len(c.want) {
				t.Fatalf("read %d trees, want %d: %+v", len(got), len(c.want), got)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("tree %d = %+v, want %+v", i, got[i], c.want[i])
				}
			}
		})
	}
}

// The parser reads bytes git printed, and a read cut short prints a partial
// record. Two panics in parseStatus were found this way.
func FuzzParseWorktreeList(f *testing.F) {
	f.Add("worktree /repo\nHEAD abc\nbranch refs/heads/main\n")
	f.Add("worktree /a\n\nworktree /b\ndetached\n")
	f.Add("worktree ")
	f.Add("branch ")
	f.Add("\x00\n\x00")
	f.Fuzz(func(t *testing.T, out string) {
		for _, tree := range parseWorktreeList([]byte(out)) {
			if tree.path == "" {
				t.Errorf("a tree with no path was read from %q", out)
			}
		}
	})
}
