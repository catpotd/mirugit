package git

import (
	"reflect"
	"testing"
)

// git separates with NUL because it is the one byte a path cannot hold. Twelve
// places split it and each decided on its own whether an empty record counts.
// A mutation of one of those decisions survived the suite: it kept the empty
// record and dropped every real path, which turned a stash that collides with
// the working tree into one the pane said would apply cleanly.
func TestSplittingOnNUL(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name      string
		in        string
		records   []string
		pathsOnly []string
	}{
		{"one record", "a.txt\x00", []string{"a.txt"}, []string{"a.txt"}},
		{"two records", "a.txt\x00b.txt\x00",
			[]string{"a.txt", "b.txt"}, []string{"a.txt", "b.txt"}},
		{"an empty field between two", "a.txt\x00\x00b.txt\x00",
			[]string{"a.txt", "", "b.txt"}, []string{"a.txt", "b.txt"}},
		{"nothing at all", "", nil, nil},
		{"only the separator", "\x00", []string{""}, nil},
		{"no trailing separator", "a.txt", []string{"a.txt"}, []string{"a.txt"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			// Records keep the empty field: the caller reads them by position,
			// and dropping one moves every field after it.
			if got := splitNUL([]byte(c.in)); !reflect.DeepEqual(got, c.records) {
				t.Errorf("splitNUL = %q, want %q", got, c.records)
			}
			// A path list has no empty path to name, so an empty record is the
			// trailing separator or a short read.
			if got := splitNULPaths([]byte(c.in)); !reflect.DeepEqual(got, c.pathsOnly) {
				t.Errorf("splitNULPaths = %q, want %q", got, c.pathsOnly)
			}
		})
	}
}
