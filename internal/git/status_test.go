package git

import "testing"

func rec(fields ...string) string {
	s := ""
	for _, f := range fields {
		s += f + "\x00"
	}
	return s
}

func TestParseStatusReadsARenameAsOneEntryWithBothPaths(t *testing.T) {
	t.Parallel()
	// In -z output a rename writes the new path, then the old path, as two
	// separate NUL-terminated fields. Splitting naively yields a phantom entry.
	out := rec(
		"# branch.head main",
		"2 R. N... 100644 100644 100644 aaa bbb R100 new name.txt",
		"old name.txt",
		"1 .M N... 100644 100644 100644 ccc ddd after.txt",
	)
	entries, _, err := parseStatus([]byte(out))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d: %+v", len(entries), entries)
	}
	if entries[0].Path != "new name.txt" || entries[0].OldPath != "old name.txt" {
		t.Errorf("rename paths wrong: %+v", entries[0])
	}
	if entries[1].Path != "after.txt" {
		t.Errorf("the field after a rename was consumed: %+v", entries[1])
	}
}

func TestParseStatusKeepsAPathWithASpace(t *testing.T) {
	t.Parallel()
	// Taking the text after the last space would return "ace.txt", and that
	// string as a pathspec can match a different file that exists.
	out := rec(
		"# branch.head main",
		"1 .M N... 100644 100644 100644 aaa bbb sp ace.txt",
		"u UU N... 100644 100644 100644 100644 aaa bbb ccc con flict.txt",
	)
	entries, _, err := parseStatus([]byte(out))
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []string{"sp ace.txt", "con flict.txt"} {
		if entries[i].Path != want {
			t.Errorf("[%d] got %q, want %q", i, entries[i].Path, want)
		}
	}
}

func TestParseStatusReadsACopyLikeARename(t *testing.T) {
	t.Parallel()
	out := rec(
		"# branch.head main",
		"2 C. N... 100644 100644 100644 aaa bbb C097 copy.txt",
		"source.txt",
	)
	entries, _, err := parseStatus([]byte(out))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].OldPath != "source.txt" || entries[0].Index != Copied {
		t.Fatalf("got %+v", entries)
	}
}

func TestParseStatusMarksASubmodule(t *testing.T) {
	t.Parallel()
	out := rec(
		"# branch.head main",
		"1 .M SC.. 160000 160000 160000 aaa bbb sub",
	)
	entries, _, err := parseStatus([]byte(out))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || !entries[0].Submodule {
		t.Fatalf("want a submodule, got %+v", entries)
	}
}

func TestParseStatusReadsAnUnmergedEntry(t *testing.T) {
	t.Parallel()
	out := rec(
		"# branch.head main",
		"u UU N... 100644 100644 100644 100644 aaa bbb ccc both.txt",
	)
	entries, _, err := parseStatus([]byte(out))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || !entries[0].IsConflicted() || entries[0].Path != "both.txt" {
		t.Fatalf("got %+v", entries)
	}
}

func TestParseStatusReadsTheHead(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want Head
	}{
		{"a branch with an upstream",
			rec("# branch.head feature", "# branch.upstream origin/feature", "# branch.ab +2 -3"),
			Head{Branch: "feature", HasUpstream: true, Ahead: 2, Behind: 3}},
		{"detached",
			rec("# branch.head (detached)"),
			Head{Branch: "(detached)", Detached: true}},
		{"no commits yet",
			rec("# branch.oid (initial)", "# branch.head main"),
			Head{Branch: "main", Initial: true}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, got, err := parseStatus([]byte(c.in))
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Errorf("got %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestParseStatusKeepsAPathThatBeginsWithAColon(t *testing.T) {
	t.Parallel()
	out := rec("# branch.head main", "1 .M N... 100644 100644 100644 aaa bbb :magic.txt")
	entries, _, err := parseStatus([]byte(out))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Path != ":magic.txt" {
		t.Fatalf("got %+v", entries)
	}
}

// A rename is two records: the one that names the file now, and the one that
// names where it came from. A read cut between them leaves the first with
// nothing after it, and reaching for the second one then reads past the end of
// what git wrote.
func TestARenameCutOffFromItsOldPathIsStillRead(t *testing.T) {
	t.Parallel()
	const rename = "2 R. N... 100644 100644 100644 aaaa bbbb R100 new.txt"
	for _, c := range []struct {
		name string
		out  string
		want []Entry
	}{
		{"a whole rename", rename + "\x00old.txt\x00",
			[]Entry{{Path: "new.txt", OldPath: "old.txt", Index: Renamed}}},
		{"a rename with nothing after it", rename + "\x00",
			[]Entry{{Path: "new.txt", Index: Renamed}}},
		{"a rename with an empty old path", rename + "\x00\x00",
			[]Entry{{Path: "new.txt", Index: Renamed}}},
		{"a file, then a rename cut off",
			"1 M. N... 100644 100644 100644 aaaa bbbb a.txt\x00" + rename + "\x00",
			[]Entry{{Path: "a.txt", Index: Modified},
				{Path: "new.txt", Index: Renamed}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			entries, _, err := parseStatus([]byte(c.out))
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != len(c.want) {
				t.Fatalf("read %d entries, want %d: %+v", len(entries), len(c.want), entries)
			}
			for i := range entries {
				if entries[i].Path != c.want[i].Path ||
					entries[i].OldPath != c.want[i].OldPath {
					t.Errorf("entry %d is %q from %q, want %q from %q", i,
						entries[i].Path, entries[i].OldPath,
						c.want[i].Path, c.want[i].OldPath)
				}
			}
		})
	}
}
