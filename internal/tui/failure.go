package tui

import (
	"context"
	"errors"
	"os/exec"
	"strings"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// failureLine is the sentence a failure gets on screen. The type decides it:
// Error() is written for a person reading a log, and cutting it up in the layer
// that draws — which is what state used to do with firstLine — makes the shape
// of another package's message part of this program's behavior.
//
// cmd/mirugit already asks with errors.As, so this is the same question already
// answered once.
func failureLine(err error) string {
	if err == nil {
		return ""
	}

	if errors.Is(err, git.ErrGitMissing) {
		return "git is not on PATH"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "git took too long and was stopped"
	}
	if errors.Is(err, context.Canceled) {
		return "the read was canceled"
	}
	if errors.Is(err, exec.ErrNotFound) {
		return "a program mirugit needs is not on PATH"
	}

	var exit *git.ExitError
	if errors.As(err, &exit) {
		// git writes a usage block after the first line, and only the first
		// line fits. An exit with nothing on stderr says only its number.
		if line := firstLine(strings.TrimSpace(exit.Stderr)); line != "" {
			return line
		}
		return "git " + strings.Join(exit.Args, " ") + " failed"
	}

	return firstLine(err.Error())
}

// firstLine is what fits on the notice row.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// failed is the event for an error, with the sentence already chosen.
func failed(err error) state.Failed {
	return state.Failed{Line: failureLine(err)}
}

// countsIncomplete names what the reader lost. git's own line says a path would
// not open, which does not say that the rows are still every changed file and
// that only some of the figures beside them are gone. "Incomplete" rather than
// "missing" because the paths git could read keep their counts.
func countsIncomplete(err error) state.Failed {
	return state.Failed{Line: "line counts are incomplete: " + failureLine(err)}
}
