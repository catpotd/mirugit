package state

import (
	"net/url"
	"strings"

	"github.com/catpotd/mirugit/internal/git"
)

// StashVerbs returns the verbs a stash row can show. Conflicting and unrelated
// histories offer branch before restore would leave a dirty tree that the pane
// cannot finish.
func StashVerbs(status git.StashStatus) []VerbName {
	switch status {
	case git.StashApplies:
		return []VerbName{VerbNameRestore, VerbNameDrop}
	case git.StashConflicts, git.StashUnrelated:
		return []VerbName{VerbNameBranch, VerbNameDrop}
	case git.StashUnknown:
	}
	// A stash whose verdict is still being worked out offers nothing yet.
	return nil
}

func StashVerbApplies(verb VerbName, status git.StashStatus) bool {
	return verbIn(StashVerbs(status), verb)
}

// WorktreeVerbs returns the verbs a worktree row can show. Remove is withheld
// when unpushed commits remain, because git would delete them without asking.
func WorktreeVerbs(row git.WorktreeRow, here bool) []VerbName {
	var verbs []VerbName
	if !here {
		verbs = append(verbs, VerbNameGo)
	}
	if row.CanRemove() {
		verbs = append(verbs, VerbNameRemove)
	}
	return verbs
}

func WorktreeVerbApplies(verb VerbName, row git.WorktreeRow, here bool) bool {
	return verbIn(WorktreeVerbs(row, here), verb)
}

// HostTools says which host programs are available, so a verb no tool can run
// is never offered. It replaced two positional booleans, which read as nothing
// at the call site: CommitVerbApplies(verb, commit, index, false, false).
type HostTools struct {
	Clipboard bool
	// Browser is false both when no opener exists and when the remote URL
	// cannot be turned into an https address; the reader cannot tell the two
	// apart and neither can the verb.
	Browser bool
}

// CommitVerbs returns the verbs a history row can show. Undo is only on the tip,
// because reset only reaches one commit, and only while no remote holds it. A
// branch with no upstream is the safest case for it: nothing else can be built
// on a commit no remote has seen. Push and the arrow stay off there, since
// there is nowhere to push to. Open is only offered when the remote URL can be
// turned into https; a filesystem path has nowhere to open.
func CommitVerbs(commit git.CommitInfo, index int, tools HostTools) []VerbName {
	var verbs []VerbName
	if index == 0 && commit.Unpushed {
		verbs = append(verbs, VerbNameUncommit)
	}
	verbs = append(verbs, VerbNameDiff)
	if tools.Clipboard {
		verbs = append(verbs, VerbNameSHA)
	}
	if tools.Browser && !commit.Unpushed {
		verbs = append(verbs, VerbNameOpen)
	}
	return verbs
}

func CommitVerbApplies(verb VerbName, commit git.CommitInfo, index int, tools HostTools) bool {
	return verbIn(CommitVerbs(commit, index, tools), verb)
}

// CommitIndexAt counts commit rows above index in s.Rows.
func CommitIndexAt(s State, index int) int {
	commitIndex := 0
	for i, r := range s.Rows {
		if i >= index {
			break
		}
		if r.Kind() == RowCommit {
			commitIndex++
		}
	}
	return commitIndex
}

// HTTPSRemoteBase turns a git remote URL into an https base when possible.
func HTTPSRemoteBase(remote string) (string, bool) {
	remote = strings.TrimSpace(remote)
	remote = strings.TrimSuffix(remote, ".git")
	if strings.HasPrefix(remote, "https://") || strings.HasPrefix(remote, "http://") {
		return webURL(remote)
	}
	if !strings.HasPrefix(remote, "git@") {
		return "", false
	}
	rest := strings.TrimPrefix(remote, "git@")
	colon := strings.Index(rest, ":")
	if colon < 0 {
		return "", false
	}
	host := rest[:colon]
	path := rest[colon+1:]
	return webURL("https://" + host + "/" + path)
}

// webURL is the last thing between remote.origin.url and the browser. That
// value belongs to whoever the repository was cloned from, so what this package
// builds out of it is checked before a caller hands it to a program: a string
// that parses as a URL, on a scheme a browser opens, with a host to open it on.
func webURL(built string) (string, bool) {
	u, err := url.Parse(built)
	if err != nil {
		return "", false
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return "", false
	}
	if u.Host == "" {
		return "", false
	}
	// A control character cannot appear in a URL and can end a line in whatever
	// reads the string next. The parse above turns every one of them away
	// first, so nothing reaches this line; it stays because what follows is a
	// program being started, and the refusal is stated here rather than left
	// to another package's reading of the same string.
	if strings.ContainsFunc(built, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
		return "", false
	}
	return built, true
}

func verbIn(verbs []VerbName, name VerbName) bool {
	for _, v := range verbs {
		if v == name {
			return true
		}
	}
	return false
}
