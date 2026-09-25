package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Two of this package's commands hand a string to a program on the machine: y
// runs the clipboard, o runs the browser. Both take the program as an argument
// so a test can pass one that does nothing, and a test that forgets reaches the
// real one — measured, one run of the suite opened a browser tab at the address
// the fixture's remote builds and replaced the clipboard, once per run, for as
// long as that test existed.
//
// Setting them per test cannot be relied on: the fields are on a struct that
// forty tests build as a literal, and a new one that presses those keys is one
// line. This puts programs that do nothing in front of the real ones for the
// whole binary, so forgetting costs a failed assertion rather than a browser
// tab on someone's screen.
func standInForTheDesktop() (undo func(), err error) {
	dir, err := os.MkdirTemp("", "mirugit-no-desktop")
	if err != nil {
		return nil, err
	}
	was := os.Getenv("PATH")
	undo = func() {
		if err := os.Setenv("PATH", was); err != nil {
			panic(err)
		}
		if err := os.RemoveAll(dir); err != nil {
			panic(err)
		}
	}
	for _, name := range desktopPrograms {
		body := "#!/bin/sh\n" +
			"echo 'a test reached " + name + ": pass a command to the Model instead' >&2\n" +
			"cat > /dev/null 2>&1\n" +
			"exit 1\n"
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
			undo()
			return nil, err
		}
	}
	if err := os.Setenv("PATH", dir+string(os.PathListSeparator)+was); err != nil {
		undo()
		return nil, err
	}
	return undo, nil
}

// The programs osproc names, on both platforms it builds for: a test runs on
// one of them and the guard is written once.
var desktopPrograms = []string{"open", "xdg-open", "pbcopy", "xclip"}

// The stand-ins are only in front while PATH says so, and a test that sets PATH
// for itself puts the real ones back. This says so out loud rather than leaving
// it to be discovered by a browser tab.
func TestNoTestPutsTheRealDesktopProgramsBack(t *testing.T) {
	t.Parallel()
	path := os.Getenv("PATH")
	first, _, _ := strings.Cut(path, string(os.PathListSeparator))
	for _, name := range desktopPrograms {
		if _, err := os.Stat(filepath.Join(first, name)); err != nil {
			t.Errorf("%s is not the stand-in directory: %v", first, err)
			return
		}
	}
}

// y and o hand a string to a program on the machine, and both take that program
// as a field so a test can pass one that does nothing. The field is what the
// Model must use: falling back to the real one while a test has set a stand-in
// opens a browser tab on the reader's screen and replaces their clipboard,
// which is what the stand-ins in this file exist to stop being discovered by.
func TestTheModelRunsTheCommandItWasGiven(t *testing.T) {
	t.Parallel()
	const marker = "mirugit-test-marker"
	var m Model
	m.clipboard = func(string) *exec.Cmd { return exec.Command(marker, "clipboard") }
	m.browser = func(string) *exec.Cmd { return exec.Command(marker, "browser") }

	if got := m.clipboardCommand()("x").Args; got[0] != marker || got[1] != "clipboard" {
		t.Errorf("y runs %v, not the command the test gave it", got)
	}
	if got := m.browserCommand()("x").Args; got[0] != marker || got[1] != "browser" {
		t.Errorf("o runs %v, not the command the test gave it", got)
	}

	// A Model with no stand-in runs the real one, or the answers above are
	// what it always gives.
	var plain Model
	if plain.clipboardCommand() == nil {
		t.Error("a Model with no clipboard command answers nothing")
	}
	if plain.browserCommand() == nil {
		t.Error("a Model with no browser command answers nothing")
	}
	if got := plain.browserCommand()("https://example.invalid/x").Args; got[0] == marker {
		t.Errorf("a Model with no stand-in runs %v", got)
	}
}
