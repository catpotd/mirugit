// Package main reads the flags, resolves the top level of the repository,
// and runs the tui model under bubbletea.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/tui"
)

// version is filled in at release time with -ldflags -X main.version. A build
// without it falls back to what the module system recorded, so a binary from
// go install still names the tag it came from.
var version string

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "mirugit:", err)
		os.Exit(1)
	}
}

func run(args []string, out, errOut io.Writer) error {
	flags := flag.NewFlagSet("mirugit", flag.ContinueOnError)
	flags.SetOutput(out)
	dir := flags.String("C", ".", "repository to show")
	showVersion := flags.Bool("version", false, "print the version and exit")
	flags.Usage = func() { usage(out, flags) }
	if err := flags.Parse(args); err != nil {
		// Asking for help is not a failure. Returning it as one made -h exit 1
		// with "flag: help requested" on stderr, which a packaging script or a
		// completion generator reads as a broken binary.
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		flags.SetOutput(errOut)
		return err
	}
	if *showVersion {
		_, err := fmt.Fprintln(out, "mirugit", versionString())
		return err
	}

	if unsupportedPlatform != "" {
		return errors.New(unsupportedPlatform)
	}

	// Every git this program starts descends from this one, so the deferred
	// cancel reaches the ones still running when the pane closes. StopAll below
	// is what ends a process the cancel does not reach in time.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	top, err := topLevel(ctx, *dir)
	if err != nil {
		return err
	}
	if err := git.CheckVersion(ctx, top); err != nil {
		return err
	}
	stateDir, err := stateDirectory()
	if err != nil {
		return err
	}

	model, err := tui.New(ctx, top, stateDir)
	if err != nil {
		return err
	}
	defer model.Close()
	// bubbletea leaves a running command's goroutine behind when the program
	// exits, so a git started just before q outlives the pane with init as its
	// parent and no one holding its deadline.
	// The answer is for a test that checks what a term got git to do; on the
	// way out there is nothing left to decide.
	defer func() { _ = git.StopAll() }()
	_, err = tea.NewProgram(model).Run()
	return err
}

// versionString answers "(unknown)" rather than an empty line, because a bug
// report that says nothing about the build is the case this flag exists for.
func versionString() string {
	if version != "" {
		return version
	}
	return versionFromBuild(debug.ReadBuildInfo())
}

// versionFromBuild is what the module system said about this build. It takes
// the answer rather than asking for it, because the two ways of saying nothing
// — no build info at all, and build info with no version in it — are what this
// has to tell apart, and neither can be arranged around a real build.
func versionFromBuild(info *debug.BuildInfo, ok bool) string {
	if !ok || info.Main.Version == "" {
		return "(unknown)"
	}
	return info.Main.Version
}

// topLevel answers why mirugit will not open dir, in mirugit's own words. git
// refuses three ways at startup and its lines all begin with "fatal: ", which
// main prefixes again with "mirugit: "; two prefixes on one sentence name the
// same thing twice. Which of the three it is comes from the state of the
// directory rather than from git's prose, because prose is not a contract:
// this program already had to fix one place that matched an English line git
// later reworded.
func topLevel(ctx context.Context, dir string) (string, error) {
	top, err := git.TopLevel(ctx, dir)
	if err == nil {
		return top, nil
	}
	if _, statErr := os.Stat(dir); os.IsNotExist(statErr) {
		return "", fmt.Errorf("no such directory: %s", dir)
	}
	bare, bareErr := git.IsBare(ctx, dir)
	switch {
	case bareErr != nil:
		// rev-parse refuses a path that is not a repository the same way it
		// refuses --show-toplevel, and the directory is there, so that is what
		// is left.
		return "", fmt.Errorf("not a git repository: %s", dir)
	case bare:
		// Pointing at a bare repository is a reasonable thing to do. mirugit
		// draws a working tree, and this one has none.
		return "", fmt.Errorf("no working tree to draw: %s is a bare repository", dir)
	}
	var exit *git.ExitError
	if errors.As(err, &exit) {
		if line, _, _ := strings.Cut(strings.TrimSpace(exit.Stderr), "\n"); line != "" {
			return "", errors.New(strings.TrimPrefix(line, "fatal: "))
		}
	}
	return "", err
}

// usage says what the program is and which environment variables reach it. The
// flag package prints neither, and both were only in the README, which is not
// where a reader who typed -h is looking.
func usage(out io.Writer, flags *flag.FlagSet) {
	// A failed write to the help output is the terminal going away, which the
	// caller is about to find out about anyway.
	_, _ = fmt.Fprint(out, `mirugit shows what a git repository has changed, one pane, no subcommands.

Usage:
  mirugit [flags]

Flags:
`)
	flags.SetOutput(out)
	flags.PrintDefaults()
	_, _ = fmt.Fprint(out, `
Environment:
  NO_COLOR              set to anything: draw without color
  MIRUGIT_THEME         slate (default) or cozmic
  RUNEWIDTH_EASTASIAN   set to 1 when the terminal draws · → ▌ two columns wide
  XDG_STATE_HOME        where read marks are kept
                        (default ~/.local/state/mirugit)

Keys are listed by ? inside the pane.
`)
}

func stateDirectory() (string, error) {
	if x := os.Getenv("XDG_STATE_HOME"); x != "" {
		return filepath.Join(x, "mirugit"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "mirugit"), nil
}
