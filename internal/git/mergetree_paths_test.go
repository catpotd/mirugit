package git

import (
	"reflect"
	"testing"
)

// merge-tree prints one line per unmerged stage, and a stage of 0 means the
// path merged. Reading a stage-0 line as a conflict turns a stash that applies
// cleanly into one the pane says cannot be restored, and the reader is offered
// branch instead of restore for a stash with nothing wrong with it.
//
// A mutation that made the stage test stop rejecting stage 0 survived the whole
// suite: nothing fed this a merged line.
func TestMergeTreeReadsOnlyTheUnmergedStages(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		text string
		want []string
	}{
		{"a merged path is not a conflict",
			"100644 aaaaaaa 0\tapp/a.txt\n",
			nil},
		{"the three stages of one conflict name it once",
			"100644 aaa 1\tc.txt\n100644 bbb 2\tc.txt\n100644 ccc 3\tc.txt\n",
			[]string{"c.txt"}},
		{"a merged path beside a conflicted one",
			"100644 aaa 0\tclean.txt\n100644 bbb 2\tc.txt\n",
			[]string{"c.txt"}},
		{"a line with no tab holds no path",
			"100644 aaa 2\n",
			nil},
		{"a mode that is not six characters is not a stage line",
			"Auto-merging app/a.txt\n",
			nil},
		{"nothing printed",
			"",
			nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			paths, seen := mergeTreeConflictPaths(c.text)
			if !reflect.DeepEqual(paths, c.want) {
				t.Errorf("paths = %v, want %v", paths, c.want)
			}
			for _, p := range c.want {
				if !seen[p] {
					t.Errorf("%q is missing from seen, so the untracked and edited "+
						"paths appended after it would list it twice", p)
				}
			}
			if len(seen) != len(c.want) {
				t.Errorf("seen = %v, want the %d conflicted paths", seen, len(c.want))
			}
		})
	}
}

// The paths a stash would collide on come from three reads, and a file can
// appear in more than one of them: a path that is both untracked here and
// edited here is one file, not two. The row names them, and a name drawn twice
// reads as two files with the same name.
func TestTheCollidingPathsAreNamedOnce(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name  string
		seen  []string
		first []string
		more  []string
		want  []string
	}{
		{"nothing yet", nil, nil, []string{"a.txt", "b.txt"}, []string{"a.txt", "b.txt"}},
		{"one already named", []string{"a.txt"}, []string{"a.txt"},
			[]string{"a.txt", "b.txt"}, []string{"a.txt", "b.txt"}},
		{"the same path twice in one read", nil, nil,
			[]string{"a.txt", "a.txt"}, []string{"a.txt"}},
		{"nothing to add", []string{"a.txt"}, []string{"a.txt"}, nil, []string{"a.txt"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			seen := map[string]bool{}
			for _, p := range c.seen {
				seen[p] = true
			}
			got := appendConflictPaths(seen, append([]string{}, c.first...), c.more)
			if len(got) != len(c.want) {
				t.Fatalf("named %v, want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("path %d is %q, want %q", i, got[i], c.want[i])
				}
			}
		})
	}
}

// merge-tree prints one line per unmerged stage: a six-character mode, the
// object id, then the stage and the path with a tab between them. A line that
// does not have that shape is not a conflict record — it is the prose git
// writes around them — and reading one as a path names a file the reader does
// not have.
func TestOnlyAWholeConflictRecordNamesAPath(t *testing.T) {
	t.Parallel()
	const whole = "100644 1111111111111111111111111111111111111111 1\ta.txt"
	for _, c := range []struct {
		name string
		text string
		want []string
	}{
		{"one record", whole, []string{"a.txt"}},
		{"a record and prose around it",
			"Auto-merging a.txt\n" + whole + "\nCONFLICT (content): x", []string{"a.txt"}},
		{"a mode of the wrong length",
			"10064 1111111111111111111111111111111111111111 1\ta.txt", nil},
		{"a line with no space at all", "nospace", nil},
		{"a line with no tab",
			"100644 1111111111111111111111111111111111111111 1 a.txt", nil},
		{"stage zero, which is merged",
			"100644 1111111111111111111111111111111111111111 0\ta.txt", nil},
		{"an empty path",
			"100644 1111111111111111111111111111111111111111 1\t", nil},
		{"nothing at all", "", nil},
		{"the same path in three stages",
			whole + "\n" +
				"100644 2222222222222222222222222222222222222222 2\ta.txt\n" +
				"100644 3333333333333333333333333333333333333333 3\ta.txt",
			[]string{"a.txt"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, _ := mergeTreeConflictPaths(c.text)
			if len(got) != len(c.want) {
				t.Fatalf("named %v, want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("path %d is %q, want %q", i, got[i], c.want[i])
				}
			}
		})
	}
}
