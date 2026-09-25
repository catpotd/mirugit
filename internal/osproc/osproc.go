// Package osproc runs host tools other than git. Clipboard and browser helpers
// live here so layout stays free of exec.
package osproc

import (
	"os/exec"
	"runtime"
	"strings"
)

// ClipboardReady reports whether a clipboard tool exists on this machine.
func ClipboardReady() bool { return toolReady("pbcopy", "xclip") }

// BrowserReady reports whether a browser opener exists on this machine.
func BrowserReady() bool { return toolReady("open", "xdg-open") }

// toolReady looks for the program that does the job on this platform. macOS
// ships its own name for each of these, and the Linux one is not installed by
// default, so the answer decides whether the verb is offered at all.
func toolReady(darwin, other string) bool {
	name := other
	if runtime.GOOS == "darwin" {
		name = darwin
	}
	_, err := exec.LookPath(name)
	return err == nil
}

// ClipboardCommand builds the command that copies sha to the clipboard.
func ClipboardCommand(sha string) *exec.Cmd {
	if runtime.GOOS == "darwin" {
		cmd := exec.Command("pbcopy")
		cmd.Stdin = strings.NewReader(sha)
		return cmd
	}
	cmd := exec.Command("xclip", "-selection", "clipboard")
	cmd.Stdin = strings.NewReader(sha)
	return cmd
}

// BrowserCommand builds the command that opens url in the default browser.
func BrowserCommand(url string) *exec.Cmd {
	if runtime.GOOS == "darwin" {
		return exec.Command("open", url)
	}
	return exec.Command("xdg-open", url)
}
