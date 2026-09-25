package layout

import (
	"slices"
	"strings"
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

func commitInfo(subject string, unpushed bool) git.CommitInfo {
	return git.CommitInfo{
		SHA:      "abc123def456",
		ShortSHA: "abc123",
		Subject:  subject,
		Unpushed: unpushed,
		Age:      "2m",
	}
}

func verbsContain(verbs []state.VerbName, name state.VerbName) bool {
	for _, v := range verbs {
		if v == name {
			return true
		}
	}
	return false
}

func TestCommitRowIsExactlyPaneWidth(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	cases := []commitRowLayout{
		{Info: commitInfo("chore: short", false)},
		{Info: commitInfo(strings.Repeat("long subject ", 8), true)},
		{Info: commitInfo("refactor: 同期所有権の判定を行う", false), State: commitRowState{Cursor: true}, Verbs: []state.VerbName{state.VerbNameDiff}},
	}
	for _, c := range cases {
		line, _ := w.CommitRow(c, paneWidth)
		if got := w.Of(line); got != paneWidth {
			t.Errorf("%q is %d cells, want %d", line, got, paneWidth)
		}
	}
}

func TestCommitRowSurvivesEveryPaneWidth(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	cases := []commitRowLayout{
		{Info: commitInfo(strings.Repeat("long subject ", 8), true)},
		{Info: commitInfo("refactor: 同期所有権の判定を行う", false),
			State: commitRowState{Cursor: true},
			Verbs: []state.VerbName{state.VerbNameUndo, state.VerbNameDiff, state.VerbNameSHA}},
	}
	for _, c := range cases {
		for width := 1; width <= 80; width++ {
			line, _ := w.CommitRow(c, width)
			if width >= 40 && w.Of(line) != width {
				t.Fatalf("width %d: %q is %d cells, want %d", width, line, w.Of(line), width)
			}
		}
	}
}

func TestTipUnpushedCommitShowsUncommit(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	tip := commitRowLayout{
		Info:  commitInfo("local only", true),
		State: commitRowState{Cursor: true},
		Verbs: state.CommitVerbs(commitInfo("local only", true), 0, state.HostTools{Clipboard: false, Browser: false}),
	}
	line, regions := w.CommitRow(tip, paneWidth)
	if !strings.Contains(line, "uncommit") {
		t.Fatalf("line = %q, want uncommit", line)
	}
	var uncommit bool
	for _, r := range regions {
		if r.Target.Kind == TargetVerb && r.Target.Verb == state.VerbNameUncommit {
			uncommit = true
		}
	}
	if !uncommit {
		t.Fatal("no uncommit region on the tip")
	}

	older := commitRowLayout{
		Info:  commitInfo("pushed", true),
		State: commitRowState{Cursor: true},
		Verbs: state.CommitVerbs(commitInfo("pushed", true), 1, state.HostTools{Clipboard: false, Browser: false}),
	}
	line, _ = w.CommitRow(older, paneWidth)
	if strings.Contains(line, "undo") {
		t.Fatalf("older unpushed row should not show undo: %q", line)
	}
}

func TestCommitVerbsDifferForPushedAndUnpushed(t *testing.T) {
	t.Parallel()
	pushed := state.CommitVerbs(commitInfo("on remote", false), 1, state.HostTools{Clipboard: true, Browser: true})
	unpushed := state.CommitVerbs(commitInfo("local only", true), 1, state.HostTools{Clipboard: true, Browser: true})
	if !verbsContain(pushed, state.VerbNameOpen) {
		t.Fatalf("pushed commit should offer open: %v", pushed)
	}
	if verbsContain(unpushed, state.VerbNameOpen) {
		t.Fatalf("unpushed commit should not offer open: %v", unpushed)
	}
	if !verbsContain(pushed, state.VerbNameSHA) || !verbsContain(unpushed, state.VerbNameSHA) {
		t.Fatalf("both should offer sha when tools exist: pushed=%v unpushed=%v", pushed, unpushed)
	}
}

func TestCommitVerbsOmitShaWithoutClipboard(t *testing.T) {
	t.Parallel()
	verbs := state.CommitVerbs(commitInfo("on remote", false), 1, state.HostTools{Clipboard: false, Browser: true})
	if verbsContain(verbs, state.VerbNameSHA) {
		t.Fatalf("want no sha without clipboard: %v", verbs)
	}
}

func TestCommitVerbsOmitOpenWithoutBrowser(t *testing.T) {
	t.Parallel()
	verbs := state.CommitVerbs(commitInfo("on remote", false), 1, state.HostTools{Clipboard: true, Browser: false})
	if verbsContain(verbs, state.VerbNameOpen) {
		t.Fatalf("want no open without browser: %v", verbs)
	}
}

func TestCommitVerbsOmitOpenForNonURLRemote(t *testing.T) {
	t.Parallel()
	info := commitInfo("on remote", false)
	if verbsContain(commitVerbsForRemote(info, 1, "/private/tmp/origin.git", true, true), state.VerbNameOpen) {
		t.Fatal("filesystem remote should not offer open")
	}
	if !verbsContain(commitVerbsForRemote(info, 1, "git@github.com:owner/repo.git", true, true), state.VerbNameOpen) {
		t.Fatal("ssh remote should offer open when browser exists")
	}
}

func TestRemoteCommitURLRejectsFilesystemPath(t *testing.T) {
	t.Parallel()
	_, ok := RemoteCommitURL("/private/tmp/origin.git", "deadbeef")
	if ok {
		t.Fatal("want filesystem remote rejected")
	}
}

func TestRemoteCommitURLFromSSHRemote(t *testing.T) {
	t.Parallel()
	url, ok := RemoteCommitURL("git@github.com:owner/repo.git", "deadbeef")
	if !ok {
		t.Fatal("want ok")
	}
	want := "https://github.com/owner/repo/commit/deadbeef"
	if url != want {
		t.Fatalf("got %q, want %q", url, want)
	}
}

func TestRemoteCommitURLFromHTTPSRemote(t *testing.T) {
	t.Parallel()
	url, ok := RemoteCommitURL("https://host/owner/repo.git", "deadbeef")
	if !ok {
		t.Fatal("want ok")
	}
	want := "https://host/owner/repo/commit/deadbeef"
	if url != want {
		t.Fatalf("got %q, want %q", url, want)
	}
}

// A subject carries its type and its gist at the front, so it gives up its end
// — the opposite of a path, which is named by its tail.
func TestASubjectLosesItsEndNotItsStart(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	line, _ := w.CommitRow(commitRowLayout{Info: git.CommitInfo{
		Subject:  "refactor: 同期所有権の判定をユースケース層へ移動して重複を除去する",
		Author:   "Someone",
		ShortSHA: "a5de620", Age: "2m",
	}}, paneWidth)
	if !strings.Contains(line, "refactor:") {
		t.Errorf("the subject lost its front: %q", line)
	}
}

// A repository with no remote is the common case for a scratch tree, and its
// tip commit is the safest thing there is to reset: no remote has seen it.
func TestTipOffersUncommitWhenThereIsNoRemote(t *testing.T) {
	t.Parallel()
	info := git.CommitInfo{SHA: "a", Subject: "x", Unpushed: true, HasUpstream: false}
	if !slices.Contains(state.CommitVerbs(info, 0, state.HostTools{Clipboard: true, Browser: true}), state.VerbNameUncommit) {
		t.Errorf("uncommit withheld from a local-only tip: %v",
			state.CommitVerbs(info, 0, state.HostTools{Clipboard: true, Browser: true}))
	}
}

func TestPushedTipDoesNotOfferUndo(t *testing.T) {
	t.Parallel()
	info := git.CommitInfo{SHA: "a", Subject: "x", Unpushed: false, HasUpstream: true}
	if slices.Contains(state.CommitVerbs(info, 0, state.HostTools{Clipboard: true, Browser: true}), state.VerbNameUndo) {
		t.Errorf("undo offered on a commit a remote already holds: %v",
			state.CommitVerbs(info, 0, state.HostTools{Clipboard: true, Browser: true}))
	}
}

func TestCommitRowSubjectDoesNotMoveWhenCursorLandsOnAPushedCommit(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	info := commitInfo("pushed subject", false)
	plain, _ := w.CommitRow(commitRowLayout{Info: info}, paneWidth)
	cursor, _ := w.CommitRow(commitRowLayout{
		Info:  info,
		State: commitRowState{Cursor: true},
	}, paneWidth)
	if got := cellsBefore(plain, info.Subject); got != markColumn+2 {
		t.Fatalf("subject starts at %d, want %d", got, markColumn+2)
	}
	if cellsBefore(plain, info.Subject) != cellsBefore(cursor, info.Subject) {
		t.Errorf("the subject moved under the cursor:\n%q\n%q", plain, cursor)
	}
}

func TestHistoryPaneDoesNotCallLookPath(t *testing.T) {
	t.Parallel()
	commit := commitInfo("on remote", false)
	s := state.State{Width: paneWidth, Height: 53, Tab: state.TabHistory, Cursor: 0, Rows: []state.Row{state.CommitRow(commit)}, History: state.History{RemoteURL: "https://github.com/owner/repo.git", Commits: []git.CommitInfo{commit}}}
	frame := Pane(s, nil, Renderer{})
	if frameHasVerb(frame, state.VerbNameSHA) {
		t.Fatal("sha verb should not appear without clipboard")
	}
	frame = Pane(s, nil, Renderer{Clipboard: true, Browser: true})
	if !frameHasVerb(frame, state.VerbNameSHA) {
		t.Fatal("sha verb should appear when clipboard is ready")
	}
}

func frameHasVerb(frame Frame, name state.VerbName) bool {
	for _, r := range frame.Regions {
		if r.Target.Kind == TargetVerb && r.Target.Verb == name {
			return true
		}
	}
	return false
}

// The ↑ says this commit is on the branch and not on the remote it tracks. It
// needs both halves: without an upstream there is nowhere it could have been
// pushed to, and a mark that says "ahead" of nothing sends the reader looking
// for a push that cannot be made.
//
// commitInfo leaves HasUpstream false, so the mark is drawn by none of the
// tests above and the condition could be written either way.
func TestTheUnpushedMarkNeedsBothAnUpstreamAndACommitNotOnIt(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	const mark = "↑"

	for _, c := range []struct {
		name                  string
		unpushed, hasUpstream bool
		want                  bool
	}{
		{"not pushed, and there is an upstream", true, true, true},
		{"not pushed, and there is nowhere to push", true, false, false},
		{"pushed, with an upstream", false, true, false},
		{"pushed, with no upstream", false, false, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			info := commitInfo("chore: short", c.unpushed)
			info.HasUpstream = c.hasUpstream
			line, _ := w.CommitRow(commitRowLayout{Info: info}, paneWidth)
			if got := strings.Contains(line, mark); got != c.want {
				t.Errorf("the %s is %v on the row, want %v: %q", mark, got, c.want, line)
			}
		})
	}
}
