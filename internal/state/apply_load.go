package state

import (
	"fmt"
	"slices"

	"github.com/catpotd/mirugit/internal/git"
)

func applyLoaded(s State, e Event) (State, bool) {
	switch e := e.(type) {
	case StatusLoaded:
		return applyStatusLoaded(s, e), true
	case HistoryLoaded:
		return applyHistoryLoaded(s, e), true
	case HistoryMoreRequested:
		return applyHistoryMoreRequested(s), true
	case HistoryMoreLoaded:
		return applyHistoryMoreLoaded(s, e), true
	case HistoryMoreFailed:
		return applyHistoryMoreFailed(s), true
	case StashedLoaded:
		return applyStashedLoaded(s, e), true
	case WorktreesLoaded:
		return applyWorktreesLoaded(s, e), true
	case WorktreeRowUpdated:
		return applyWorktreeRowUpdated(s, e), true
	case StashRowUpdated:
		return applyStashRowUpdated(s, e), true
	case WorktreeFilesLoaded:
		return applyWorktreeFilesLoaded(s, e), true
	case StashFilesLoaded:
		return applyStashFilesLoaded(s, e), true
	case RowCollapsed:
		return applyRowCollapsed(s), true
	case DiffLoaded:
		return applyDiffLoaded(s, e), true
	case CommitExpanded:
		return applyCommitExpanded(s, e), true
	}
	return s, false
}

func applyStatusLoaded(s State, e StatusLoaded) State {
	firstLoad := len(s.Rows) == 0
	s.Changes.Entries, s.Head, s.History.RemoteURL = e.Rows, e.Head, e.RemoteURL
	s.Unfinished = e.Unfinished
	if s.Tab == TabChanges {
		s = refreshRows(s, keepCursorRow)
	}
	if firstLoad && s.Tab == TabChanges {
		if i := FirstFileRow(s.Rows); i >= 0 {
			s.Cursor = i
		}
	}
	if s.Open.Path != "" && !hasOpenRow(s) {
		s.Open = OpenDiff{}
	}
	before := len(s.Changes.Selected)
	s.Changes.Selected = stillPresent(s.Changes.Selected, s.Rows)
	if after := len(s.Changes.Selected); before > after {
		setNotice(&s, fmt.Sprintf("%d of %d still here", after, before))
	}
	if s.Changes.DiscardConfirm != nil {
		for _, t := range s.Changes.DiscardConfirm.Targets {
			if !contains(s.Rows, t) {
				s.Changes.DiscardConfirm = nil
				setNotice(&s, "nothing to discard · the changes are gone")
				break
			}
		}
	}
	return s
}

func applyHistoryLoaded(s State, e HistoryLoaded) State {
	s.History.Commits, s.History.HasMore, s.History.LoadingMore = e.Commits, e.HasMore, false
	if s.Tab == TabHistory {
		if len(s.History.Commits) == 0 {
			return leaveForChanges(s)
		}
		s = refreshRows(s, keepCursorIndex)
	}
	return s
}

func applyHistoryMoreRequested(s State) State {
	if !s.History.HasMore || s.History.LoadingMore {
		return s
	}
	s.History.LoadingMore = true
	return s
}

func applyHistoryMoreLoaded(s State, e HistoryMoreLoaded) State {
	s.History.LoadingMore = false
	if e.Offset != len(s.History.Commits) {
		return s
	}
	s.History.Commits = slices.Concat(s.History.Commits, e.Commits)
	s.History.HasMore = e.HasMore
	if s.Tab == TabHistory {
		s = refreshRows(s, keepCursorIndex)
	}
	return s
}

func applyHistoryMoreFailed(s State) State {
	s.History.LoadingMore = false
	return s
}

func applyWorktreeFilesLoaded(s State, e WorktreeFilesLoaded) State {
	return worktreeParents.filesLoaded(s, e.Path, e.Files)
}

func applyStashFilesLoaded(s State, e StashFilesLoaded) State {
	return stashParents.filesLoaded(s, e.Ref, e.Files)
}

// applyRowCollapsed closes the open row on whichever tab the reader is on.
func applyRowCollapsed(s State) State {
	return refreshRows(Facts[s.Tab].Collapse(s), keepCursorRow)
}

func applyStashedLoaded(s State, e StashedLoaded) State {
	s.Stashed.Stashes, s.Head = carryStashReads(s.Stashed.Stashes, e.Stashes), e.Head
	s.Stashed.RefsStale = false
	if s.Tab == TabStashed {
		// Restoring the last stash empties this tab and the bar stops
		// drawing it, which would leave the reader where nothing says
		// they are.
		if len(s.Stashed.Stashes) == 0 {
			return leaveForChanges(s)
		}
		s = refreshRows(s, keepCursorRow)
	}
	return s
}

func applyWorktreesLoaded(s State, e WorktreesLoaded) State {
	s.Worktrees.List = carryWorktreeReads(s.Worktrees.List, e.Worktrees)
	s.Worktrees.Base = e.Base
	s.Worktrees.Here = e.Here
	if s.Tab == TabWorktrees {
		// A lone worktree is every repository, so the bar hides the tab.
		if len(s.Worktrees.List) <= 1 {
			return leaveForChanges(s)
		}
		s = refreshRows(s, keepCursorRow)
	}
	return s
}

func applyWorktreeRowUpdated(s State, e WorktreeRowUpdated) State {
	// The path is what a worktree is.
	list, ok := replaceRowAt(s.Worktrees.List, e.Index, e.Row,
		func(r git.WorktreeRow) string { return r.Path })
	if !ok {
		return s
	}
	s.Worktrees.List = list
	if s.Tab == TabWorktrees {
		s = refreshRows(s, keepCursorIndex)
	}
	return s
}

// replaceRowAt puts row at index, and answers false when the list has moved
// under the read that produced it: a tree removed elsewhere shortens the list
// and an external drop renames every stash below it, so the place a read was
// issued for can hold something else by the time the answer arrives. identity
// is what the row is, as against where it sits.
//
// The clone is why this returns the slice rather than writing through: a State
// copy still points at the caller's array, so writing an element here would
// change the value the caller is holding. Apply copies the three maps at its
// entry for the same reason.
func replaceRowAt[T any](list []T, index int, row T, identity func(T) string) ([]T, bool) {
	if index < 0 || index >= len(list) {
		return nil, false
	}
	if identity(list[index]) != identity(row) {
		return nil, false
	}
	out := slices.Clone(list)
	out[index] = row
	return out, true
}

func applyStashRowUpdated(s State, e StashRowUpdated) State {
	// The SHA is what a stash is; its ref is only where it sits.
	list, ok := replaceRowAt(s.Stashed.Stashes, e.Index, e.Row,
		func(r git.StashRow) string { return r.SHA })
	if !ok {
		return s
	}
	s.Stashed.Stashes = list
	if s.Tab == TabStashed {
		s = refreshRows(s, keepCursorIndex)
	}
	return s
}

func applyCommitExpanded(s State, e CommitExpanded) State {
	s.History.ExpandedSHA, s.History.ExpandedFiles = e.SHA, e.Files
	if s.Tab == TabHistory {
		s = refreshRows(s, keepCursorRow)
	}
	return s
}

func applyDiffLoaded(s State, e DiffLoaded) State {
	// Moving the cursor asks for a diff per row, and the answers do not arrive in
	// the order they were asked for. An answer for a file the reader has left is
	// dropped, the same way StashFilesLoaded checks its Ref.
	if e.Diff.Path != "" && e.Diff.Path != s.Open.Path {
		return s
	}
	s.Open.Diff = e.Diff
	s.Open.BlockCursor = clampIndex(s.Open.BlockCursor, len(e.Diff.Blocks)-1)
	s.Open.Scroll = clampIndex(s.Open.Scroll, len(e.Diff.Blocks)-1)
	s.Open.BlockLine = 0
	delete(s.Changes.Stale, s.Open.Path)
	return s
}

func leaveForChanges(s State) State {
	clearSelected(&s)
	return goToTab(s, TabChanges)
}

// carryReads keeps what was already read about a row the new list still holds.
// A list names its rows; what each one holds is read separately, one git per
// row, and does not change because the list was read again. Replacing every row
// with an unread one blanked the counts and the verbs on every five-second
// poll, and they came back a few seconds later when the reads landed.
//
// key is what the row is, rather than where it sits: a stash's ref moves when a
// stash below it is dropped, and a worktree's place moves when one is removed.
// isRead reports whether a row has been read, so an unread one does not overwrite
// what is on screen with nothing.
func carryReads[T any](had, now []T, key func(T) string, isRead func(T) bool,
	carry func(into *T, from T)) []T {
	if len(had) == 0 {
		return now
	}
	before := make(map[string]T, len(had))
	for _, row := range had {
		if isRead(row) {
			before[key(row)] = row
		}
	}
	// A copy rather than a write into the caller's slice. Nothing observes the
	// write today — the two callers hand over a slice their own goroutine built
	// — but that is a promise every future caller would have to keep, and
	// applyStashRowUpdated clones for the same reason a few lines down.
	out := slices.Clone(now)
	for i := range out {
		if was, ok := before[key(out[i])]; ok && !isRead(out[i]) {
			carry(&out[i], was)
		}
	}
	return out
}

func carryStashReads(had, now []git.StashRow) []git.StashRow {
	return carryReads(had, now,
		func(r git.StashRow) string { return r.SHA },
		func(r git.StashRow) bool { return r.FileCount >= 0 },
		func(into *git.StashRow, from git.StashRow) {
			into.FileCount, into.Status, into.Collides = from.FileCount, from.Status, from.Collides
		})
}

func carryWorktreeReads(had, now []git.WorktreeRow) []git.WorktreeRow {
	return carryReads(had, now,
		func(r git.WorktreeRow) string { return r.Path },
		func(r git.WorktreeRow) bool { return r.Dirty >= 0 },
		func(into *git.WorktreeRow, from git.WorktreeRow) {
			into.Dirty, into.WroteAge = from.Dirty, from.WroteAge
			into.Merge, into.Conflicts = from.Merge, from.Conflicts
		})
}
