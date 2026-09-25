package osproc

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestClipboardCommandCarriesTheSha(t *testing.T) {
	t.Parallel()
	cmd := ClipboardCommand("abc123")
	want := "xclip"
	if runtime.GOOS == "darwin" {
		want = "pbcopy"
	}
	if cmd.Args[0] != want {
		t.Errorf("args = %v, want %s first", cmd.Args, want)
	}
	if cmd.Process != nil {
		t.Error("the command should be built, not run")
	}
	sha, err := io.ReadAll(cmd.Stdin)
	if err != nil {
		t.Fatal(err)
	}
	if string(sha) != "abc123" {
		t.Errorf("stdin = %q", sha)
	}
}

func TestBrowserCommandCarriesTheURL(t *testing.T) {
	t.Parallel()
	url := "https://example.com/commit/abc"
	cmd := BrowserCommand(url)
	want := "xdg-open"
	if runtime.GOOS == "darwin" {
		want = "open"
	}
	if cmd.Args[0] != want {
		t.Errorf("args = %v, want %s first", cmd.Args, want)
	}
	if len(cmd.Args) != 2 || cmd.Args[1] != url {
		t.Errorf("args = %v, want the url last", cmd.Args)
	}
	if cmd.Process != nil {
		t.Error("the command should be built, not run")
	}
}

// The two Ready answers decide whether y and o are offered at all: the Renderer
// carries them and the verb lists read them. Nothing reached either, nor
// toolReady under them: measured with go tool cover, all three were at 0.0%.
//
// PATH is replaced with a directory holding one name, so the answer is this
// machine's question rather than whichever tools happen to be installed.
func TestAToolIsOfferedOnlyWhenThisPlatformsNameIsOnPath(t *testing.T) {
	for _, c := range []struct {
		name    string
		present string
		ready   func() bool
	}{
		{"a clipboard", clipboardToolName(), ClipboardReady},
		{"a browser", browserToolName(), BrowserReady},
	} {
		t.Run(c.name, func(t *testing.T) {
			empty := t.TempDir()
			t.Setenv("PATH", empty)
			if c.ready() {
				t.Errorf("%s is offered with nothing on PATH", c.name)
			}

			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, c.present),
				[]byte("#!/bin/sh\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir)
			if !c.ready() {
				t.Errorf("%s is not offered with %s on PATH", c.name, c.present)
			}
		})
	}
}

// The name is chosen by platform, and choosing the other one answers "no tool"
// on a machine that has the right one. macOS has pbcopy and open; a Linux box
// has neither, and xclip is not installed by default.
func TestTheOtherPlatformsNameIsNotWhatIsLookedFor(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{otherClipboardName(), otherBrowserName()} {
		if err := os.WriteFile(filepath.Join(dir, name),
			[]byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)

	if ClipboardReady() {
		t.Errorf("a clipboard is offered with only %s on PATH", otherClipboardName())
	}
	if BrowserReady() {
		t.Errorf("a browser is offered with only %s on PATH", otherBrowserName())
	}
}

func clipboardToolName() string {
	if runtime.GOOS == "darwin" {
		return "pbcopy"
	}
	return "xclip"
}

func browserToolName() string {
	if runtime.GOOS == "darwin" {
		return "open"
	}
	return "xdg-open"
}

func otherClipboardName() string {
	if runtime.GOOS == "darwin" {
		return "xclip"
	}
	return "pbcopy"
}

func otherBrowserName() string {
	if runtime.GOOS == "darwin" {
		return "xdg-open"
	}
	return "open"
}
