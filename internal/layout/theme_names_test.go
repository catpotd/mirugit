package layout

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The README names the palettes a reader can ask for. A name it lists that
// ThemeByName does not answer sends them to a default they did not choose, and
// a palette the README omits is one nobody finds.
func TestTheReadmeNamesEveryPalette(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	readme := string(raw)

	for name, want := range map[string]Theme{"slate": slate, "cozmic": cozmic} {
		if got := ThemeByName(name); got != want {
			t.Errorf("MIRUGIT_THEME=%s gives a palette that is not %s", name, name)
		}
		if !strings.Contains(readme, "`"+name+"`") {
			t.Errorf("the README does not name the %s palette", name)
		}
	}
	// An unknown name has to land somewhere the README names, or the reader is
	// left with a palette that has no name at all.
	if ThemeByName("no such palette") != slate {
		t.Error("an unknown MIRUGIT_THEME does not fall back to slate")
	}
	if !strings.Contains(readme, "MIRUGIT_THEME") || !strings.Contains(readme, "NO_COLOR") {
		t.Error("the README does not name both environment variables")
	}
}
