package git

import "testing"

// The listing reaches this from git, so a short record means a truncated read,
// and a mutation of the field-count guard survived the suite: nothing fed it a
// record with fewer than three fields, and valueOf reads fields[2].
func TestTreeEntriesKeepsTheBlobsAndSurvivesAShortRecord(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		out  string
		want map[string]string
	}{
		{"a blob is kept with its mode and sha",
			"100644 blob aaa\tapp/a.txt\x00",
			map[string]string{"app/a.txt": "100644 aaa"}},
		{"a tree is not a file to compare",
			"040000 tree bbb\tapp\x00100644 blob aaa\tapp/a.txt\x00",
			map[string]string{"app/a.txt": "100644 aaa"}},
		{"a record with two fields is dropped, not read",
			"100644 blob\tapp/a.txt\x00",
			map[string]string{}},
		{"a record with no tab is dropped",
			"100644 blob aaa\x00",
			map[string]string{}},
		{"a path holding a space stays whole",
			"100644 blob aaa\tapp/a file.txt\x00",
			map[string]string{"app/a file.txt": "100644 aaa"}},
		{"nothing listed",
			"",
			map[string]string{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			sameEntries(t, parseTabbedEntries([]byte(c.out), treeBlob), c.want)
		})
	}
}

func TestIndexEntriesKeepsStageZeroAndSurvivesAShortRecord(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		out  string
		want map[string]string
	}{
		{"stage 0 is the entry of a file not in a conflict",
			"100644 aaa 0\tapp/a.txt\x00",
			map[string]string{"app/a.txt": "100644 aaa"}},
		{"the three stages of a conflict are all dropped",
			"100644 aaa 1\tc.txt\x00100644 bbb 2\tc.txt\x00100644 ccc 3\tc.txt\x00",
			map[string]string{}},
		{"a record with two fields is dropped, not read",
			"100644 aaa\tapp/a.txt\x00",
			map[string]string{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			sameEntries(t, parseTabbedEntries([]byte(c.out), indexStageZero), c.want)
		})
	}
}

func sameEntries(t *testing.T, got, want map[string]string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for path, value := range want {
		if got[path] != value {
			t.Errorf("%q is %q, want %q", path, got[path], value)
		}
	}
}
