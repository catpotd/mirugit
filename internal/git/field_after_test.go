package git

import "testing"

// fieldAfter reads what git printed past the first n spaces. A line with
// exactly n spaces has nothing past them, and a line with fewer has nothing at
// all: both answer with the empty string rather than reaching past the end of
// what SplitN returned.
func TestFieldAfterReadsPastTheSpacesOrAnswersWithNothing(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, in string
		n        int
		want     string
	}{
		{"past one space", "1 rest of it", 1, "rest of it"},
		{"past two spaces", "1 2 rest of it", 2, "rest of it"},
		{"a line that ends at the space", "1 ", 1, ""},
		{"a line with exactly n spaces and nothing after", "1 2", 2, ""},
		{"a line with fewer spaces than asked for", "1 2", 3, ""},
		{"no spaces at all", "whole", 1, ""},
		{"nothing at all", "", 1, ""},
		{"past no spaces", "whole", 0, "whole"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := fieldAfter(c.in, c.n); got != c.want {
				t.Errorf("fieldAfter(%q, %d) = %q, want %q", c.in, c.n, got, c.want)
			}
		})
	}
}
