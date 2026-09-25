package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
)

// クリックが届くのは、前に描いたフレームの座標である。ファイル監視の再読み込みが
// 行を減らしたあとにその座標が来ると、消えた行を添字で引くことになる。
func TestClickOnARowThatIsGoneDoesNotPanic(t *testing.T) {
	t.Parallel()
	rows := []state.Row{
		state.FileRow(git.Entry{Path: "a.txt"}, state.SectionUnstaged),
		state.FileRow(git.Entry{Path: "b.txt"}, state.SectionUnstaged),
		state.FileRow(git.Entry{Path: "c.txt"}, state.SectionUnstaged),
	}
	m := &Model{
		state: state.State{
			Tab: state.TabChanges, Width: 77, Height: 24,
			Rows: rows, Changes: state.Changes{Selected: map[string]bool{}},
		},
		render: layout.Renderer{},
		read:   emptyRead(),
	}
	m.state.Rows = rows[:1]
	gone := layout.Target{Kind: layout.TargetCheckbox, Row: 2, Path: "c.txt"}
	for _, mod := range []tea.KeyMod{0, tea.ModShift} {
		func() {
			defer func() {
				if p := recover(); p != nil {
					t.Fatalf("消えた行のチェックボックスをクリックして落ちた: %v", p)
				}
			}()
			m.checkboxClick(gone, mod)
		}()
	}
}
