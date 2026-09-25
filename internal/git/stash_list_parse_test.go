package git

import "testing"

// stash list prints four fields per entry with a NUL after each, so a whole
// list ends with a trailing empty record. The loop reads three past its own
// position: a list that ends one field short would reach past the end, and the
// bound that stops it is one the tests never drew — every list they parse is
// whole.
func TestEveryWholeStashEntryIsReadAndAShortOneIsNot(t *testing.T) {
	t.Parallel()
	const entry = "abc\x00stash@{0}\x00WIP on main: x\x001700000000\x00"

	for _, c := range []struct {
		name string
		out  string
		want []string
	}{
		{"one entry", entry, []string{"stash@{0}"}},
		{
			name: "two entries",
			out:  entry + "def\x00stash@{1}\x00On main: y\x001700000001\x00",
			want: []string{"stash@{0}", "stash@{1}"},
		},
		{"nothing at all", "", nil},
		{"a read that ends after the subject", "abc\x00stash@{0}\x00WIP on main: x\x00", nil},
		{"a read that ends after the ref", "abc\x00stash@{0}\x00", nil},
		{"a read that ends after the sha", "abc\x00", nil},
		{
			name: "a whole entry and then a short one",
			out:  entry + "def\x00stash@{1}\x00On main: y\x00",
			want: []string{"stash@{0}"},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			rows, err := parseStashList([]byte(c.out))
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) != len(c.want) {
				t.Fatalf("read %d stashes, want %d: %+v", len(rows), len(c.want), rows)
			}
			for i := range rows {
				if rows[i].Ref != c.want[i] {
					t.Errorf("stash %d is %q, want %q", i, rows[i].Ref, c.want[i])
				}
			}
		})
	}
}

// The records come from a read that can be cut anywhere.
func FuzzParseStashList(f *testing.F) {
	f.Add("abc\x00stash@{0}\x00WIP on main: x\x001700000000\x00")
	f.Add("abc\x00stash@{0}\x00")
	f.Add("\x00\x00\x00\x00")
	f.Add("abc\x00stash@{0}\x00s\x00not-a-number\x00")
	f.Fuzz(func(t *testing.T, out string) {
		rows, err := parseStashList([]byte(out))
		if err != nil {
			return
		}
		for _, row := range rows {
			if row.SHA == "" {
				t.Errorf("a stash with no sha was read from %q", out)
			}
		}
	})
}
