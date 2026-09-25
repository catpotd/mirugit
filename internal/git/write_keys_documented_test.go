package git

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The README's table is where a reader looks before pressing a key that changes
// their repository. A key that writes and is not in the table leaves them
// guessing whether it can be undone, which is the one thing the table is for.
//
// The list is written out rather than derived, because deriving it from the
// same place the table is built from would compare a thing with itself.
func TestEveryWritingKeyIsInTheUndoTable(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	readme := string(raw)
	table, ok := sectionOf(readme, "## What each key can undo")
	if !ok {
		t.Fatal("the README has no table of what a key costs")
	}

	for _, c := range []struct {
		// verb is the word the footer prints, which is the word the table's
		// first column uses.
		key, verb string
		// inTable is false for the verbs another key in the pane reverses: they
		// are in the key list rather than in the table of what a key costs.
		inTable bool
	}{
		{"x", "discard", true},
		{"x", "drop", true},
		{"x", "remove", true},
		{"u", "uncommit", true},
		{"b", "branch", true},
		{"S", "sync", true},
		{"z", "stash", false},
		{"s", "stage", false},
		{"u", "unstage", false},
		{"p", "restore", false},
		{"c", "commit", false},
	} {
		t.Run(c.key+" "+c.verb, func(t *testing.T) {
			t.Parallel()
			named := strings.Contains(table, "`"+c.key+"` "+c.verb)
			if named != c.inTable {
				if c.inTable {
					t.Errorf("%q %s changes the repository in a way no key in the "+
						"pane reverses, and the table does not name it", c.key, c.verb)
				} else {
					t.Errorf("%q %s is reversible in the pane; the table is for "+
						"the ones that are not", c.key, c.verb)
				}
			}
			if !strings.Contains(readme, "`"+c.key+"`") {
				t.Errorf("the README does not name the key %q at all", c.key)
			}
		})
	}
}

// sectionOf is the body of one markdown heading, up to the next heading of the
// same level.
func sectionOf(body, heading string) (string, bool) {
	start := strings.Index(body, heading)
	if start < 0 {
		return "", false
	}
	rest := body[start+len(heading):]
	next := regexp.MustCompile(`(?m)^## `).FindStringIndex(rest)
	if next == nil {
		return rest, true
	}
	return rest[:next[0]], true
}
