package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The read marks are opened before the pane is. A start that cannot open them
// has to stop and say so: carrying on runs the pane over a model that was never
// built, and the reader is left with a crash where a sentence belonged.
func TestAStartThatCannotOpenTheReadMarksStops(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to git")
	}
	repo := t.TempDir() + "/repo"
	if out, err := exec.Command("git", "init", "-q", "-b", "main", repo).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	// A file where the state directory has to be: every path under it is
	// unreadable, and the failure is not "it is not there yet".
	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_STATE_HOME", blocker)

	var out, errOut bytes.Buffer
	if err := run([]string{"-C", repo}, &out, &errOut); err == nil {
		t.Error("a start that could not open the read marks reported success")
	}
}
