package state

import "fmt"

func applyVerb(s State, e Event) (State, bool) {
	switch e := e.(type) {
	case VerbRequested:
		return applyVerbRequested(s, e), true
	case VerbFinished:
		return applyVerbFinished(s, e), true
	case DiscardConfirmationShown:
		return applyDiscardConfirmationShown(s, e), true
	case DiscardCancelled:
		return applyDiscardCancelled(s, e), true
	case DiscardConfirmed:
		return applyDiscardConfirmed(s), true
	case DiscardFinished:
		return applyDiscardFinished(s, e), true
	case StashDropConfirmationShown:
		return applyStashDropConfirmationShown(s, e), true
	case StashDropCancelled:
		return applyStashDropCancelled(s), true
	case StashDropConfirmed:
		return applyStashDropConfirmed(s), true
	case WorktreeRemoveConfirmationShown:
		return applyWorktreeRemoveConfirmationShown(s, e), true
	case WorktreeRemoveCancelled:
		return applyWorktreeRemoveCancelled(s), true
	case WorktreeRemoveConfirmed:
		return applyWorktreeRemoveConfirmed(s), true
	case UndiscardConfirmationShown:
		return applyUndiscardConfirmationShown(s, e), true
	case UndiscardCancelled:
		return applyUndiscardCancelled(s), true
	case UndiscardConfirmed:
		return applyUndiscardConfirmed(s), true
	case UndoFinished:
		return applyUndoFinished(s), true
	case StashBranchFinished:
		return applyStashBranchFinished(s, e), true
	case StashRestoreFinished:
		return applyStashRestoreFinished(s, e), true
	case StashRefsMoved:
		return applyStashRefsMoved(s), true
	case Failed:
		return applyFailed(s, e), true
	}
	return s, false
}

func applyVerbRequested(s State, e VerbRequested) State {
	s.Changes.Pending = &e
	setNotice(&s, "")
	return s
}

func applyVerbFinished(s State, e VerbFinished) State {
	if e.Applied > 0 {
		consumeSelection(&s, s.Changes.Pending, e.Ignored)
	}
	s.Changes.Pending = nil
	switch {
	case e.Notice != "":
		setNotice(&s, e.Notice)
	case e.Verb == VerbStash && len(e.Ignored) > 0:
		setNotice(&s, fmt.Sprintf("stashed nothing · %d %s had no changes", len(e.Ignored), FileWord(len(e.Ignored))))
	case e.Verb != VerbDiscard && len(e.Ignored) > 0:
		setNotice(&s, fmt.Sprintf("%d unchanged", len(e.Ignored)))
	case e.Verb == VerbStash && e.Applied > 0:
		setNotice(&s, fmt.Sprintf("stashed %d %s", e.Applied, FileWord(e.Applied)))
	}
	return s
}

func applyDiscardConfirmationShown(s State, e DiscardConfirmationShown) State {
	s.Changes.DiscardConfirm = &e.Confirm
	setNotice(&s, "")
	return s
}

func applyDiscardCancelled(s State, e DiscardCancelled) State {
	s.Changes.DiscardConfirm = nil
	if e.Notice != "" {
		setNotice(&s, e.Notice)
	} else {
		setNotice(&s, "canceled · nothing discarded")
	}
	return s
}

func applyStashDropConfirmationShown(s State, e StashDropConfirmationShown) State {
	s.Stashed.DropConfirm = &e.Confirm
	setNotice(&s, "")
	return s
}

func applyStashDropCancelled(s State) State {
	s.Stashed.DropConfirm = nil
	setNotice(&s, "canceled · stash kept")
	return s
}

func applyWorktreeRemoveConfirmationShown(s State, e WorktreeRemoveConfirmationShown) State {
	s.Worktrees.RemoveConfirm = &e.Confirm
	setNotice(&s, "")
	return s
}

func applyWorktreeRemoveCancelled(s State) State {
	s.Worktrees.RemoveConfirm = nil
	setNotice(&s, "canceled · worktree kept")
	return s
}

func applyWorktreeRemoveConfirmed(s State) State {
	s.Worktrees.RemoveConfirm = nil
	return s
}

func applyStashDropConfirmed(s State) State {
	s.Stashed.DropConfirm = nil
	s.Stashed.RefsStale = true
	return s
}

func applyUndiscardConfirmationShown(s State, e UndiscardConfirmationShown) State {
	s.Changes.UndiscardConfirm = &e.Confirm
	setNotice(&s, "")
	return s
}

func applyUndiscardCancelled(s State) State {
	s.Changes.UndiscardConfirm = nil
	setNotice(&s, "canceled · discard kept")
	return s
}

func applyUndiscardConfirmed(s State) State {
	s.Changes.UndiscardConfirm = nil
	return s
}

func applyDiscardConfirmed(s State) State {
	if s.Changes.DiscardConfirm != nil {
		s.Changes.Pending = &VerbRequested{
			Verb:    VerbDiscard,
			Targets: s.Changes.DiscardConfirm.Targets,
			Block:   -1,
		}
	}
	return s
}

func applyDiscardFinished(s State, e DiscardFinished) State {
	if !e.Restored && e.Applied > 0 {
		consumeSelection(&s, s.Changes.Pending, nil)
	}
	s.Changes.Pending = nil
	s.Changes.DiscardConfirm = nil
	s.Changes.UndiscardConfirm = nil
	fileWord := FileWord(e.Applied)
	if e.Restored {
		s.LastUndo = nil
		setNotice(&s, fmt.Sprintf("restored %d %s", e.Applied, fileWord))
		return s
	}
	u := e.Undo
	s.LastUndo = &u
	setNotice(&s, fmt.Sprintf("discarded %d %s · %s %s · U undo",
		e.Applied, fileWord, Plus(e.Added), Minus(e.Deleted)))
	return s
}

func applyUndoFinished(s State) State {
	setNotice(&s, "undid commit · reset")
	return s
}

func applyStashBranchFinished(s State, e StashBranchFinished) State {
	setNotice(&s, "branched to "+e.Branch)
	return s
}

// applyStashRestoreFinished names the stash because the reader has two ways to
// put files back — U after a discard says "restored N files" too — and the two
// land in the same place from different sources.
func applyStashRestoreFinished(s State, e StashRestoreFinished) State {
	setNotice(&s, fmt.Sprintf("restored %d %s from the stash", e.Files, FileWord(e.Files)))
	return s
}

// applyStashRefsMoved says the refs on hand no longer name the stashes they
// did. Every verb that takes a stash off the list applies it.
func applyStashRefsMoved(s State) State {
	s.Stashed.RefsStale = true
	return s
}

func applyFailed(s State, e Failed) State {
	s.Changes.Pending = nil
	// A verb that fails silently leaves the reader pressing a key that never
	// moves the tree, with nothing on screen to say why.
	if e.Line != "" {
		setFailedNotice(&s, e.Line)
	}
	return s
}
