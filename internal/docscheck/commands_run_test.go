package docscheck_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// A command in a document is one a reader will paste. This asks the shell to
// read each of them and stops at the ones it cannot: an unbalanced quote, a
// stray operator, a pipe with nothing after it.
//
// What it does not catch is a command that parses and then does the wrong
// thing. The one that started this — `grep -c -- --- SKIP`, which reads `SKIP`
// as a filename — parses fine and fails at runtime, and `sh -n` returns 0 for
// it. Running the commands instead is not the answer either: they build
// repositories, install binaries and reach the network. That half is the
// author's, and CONTRIBUTING asks for it in the same words it asks a guard be
// broken once: run what you wrote and paste what it printed.
func TestEveryCommandInTheDocumentsParses(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no shell to ask")
	}
	root := filepath.Join("..", "..")
	// The opening fence may name a language and may be indented; both forms
	// are in these files. Matching only the bare form let a block be hidden by
	// adding one word to its fence — measured.
	fence := regexp.MustCompile("(?s)```[a-zA-Z]*\n(.*?)```")

	commands := make(map[string]bool)
	for _, name := range []string{"README.md", "CONTRIBUTING.md"} {
		body, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		for _, block := range fence.FindAllStringSubmatch(string(body), -1) {
			for _, line := range strings.Split(block[1], "\n") {
				line = strings.TrimSpace(line)
				if !isShellCommand(line) {
					continue
				}
				commands[line] = true
				if out, err := exec.Command("sh", "-n", "-c", line).CombinedOutput(); err != nil {
					t.Errorf("%s documents a command the shell cannot read:\n  %s\n  %s",
						name, line, strings.TrimSpace(string(out)))
				}
			}
		}
	}
	// Pin commands contributors and users copy so a stale prefix list cannot
	// silently skip the setup, check, installation, or launch instructions.
	for _, prefix := range []string{
		"git clone ", "cd mirugit", "make install", "make quick", "make ci-local",
		"make check", "make test-race", "make fuzz ", "make supply-chain",
		"make tools", "make cover-zero", "tar -xzf ", "mkdir -p ",
		"install -m 0755 ", "go install ", "mirugit",
	} {
		found := false
		for command := range commands {
			if strings.HasPrefix(command, prefix) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("README or CONTRIBUTING has no shell-checked command starting with %q", prefix)
		}
	}
}

// isShellCommand skips what the fenced blocks hold besides commands: drawings
// of the pane, output of a command, and prose about one.
//
// Keep command prefixes explicit so terminal drawings and command output are
// not sent to the shell parser.
func isShellCommand(line string) bool {
	if line == "" || strings.HasPrefix(line, "#") {
		return false
	}
	for _, start := range []string{
		"git ", "go ", "make", "tar ", "mkdir ", "install ", "mirugit", "cd ",
		"grep ", "sh ", "printf ", "echo ",
	} {
		if strings.HasPrefix(line, start) {
			return true
		}
	}
	return false
}
