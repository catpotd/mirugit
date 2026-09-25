package git

import "testing"

// diff-tree --name-status -z writes a status field and then a path, and a
// rename writes the old path and then the new one. The loop reads one field
// past its own position, two for a rename, so a read cut short leaves a status
// with no path after it. Both bounds turn that away.
func TestACommitFileNeedsItsPathAndARenameNeedsTwo(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		out  string
		want []string
		olds []string
	}{
		{"one modified file", "M\x00a.txt\x00", []string{"a.txt"}, []string{""}},
		{
			name: "two files",
			out:  "M\x00a.txt\x00A\x00b.txt\x00",
			want: []string{"a.txt", "b.txt"}, olds: []string{"", ""},
		},
		{
			name: "a rename",
			out:  "R100\x00old.txt\x00new.txt\x00",
			want: []string{"new.txt"}, olds: []string{"old.txt"},
		},
		{"a status with no path after it", "M\x00", nil, nil},
		// git writes a name after every status, so an empty field there is a
		// read cut short. Taken as a file it becomes a row with no name, which
		// the reader can put the cursor on and press a verb at.
		{"a status with an empty path after it", "M\x00\x00", nil, nil},
		{"a rename whose new path is empty", "R100\x00old.txt\x00\x00", nil, nil},
		{"a rename with only its old path", "R100\x00old.txt\x00", nil, nil},
		{
			name: "a whole file and then a status with no path",
			out:  "M\x00a.txt\x00A\x00",
			want: []string{"a.txt"}, olds: []string{""},
		},
		{"nothing at all", "", nil, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := parseCommitFiles([]byte(c.out), nil)
			if len(got) != len(c.want) {
				t.Fatalf("read %d files, want %d: %+v", len(got), len(c.want), got)
			}
			for i := range got {
				if got[i].Path != c.want[i] {
					t.Errorf("file %d is %q, want %q", i, got[i].Path, c.want[i])
				}
				if got[i].OldPath != c.olds[i] {
					t.Errorf("file %d came from %q, want %q", i, got[i].OldPath, c.olds[i])
				}
			}
		})
	}
}

// The fields come from a read that can be cut anywhere.
func FuzzParseCommitFiles(f *testing.F) {
	f.Add("M\x00a.txt\x00")
	f.Add("R100\x00old.txt\x00new.txt\x00")
	f.Add("M\x00")
	f.Add("\x00\x00\x00")
	f.Fuzz(func(t *testing.T, out string) {
		for _, e := range parseCommitFiles([]byte(out), nil) {
			if e.Path == "" {
				t.Errorf("a file with no path was read from %q", out)
			}
		}
	})
}
