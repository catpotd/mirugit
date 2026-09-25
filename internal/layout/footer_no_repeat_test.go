package layout

import (
	"strings"
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// The footer names the keys the row under the cursor answers, and a key is one
// key. A file row on the history tab printed "d diff" twice: the file's own
// diff and the commit's are the same d, and the line offered it as two.
//
// The keys a row answers come from two places on the tabs whose rows expand —
// the file's and the parent's — so the join is where the same key can arrive
// twice.
func TestTheFooterNamesAKeyOnce(t *testing.T) {
	t.Parallel()
	w := Renderer{Clipboard: true, Browser: true}
	for _, c := range []struct {
		name  string
		build func() state.State
	}{
		{"a file under a commit", func() state.State {
			s := footerState(state.TabHistory)
			s = state.Apply(s, state.HistoryLoaded{Commits: []git.CommitInfo{
				{SHA: "aaa1111", Subject: "one", Unpushed: true},
			}})
			s = state.Apply(s, state.CommitExpanded{SHA: "aaa1111",
				Files: []git.Entry{{Path: "a.txt", Index: git.Added}}})
			return atFirstFileRow(s)
		}},
		{"a file under a stash", func() state.State {
			s := footerState(state.TabStashed)
			s = state.Apply(s, state.StashedLoaded{Stashes: []git.StashRow{
				{Ref: "stash@{0}", SHA: "s0", Branch: "main", Status: git.StashApplies},
			}})
			s = state.Apply(s, state.StashFilesLoaded{Ref: "stash@{0}",
				Files: []git.Entry{{Path: "a.txt", Worktree: git.Modified}}})
			return atFirstFileRow(s)
		}},
		{"a file under a worktree", func() state.State {
			s := footerState(state.TabWorktrees)
			s = state.Apply(s, state.WorktreesLoaded{Base: "main", Here: "/repo",
				Worktrees: []git.WorktreeRow{
					{Path: "/repo", Name: "repo", Main: true, SHA: "aaa"},
					{Path: "/wt", Name: "wt", SHA: "bbb"},
				}})
			s = state.Apply(s, state.WorktreeFilesLoaded{Path: "/repo",
				Files: []git.Entry{{Path: "a.txt", Worktree: git.Modified}}})
			return atFirstFileRow(s)
		}},
		{"a file on the changes tab", func() state.State {
			s := footerState(state.TabChanges)
			s = state.Apply(s, state.StatusLoaded{Head: git.Head{Branch: "main"},
				Rows: []git.Entry{{Path: "a.txt", Worktree: git.Modified}}})
			return atFirstFileRow(s)
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := c.build()
			verbs := footerVerbs(s, w)
			if len(verbs) == 0 {
				t.Fatalf("no verbs to check; the footer reads %q", w.Footer(s, 120))
			}
			seen := map[state.VerbName]bool{}
			for _, v := range verbs {
				if seen[v] {
					t.Errorf("the footer offers %q twice: %q", v.String(), w.Footer(s, 120))
				}
				seen[v] = true
			}
		})
	}
}

// footerState is a pane wide enough that the footer prints every verb it has.
func footerState(tab state.Tab) state.State {
	s := state.State{Width: 120, Height: 40}
	s = state.Apply(s, state.TabChanged{Tab: tab})
	return s
}

// atFirstFileRow puts the cursor on the first file row, which is the row whose
// footer joins the file's keys with the parent's.
func atFirstFileRow(s state.State) state.State {
	for i, row := range s.Rows {
		if row.Kind() == state.RowFile {
			s.Cursor = i
			return s
		}
	}
	return s
}

// The name is what the reader reads, so a repeated verb is a repeated word.
func TestTheFooterPrintsNoWordTwice(t *testing.T) {
	t.Parallel()
	w := Renderer{Clipboard: true, Browser: true}
	s := footerState(state.TabHistory)
	s = state.Apply(s, state.HistoryLoaded{Commits: []git.CommitInfo{
		{SHA: "aaa1111", Subject: "one", Unpushed: true},
	}})
	s = state.Apply(s, state.CommitExpanded{SHA: "aaa1111",
		Files: []git.Entry{{Path: "a.txt", Index: git.Added}}})
	s = atFirstFileRow(s)

	footer := w.Footer(s, 120)
	if n := strings.Count(footer, "d diff"); n != 1 {
		t.Errorf("the footer says \"d diff\" %d times: %q", n, footer)
	}
}

// A stash whose contents have not been read yet draws no count. StashList
// leaves it at -1 so a reload does not pay a git per stash, and a row that
// printed the placeholder would say "-1f".
func TestAnUnreadStashDrawsNoFileCount(t *testing.T) {
	t.Parallel()
	if got := formatStashFiles(-1); got != "" {
		t.Errorf("an unread stash draws %q", got)
	}
	if got := formatStashFiles(0); got != "0f" {
		t.Errorf("an empty stash draws %q, want 0f", got)
	}
	if got := formatStashFiles(3); got != "3f" {
		t.Errorf("three files draw %q, want 3f", got)
	}
}
