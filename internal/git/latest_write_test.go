package git

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The worktrees tab says how long ago the tree was last written to, and this
// picks the newest of the files that changed. Taking the oldest instead says a
// tree nobody has touched for a week was written to a week ago when one of its
// files was edited a minute before.
func TestTheLatestWriteIsTheNewestOfTheFilesThatChanged(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	when := func(name string, ago time.Duration) int64 {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		at := time.Now().Add(-ago).Truncate(time.Second)
		if err := os.Chtimes(path, at, at); err != nil {
			t.Fatal(err)
		}
		return at.Unix()
	}
	old := when("old.txt", time.Hour)
	newest := when("new.txt", time.Minute)
	if old >= newest {
		t.Fatalf("the two files carry %d and %d, so this proves nothing", old, newest)
	}

	for _, c := range []struct {
		name    string
		entries []Entry
		want    int64
		found   bool
	}{
		{"the newer one first", []Entry{{Path: "new.txt"}, {Path: "old.txt"}}, newest, true},
		{"the older one first", []Entry{{Path: "old.txt"}, {Path: "new.txt"}}, newest, true},
		{"one file", []Entry{{Path: "old.txt"}}, old, true},
		{"a file that is not there", []Entry{{Path: "gone.txt"}}, 0, false},
		{"nothing at all", nil, 0, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, found := latestWrite(dir, c.entries)
			if found != c.found {
				t.Fatalf("found = %v, want %v", found, c.found)
			}
			if found && got != c.want {
				t.Errorf("the latest write is %d, want %d", got, c.want)
			}
		})
	}
}
