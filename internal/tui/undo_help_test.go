package tui

import (
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
)

func TestHelpUndoClickOnHistoryMatchesShiftU(t *testing.T) {
	t.Parallel()
	commit := git.CommitInfo{SHA: "abc", Subject: "one", Unpushed: true}
	base := state.State{Tab: state.TabHistory, Cursor: 0, Width: 80, Height: 24, Rows: []state.Row{state.CommitRow(commit)}, LastUndo: &git.Undo{Ref: "refs/mirugit/undo/1", Commit: "deadbeef"}, History: state.History{Commits: []git.CommitInfo{commit}}}

	dir := t.TempDir()
	shiftModel := &Model{state: base, render: layout.Renderer{}, dir: dir}
	shiftNext, shiftCmd := shiftModel.Update(tea.KeyPressMsg{Code: 'u', Text: "U", Mod: tea.ModShift})
	shift := shiftNext.(*Model)

	helpModel := &Model{state: base, render: layout.Renderer{}, dir: dir}
	helpNext, helpCmd := helpModel.runVerb(state.VerbNameUndo)
	help := helpNext.(*Model)

	if (shift.state.Changes.UndiscardConfirm == nil) != (help.state.Changes.UndiscardConfirm == nil) {
		t.Fatalf("shift confirm %v, help confirm %v, want the same", shift.state.Changes.UndiscardConfirm, help.state.Changes.UndiscardConfirm)
	}
	// 型まで見ないと、どちらも非 nil というだけで通ってしまう。
	// history の小文字 u は undoMsg(git reset)を出すので、そこと分かれることも見る。
	if reflect.TypeOf(msgOf(shiftCmd)) != reflect.TypeOf(msgOf(helpCmd)) {
		t.Fatalf("shift+U は %T, ヘルプの undo は %T を出した。同じであるべき",
			msgOf(shiftCmd), msgOf(helpCmd))
	}
	if _, ok := msgOf(shiftCmd).(undoMsg); ok {
		t.Fatal("shift+U should not run history reset undo")
	}
	if _, ok := msgOf(helpCmd).(undoMsg); ok {
		t.Fatal("help undo should not run history reset undo")
	}

	lower := &Model{state: base, render: layout.Renderer{}, dir: dir}
	_, lowerCmd := lower.Update(tea.KeyPressMsg{Code: 'u'})
	if _, ok := msgOf(lowerCmd).(undoMsg); !ok {
		t.Fatalf("history の小文字 u は git reset の undoMsg を出すべきだが %T だった", msgOf(lowerCmd))
	}
}

func msgOf(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}
