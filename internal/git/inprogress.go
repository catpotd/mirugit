package git

import (
	"os"
	"path/filepath"
)

// InProgress names a multi-step git operation that is part way through. git
// leaves the working tree in a state where the next step is a commit or an
// abort, and status --porcelain=v2 does not mention it: a merge whose conflicts
// are all resolved looks exactly like a clean tree.
type InProgress int

const (
	NothingInProgress InProgress = iota
	MergeInProgress
	RebaseInProgress
	CherryPickInProgress
	RevertInProgress
	BisectInProgress
	// AmInProgress is git am, which shares rebase's directory. Telling a reader
	// mid-am to run git rebase --continue prints a command git refuses.
	AmInProgress
)

// Verb is what the operation is called, for a line the reader is meant to act
// on rather than a field name.
func (p InProgress) Verb() string {
	switch p {
	case MergeInProgress:
		return "merge"
	case RebaseInProgress:
		return "rebase"
	case CherryPickInProgress:
		return "cherry-pick"
	case RevertInProgress:
		return "revert"
	case BisectInProgress:
		return "bisect"
	case AmInProgress:
		return "am"
	case NothingInProgress:
	}
	return ""
}

// Finish is the command that ends the operation, and Abandon the one that
// undoes it. mirugit runs neither: both take arguments and decisions this pane
// does not ask for, and a reader mid-rebase is better served by the command
// than by a key that guesses.
func (p InProgress) Finish() string {
	switch p {
	case MergeInProgress:
		return "git commit"
	case RebaseInProgress:
		return "git rebase --continue"
	case CherryPickInProgress:
		return "git cherry-pick --continue"
	case RevertInProgress:
		return "git revert --continue"
	case BisectInProgress:
		return "git bisect reset"
	case AmInProgress:
		return "git am --continue"
	case NothingInProgress:
	}
	return ""
}

func (p InProgress) Abandon() string {
	switch p {
	case MergeInProgress:
		return "git merge --abort"
	case RebaseInProgress:
		return "git rebase --abort"
	case CherryPickInProgress:
		return "git cherry-pick --abort"
	case RevertInProgress:
		return "git revert --abort"
	case AmInProgress:
		return "git am --abort"
	case BisectInProgress, NothingInProgress:
	}
	return ""
}

// inProgressMarkers is read in order: a rebase that stops on a conflict leaves
// both its own directory and MERGE_HEAD, and the rebase is what the reader has
// to finish.
var inProgressMarkers = []struct {
	path  string
	state InProgress
}{
	{"rebase-merge", RebaseInProgress},
	// git am and git rebase --apply share rebase-apply; the file inside it is
	// what git itself reads to tell them apart.
	{filepath.Join("rebase-apply", "applying"), AmInProgress},
	{"rebase-apply", RebaseInProgress},
	{"CHERRY_PICK_HEAD", CherryPickInProgress},
	{"REVERT_HEAD", RevertInProgress},
	{"MERGE_HEAD", MergeInProgress},
	{"BISECT_LOG", BisectInProgress},
}

// OperationInProgress reports what git is part way through, given the directory
// git keeps this worktree's state in. The caller resolves that once at startup
// rather than here, because this runs on every reload and the answer does not
// change while the program is pointed at one worktree.
func OperationInProgress(gitDir string) (InProgress, error) {
	for _, marker := range inProgressMarkers {
		_, err := os.Stat(filepath.Join(gitDir, marker.path))
		if err == nil {
			return marker.state, nil
		}
		if !os.IsNotExist(err) {
			return NothingInProgress, err
		}
	}
	return NothingInProgress, nil
}
