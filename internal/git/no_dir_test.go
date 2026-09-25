package git

import (
	"context"
	"errors"
	"testing"
	"time"
)

// An empty dir makes exec run in this process's own directory. git clean -f
// there deletes untracked files in whatever repository the binary was started
// in, which is the accident the check exists to stop. The check has to sit in
// execGit: apply and clean reach exec without going through the wrappers.
func TestEveryGitCallRefusesAnEmptyDirectory(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		call func() error
	}{
		{"runRead", func() error { _, err := runRead(context.Background(), "", "status"); return err }},
		{"runWrite", func() error { _, err := runWrite(context.Background(), "", "update-ref", "-d", "HEAD"); return err }},
		{"runWritePaths", func() error { err := runWritePaths(context.Background(), "", nil, "add"); return err }},
		{"runClean", func() error { _, err := runClean(context.Background(), "", []string{"a.txt"}); return err }},
		{"StageBlock", func() error { return StageBlock(context.Background(), "", FileDiff{Blocks: []Block{{}}}, 0) }},
		{"execGit", func() error { _, err := execGit(context.Background(), "", nil, nil, []string{"status"}); return err }},
		{"runNetworkWithin", func() error {
			return runNetworkWithin(context.Background(), time.Second, "", "fetch")
		}},
		// The refusal has to reach the caller rather than being read as a git
		// whose version could not be parsed, which is not an error at all.
		{"CheckVersion", func() error { return CheckVersion(context.Background(), "") }},
	} {
		if err := tc.call(); !errors.Is(err, errNoDir) {
			t.Errorf("%s は空の dir を受け入れた: err=%v", tc.name, err)
		}
	}
}

// ":(literal)" with no path after it reads as every path: measured, one handed
// to git add staged the whole repository. A verb aimed at one file would act on
// all of them, so every call is refused before git starts.
//
// The commands below only read, and the directory is a repository of this
// test's own: a guard that let one through would print a status rather than
// touch anything.
func TestEveryGitCallRefusesAPathspecWithNoPath(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	bare := pathspec("")

	for _, tc := range []struct {
		name string
		call func() error
	}{
		{"in the arguments", func() error {
			_, err := execGit(context.Background(), dir, nil, nil, []string{"status", "--porcelain", "--", bare})
			return err
		}},
		{"after a real path in the arguments", func() error {
			_, err := execGit(context.Background(), dir, nil, nil,
				[]string{"status", "--porcelain", "--", pathspec("a.txt"), bare})
			return err
		}},
		{"on standard input", func() error {
			_, err := execGit(context.Background(), dir, nil, []byte(bare+"\x00"), []string{"status", "--porcelain"})
			return err
		}},
		{"after a real path on standard input", func() error {
			_, err := execGit(context.Background(), dir, nil, []byte(pathspec("a.txt")+"\x00"+bare+"\x00"),
				[]string{"status", "--porcelain"})
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := tc.call(); !errors.Is(err, errBarePathspec) {
				t.Errorf("a pathspec with no path was passed to git: err=%v", err)
			}
		})
	}

	// A pathspec that names a path is not refused, or the check above would
	// pass by refusing everything.
	if _, err := execGit(context.Background(), dir, nil, nil,
		[]string{"status", "--porcelain", "--", pathspec("a.txt")}); err != nil {
		t.Errorf("a pathspec naming a file was refused: %v", err)
	}
}

// The option that keeps an external differ out is added by looking at the
// subcommand, which is the first argument. A call with no arguments has none,
// and reading it reaches past the end of the list before git is even started.
func TestAGitCallWithNoArgumentsIsNotReadForASubcommand(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	// git with no arguments prints its usage and exits non-zero; what matters
	// here is that this program reaches that point at all.
	if _, err := execGit(context.Background(), dir, nil, nil, nil); err == nil {
		t.Error("git with no arguments answered without complaint")
	}
	if _, err := execGit(context.Background(), dir, nil, nil, []string{}); err == nil {
		t.Error("git with an empty argument list answered without complaint")
	}
}
