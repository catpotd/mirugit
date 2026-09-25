package git

import "testing"

// The stashed tab draws the files in this order, and the row the reader opens
// is picked out of it by position. A path can arrive twice — tracked in the
// stash and untracked beside it — so which of the two comes first has to be the
// same answer every run.
func TestTheFilesOfAStashAreOrderedByPathAndTiesStayPut(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		in   []Entry
		want []string
	}{
		{"nothing", nil, nil},
		{"one file", []Entry{{Path: "a.txt"}}, []string{"a.txt"}},
		{"out of order", []Entry{{Path: "b.txt"}, {Path: "a.txt"}},
			[]string{"a.txt", "b.txt"}},
		{"one path twice, told apart by where it came from",
			[]Entry{{Path: "a.txt", OldPath: "z"}, {Path: "a.txt", OldPath: "m"}},
			[]string{"m", "z"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			files := append([]Entry(nil), c.in...)
			byPath(files)
			if len(files) != len(c.want) {
				t.Fatalf("ordered %d files, want %d", len(files), len(c.want))
			}
			for i := range files {
				got := files[i].Path
				if files[i].OldPath != "" {
					got = files[i].OldPath
				}
				if got != c.want[i] {
					t.Errorf("file %d is %q, want %q: %+v", i, got, c.want[i], files)
				}
			}
		})
	}
}

// The order must not depend on the order the two halves arrived in.
func TestTheStashFileOrderDoesNotDependOnHowTheyArrived(t *testing.T) {
	t.Parallel()
	forwards := []Entry{{Path: "a.txt", OldPath: "1"}, {Path: "a.txt", OldPath: "2"}, {Path: "b.txt"}}
	backwards := []Entry{{Path: "b.txt"}, {Path: "a.txt", OldPath: "2"}, {Path: "a.txt", OldPath: "1"}}
	byPath(forwards)
	byPath(backwards)
	for i := range forwards {
		if forwards[i] != backwards[i] {
			t.Fatalf("the two orders differ at %d: %+v and %+v", i, forwards, backwards)
		}
	}
}
