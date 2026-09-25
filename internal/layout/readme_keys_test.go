package layout

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The README's key table is where a reader looks before running the program.
// A key the help lists and the README omits is one they never learn about
// without opening the overlay.
func TestTheReadmeListsEveryHelpKey(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	readme := string(raw)

	for _, row := range helpRows {
		if row.verb.IsZero() {
			continue
		}
		for _, key := range strings.Fields(row.keys) {
			// The README writes shift+n rather than the overlay's N, and enter
			// and ctrl+c appear inside other rows.
			// The overlay writes a range and a bare modifier where the
			// README spells them out, so the two are compared through what
			// each form stands for.
			wants := []string{key}
			switch key {
			case "N":
				wants = []string{"shift"}
			case "+", "shift":
				continue
			case "ctrl+c":
				wants = []string{"ctrl"}
			case "1-4":
				wants = []string{"1", "2", "3", "4"}
			}
			for _, want := range wants {
				if !strings.Contains(readme, "`"+want+"`") {
					t.Errorf("the help offers %q and the README's key table omits %q",
						key, want)
				}
			}
		}
	}
}
