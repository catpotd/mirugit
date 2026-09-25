package git

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// README tells the reader which git commands mirugit never runs, and calls the
// list closed. A closed list has to be checked: a key added later that reaches
// for one of these would make the README a promise the program breaks.
//
// Every git this program runs goes through one of the run helpers in run.go,
// and today every one of them names its subcommand as a string literal, which
// is what this reads. A subcommand handed in through a variable gets past it —
// measured, by writing one and watching this stay green. Nothing stops that
// from being written; what stops it from surviving is the reviewer, and this
// comment saying so.
func TestTheCommandsTheReadmeSaysAreNeverRunAreNeverRun(t *testing.T) {
	t.Parallel()
	// The commands the README names. checkout and switch move you between
	// commits; the rest rewrite history or resolve a merge.
	// "branch" is not on this list and cannot be: b runs `git stash branch`,
	// and the check matches the literal, which is the point — README names that
	// key as an exception rather than pretending the command is never reached.
	for _, command := range []string{
		"checkout", "switch", "merge", "rebase", "cherry-pick", "revert", "am",
		"filter-branch", "tag",
	} {
		if where := runsGitCommand(t, command); where != "" {
			t.Errorf("README says mirugit never runs git %s, and %s does",
				command, where)
		}
	}
}

// runsGitCommand answers where a subcommand is handed to a run helper, and ""
// when nothing does. It reads the sources rather than the binary because a
// string that never runs is still a promise broken the moment someone calls it.
func runsGitCommand(t *testing.T, command string) string {
	t.Helper()
	// A run helper takes the subcommand as the first string after its
	// arguments: runWrite(dir, "stash", "push") and runRead(dir, "log", ...).
	call := regexp.MustCompile(
		`run(?:Read|Write|WritePaths|Network|WithEnv)\([^)]*?"` + regexp.QuoteMeta(command) + `"`)

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(".", name))
		if err != nil {
			t.Fatal(err)
		}
		if loc := call.FindIndex(body); loc != nil {
			line := 1 + strings.Count(string(body[:loc[0]]), "\n")
			return name + ":" + itoa(line)
		}
	}
	return ""
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
