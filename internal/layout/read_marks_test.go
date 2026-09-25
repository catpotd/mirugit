package layout

import (
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

type emptyReadMarks struct{}

func (emptyReadMarks) MarkIn(state.Origin, string, []string) state.Mark { return state.Unread }
func (emptyReadMarks) StashMark(string) state.Mark                      { return state.Unread }
func (emptyReadMarks) WorktreeMark(string, string) state.Mark           { return state.Unread }
func (emptyReadMarks) BlockRead(string, string) bool                    { return false }
func (emptyReadMarks) HashesIn(state.Origin, string) []string           { return nil }

type artifactMarks struct{}

func (artifactMarks) MarkIn(o state.Origin, path string, _ []string) state.Mark {
	if o.Side() == state.SectionStaged && path == "app/Sources/Infrastructure/Dependencies.swift" {
		return state.Read
	}
	return state.Unread
}
func (artifactMarks) StashMark(string) state.Mark            { return state.Unread }
func (artifactMarks) WorktreeMark(string, string) state.Mark { return state.Unread }
func (artifactMarks) BlockRead(string, string) bool          { return false }
func (artifactMarks) HashesIn(state.Origin, string) []string {
	return nil
}

// The open file's read state is judged against the blocks on screen, because
// those are the ones the reader can have read. Every other file is judged
// against what the record holds for it: taking the open file's blocks for them
// asks whether one file was read using another file's lines, and the answer
// lands on whichever row the pane happens to be drawing.
func TestOnlyTheOpenFileIsJudgedAgainstTheBlocksOnScreen(t *testing.T) {
	t.Parallel()
	s := state.State{Width: 90, Height: 30}
	s.Open.Path = "a.txt"
	s.Open.Origin = state.WorkingTree(state.SectionUnstaged)
	s.Open.Diff = git.FileDiff{Path: "a.txt", Blocks: []git.Block{
		{Hash: "open-1"}, {Hash: "open-2"}}}

	marks := &recordingMarks{hashes: map[string][]string{
		"a.txt": {"recorded-a"},
		"b.txt": {"recorded-b"},
	}}
	unstaged := state.WorkingTree(state.SectionUnstaged)

	fileMark(marks, unstaged, "a.txt", s)
	if got := marks.askedWith["a.txt"]; len(got) != 2 || got[0] != "open-1" {
		t.Errorf("the open file was judged against %v, want the blocks on screen", got)
	}

	fileMark(marks, unstaged, "b.txt", s)
	if got := marks.askedWith["b.txt"]; len(got) != 1 || got[0] != "recorded-b" {
		t.Errorf("another file was judged against %v, want what the record holds", got)
	}

	// The same file under another origin is another file: a commit's copy of
	// a.txt is not the one the working tree has open.
	fileMark(marks, state.FromCommit("abc"), "a.txt", s)
	if got := marks.askedWith["a.txt"]; len(got) != 1 || got[0] != "recorded-a" {
		t.Errorf("a.txt from a commit was judged against %v, want what the record holds", got)
	}
}

// recordingMarks keeps the hashes it was asked about, which is what tells the
// blocks on screen apart from the ones the record holds.
type recordingMarks struct {
	hashes    map[string][]string
	askedWith map[string][]string
}

func (m *recordingMarks) MarkIn(_ state.Origin, path string, hashes []string) state.Mark {
	if m.askedWith == nil {
		m.askedWith = map[string][]string{}
	}
	m.askedWith[path] = hashes
	return state.Read
}
func (m *recordingMarks) StashMark(string) state.Mark            { return state.Read }
func (m *recordingMarks) WorktreeMark(string, string) state.Mark { return state.Read }
func (m *recordingMarks) BlockRead(string, string) bool          { return true }
func (m *recordingMarks) HashesIn(_ state.Origin, path string) []string {
	return m.hashes[path]
}
