package main

import (
	"path/filepath"
	"testing"
)

// The read marks live under this path. Getting it wrong loses every mark, and
// the program has no other way to tell the reader that it did.
func TestStateDirectoryFollowsXDG(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/tmp/xdg")
	got, err := stateDirectory()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join("/tmp/xdg", "mirugit"); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStateDirectoryFallsBackToHome(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", "/tmp/home")
	got, err := stateDirectory()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join("/tmp/home", ".local", "state", "mirugit"); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
