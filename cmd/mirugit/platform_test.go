package main

import (
	"runtime"
	"strings"
	"testing"
)

// The refusal has to name the platform and the reason. "not supported" alone
// sends a reader looking for a flag that turns it on.
func TestTheRefusalSaysWhichPlatformAndWhy(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "windows" {
		if unsupportedPlatform != "" {
			t.Fatalf("%s refuses to run: %q", runtime.GOOS, unsupportedPlatform)
		}
		return
	}
	for _, want := range []string{"Windows", "process group"} {
		if !strings.Contains(unsupportedPlatform, want) {
			t.Errorf("the refusal does not mention %q: %q", want, unsupportedPlatform)
		}
	}
}
