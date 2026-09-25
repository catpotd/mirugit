//go:build windows

package git

import (
	"os/exec"
	"time"
)

// killWholeTree is the Unix process-group kill, which Windows has no equivalent
// of: it groups processes with job objects instead. This file exists so the
// package compiles there and the refusal in cmd/mirugit is what a Windows
// reader meets, rather than a compile error inside a package they did not name.
func killWholeTree(*exec.Cmd) {}

// stopGroup has nothing to signal: see killWholeTree. Nothing was killed, so
// nothing was cut short.
func stopGroupOwned(int, <-chan struct{}, time.Duration) bool { return true }
