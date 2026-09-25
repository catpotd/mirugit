package tui

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/catpotd/mirugit/internal/state"
)

// A read answers about the tab that asked for it. Moving to another tab makes
// the answer unwanted, and on a repository where a read is slow the reader is
// already looking at rows it says nothing about.
func TestChangingTabsEndsTheReadsTheOldOneAsked(t *testing.T) {
	t.Parallel()
	m := &Model{base: context.Background()}

	asked := m.readsForTab()
	if asked.Err() != nil {
		t.Fatal("the reads for the tab being drawn are already over")
	}
	m.switchTab(state.TabHistory)
	if asked.Err() == nil {
		t.Error("the reads for the tab that was left are still running")
	}
	if now := m.readsForTab(); now.Err() != nil {
		t.Error("the tab that was chosen has nothing to read under")
	}
}

// Opening another diff ends the read for the one that was open. Moving the
// cursor down five rows started five reads, and every one of them ran to the
// end even though four of the answers were thrown away.
func TestOpeningAnotherDiffEndsTheReadForTheLastOne(t *testing.T) {
	t.Parallel()
	m := &Model{base: context.Background()}

	first := m.startDiffRead()
	if first.Err() != nil {
		t.Fatal("the read that was just started is already over")
	}
	second := m.startDiffRead()
	if first.Err() == nil {
		t.Error("the read for the diff that was replaced is still running")
	}
	if second.Err() != nil {
		t.Error("the read for the diff being opened is already over")
	}
}

// The block hashes behind a read mark are a second read of the same diff.
// Scoping them to the diff read had following the cursor cancel them, and the
// mark was never recorded.
func TestASecondReadOfTheSameDiffDoesNotEndTheFirst(t *testing.T) {
	t.Parallel()
	m := &Model{base: context.Background()}

	body := m.startDiffRead()
	if again := m.readsForDiff(); again != body {
		t.Error("re-reading the diff on screen started a new scope")
	}
	if body.Err() != nil {
		t.Error("the read for the diff on screen was ended by a second read of it")
	}
}

// A write is not a read. Staging finished is a change to the repository, and the
// reader has to be told even if the cursor moved while git ran; canceling it
// part way would leave the index in a state nobody asked for.
func TestAWriteRunsUnderTheContextThatOnlyShutdownEnds(t *testing.T) {
	t.Parallel()
	m := &Model{base: context.Background()}

	m.readsForTab()
	m.startDiffRead()
	root := m.rootContext()
	m.switchTab(state.TabHistory)
	m.startDiffRead()
	if root.Err() != nil {
		t.Error("moving around ended the context a write runs under")
	}
}

// wanted is what keeps a canceled read off the notice row. git answers a cancel
// with an error, and reporting it would name a failure the reader caused by
// pressing a key.
func TestAnAnswerToACanceledReadIsNotDelivered(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cmd := wanted(ctx, func() tea.Msg { return diffMsg{} })

	if msg := cmd(); msg == nil {
		t.Fatal("a read nobody canceled delivered nothing")
	}
	cancel()
	if msg := cmd(); msg != nil {
		t.Errorf("a canceled read delivered %T", msg)
	}
}
