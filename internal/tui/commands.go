package tui

import (
	"context"
	"errors"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"os/exec"

	"github.com/catpotd/mirugit/internal/state"
)

// Every repository call is a command. git costs about 20 ms per invocation and
// a frame cannot wait for it.
// answering runs a repository call and turns it into the message the update
// loop reads. Ten commands wrote the same shape by hand, and a mutation that
// swapped the test in one of them survived the suite: a failed fetch reported
// success and a successful one reported an error nobody could see.
//
// The failure carries only the error, because a call that did not run has
// nothing else to say.
// wanted drops the answer to a read the reader has moved past. A canceled git
// comes back as an error, and reporting it would put a failure on the notice
// row for something the reader caused by pressing a key. A command that returns
// nil sends no message at all.
//
// Only reads are wrapped. A write that finished has changed the repository, and
// the reader has to be told even if the cursor moved while it ran.
func wanted(ctx context.Context, cmd tea.Cmd) tea.Cmd {
	return func() tea.Msg {
		msg := cmd()
		if ctx.Err() != nil {
			return nil
		}
		return msg
	}
}

func answering[T failing](run func() error, failed func(error) T, done T) tea.Cmd {
	return func() tea.Msg {
		if err := run(); err != nil {
			return failed(err)
		}
		return done
	}
}

func loadRepo(ctx context.Context, dir, gitDir string) tea.Cmd {
	return wanted(ctx, func() tea.Msg {
		r, err := git.Load(ctx, dir, gitDir)
		if err != nil {
			return repoMsg{Err: err}
		}
		page, logErr := git.LogPage(ctx, dir, 0)
		if logErr != nil {
			return repoMsg{Repo: r, Err: logErr}
		}
		// The list only: what each stash holds costs a git per stash and is
		// drawn by the stashed tab, which fills it when it is opened.
		stashes, stashErr := git.StashList(ctx, dir)
		if stashErr != nil {
			return repoMsg{Repo: r, Commits: page.Commits, HasMore: page.HasMore, Err: stashErr}
		}
		hashes, hashErr := state.ReadBlockHashes(ctx, dir)
		if hashErr != nil {
			return repoMsg{Repo: r, Commits: page.Commits, HasMore: page.HasMore, Stashes: stashes, Err: hashErr}
		}
		return repoMsg{Repo: r, Commits: page.Commits, HasMore: page.HasMore, Stashes: stashes, Hashes: hashes}
	})
}

func loadStashes(ctx context.Context, dir string) tea.Cmd {
	return wanted(ctx, func() tea.Msg {
		_, head, err := git.Status(ctx, dir)
		if err != nil {
			return stashedMsg{err: err}
		}
		stashes, err := git.StashList(ctx, dir)
		if err != nil {
			return stashedMsg{err: err}
		}
		return stashedMsg{stashes: stashes, head: head}
	})
}

// stashStatusCmds reads what each stash holds, one command per stash. Reading
// them inside the list would lose the list when one failed, and the reads can
// fail on their own: a stash dropped from another terminal between the list and
// the read is gone by the time this asks.
//
// This is worktreeStatusCmds for stashes, for the same reasons.
func stashStatusCmds(ctx context.Context, dir string, stashes []git.StashRow, head git.Head) tea.Cmd {
	cmds := make([]tea.Cmd, len(stashes))
	for i := range stashes {
		cmds[i] = fillStashStatus(ctx, dir, i, stashes[i], head)
	}
	return tea.Batch(cmds...)
}

func fillStashStatus(ctx context.Context, dir string, index int, row git.StashRow, head git.Head) tea.Cmd {
	return fillRowStatus(ctx, row,
		func(into *git.StashRow) error { return git.FillStashStatus(ctx, dir, into, head) },
		func(updated git.StashRow, err error) stashStatusMsg {
			return stashStatusMsg{index: index, sha: row.SHA, row: updated, err: err}
		})
}

// fillRowStatus reads one row's status in the background and answers with the
// row it read, or with the error. The two tabs that do this wrote it twice, and
// the copies disagreed about whether the answer carries the row it failed on.
func fillRowStatus[Row any, Msg tea.Msg](ctx context.Context, row Row, read func(*Row) error,
	answer func(Row, error) Msg) tea.Cmd {
	return wanted(ctx, func() tea.Msg {
		updated := row
		if err := read(&updated); err != nil {
			var zero Row
			return answer(zero, err)
		}
		return answer(updated, nil)
	})
}

func loadWorktrees(ctx context.Context, dir string) tea.Cmd {
	return wanted(ctx, func() tea.Msg {
		trees, base, err := git.WorktreeList(ctx, dir)
		if err != nil {
			return worktreesMsg{err: err}
		}
		return worktreesMsg{trees: trees, base: base}
	})
}

func worktreeStatusCmds(ctx context.Context, trees []git.WorktreeRow, base string) tea.Cmd {
	cmds := make([]tea.Cmd, len(trees))
	for i := range trees {
		idx, row := i, trees[i]
		cmds[i] = fillWorktreeStatus(ctx, idx, row, base)
	}
	return tea.Batch(cmds...)
}

func fillWorktreeStatus(ctx context.Context, index int, row git.WorktreeRow, base string) tea.Cmd {
	return fillRowStatus(ctx, row,
		func(into *git.WorktreeRow) error { return git.FillWorktreeStatus(ctx, into, base) },
		func(updated git.WorktreeRow, err error) worktreeStatusMsg {
			return worktreeStatusMsg{index: index, path: row.Path, row: updated, err: err}
		})
}

func runWorktreeRemove(ctx context.Context, startDir, path string) tea.Cmd {
	return answering(
		func() error { return git.RemoveWorktree(ctx, startDir, path) },
		func(err error) worktreeRemoveMsg { return worktreeRemoveMsg{err: err} },
		worktreeRemoveMsg{})
}

func loadHistory(ctx context.Context, dir string) tea.Cmd {
	return wanted(ctx, func() tea.Msg {
		page, err := git.LogPage(ctx, dir, 0)
		if err != nil {
			return historyMsg{err: err}
		}
		return historyMsg{commits: page.Commits, hasMore: page.HasMore}
	})
}

func loadHistoryMore(ctx context.Context, dir string, offset int, newestSHA string) tea.Cmd {
	return wanted(ctx, func() tea.Msg {
		page, err := git.LogPage(ctx, dir, offset)
		return historyMoreMsg{offset: offset, newestSHA: newestSHA, page: page, err: err}
	})
}

func loadFetched(ctx context.Context, dir string) tea.Cmd {
	return wanted(ctx, func() tea.Msg {
		at, err := git.FetchHeadModTime(ctx, dir)
		if err != nil {
			return fetchedMsg{err: err}
		}
		return fetchedMsg{at: at}
	})
}

func loadCommitFiles(ctx context.Context, dir, sha string) tea.Cmd {
	return wanted(ctx, func() tea.Msg {
		files, err := git.CommitFiles(ctx, dir, sha)
		if err != nil {
			return commitFilesMsg{err: err}
		}
		return commitFilesMsg{sha: sha, files: files}
	})
}

func loadCommitDiff(ctx context.Context, dir, sha, path string) tea.Cmd {
	return wanted(ctx, func() tea.Msg {
		d, err := git.CommitDiff(ctx, dir, sha, path)
		return diffMsg{Diff: d, Err: err}
	})
}

func runUndoCommit(ctx context.Context, dir string) tea.Cmd {
	return answering(
		func() error { return git.ResetSoft(ctx, dir) },
		func(err error) undoMsg { return undoMsg{err: err} },
		undoMsg{})
}

func loadDiff(ctx context.Context, dir, path string, staged bool) tea.Cmd {
	return wanted(ctx, func() tea.Msg {
		d, err := git.Diff(ctx, dir, path, staged)
		return diffMsg{Diff: d, Err: err}
	})
}

func loadStashDiff(ctx context.Context, dir, sha, path string) tea.Cmd {
	return wanted(ctx, func() tea.Msg {
		d, err := git.StashDiff(ctx, dir, sha, path)
		return diffMsg{Diff: d, Err: err}
	})
}

func loadWorktreeDiff(ctx context.Context, worktreeDir, path string, staged bool) tea.Cmd {
	return wanted(ctx, func() tea.Msg {
		d, err := git.Diff(ctx, worktreeDir, path, staged)
		return diffMsg{Diff: d, Err: err}
	})
}

func loadReadMarks(ctx context.Context, dir, path string, staged bool, origin state.Origin) tea.Cmd {
	return wanted(ctx, func() tea.Msg {
		d, err := git.Diff(ctx, dir, path, staged)
		if err != nil {
			return readMsg{err: err}
		}
		hashes := make([]string, len(d.Blocks))
		for i, b := range d.Blocks {
			hashes[i] = b.Hash
		}
		return readMsg{path: path, origin: origin, hashes: hashes}
	})
}

func runCopySHA(clipboard func(string) *exec.Cmd, sha string) tea.Cmd {
	return answering(
		func() error { return clipboard(sha).Run() },
		func(err error) verbMsg { return verbMsg{err: err} },
		verbMsg{finished: state.VerbFinished{Notice: "copied " + sha}})
}

func runOpenCommit(browser func(string) *exec.Cmd, url string) tea.Cmd {
	return answering(
		func() error { return browser(url).Run() },
		func(err error) verbMsg { return verbMsg{err: err} },
		verbMsg{finished: state.VerbFinished{}})
}

func verbCmd(ctx context.Context, dir string, verb state.Verb, entries []git.Entry) tea.Cmd {
	return func() tea.Msg {
		var (
			outcome git.Outcome
			err     error
		)
		switch verb {
		case state.VerbStage:
			outcome, err = git.Stage(ctx, dir, entries)
		case state.VerbUnstage:
			outcome, err = git.Unstage(ctx, dir, entries)
		case state.VerbStash:
			outcome, err = git.Stash(ctx, dir, entries)
			if err != nil {
				var oversized *git.OversizedPathspecError
				if errors.As(err, &oversized) {
					return verbMsg{finished: state.VerbFinished{
						Verb:   state.VerbStash,
						Notice: err.Error(),
					}}
				}
				return verbMsg{err: err}
			}
		case state.VerbDiscard, state.VerbUndo:
			// Both go through their own confirmation and their own command,
			// because each takes a snapshot before it writes.
			return verbMsg{err: fmt.Errorf("verb %d does not run here", verb)}
		}
		if err != nil {
			return verbMsg{err: err}
		}
		ignored := make([]string, len(outcome.Ignored))
		for i, e := range outcome.Ignored {
			ignored[i] = e.Path
		}
		return verbMsg{finished: state.VerbFinished{
			Verb:    verb,
			Applied: len(outcome.Applied),
			Ignored: ignored,
		}}
	}
}

func snapshotForDiscard(ctx context.Context, dir string, entries []git.Entry, confirm state.DiscardConfirm) tea.Cmd {
	return func() tea.Msg {
		undo, err := git.Snapshot(ctx, dir, entries, "mirugit discard")
		if err != nil {
			return snapshotMsg{err: err}
		}
		confirm.Undo = undo
		return snapshotMsg{confirm: confirm}
	}
}

func runDiscard(ctx context.Context, dir string, entries []git.Entry, undo git.Undo, counts state.DiscardConfirm) tea.Cmd {
	return func() tea.Msg {
		outcome, err := git.Discard(ctx, dir, entries, undo)
		if err != nil {
			return discardMsg{err: err}
		}
		return discardMsg{finished: state.DiscardFinished{
			Applied: len(outcome.Applied),
			Added:   counts.Added,
			Deleted: counts.Deleted,
			Undo:    git.RecordWorktreeAfter(dir, undo),
		}}
	}
}

func runUndiscard(ctx context.Context, dir string, undo git.Undo) tea.Cmd {
	return func() tea.Msg {
		outcome, err := git.Undiscard(ctx, dir, undo)
		if err != nil {
			return undiscardMsg{err: err}
		}
		return undiscardMsg{finished: state.DiscardFinished{
			Applied:  len(outcome.Applied),
			Undo:     undo,
			Restored: true,
		}}
	}
}

func runCommit(ctx context.Context, dir, message string, files int) tea.Cmd {
	return func() tea.Msg {
		sha, err := git.Commit(ctx, dir, message)
		if err != nil {
			return commitMsg{err: err}
		}
		return commitMsg{finished: state.CommitFinished{SHA: sha, Files: files}}
	}
}

func runFetch(ctx context.Context, dir string) tea.Cmd {
	return answering(
		func() error { return git.Fetch(ctx, dir) },
		func(err error) verbMsg { return verbMsg{err: err} },
		verbMsg{finished: state.VerbFinished{}})
}

func runSync(ctx context.Context, dir string) tea.Cmd {
	return func() tea.Msg {
		err := git.Sync(ctx, dir)
		if errors.Is(err, git.ErrWouldMerge) {
			return verbMsg{finished: state.VerbFinished{
				Notice: "pull would merge · resolve in your terminal",
			}}
		}
		if err != nil {
			return verbMsg{err: err}
		}
		return verbMsg{finished: state.VerbFinished{}}
	}
}

func runStageBlock(ctx context.Context, dir string, diff git.FileDiff, block int) tea.Cmd {
	return answering(
		func() error { return git.StageBlock(ctx, dir, diff, block) },
		func(err error) verbMsg { return verbMsg{err: err} },
		verbMsg{finished: state.VerbFinished{Verb: state.VerbStage, Applied: 1}})
}

// runStashRestore carries the file count from the row the list drew rather than
// reading it back afterwards: the stash is gone by then.
func runStashRestore(ctx context.Context, dir, ref string, files int) tea.Cmd {
	return answering(
		func() error { return git.StashPop(ctx, dir, ref) },
		func(err error) stashVerbMsg { return stashVerbMsg{err: err} },
		stashVerbMsg{restored: files})
}

func runStashBranch(ctx context.Context, dir, ref, base string) tea.Cmd {
	return func() tea.Msg {
		branch := git.FreeBranchName(ctx, dir, base)
		if err := git.StashBranch(ctx, dir, ref, branch); err != nil {
			return stashVerbMsg{err: err}
		}
		return stashVerbMsg{branch: branch}
	}
}

func runStashDrop(ctx context.Context, dir, sha string) tea.Cmd {
	return answering(
		func() error { return git.StashDropBySHA(ctx, dir, sha) },
		func(err error) stashDropMsg { return stashDropMsg{err: err} },
		stashDropMsg{})
}

// saveRead writes the read marks off the update goroutine. The bytes are taken
// before the command runs, so the model is free to change the marks while the
// file is being replaced.
func (m *Model) saveRead() tea.Cmd {
	encoded, path, err := m.read.Snapshot()
	if err != nil {
		return func() tea.Msg { return readSavedMsg{err: err} }
	}
	return func() tea.Msg {
		return readSavedMsg{err: state.WriteSnapshot(encoded, path)}
	}
}
