// Package git runs the git binary and returns values. It is the only package
// that shells out, so that every other layer can be tested without a
// repository.
package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// ExitError carries the arguments and stderr because a caller three layers up
// has no other way to say which call failed.
type ExitError struct {
	Args   []string
	Code   int
	Stderr string
	// Signal is what ended the process, when a signal did. Code is -1 then, and
	// stderr is empty, so "exit -1: " was the whole of what a reader was told.
	Signal syscall.Signal
}

// Killed reports that a signal ended the process, which means git never
// answered: not the same as git answering that it refused.
func (e *ExitError) Killed() bool { return e.Signal != 0 }

func (e *ExitError) Error() string {
	if e.Killed() {
		return fmt.Sprintf("git %s: killed by %v", strings.Join(e.Args, " "), e.Signal)
	}
	return fmt.Sprintf("git %s: exit %d: %s",
		strings.Join(e.Args, " "), e.Code, strings.TrimSpace(e.Stderr))
}

// errNoDir names the case where a caller lost the repository path. exec runs a
// command with an empty Dir in the process's own working directory, so a
// forgotten path silently rewrites whatever repository the binary was started
// in. A test that built a Model without dir reset this repository's HEAD once
// per run before this check existed. The check sits in execGit rather than the
// wrappers because git clean -f and git apply --cached reach exec directly.
var errNoDir = errors.New("git: repository directory is empty")

// runWrite takes the index lock, because a write that races a read can leave
// the index in a state status does not describe. There is no unmarked wrapper:
// every call site has to say read or write, which is what kept six read-only
// commands from quietly blocking an agent writing to the same repository.
func runWrite(ctx context.Context, dir string, args ...string) ([]byte, error) {
	return execGit(ctx, dir, nil, nil, args)
}

// ErrGitMissing is the whole of what a reader can act on when git is absent:
// the arguments that were about to run say nothing about it.
var ErrGitMissing = errors.New("git is not on PATH: mirugit runs git and cannot work without it")

// ErrStopped is what a git started after the program began shutting down
// answers with. Nothing is left to draw it, and the process it would have
// waited on has been ended.
var ErrStopped = errors.New("mirugit is shutting down")

// Refusing the index lock keeps a pane that polls from blocking the agent
// writing to the same repository.
func runRead(ctx context.Context, dir string, args ...string) ([]byte, error) {
	env := []string{"GIT_OPTIONAL_LOCKS=0"}
	out, err := execGit(ctx, dir, env, nil, args)
	if !killedBeforeItAnswered(err) || ctx.Err() != nil {
		return out, err
	}
	// A signal means git never answered, and a read changes nothing, so asking
	// again is the same question rather than a second action. Measured on this
	// machine: under -race, the child of a fork segfaults before it reaches
	// exec about twice in eight runs of the tui suite, and the crash report
	// names the test binary rather than git — the process died still being a
	// copy of the parent. A write is not retried here: this cannot tell a fork
	// that died from a git that ran and was killed part way.
	return execGit(ctx, dir, env, nil, args)
}

// killedBeforeItAnswered reports a git that a signal ended. The caller checks
// the context beside it: canceling one ends git with a signal too, and asking
// again there would restart a read the caller has already given up on. A retry
// after StopAll needs no guard, because execGit answers ErrStopped for a
// process started once the program is on its way out.
func killedBeforeItAnswered(err error) bool {
	var exit *ExitError
	return errors.As(err, &exit) && exit.Killed()
}

// runWritePaths is runWrite for a path list. Paths go on stdin because an
// argument vector has a size limit, and because a path beginning with a dash
// would otherwise read as an option.
func runWritePaths(ctx context.Context, dir string, paths []string, args ...string) error {
	return runWriteEnvPaths(ctx, dir, nil, paths, args...)
}

// runWriteEnvPaths is runWritePaths for a call that needs its own environment.
// Only the snapshot writes to a temporary index, and it says so through env.
func runWriteEnvPaths(ctx context.Context, dir string, env, paths []string, args ...string) error {
	args = append(args, "--pathspec-from-file=-", "--pathspec-file-nul")
	stdin := []byte(strings.Join(paths, "\x00"))
	if len(paths) > 0 {
		stdin = append(stdin, 0)
	}
	_, err := execGit(ctx, dir, env, stdin, args)
	return err
}

// plainOutput are the settings that keep git's output parseable. The reader's
// own config reaches this process, and color.ui=always makes every line start
// with an escape, which left parseDiff finding no files and the pane silently
// empty. mirugit colors the screen itself, so git's colors are never wanted.
var plainOutput = []string{
	"--no-pager",
	"-c", "color.ui=false",
	"-c", "color.diff=false",
	"-c", "color.status=false",
	"-c", "core.quotePath=false",
}

// noExtDiff takes an option rather than a config because setting diff.external
// to the empty string makes git try to run "" and die. Only the subcommands
// that accept it get it; status rejects the flag outright.
var noExtDiff = map[string]bool{"diff": true, "log": true, "show": true}

// plainEnv keeps messages and number formats in one language, because a few
// call sites still read git's prose.
var plainEnv = []string{"LC_ALL=C", "GIT_TERMINAL_PROMPT=0"}

// networkDeadline bounds the calls that leave this machine. An unreachable host
// blocks until the TCP layer gives up, which is minutes on some networks, and
// the f key answers nothing for that whole time.
const networkDeadline = 30 * time.Second

// runNetwork is run with a deadline. Only fetch, pull and push use it: a local
// read that hangs is a slow frame, while a network call that hangs is a pane
// that stops answering.
func runNetwork(ctx context.Context, dir string, args ...string) error {
	return runNetworkWithin(ctx, networkDeadline, dir, args...)
}

// runNetworkWithin takes the deadline as an argument so a test can prove one is
// applied without waiting out the real one.
func runNetworkWithin(ctx context.Context, deadline time.Duration, dir string, args ...string) error {
	if dir == "" {
		return errNoDir
	}
	ctx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()
	_, err := execGitIn(ctx, dir, nil, nil, args)
	if ctx.Err() != nil {
		return fmt.Errorf("git %s: %w after %s",
			strings.Join(args, " "), ctx.Err(), deadline)
	}
	return err
}

// errBarePathspec names a pathspec that carries no path. git reads
// ":(literal)" as every path: measured, one of them handed to git add staged
// the whole repository. A verb aimed at one file would act on all of them, so
// the check sits here rather than at each caller, next to the directory check
// for the same reason.
var errBarePathspec = errors.New("git: a pathspec with no path")

func execGit(ctx context.Context, dir string, env []string, stdin []byte, args []string) ([]byte, error) {
	if dir == "" {
		return nil, errNoDir
	}
	if namesEveryPath(args) || namesEveryPath(splitNUL(stdin)) {
		return nil, errBarePathspec
	}
	return execGitIn(ctx, dir, env, stdin, args)
}

// signalThatEnded names the signal that ended a process, or zero when the
// process chose its own exit code.
func signalThatEnded(ee *exec.ExitError) syscall.Signal {
	ws, ok := ee.Sys().(syscall.WaitStatus)
	if !ok || !ws.Signaled() {
		return 0
	}
	return ws.Signal()
}

func namesEveryPath(values []string) bool {
	for _, v := range values {
		if v == pathspec("") {
			return true
		}
	}
	return false
}

func splitNUL(b []byte) []string {
	if len(b) == 0 {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(b), "\x00"), "\x00")
}

// splitNULPaths is splitNUL for a listing of paths. git never names an empty
// path, so an empty record is the trailing separator or a short read, and every
// caller dropped it with a test of its own. A mutation of one of those tests
// survived the suite: it kept the empty record and dropped every real path,
// which turned a stash that collides with the working tree into one the pane
// said would apply cleanly.
func splitNULPaths(b []byte) []string {
	var out []string
	for _, f := range splitNUL(b) {
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

// execGitIn is execGit without the directory check. TopLevel asks where the
// repository is, so it has no directory to run in yet and passes -C instead;
// runNetworkWithin has already made the check itself.
func execGitIn(ctx context.Context, dir string, env []string, stdin []byte, args []string) ([]byte, error) {
	full := append(append([]string{}, plainOutput...), args...)
	if len(args) > 0 && noExtDiff[args[0]] {
		full = append(append(append([]string{}, plainOutput...), args[0], "--no-ext-diff"), args[1:]...)
	}
	cmd := exec.CommandContext(ctx, "git", full...)
	killWholeTree(cmd)
	cmd.WaitDelay = time.Second
	cmd.Dir = dir
	cmd.Env = append(cmd.Environ(), plainEnv...)
	if len(env) > 0 {
		cmd.Env = append(cmd.Env, env...)
	}
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf

	if err := cmd.Start(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, ErrGitMissing
		}
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	// The number is taken here, on the goroutine that owns the command, because
	// Wait writes the same field a shutdown would otherwise read.
	pid := cmd.Process.Pid
	reaped, keep := addRunning(pid)
	if !keep {
		// StopAll already ran and will not come back for this one. This
		// goroutine owns it, so it does the stopping and the reaping in the
		// order stopGroupOwned needs.
		own := make(chan struct{})
		go func() { _ = cmd.Wait(); close(own) }()
		stopGroupOwned(pid, own, termGrace)
		<-own
		return nil, ErrStopped
	}
	defer removeRunning(pid, reaped)

	if err := cmd.Wait(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return out.Bytes(), &ExitError{
				Args: args, Code: ee.ExitCode(), Stderr: errBuf.String(),
				Signal: signalThatEnded(ee),
			}
		}
		// Every command this program runs is git, so the reader needs the name
		// of the missing program rather than the arguments it was going to
		// take. "exec: git: executable file not found in $PATH" beside a
		// rev-parse reads as a fault in the repository.
		if errors.Is(err, exec.ErrNotFound) {
			return nil, ErrGitMissing
		}
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return out.Bytes(), nil
}
