package tui

import (
	"context"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
)

func applyReloadCmd(t *testing.T, m *Model, cmd tea.Cmd) *Model {
	t.Helper()
	if cmd == nil {
		return m
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("want a batch, got %T", msg)
	}
	for _, c := range batch {
		if c == nil {
			continue
		}
		switch msg := c().(type) {
		case repoMsg:
			next, _ := m.Update(msg)
			m = next.(*Model)
		case diffMsg:
			next, _ := m.Update(msg)
			m = next.(*Model)
		}
	}
	return m
}

func TestStageIsRefusedWhileTheDiffIsStale(t *testing.T) {
	t.Parallel()
	dir := repoWithUnstagedChange(t)
	m, err := New(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.probe.settled = true
	m.state.Width = 77
	m.state.Height = 53

	repo, err := git.Load(context.Background(), dir, mustGitDir(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	next, cmd := m.Update(repoMsgFor(t, dir, repo))
	m = next.(*Model)
	m = applyDiffCmd(t, m, cmd)

	if err := os.WriteFile(dir+"/a.txt", []byte("one\nTWICE\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m = applyOnce(t, m, m.checkStaleFirst(noBlockYet))
	if !m.state.Changes.Stale["a.txt"] {
		t.Fatal("the open diff should be stale after the file changed")
	}

	next, cmd = m.Update(tea.KeyPressMsg{Code: 's'})
	// s asks whether the diff on screen still matches the file before it
	// stages. The answer is what refuses, so the refusal is read after it.
	m = applyOnce(t, next.(*Model), cmd)
	if m.state.Changes.Pending != nil {
		t.Fatalf("%s should be refused: %+v", "stage", m.state.Changes.Pending)
	}
	if !m.state.Changes.Stale[m.state.Open.Path] {
		t.Errorf("the row stopped saying it is stale: %v", m.state.Changes.Stale)
	}

	_, cmd = m.Update(tea.KeyPressMsg{Code: 'd'})
	if cmd == nil {
		t.Fatal("open should still run while stale")
	}
}

func TestStaleCursorRowDrawsStaleReload(t *testing.T) {
	t.Parallel()
	dir := repoWithUnstagedChange(t)
	m, err := New(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.probe.settled = true
	m.state.Width = 77
	m.state.Height = 53

	repo, err := git.Load(context.Background(), dir, mustGitDir(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	next, cmd := m.Update(repoMsgFor(t, dir, repo))
	m = next.(*Model)
	m = applyDiffCmd(t, m, cmd)

	if err := os.WriteFile(dir+"/a.txt", []byte("one\nTWICE\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m = applyOnce(t, m, m.checkStaleFirst(noBlockYet))
	m.View()

	found := false
	for _, line := range m.frame.Lines {
		if strings.Contains(line, "stale · R reload") {
			found = true
			if got := m.render.Of(line); got != 77 {
				t.Fatalf("stale row is %d cells, want 77: %q", got, line)
			}
		}
	}
	if !found {
		t.Fatal("want stale · R reload on the stale cursor row")
	}
}

func TestReloadClearsStaleOnTheOpenFile(t *testing.T) {
	t.Parallel()
	dir := repoWithUnstagedChange(t)
	m, err := New(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.probe.settled = true
	m.state.Width = 77
	m.state.Height = 53

	repo, err := git.Load(context.Background(), dir, mustGitDir(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	next, cmd := m.Update(repoMsgFor(t, dir, repo))
	m = next.(*Model)
	m = applyDiffCmd(t, m, cmd)

	if err := os.WriteFile(dir+"/a.txt", []byte("one\nTWICE\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m = applyOnce(t, m, m.checkStaleFirst(noBlockYet))
	if !m.state.Changes.Stale["a.txt"] {
		t.Fatal("want stale before reload")
	}

	next, cmd = m.Update(tea.KeyPressMsg{Code: 'r', Mod: tea.ModShift})
	m = next.(*Model)
	m = applyReloadCmd(t, m, cmd)

	if m.state.Changes.Stale["a.txt"] {
		t.Fatal("reload should clear stale")
	}
}
