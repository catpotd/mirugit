package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
)

// U と R は verb のキーでもあるが、コミット欄では打てる文字である。verb が先に
// 取ると、書けないうえに U は discard の取り消しを開く。その確認はフッターに
// 出ないので、読み手には何が起きたか分からない。
func TestCapitalsTypeIntoTheCommitBox(t *testing.T) {
	t.Parallel()
	// 端末は shift+u を Code='u' Mod=ModShift で送る。Code='U' では isUndoKey が
	// 発火せず、テストが本番の経路を通らない。
	for _, tc := range []struct {
		code rune
		text string
		mod  tea.KeyMod
	}{
		{'u', "U", tea.ModShift},
		{'r', "R", tea.ModShift},
		{'a', "A", tea.ModShift},
	} {
		m := &Model{
			state: state.State{
				Tab: state.TabChanges, Width: 77, Height: 24,
				Changes:  state.Changes{MessageFocused: true},
				LastUndo: &git.Undo{Ref: "refs/mirugit/undo/1", Commit: "deadbeef"},
			},
			render: layout.Renderer{},
			read:   emptyRead(),
		}
		next, _ := m.Update(tea.KeyPressMsg{Code: tc.code, Text: tc.text, Mod: tc.mod})
		after := next.(*Model)
		if after.state.Changes.Message != tc.text {
			t.Errorf("%q を打った後の Message = %q, want %q", tc.text, after.state.Changes.Message, tc.text)
		}
		if after.state.Changes.UndiscardConfirm != nil {
			t.Errorf("%q を打って discard の取り消しが開いた", tc.text)
		}
	}
}
