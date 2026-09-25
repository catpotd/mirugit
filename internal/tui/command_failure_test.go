package tui

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// writeCommands names the command each case drives, so that the check below can
// ask whether commands.go declares one no case here reaches.
type writeCommand struct {
	runs string
	name string
	cmd  func(dir string) tea.Cmd
}

func writeCommandCases() []writeCommand {
	return []writeCommand{
		{"runWorktreeRemove", "removing a worktree",
			func(dir string) tea.Cmd { return runWorktreeRemove(context.Background(), dir, dir+"/wt") }},
		{"runUndoCommit", "undoing a commit",
			func(dir string) tea.Cmd { return runUndoCommit(context.Background(), dir) }},
		{"runCopySHA", "copying a sha",
			func(string) tea.Cmd { return runCopySHA(alwaysCommand("false"), "abc") }},
		{"runFetch", "fetching",
			func(dir string) tea.Cmd { return runFetch(context.Background(), dir) }},
		{"runStageBlock", "staging a block", func(dir string) tea.Cmd {
			return runStageBlock(context.Background(), dir, git.FileDiff{Path: "a.txt",
				Blocks: []git.Block{{Header: "@@ -1 +1 @@", Lines: []string{"+x"}}}}, 0)
		}},
		{"runStashRestore", "restoring a stash",
			func(dir string) tea.Cmd { return runStashRestore(context.Background(), dir, "stash@{0}", 1) }},
		{"runStashBranch", "branching from a stash",
			func(dir string) tea.Cmd { return runStashBranch(context.Background(), dir, "stash@{0}", "b") }},
		{"runStashDrop", "dropping a stash",
			func(dir string) tea.Cmd { return runStashDrop(context.Background(), dir, "abc") }},
		{"runDiscard", "discarding changes", func(dir string) tea.Cmd {
			return runDiscard(context.Background(), dir, []git.Entry{{Path: "a.txt"}}, git.Undo{}, state.DiscardConfirm{})
		}},
		{"runUndiscard", "putting discarded changes back",
			func(dir string) tea.Cmd { return runUndiscard(context.Background(), dir, git.Undo{}) }},
		{"runCommit", "committing",
			func(dir string) tea.Cmd { return runCommit(context.Background(), dir, "m", 1) }},
		{"runSync", "syncing",
			func(dir string) tea.Cmd { return runSync(context.Background(), dir) }},
	}
}

// Commands this sweep cannot drive. Each case runs against a directory that is
// not a repository, so a command that never reads the repository succeeds there
// and asserting on it would say nothing.
var outsideTheWriteSweep = map[string]string{
	"runOpenCommit": "hands a URL to the platform's browser and never reads the repository",
}

// Every command that writes to the repository answers with a message, and the
// update loop reads the error off it. A command that reports a failure as a
// success leaves the reader pressing a key that does nothing, with nothing on
// screen to say why.
//
// Each is run against a directory that is not a repository, so every git call
// fails. Nine commands wrote this shape by hand and a mutation of one of them
// survived the suite.
func TestEveryWriteReportsAFailureItCouldNotRun(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := t.TempDir()

	for _, c := range writeCommandCases() {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			msg := c.cmd(dir)()
			f, ok := msg.(failing)
			if !ok {
				t.Fatalf("%T carries no error, so the update loop cannot report one", msg)
			}
			if f.failed() == nil {
				t.Errorf("%s could not run and answered with no error: %+v", c.name, msg)
			}
		})
	}
}

// The sweep above names its commands by hand, and five of the thirteen were
// missing when this was written. A command left off is not skipped: it is never
// mentioned, so the sweep stays green with fewer cases in it and nothing says
// which ones went.
func TestTheWriteSweepDrivesEveryCommand(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "commands.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	declared := map[string]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || !strings.HasPrefix(fn.Name.Name, "run") {
			return true
		}
		results := fn.Type.Results
		if results == nil || len(results.List) != 1 {
			return true
		}
		if sel, ok := results.List[0].Type.(*ast.SelectorExpr); ok && sel.Sel.Name == "Cmd" {
			declared[fn.Name.Name] = true
		}
		return true
	})
	if len(declared) == 0 {
		t.Fatal("commands.go declares no run… command, so this checks nothing")
	}

	driven := map[string]bool{}
	for _, c := range writeCommandCases() {
		if !declared[c.runs] {
			t.Errorf("the sweep names %q and commands.go declares no such command", c.runs)
		}
		driven[c.runs] = true
	}

	for name := range declared {
		reason := outsideTheWriteSweep[name]
		switch {
		case reason != "" && driven[name]:
			t.Errorf("%s is listed as outside the sweep (%s) and the sweep drives it; "+
				"drop it from outsideTheWriteSweep", name, reason)
		case reason == "" && !driven[name]:
			t.Errorf("%s answers with a message the update loop reads an error off, and "+
				"the sweep does not drive it. Add a case, or add %s to "+
				"outsideTheWriteSweep with the reason", name, name)
		}
	}
}

// A stash that worked answers with what it did: the verb, and the paths git
// left behind. The oversized-pathspec case is read off a stash that failed, and
// reading it off one that worked returns before the outcome is gathered — the
// pane then says nothing about a stash it just made.
func TestAStashThatWorkedAnswersWithWhatItDid(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := repoWithUnstagedChange(t)

	msg := verbCmd(context.Background(), dir, state.VerbStash, []git.Entry{{Path: "a.txt", Worktree: git.Modified}})()
	verb, ok := msg.(verbMsg)
	if !ok {
		t.Fatalf("want verbMsg, got %T", msg)
	}
	if verb.err != nil {
		t.Fatalf("the stash failed: %v", verb.err)
	}
	if verb.finished.Verb != state.VerbStash {
		t.Errorf("the answer names verb %v, want stash", verb.finished.Verb)
	}
	if verb.finished.Applied != 1 {
		t.Errorf("the answer says %d paths were stashed, want 1", verb.finished.Applied)
	}
}

// The undo of a discard answers with the flag that tells it apart from a
// discard. Without it the pane reads its own undo as a discard: the bar offers
// U to undo what was just undone, and the record of what to undo points at the
// undo's own snapshot.
func TestTheUndoOfADiscardSaysThatIsWhatItWas(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := repoWithUnstagedChange(t)
	entries := []git.Entry{{Path: "a.txt", Worktree: git.Modified}}
	undo, err := git.Snapshot(context.Background(), dir, entries, "discard")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := git.Discard(context.Background(), dir, entries, undo); err != nil {
		t.Fatal(err)
	}
	undo = git.RecordWorktreeAfter(dir, undo)

	msg := runUndiscard(context.Background(), dir, undo)()
	un, ok := msg.(undiscardMsg)
	if !ok {
		t.Fatalf("want undiscardMsg, got %T", msg)
	}
	if un.err != nil {
		t.Fatal(un.err)
	}
	if !un.finished.Restored {
		t.Error("the undo of a discard did not say that is what it was")
	}

	after := state.Apply(state.State{}, un.finished)
	if after.LastUndo != nil {
		t.Errorf("the undo left something to undo: %+v", after.LastUndo)
	}
	if strings.Contains(after.Notice, "U undo") {
		t.Errorf("the bar offers U to undo the undo: %q", after.Notice)
	}
}
