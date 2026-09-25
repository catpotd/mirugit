package tui

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// The notice row used to hold the first line of Error(). Error() is written for
// a person reading a log, and its shape is not a promise: cutting it up in the
// layer that draws made another package's wording part of this program's
// behavior, and put the decision in state, which is the furthest from the
// screen.
func TestTheFailureLineIsChosenByTheTypeOfTheError(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		err  error
		says string
	}{
		{"git is absent", git.ErrGitMissing, "git is not on PATH"},
		{"wrapped", fmt.Errorf("reading the log: %w", git.ErrGitMissing), "git is not on PATH"},
		{"a deadline", context.DeadlineExceeded, "took too long"},
		{"a cancel", context.Canceled, "canceled"},
		{"another program is absent", exec.ErrNotFound, "not on PATH"},
		{"git exited with a message",
			&git.ExitError{Args: []string{"stash", "pop"}, Code: 1,
				Stderr: "error: could not restore\nusage: git stash\n"},
			"could not restore"},
		{"git exited saying nothing",
			&git.ExitError{Args: []string{"stash", "pop"}, Code: 129},
			"git stash pop failed"},
		{"something else", errors.New("a plain failure\nand more"), "a plain failure"},
		{"nothing at all", nil, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := failureLine(c.err)
			if c.says == "" {
				if got != "" {
					t.Errorf("a nil error produced %q", got)
				}
				return
			}
			if !strings.Contains(got, c.says) {
				t.Errorf("failureLine = %q, want it to say %q", got, c.says)
			}
			// The notice row holds one line.
			if strings.Contains(got, "\n") {
				t.Errorf("the line has a newline in it: %q", got)
			}
		})
	}
}

// git writes a usage block under the first line of an error, and the row holds
// one line. Showing the block would push the sentence off the screen.
func TestOnlyTheFirstLineOfGitsComplaintIsShown(t *testing.T) {
	t.Parallel()
	err := &git.ExitError{
		Args:   []string{"stash", "pop"},
		Code:   1,
		Stderr: "error: your local changes would be overwritten\nusage: git stash pop\n   or: git stash list\n",
	}
	got := failureLine(err)
	if got != "error: your local changes would be overwritten" {
		t.Errorf("failureLine = %q", got)
	}
}

// The rows and the figures beside them come from two different git calls, and
// the second refuses the whole repository when one path will not open. The
// reader asked which files changed; the answer is still every one of them, and
// the notice has to say that the missing part is the figures.
func TestCountsThatCouldNotBeReadLeaveTheRowsAndSaySo(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := repoWithTwoChangedFiles(t)
	m, err := New(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo, err := git.Load(context.Background(), dir, mustGitDir(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	repo.CountsErr = &git.ExitError{
		Args:   []string{"diff", "--numstat", "-z"},
		Code:   128,
		Stderr: "error: open(\"a.txt\"): Permission denied\nfatal: cannot hash a.txt\n",
	}

	next, _ := m.Update(repoMsgFor(t, dir, repo))
	after := next.(*Model)
	files := 0
	for _, row := range after.state.Rows {
		if row.Kind() == state.RowFile {
			files++
		}
	}
	if files != 2 {
		t.Errorf("file rows = %d, want both files", files)
	}
	if !strings.Contains(after.state.Notice, "line counts are incomplete") {
		t.Errorf("the notice does not say what is missing: %q", after.state.Notice)
	}
	if !strings.Contains(after.state.Notice, "Permission denied") {
		t.Errorf("the notice does not say why: %q", after.state.Notice)
	}
}
