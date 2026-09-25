package git

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The README tells a reader how long the undo commits stay in their repository
// and how many are kept, because those commits are the only trace this program
// leaves behind. A number that drifts from the code turns the page into a
// wrong answer about someone else's repository.
func TestTheReadmeNamesTheUndoLifetimeTheCodeKeeps(t *testing.T) {
	t.Parallel()
	body := readReadme(t)

	for _, c := range []struct {
		what    string
		pattern string
		want    string
	}{
		{"how many are kept", `keeps the newest (\d+)`, strconv.Itoa(undoKeepCount)},
		{"how long they live", `older than (\d+) days`, strconv.Itoa(int(undoMaxAge / (24 * time.Hour)))},
		{"where they live", `(refs/mirugit/undo/)`, undoRefPrefix},
	} {
		t.Run(c.what, func(t *testing.T) {
			t.Parallel()
			m := regexp.MustCompile(c.pattern).FindStringSubmatch(body)
			if m == nil {
				t.Fatalf("the README does not say %s: no match for %s", c.what, c.pattern)
			}
			if m[1] != c.want {
				t.Errorf("the README says %q and the code says %q", m[1], c.want)
			}
		})
	}
}

// A key that cannot be undone has to say so where a reader looks before
// pressing it. drop is the only one, and the README and the footer both name it.
func TestTheReadmeSaysWhichKeyCannotBeUndone(t *testing.T) {
	t.Parallel()
	body := readReadme(t)
	if !strings.Contains(body, "Not recoverable from mirugit") {
		t.Error("the README does not name the operation that cannot be undone")
	}
	if !strings.Contains(body, "reset --soft HEAD^") {
		t.Error("the README does not say what uncommit runs, so a reader cannot " +
			"know the commit is still in the reflog")
	}
}

func readReadme(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
