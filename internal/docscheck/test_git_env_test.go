package docscheck_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A test that builds a repository has to stop git from forking a repack of its
// own. That child outlives the test: it races t.TempDir's removal, and it holds
// the repository's locks while another test is reading the same directory —
// which is how `git stash list ... failed` reached a pane in a scenario test,
// with nothing on stderr because the child had been signaled.
//
// One fixture set gc.auto and maintenance.auto and the twenty-five beside it
// did not, so the rule held wherever someone had already been bitten. It is
// checked here rather than written down.
func TestEveryTestGitEnvironmentTurnsOffTheBackgroundRepack(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
	checked := 0
	for _, pkg := range []string{"git", "state", "layout", "tui", "osproc"} {
		dir := filepath.Join(root, "internal", pkg)
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			name := entry.Name()
			if !strings.HasSuffix(name, "_test.go") {
				continue
			}
			body, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			text := string(body)
			// The marker of an environment built for git: every one of them
			// silences the system config so the machine's own settings stay out.
			systems := strings.Count(text, `"GIT_CONFIG_SYSTEM=/dev/null"`)
			if systems == 0 {
				continue
			}
			checked += systems
			// Both settings and both values. Counting the key alone let one
			// site say gc.auto=1, which turns the repack on and reads as
			// having thought about it — measured, and green until this looked
			// at the value too.
			for _, part := range []string{
				`"GIT_CONFIG_COUNT=2"`,
				`"GIT_CONFIG_KEY_0=gc.auto", "GIT_CONFIG_VALUE_0=0"`,
				`"GIT_CONFIG_KEY_1=maintenance.auto", "GIT_CONFIG_VALUE_1=false"`,
			} {
				if off := strings.Count(text, part); off != systems {
					t.Errorf("internal/%s/%s builds %d git environments and has %s "+
						"in %d of them", pkg, name, systems, part, off)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no git test environments found, so this proves nothing")
	}
}
