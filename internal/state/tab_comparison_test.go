package state_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Naming a tab is how a per-tab answer gets a second copy. state.Facts,
// layout.renderFor and tui.commandsFor are the three places that hold one, and
// every other comparison against a tab is either a guard that says which
// handler this is, or an answer the table already has.
//
// followCursor held the second kind: it asked whether the tab was history to
// decide where a file's diff comes from, which the table answers with OpenFile.
//
// The list is keyed by the text of the comparison rather than by line, because
// a line number moves with any edit above it, and a list that has to be updated
// for unrelated edits gets updated without reading the reason.
var allowedTabComparisons = map[string]string{
	"internal/layout/tab_render.go|switch tab {":     "the layout table",
	"internal/tui/tab_commands.go|switch tab {":      "the tui table",
	"internal/layout/help.go|switch tab {":           "the help filter builds the union of what each tab can offer",
	"internal/state/apply_load.go|== TabChanges":     "a loader writes only the tab it loaded",
	"internal/state/apply_load.go|== TabHistory":     "a loader writes only the tab it loaded",
	"internal/state/apply_load.go|== TabStashed":     "a loader writes only the tab it loaded",
	"internal/state/apply_load.go|== TabWorktrees":   "a loader writes only the tab it loaded",
	"internal/tui/history.go|!= state.TabHistory":    "a key that belongs to one tab",
	"internal/tui/stash.go|!= state.TabStashed":      "a key that belongs to one tab",
	"internal/tui/worktree.go|!= state.TabWorktrees": "a key that belongs to one tab",
	"internal/tui/actions.go|!= state.TabChanges":    "the room a diff needs is a changes-tab measurement",
	"internal/layout/footer.go|!= state.TabChanges":  "the conflict note names an editor, which only this tab reaches",
}

// tabComparison matches a tab being named, in every spelling. Looking for one
// spelling is how four hand-written row bounds survived the change that was
// supposed to remove them.
//
// A switch on a fact the table answers is not a match: dispatching on
// Facts[tab].OpenFile is the shape this test exists to steer towards.
var tabComparison = regexp.MustCompile(
	`(?:==|!=)\s*(?:state\.)?Tab[A-Z]\w*|(?:state\.)?Tab[A-Z]\w*\s*(?:==|!=)|switch\s+(?:[A-Za-z_][\w.]*\.Tab|tab)\s*\{`)

func TestATabIsNamedOnlyWhereTheTableIs(t *testing.T) {
	t.Parallel()
	for _, pkg := range []string{"state", "layout", "tui"} {
		dir := filepath.Join(root(), "internal", pkg)
		for _, file := range goFilesIn(t, dir) {
			body, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			rel := filepath.ToSlash(filepath.Join("internal", pkg, filepath.Base(file)))
			for i, line := range strings.Split(string(body), "\n") {
				match := tabComparison.FindString(line)
				if match == "" {
					continue
				}
				key := rel + "|" + match
				if _, allowed := allowedTabComparisons[key]; allowed {
					continue
				}
				t.Errorf("%s:%d names a tab outside the table; use the fact it answers, "+
					"or add this to allowedTabComparisons with the reason:\n\t%q",
					rel, i+1, key)
			}
		}
	}
}
