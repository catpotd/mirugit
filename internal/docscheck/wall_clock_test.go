package docscheck_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// A wall-clock number in a document is true on one machine on one day. Three of
// them drifted here at once — `make quick` said 7 seconds and took 18, `check`
// said 40 and took 77, `test-race` said 45 and took 83 — and nothing noticed,
// because no test can hold a stopwatch and compare.
//
// So they are not written. What a reader needs instead is in CONTRIBUTING: a
// run that is working keeps printing a line per package, and a run that is
// stuck goes quiet.
func TestNoDocumentPromisesAWallClockTime(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")

	// A promise reads as "about 40 seconds" or "takes 7 seconds": a number a
	// reader will hold a stopwatch to. A setting — FUZZTIME — is
	// not a promise, and is written without the words that make one.
	//
	// A number with the unit left off — "takes about 40, because" — reads past
	// this, and that sentence was here until tonight. Requiring the unit is
	// what keeps "about 100 files" from failing, and the trade is deliberate:
	// the pattern catches the sentence a reader times, not every number near a
	// verb.
	timeClaim := regexp.MustCompile(
		`(?i)\b(?:about|roughly|around|takes|took)\s+\d+\s*(?:s\b|seconds?|minutes?)\b`)
	// Nothing is allowed. A wall-clock number is one machine on one day, and
	// the count that decides the cost — FUZZTIME — is written
	// as the setting it is.
	var allowed []string

	for _, name := range []string{"README.md", "CONTRIBUTING.md", "Makefile"} {
		body, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(body), "\n") {
			claim := timeClaim.FindString(line)
			if claim == "" || allowedClaim(line, allowed) {
				continue
			}
			t.Errorf("%s:%d promises %q, which no test can check and every machine "+
				"answers differently:\n  %s", name, i+1, strings.TrimSpace(claim), line)
		}
	}
}

func allowedClaim(line string, allowed []string) bool {
	for _, s := range allowed {
		if strings.Contains(line, s) {
			return true
		}
	}
	return false
}
