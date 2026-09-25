//go:build windows

package main

// unsupportedPlatform refuses on a platform nobody has run this on. Windows has
// no process group to signal, so a git that hangs cannot be ended the way it is
// everywhere else, and no test in this repository has ever run there. Shipping
// a binary in that state would be claiming support that was never checked.
//
// Making it work is a small change — job objects for the kill, windows in the
// CI matrix and in the release targets — and this line is what has to go first.
const unsupportedPlatform = "mirugit has not been tested on Windows and does not run there: " +
	"it ends a hung git by signalling its process group, which Windows has no equivalent of"
