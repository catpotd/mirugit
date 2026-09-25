package git

import "testing"

// The diff of a stash prints a status and a path per file, with a NUL after
// each. The status is read by its first character, so a pair whose status
// arrived empty has nothing to read: keeping it reaches past the end of an
// empty string. A read can be cut anywhere, and a cut leaves exactly that.
func TestAStashFileNeedsBothItsStatusAndItsPath(t *testing.T) {
	t.Parallel()
	counts := map[string]Count{"a.txt": {Added: 3, Deleted: 1}}

	for _, c := range []struct {
		name string
		out  string
		want []string
	}{
		{"one file", "M\x00a.txt\x00", []string{"a.txt"}},
		{"two files", "M\x00a.txt\x00A\x00b.txt\x00", []string{"a.txt", "b.txt"}},
		{"nothing at all", "", nil},
		{"a status with no path after it", "M\x00", nil},
		{"a pair whose status is empty", "\x00a.txt\x00", nil},
		{"a pair whose path is empty", "M\x00\x00", nil},
		{"a whole pair and then an empty status", "M\x00a.txt\x00\x00b.txt\x00",
			[]string{"a.txt"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			files := parseStashTrackedFiles([]byte(c.out), counts)
			if len(files) != len(c.want) {
				t.Fatalf("read %d files, want %d: %+v", len(files), len(c.want), files)
			}
			for i := range files {
				if files[i].Path != c.want[i] {
					t.Errorf("file %d is %q, want %q", i, files[i].Path, c.want[i])
				}
			}
		})
	}

	// The counts come from a second command, and a file reaches its own by path.
	files := parseStashTrackedFiles([]byte("M\x00a.txt\x00"), counts)
	if len(files) != 1 {
		t.Fatalf("read %d files, want 1", len(files))
	}
	if files[0].WorktreeCount != (Count{Added: 3, Deleted: 1}) {
		t.Errorf("a.txt carries %+v, want 3 added and 1 deleted", files[0].WorktreeCount)
	}
	if files[0].Worktree != Modified {
		t.Errorf("a.txt is %q, want the status the diff printed", files[0].Worktree)
	}
}

// The records come from a read that can be cut anywhere.
func FuzzParseStashTrackedFiles(f *testing.F) {
	f.Add("M\x00a.txt\x00")
	f.Add("M\x00a.txt\x00A\x00b.txt\x00")
	f.Add("\x00\x00")
	f.Add("M\x00")
	f.Add("")
	f.Fuzz(func(t *testing.T, out string) {
		for _, file := range parseStashTrackedFiles([]byte(out), nil) {
			if file.Path == "" {
				t.Errorf("a file with no path was read from %q", out)
			}
			if file.Worktree == 0 {
				t.Errorf("a file with no status was read from %q", out)
			}
		}
	})
}
