package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// diffStale answered "fresh" when git failed, so a stage could run against
// blocks nothing had confirmed. A read it cannot make is not the same answer as
// a read that came back unchanged.
func TestAnUnreadableDiffCountsAsStale(t *testing.T) {
	t.Parallel()
	shown := git.FileDiff{
		Path:   "a.txt",
		Blocks: []git.Block{{Header: "@@", Lines: []string{"+x"}, Hash: "h1"}},
	}
	stale, err := diffStale(context.Background(), t.TempDir(), "a.txt", false, shown)
	if err == nil {
		t.Fatal("リポジトリでないディレクトリでエラーを返さなかった")
	}
	if !stale {
		t.Error("読めなかった diff を stale と答えなかった")
	}
}

// The failure has to reach the screen, not only the return value.
func TestAFailedStaleCheckShowsANotice(t *testing.T) {
	t.Parallel()
	m := modelWithOpenDiff(t)
	m.dir = t.TempDir()
	m = applyOnce(t, m, m.checkStaleFirst(noBlockYet))
	if m.state.Notice == "" {
		t.Fatal("Notice が空のまま")
	}
	if !m.state.NoticeFailed {
		t.Error("NoticeFailed が false")
	}
	if !m.state.Changes.Stale["dir/a.txt"] && !m.state.Changes.Stale[m.state.Open.Path] {
		t.Errorf("Stale に %q が入っていない", m.state.Open.Path)
	}
}

// checkStale runs as a command, so its answer arrives as a message. The error
// has to survive that hop.
func TestTheStaleMessageCarriesItsError(t *testing.T) {
	t.Parallel()
	m := modelWithOpenDiff(t)
	m.dir = t.TempDir()
	cmd := m.checkStale()
	if cmd == nil {
		t.Fatal("checkStale が nil を返した")
	}
	msg, ok := cmd().(staleMsg)
	if !ok {
		t.Fatalf("staleMsg ではない: %T", cmd())
	}
	if msg.err == nil {
		t.Error("staleMsg.err が nil")
	}
	next, _ := m.Update(msg)
	got := next.(*Model)
	if !strings.Contains(got.state.Notice, "git") {
		t.Errorf("Notice = %q, git のエラーを含まない", got.state.Notice)
	}
}

// The stale check asks git whether the working tree still matches the diff on
// the screen. Both halves of its guard are needed: a commit's file is not the
// working tree and cannot go stale, and with nothing open there is no diff to
// ask about.
func TestTheStaleCheckRunsOnlyForAnOpenWorkingTreeDiff(t *testing.T) {
	t.Parallel()
	commits := []git.CommitInfo{{SHA: "aaa", ShortSHA: "aaa", Subject: "one"}}
	files := []git.Entry{{Path: "f1.txt", Index: git.Modified}}

	for _, c := range []struct {
		name string
		open func(t *testing.T) *Model
		want bool
	}{
		{"a working tree diff", modelWithOpenDiff, true},
		{"a file of a commit", func(t *testing.T) *Model {
			m := historyModel(t, commits, "aaa", files, 1)
			m.state = state.Apply(m.state, state.DiffOpened{Path: "f1.txt",
				Origin: state.FromCommit("aaa")})
			return m
		}, false},
		{"nothing open", func(t *testing.T) *Model {
			m := modelWithOpenDiff(t)
			m.state = state.Apply(m.state, state.DiffClosed{For: m.state.Open.Path})
			return m
		}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := c.open(t)
			if got := m.checkStale() != nil; got != c.want {
				t.Errorf("a check was started = %v, want %v: open %+v",
					got, c.want, m.state.Open)
			}
		})
	}
}
