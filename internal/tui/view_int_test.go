package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/catpotd/mirugit/internal/state"
)

func installGitLogger(t *testing.T) (logPath string, restore func()) {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	logPath = filepath.Join(dir, "git.log")
	script := filepath.Join(dir, "git")
	body := fmt.Sprintf("#!/bin/sh\necho \"$*\" >> %q\nexec %q \"$@\"\n", logPath, realGit)
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	oldPath := os.Getenv("PATH")
	if err := os.Setenv("PATH", dir+string(os.PathListSeparator)+oldPath); err != nil {
		t.Fatal(err)
	}
	return logPath, func() {
		_ = os.Setenv("PATH", oldPath)
	}
}

func clearGitLog(logPath string) {
	_ = os.WriteFile(logPath, nil, 0o644)
}

func gitLogContains(logPath, needle string) bool {
	data, err := os.ReadFile(logPath)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), needle)
}

func TestViewDoesNotExecGit(t *testing.T) {
	logPath, restore := installGitLogger(t)
	defer restore()

	dir := testRepo(t)
	m, err := New(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	clearGitLog(logPath)
	m.probe.settled = true
	m.state.Open.Path = ""
	m.View()
	if gitLogContains(logPath, "rev-parse --git-path FETCH_HEAD") {
		t.Fatalf("View called git for FETCH_HEAD:\n%s", readGitLog(t, logPath))
	}
}

func readGitLog(t *testing.T, logPath string) string {
	t.Helper()
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestFetchedLabelFollowsCommand(t *testing.T) {
	t.Parallel()
	dir := testRepo(t)
	fetchHead := filepath.Join(dir, ".git", "FETCH_HEAD")
	if err := os.WriteFile(fetchHead, []byte("deadbeef\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(fetchHead, time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}

	m, err := New(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.probe.settled = true
	m.state = state.Apply(m.state, state.Resized{Width: 77, Height: 53})

	msg := loadFetched(context.Background(), dir)()
	fetched, ok := msg.(fetchedMsg)
	if !ok {
		t.Fatalf("want fetchedMsg, got %T", msg)
	}
	next, _ := m.Update(fetched)
	m = next.(*Model)
	m.View()
	if !strings.Contains(m.frame.Lines[0], "fetched") {
		t.Fatalf("tab bar = %q, want fetched on the right", m.frame.Lines[0])
	}
}

// loadFetched reads when the remote was last asked, and the bar says "fetched
// 3m ago" from it. A read that failed is not a time: reported as one it is the
// zero time, and the bar then says the remote was last asked in year one.
func TestLoadFetchedReportsAReadItCouldNotMake(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	msg := loadFetched(context.Background(), t.TempDir())()
	fetched, ok := msg.(fetchedMsg)
	if !ok {
		t.Fatalf("want fetchedMsg, got %T", msg)
	}
	if fetched.err == nil {
		t.Errorf("a directory that is not a repository answered with the time %v and no error",
			fetched.at)
	}
	if !fetched.at.IsZero() {
		t.Errorf("a read that failed answered with the time %v", fetched.at)
	}
}
