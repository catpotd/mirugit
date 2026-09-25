package git

import (
	"os"
	"path/filepath"
	"testing"
)

// worktreeRestored answers whether the undiscard changed a path's bytes. A file
// that is not there reads as no bytes and so does an empty one, so the two are
// told apart by whether the read succeeded, not by what it returned.
//
// undiscardOutcome ors this with entryChanged, and a file appearing or going
// also changes its status, so the caller reaches the right answer either way.
// The predicate is asked here on its own: without the presence check it says a
// file it deleted was left alone, and a caller that does not or it with a
// status would act on that.
func TestAFileGoingAndAnEmptyFileAreNotTheSameBytes(t *testing.T) {
	t.Parallel()
	put := map[string]func(t *testing.T, path string){
		"not there": func(t *testing.T, path string) {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
		},
		"empty": func(t *testing.T, path string) {
			if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
				t.Fatal(err)
			}
		},
		"a line in it": func(t *testing.T, path string) {
			if err := os.WriteFile(path, []byte("x\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		},
	}

	for _, c := range []struct {
		before, after string
		want          bool
	}{
		{"not there", "not there", false},
		{"not there", "empty", true},
		{"not there", "a line in it", true},
		{"empty", "not there", true},
		{"empty", "empty", false},
		{"empty", "a line in it", true},
		{"a line in it", "not there", true},
		{"a line in it", "empty", true},
		{"a line in it", "a line in it", false},
	} {
		t.Run(c.before+" → "+c.after, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			path := filepath.Join(dir, "a.txt")
			put[c.before](t, path)
			before := worktreeBytesFor(dir, []string{"a.txt"})
			put[c.after](t, path)

			if got := worktreeRestored(dir, "a.txt", before); got != c.want {
				t.Errorf("changed = %v, want %v", got, c.want)
			}
		})
	}
}
