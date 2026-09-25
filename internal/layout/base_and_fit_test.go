package layout

import (
	"testing"

	"github.com/catpotd/mirugit/internal/state"
)

// base names the file at the end of a path, and the separator can be the first
// character: "/a" is a file called a at the root. A bound that excludes that
// position hands back the whole path, and the diff header then reads "/a"
// where every other row reads a name.
func TestBaseNamesTheFileAfterTheLastSeparator(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		in, want string
	}{
		{"a.txt", "a.txt"},
		{"dir/a.txt", "a.txt"},
		{"one/two/a.txt", "a.txt"},
		{"/a.txt", "a.txt"},
		{"/", ""},
		{"", ""},
		{"dir/", ""},
	} {
		t.Run(c.in, func(t *testing.T) {
			t.Parallel()
			if got := base(c.in); got != c.want {
				t.Errorf("base(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// fitRowVerbs drops verbs from the end until the rest fit the room. Room they
// fill exactly is room they fit: taking one more away loses a verb the row had
// space to draw, and the reader loses the key for it.
func TestFitRowVerbsKeepsTheVerbsThatExactlyFill(t *testing.T) {
	t.Parallel()
	var w Renderer
	verbs := []state.VerbName{state.VerbNameSelect, state.VerbNameExtend}
	exact := w.keyedVerbJoinWidth(verbs)
	if exact <= 0 {
		t.Fatal("the verbs take no room, so this proves nothing")
	}

	for _, c := range []struct {
		name string
		room int
		want int
	}{
		{"a cell more than they need", exact + 1, len(verbs)},
		{"exactly the room they fill", exact, len(verbs)},
		{"a cell less", exact - 1, len(verbs) - 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := w.fitRowVerbs(verbs, c.room)
			if len(got) != c.want {
				t.Errorf("%d verbs fit in %d cells, want %d: %v", len(got), c.room, c.want, got)
			}
			if len(got) > 0 && w.keyedVerbJoinWidth(got) > c.room {
				t.Errorf("what was kept is %d cells, wider than the %d it had",
					w.keyedVerbJoinWidth(got), c.room)
			}
		})
	}
}
