package layout

import (
	"strings"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

type commitRowState struct {
	Cursor bool
}

type commitRowLayout struct {
	Info  git.CommitInfo
	State commitRowState
	Verbs []state.VerbName
	Index int
}

// commitVerbsForRemote is state.CommitVerbs with the browser verb dropped when
// the remote URL cannot be turned into an https address.
func commitVerbsForRemote(commit git.CommitInfo, index int, remoteURL string, clipboard, browser bool) []state.VerbName {
	_, openable := state.HTTPSRemoteBase(remoteURL)
	return state.CommitVerbs(commit, index, state.HostTools{
		Clipboard: clipboard, Browser: browser && openable})
}

// RemoteCommitURL builds a browser URL from git remote origin and a commit SHA.
func RemoteCommitURL(remote, sha string) (string, bool) {
	base, ok := state.HTTPSRemoteBase(remote)
	if !ok {
		return "", false
	}
	return strings.TrimSuffix(base, "/") + "/commit/" + sha, true
}

func (w Renderer) CommitRow(r commitRowLayout, width int) (string, []Region) {
	mark := ' '
	if r.Info.Unpushed && r.Info.HasUpstream {
		mark = '↑'
	}
	return w.subjectRow(subjectRowInput{
		Left:      w.assembleGutter(r.State.Cursor, false, true, mark),
		Right:     w.commitMeta(r.Info),
		Subject:   r.Info.Subject,
		Target:    Target{Kind: TargetFile, Commit: r.Info.SHA, Row: r.Index},
		Cursor:    r.State.Cursor,
		Verbs:     r.Verbs,
		ColorMeta: func() string { return w.colorCommitMeta(r.Info) },
	}, width)
}

func (w Renderer) commitMeta(commit git.CommitInfo) string {
	return w.padTo(w.authorColumn(commit), authorWidth) +
		" " + commit.ShortSHA + " " + commit.Age
}

const authorWidth = 10

func (w Renderer) authorColumn(commit git.CommitInfo) string {
	return w.Truncate(Printable(commit.Author), authorWidth)
}

func (w Renderer) colorCommitMeta(commit git.CommitInfo) string {
	plain := w.commitMeta(commit)
	if !w.Enabled {
		return plain
	}
	return replaceOnce(plain, commit.ShortSHA, w.Dim(commit.ShortSHA))
}
