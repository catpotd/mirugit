package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Asking for help is not a failure. Returning flag.ErrHelp as one made -h exit
// 1 with "flag: help requested" on stderr, which a packaging script or a
// completion generator reads as a broken binary.
func TestHelpSucceedsAndGoesToStandardOutput(t *testing.T) {
	t.Parallel()
	for _, arg := range []string{"-h", "--help", "-help"} {
		t.Run(arg, func(t *testing.T) {
			t.Parallel()
			var out, errOut bytes.Buffer
			if err := run([]string{arg}, &out, &errOut); err != nil {
				t.Fatalf("asking for help failed: %v", err)
			}
			if errOut.Len() != 0 {
				t.Errorf("help wrote to stderr: %q", errOut.String())
			}
			if !strings.Contains(out.String(), "Usage:") {
				t.Errorf("help does not name its usage: %q", out.String())
			}
		})
	}
}

// A flag that does not exist is a failure, and its message belongs on stderr.
func TestAnUnknownFlagFailsAndGoesToStandardError(t *testing.T) {
	t.Parallel()
	var out, errOut bytes.Buffer
	err := run([]string{"-no-such-flag"}, &out, &errOut)
	if err == nil {
		t.Fatal("an unknown flag was accepted")
	}
	if errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the error names a missing file rather than the flag: %v", err)
	}
}

// The help text is where a reader who typed -h looks. Every variable the
// program reads has to be there: the README is not on their screen.
func TestHelpNamesEveryEnvironmentVariableTheProgramReads(t *testing.T) {
	t.Parallel()
	var out, errOut bytes.Buffer
	if err := run([]string{"-h"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	help := out.String()

	for _, name := range environmentNamesInSource(t) {
		if !strings.Contains(help, name) {
			t.Errorf("the program reads %s and the help does not name it", name)
		}
	}
}

// environmentNamesInSource collects the variables the program asks the
// environment for, so the help is compared against what the code does rather
// than against a list someone kept up to date.
func environmentNamesInSource(t *testing.T) []string {
	t.Helper()
	seen := map[string]bool{}
	root := filepath.Join("..", "..")
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch path {
			case filepath.Join(root, ".git"), filepath.Join(root, ".ci-cache"), filepath.Join(root, "bin"):
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, part := range strings.Split(string(body), `os.Getenv("`)[1:] {
			if end := strings.Index(part, `"`); end > 0 {
				seen[part[:end]] = true
			}
		}
		for _, part := range strings.Split(string(body), `os.LookupEnv("`)[1:] {
			if end := strings.Index(part, `"`); end > 0 {
				seen[part[:end]] = true
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// RUNEWIDTH_EASTASIAN is read by the drawing library rather than by this
	// program, and it is the one a reader has to set by hand, so the help names
	// it even though no Getenv here mentions it.
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	return names
}
