package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// notices.sh puts every dependency's license in the release archive, which MIT
// and BSD both require. It has to run on the machine a maintainer builds from,
// and that used to mean GNU sed: the script spelled the module cache path by
// hand, and the case-escaping rule it needs (\l) is a GNU extension that macOS
// sed does not have. On macOS it turned BurntSushi into !lBurnt!lSushi and
// reported the license missing for a module that was right there.
func TestNoticesScriptDoesNotDependOnGNUSed(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile(filepath.Join("..", "..", "tools", "notices.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for i, line := range strings.Split(string(body), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if strings.Contains(line, "sed") {
			t.Errorf("notices.sh:%d runs sed; the module cache path needs "+
				"case escaping that only GNU sed does:\n\t%s",
				i+1, strings.TrimSpace(line))
		}
	}
}

// The script is what the release runs, so it has to work here: a maintainer who
// cuts a release from a Mac gets the archive this produces.
func TestNoticesScriptCollectsEveryDependency(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the binary and reads the module cache")
	}
	dir := t.TempDir()
	binary := filepath.Join(dir, "mirugit")
	build := exec.Command("go", "build", "-o", binary, "../../cmd/mirugit")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	out := filepath.Join(dir, "NOTICES.txt")
	script := exec.Command("../../tools/notices.sh", binary, out)
	if combined, err := script.CombinedOutput(); err != nil {
		t.Fatalf("notices.sh: %v\n%s", err, combined)
	}

	notices, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	// Every module the binary links has a section, and the script says so by
	// failing; this checks the file it produced is not empty of them.
	modules := exec.Command("go", "version", "-m", binary)
	listed, err := modules.Output()
	if err != nil {
		t.Fatal(err)
	}
	want := 0
	for _, line := range strings.Split(string(listed), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[0] == "dep" {
			want++
			if !strings.Contains(string(notices), fields[1]+" ") {
				t.Errorf("%s has no section in the notices", fields[1])
			}
		}
	}
	if want == 0 {
		t.Fatal("the binary reports no dependencies, so this proves nothing")
	}
}
