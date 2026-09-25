package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
)

// フッターが「押せる」と書いたキーが、そのタブのキーボードでも何かを起こすことを
// 見る。Facts の行を1つ取り違えると、フッターとヘルプとクリックだけが効いて
// キーボードが黙る形になる(実測: history の o が KeyOpenCommit=false で死んだ)。
func TestFooterKeysAnswerOnTheirTab(t *testing.T) {
	t.Parallel()
	// New は git.Prune を呼ぶので実 repo を要る。このテストが見るのはフッターの
	// 文字列とキーの対応だけなので、Model を直に組んで git を外す
	dir := t.TempDir()
	// open は push 済みのコミットにだけ付く(state.CommitVerbs:58)
	commit := git.CommitInfo{SHA: "abc", Subject: "one"}
	stash := git.StashRow{Ref: "stash@{0}", SHA: "abc", Message: "m", Status: git.StashApplies}
	conflicting := git.StashRow{Ref: "stash@{1}", SHA: "def", Message: "m", Status: git.StashConflicts}
	wt := git.WorktreeRow{Path: "/wt", Name: "wt", Branch: "f", SHA: "abc"}
	// 「何かが起きた」では足りない。キーの表を書き換えた変異は、たまたま別の
	// キーが持つ働き(q なら終了)で「起きた」を満たして通る。動詞ごとに
	// 起きるべきことを名指しする。
	didSomething := map[state.VerbName]func(before state.State, after *Model, cmd tea.Cmd) bool{
		// sha と open の命令は走らせない。前者は利用者のクリップボードを、
		// 後者はブラウザを実際に動かす。終了しない命令が返ることまでを見る。
		state.VerbNameSHA:  func(b state.State, a *Model, c tea.Cmd) bool { return c != nil },
		state.VerbNameOpen: func(b state.State, a *Model, c tea.Cmd) bool { return c != nil },
		// stash の2つは repo でない一時ディレクトリに対して走るので失敗する。
		// 返るメッセージの型が、押したキーが stash の動詞に届いた証拠になる。
		state.VerbNameRestore: func(b state.State, a *Model, c tea.Cmd) bool { return isStashVerbMsg(c) },
		state.VerbNameBranch:  func(b state.State, a *Model, c tea.Cmd) bool { return isStashVerbMsg(c) },
		state.VerbNameDrop:    func(b state.State, a *Model, c tea.Cmd) bool { return a.state.Stashed.DropConfirm != nil },
		// g answers with a command and the move lands when it is fed back:
		// the directory, its git directory and its read marks are read off the
		// update loop and swapped together.
		state.VerbNameGo: func(b state.State, a *Model, c tea.Cmd) bool {
			return isReboundMsg(c)
		},
	}
	for _, tc := range []struct {
		tab   state.Tab
		verbs []state.VerbName
		state state.State
	}{
		{state.TabHistory, []state.VerbName{state.VerbNameSHA, state.VerbNameOpen}, state.State{Tab: state.TabHistory, Rows: []state.Row{state.CommitRow(commit)}, History: state.History{Commits: []git.CommitInfo{commit}, RemoteURL: "https://github.com/o/r"}}},
		{state.TabStashed, []state.VerbName{state.VerbNameRestore, state.VerbNameDrop}, state.State{Tab: state.TabStashed, Rows: []state.Row{state.StashRowOf(stash)}, Stashed: state.Stashed{Stashes: []git.StashRow{stash}}}},
		// branch は衝突する stash にしか付く(state/tab_verbs.go:15)。applies の
		// 行で検査していたときは、行が出さない動詞を押していた。
		{state.TabStashed, []state.VerbName{state.VerbNameBranch, state.VerbNameDrop}, state.State{Tab: state.TabStashed, Rows: []state.Row{state.StashRowOf(conflicting)}, Stashed: state.Stashed{Stashes: []git.StashRow{conflicting}}}},
		{state.TabWorktrees, []state.VerbName{state.VerbNameGo}, state.State{Tab: state.TabWorktrees, Rows: []state.Row{state.WorktreeRowOf(wt)}, Worktrees: state.Worktrees{List: []git.WorktreeRow{wt}}}},
	} {
		for _, verb := range tc.verbs {
			key, ok := layout.FooterKeyFor(verb)
			if !ok || len(key) != 1 {
				t.Fatalf("%s: footerKeys に %q の1文字キーが無い", state.Facts[tc.tab].Name, verb)
			}
			s := tc.state
			s.Width, s.Height = 80, 24
			// The loop below runs what the key answered with, to see that it
			// is not a quit. y and o answer with commands that reach the
			// machine: measured, one run of this test opened a browser tab and
			// replaced the clipboard. Both are handed a command that does
			// neither.
			m := &Model{
				state:     s,
				render:    layout.Renderer{Clipboard: true, Browser: true},
				dir:       dir,
				read:      emptyRead(),
				clipboard: alwaysCommand("true"),
				browser:   alwaysCommand("true"),
			}
			m.probe.settled = true
			next, cmd := m.Update(tea.KeyPressMsg{Code: rune(key[0])})
			after := next.(*Model)
			ran, named := didSomething[verb]
			if !named {
				t.Fatalf("%q に起きるべきことが書かれていない", verb)
			}
			if !ran(s, after, cmd) {
				t.Errorf("%s タブ: フッターは %q を %q で出すが、キーボードは %q の働きをしない",
					state.Facts[tc.tab].Name, verb, key, verb)
			}
			if cmd != nil {
				if _, quit := cmd().(tea.QuitMsg); quit {
					t.Errorf("%s タブ: %q のキーが %q で、押すと終了する",
						state.Facts[tc.tab].Name, verb, key)
				}
			}
		}
	}
}

// isStashVerbMsg runs what the key answered with and reports whether any of it
// is the stash verb. Update batches the verb with the tab reload, so the type of
// the outermost message is the batch rather than the verb.
func isReboundMsg(cmd tea.Cmd) bool {
	for _, msg := range messagesFrom(cmd) {
		if _, ok := msg.(reboundMsg); ok {
			return true
		}
	}
	return false
}

func isStashVerbMsg(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	msg := cmd()
	if _, ok := msg.(stashVerbMsg); ok {
		return true
	}
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return false
	}
	for _, inner := range batch {
		if inner != nil && isStashVerbMsg(inner) {
			return true
		}
	}
	return false
}
