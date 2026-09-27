package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
)

// A bug report that cannot name the build is the case this flag exists for, so
// the flag has to answer even when nothing filled the version in.
func TestVersionFlagNamesTheBuild(t *testing.T) {
	t.Parallel()
	var out, errOut bytes.Buffer
	if err := run([]string{"-version"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	line := strings.TrimSpace(out.String())
	if !strings.HasPrefix(line, "mirugit ") {
		t.Errorf("出力 = %q, want mirugit で始まる1行", line)
	}
	if strings.TrimSpace(strings.TrimPrefix(line, "mirugit")) == "" {
		t.Error("版が空")
	}
	if errOut.Len() != 0 {
		t.Errorf("stderr = %q, want 空", errOut.String())
	}
}

func TestVersionFlagDoesNotOpenTheRepository(t *testing.T) {
	t.Parallel()
	var out, errOut bytes.Buffer
	// A directory that is not a repository. -version answers before the lookup,
	// so this must not fail.
	if err := run([]string{"-version", "-C", t.TempDir()}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
}

func TestVersionFlagDoesNotStartUpdateChecking(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)
	var out, errOut bytes.Buffer
	if err := run([]string{"-version"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(stateHome, "mirugit")); !os.IsNotExist(err) {
		t.Fatalf("version flag created state: %v", err)
	}
}

// Without ldflags the version comes from the module system, and that is what a
// build from source answers with. "(unknown)" is for a build the module system
// cannot name, so printing it over an answer it did give loses the one thing a
// bug report needs.
// Not parallel: it writes the package variable the other two read.
func TestTheModulesAnswerIsPrintedWhenNothingWasInjected(t *testing.T) {
	t.Cleanup(func() { version = "" })
	version = ""
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" {
		t.Skip("this build carries no version of its own, so this proves nothing")
	}
	if got := versionString(); got != info.Main.Version {
		t.Errorf("versionString() = %q, want the build's own %q", got, info.Main.Version)
	}
}

// An ldflags build is what a release ships, and its version has to win over the
// module system's answer.
// Not parallel: it writes the package variable the other two read.
func TestAnInjectedVersionWins(t *testing.T) {
	t.Cleanup(func() { version = "" })
	version = "v9.9.9"
	if got := versionString(); got != "v9.9.9" {
		t.Errorf("versionString() = %q, want v9.9.9", got)
	}
}

// The module system says nothing in two ways, and a bug report needs to be able
// to tell them from a build that did answer. Neither way can be arranged around
// a real build, so the answer is handed over rather than read here.
func TestAVersionTheModuleSystemDidNotGiveReadsAsUnknown(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		info *debug.BuildInfo
		ok   bool
		want string
	}{
		{"no build info at all", nil, false, "(unknown)"},
		{"build info with no version in it", &debug.BuildInfo{}, true, "(unknown)"},
		{"a build that named itself", &debug.BuildInfo{
			Main: debug.Module{Version: "v1.2.3"}}, true, "v1.2.3"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := versionFromBuild(c.info, c.ok); got != c.want {
				t.Errorf("versionFromBuild = %q, want %q", got, c.want)
			}
		})
	}
}
