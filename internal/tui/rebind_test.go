package tui

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
)

// g moves to another worktree. Writing m.dir alone left the watcher on the old
// directory and the read marks keyed by the old path, so the new worktree's
// marks went into the old one's file.
func TestGoingToAWorktreeTakesTheReadMarksWithIt(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	main := t.TempDir()
	env := []string{
		"HOME=" + main, "GIT_CONFIG_GLOBAL=" + main + "/.gitconfig",
		"GIT_CONFIG_SYSTEM=/dev/null",
		// git forks a repack of its own once a repository crosses a
		// threshold, and that child outlives the test: it races
		// t.TempDir's removal, and it takes the repository's locks
		// while another test is reading it.
		"GIT_CONFIG_COUNT=2",
		"GIT_CONFIG_KEY_0=gc.auto", "GIT_CONFIG_VALUE_0=0",
		"GIT_CONFIG_KEY_1=maintenance.auto", "GIT_CONFIG_VALUE_1=false",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@e.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@e.com",
	}
	run := func(dir string, a ...string) {
		t.Helper()
		c := exec.Command("git", a...)
		c.Dir, c.Env = dir, env
		if o, err := c.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", a, err, o)
		}
	}
	run(main, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(main, "a.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(main, "add", ".")
	run(main, "commit", "-m", "one")
	wt := filepath.Join(t.TempDir(), "wt")
	run(main, "worktree", "add", "-b", "side", wt)

	stateDir := t.TempDir()
	m, err := New(context.Background(), main, stateDir)
	if err != nil {
		t.Fatal(err)
	}
	m.probe.settled = true
	before := m.read
	row := git.WorktreeRow{Path: wt, Name: "wt", Branch: "side", SHA: "abc"}
	m.state = state.Apply(m.state, state.WorktreesLoaded{
		Worktrees: []git.WorktreeRow{{Path: main, Name: "main", Main: true}, row},
		Base:      "main",
		Here:      main,
	})
	m.state = state.Apply(m.state, state.TabChanged{Tab: state.TabWorktrees})
	m.state.Cursor = 1
	m.render = layout.Renderer{}

	next, cmd := m.requestWorktreeGo()
	// The three the move needs are read off the update loop and swapped
	// together, so the move has happened once the answer is back.
	a := applyOnce(t, next.(*Model), cmd)
	if a.dir != wt {
		t.Fatalf("dir = %q, want %q", a.dir, wt)
	}
	if a.read == before {
		t.Error("read marks が移動元のままである")
	}
	if cmd == nil {
		t.Error("watcher の作り直しが返っていない")
	}
}
