package tui

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// A file named after a tab holds only what that tab answers. What spreads a
// decision across call sites is choosing between tabs; a function refusing to
// run outside its own tab is a precondition and stays. Reading state.Facts is
// the opposite of spreading, so it stays too.
func TestTabNamedFilesDoNotBranchOnTheTab(t *testing.T) {
	t.Parallel()
	own := map[string]string{
		"history.go":  "TabHistory",
		"stash.go":    "TabStashed",
		"worktree.go": "TabWorktrees",
		"changes.go":  "TabChanges",
	}
	branch := regexp.MustCompile(`case (state\.)?Tab[A-Z]|\.Tab\s*[!=]=\s*(state\.)?(Tab[A-Z]\w*)`)
	for _, name := range []string{"changes.go", "history.go", "stash.go", "worktree.go"} {
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(src), "\n") {
			m := branch.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			// 自分のタブを名指して抜ける形は事前条件なので通す。
			if strings.Contains(line, "!=") && m[3] == own[name] {
				continue
			}
			t.Errorf("%s:%d はタブを選んで分岐している。pane.go へ移すか表を引く: %s",
				name, i+1, strings.TrimSpace(line))
		}
	}
}
