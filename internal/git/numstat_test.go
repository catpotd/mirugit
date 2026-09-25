package git

import "testing"

func TestParseNumstatReadsBinaryAsBinaryNotZero(t *testing.T) {
	t.Parallel()
	// A binary file reports dashes. Reading them as zero would draw an empty
	// bar next to a file that changed entirely.
	out := "12\t3\ta.txt\x00-\t-\timg.png\x00"
	got := parseNumstat([]byte(out))
	if c := got["a.txt"]; c.Added != 12 || c.Deleted != 3 || c.Binary {
		t.Errorf("a.txt = %+v", c)
	}
	if c := got["img.png"]; !c.Binary {
		t.Errorf("img.png = %+v, want Binary", c)
	}
}

func TestParseNumstatReadsARenameFromItsThreeFields(t *testing.T) {
	t.Parallel()
	// With -z a rename writes an empty path field, then old, then new.
	out := "0\t0\t\x00old.txt\x00new.txt\x00"
	got := parseNumstat([]byte(out))
	if _, ok := got["new.txt"]; !ok {
		t.Fatalf("want the new path as the key, got %+v", got)
	}
	if _, ok := got["old.txt"]; ok {
		t.Error("the old path should not be a key of its own")
	}
}

// A rename is written as three fields: the counts with an empty path, then the
// old name, then the new one. A read cut between them leaves the counts with
// one name after them or none, and reaching for the third one then reads past
// the end of what git wrote.
func TestParseNumstatSurvivesARenameCutShort(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		out  string
		want string
	}{
		{"a whole rename", "0\t0\t\x00old.txt\x00new.txt\x00", "new.txt"},
		{"cut after the old name", "0\t0\t\x00old.txt\x00", ""},
		{"cut after the counts", "0\t0\t\x00", ""},
		{"a whole file, then a rename cut after the old name",
			"3\t1\ta.txt\x000\t0\t\x00old.txt\x00", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := parseNumstat([]byte(c.out))
			if c.want == "" {
				if _, ok := got["old.txt"]; ok {
					t.Errorf("the old name became a key of its own: %+v", got)
				}
				if _, ok := got["new.txt"]; ok {
					t.Errorf("a name that was cut off became a key: %+v", got)
				}
				return
			}
			if _, ok := got[c.want]; !ok {
				t.Errorf("want %q as a key, got %+v", c.want, got)
			}
		})
	}

	// The file before the cut rename is still read, or the answers above would
	// hold for a parser that gave up at the first record.
	got := parseNumstat([]byte("3\t1\ta.txt\x000\t0\t\x00old.txt\x00"))
	if c := got["a.txt"]; c.Added != 3 || c.Deleted != 1 {
		t.Errorf("a.txt = %+v, want 3 added and 1 deleted", c)
	}
}
