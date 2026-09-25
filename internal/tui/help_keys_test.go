package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
)

// ヘルプが印字しているキーは、ヘルプを開いている間も効く。クリックは既に
// runVerb を通って動くので、印字されたキーだけが死んでいる形をなくす。
func TestKeysTheHelpListPrintsWorkWhileHelpIsOpen(t *testing.T) {
	t.Parallel()
	entry := git.Entry{Path: "a.txt", Worktree: git.Modified}
	base := func() *Model {
		return &Model{
			state: state.State{
				Tab: state.TabChanges, Width: 77, Height: 24, HelpOpen: true,
				Changes: state.Changes{Entries: []git.Entry{entry}, Selected: map[string]bool{}},
				Rows:    []state.Row{state.FileRow(entry, state.SectionUnstaged)},
				History: state.History{Commits: []git.CommitInfo{{SHA: "abc", Subject: "one"}}},
			},
			render: layout.Renderer{},
			read:   emptyRead(),
		}
	}
	m := base()
	if _, cmd := m.Update(tea.KeyPressMsg{Code: 'q'}); cmd == nil {
		t.Error("ヘルプを開いている間 q が終了しない")
	}
	m = base()
	next, _ := m.Update(tea.KeyPressMsg{Code: '2'})
	if got := next.(*Model).state.Tab; got != state.TabHistory {
		t.Errorf("ヘルプを開いている間 2 を押した後の Tab = %v, want history", got)
	}
	m = base()
	next, _ = m.Update(tea.KeyPressMsg{Code: 'j'})
	if !next.(*Model).state.HelpOpen {
		t.Error("j はヘルプのスクロールであって、ヘルプを閉じてはいけない")
	}
}

// writesWorkingTree の列挙は、フッターが出す動詞とずれると意味を失う。
// ファイルを書き換える動詞のキーが全部入っていることを見る。
func TestWritingVerbsAreWithheldWhileHelpIsOpen(t *testing.T) {
	t.Parallel()
	m := &Model{}
	for _, verb := range []state.VerbName{state.VerbNameStage, state.VerbNameUnstage, state.VerbNameDiscard, state.VerbNameStash, state.VerbNameRestore, state.VerbNameBranch} {
		key, ok := layout.FooterKeyFor(verb)
		if !ok || len(key) != 1 {
			t.Fatalf("footerKeys に %q の1文字キーが無い", verb)
		}
		if !m.writesWorkingTree(tea.KeyPressMsg{Code: rune(key[0])}) {
			t.Errorf("%q (%s) は作業ツリーを書き換えるのに、ヘルプ中に通る", verb, key)
		}
	}
	for _, verb := range []state.VerbName{state.VerbNameDiff, state.VerbNameRead, state.VerbNameSHA, state.VerbNameOpen, state.VerbNameGo} {
		key, ok := layout.FooterKeyFor(verb)
		if !ok || len(key) != 1 {
			continue
		}
		if m.writesWorkingTree(tea.KeyPressMsg{Code: rune(key[0])}) {
			t.Errorf("%q (%s) は読むだけなのに、ヘルプ中に止められる", verb, key)
		}
	}
}
