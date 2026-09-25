package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// status --porcelain=v2 says nothing about a merge, so a tree whose conflicts
// are all resolved reads as clean while git is waiting for a commit. A pane
// that draws only the status shows nothing to do.
func TestAnUnfinishedOperationIsReported(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	for _, c := range []struct {
		name string
		set  func(t *testing.T, dir string)
		want InProgress
	}{
		{"a clean tree", func(*testing.T, string) {}, NothingInProgress},
		{"a merge stopped on a conflict", startConflictingMerge, MergeInProgress},
		{"a merge whose conflicts are resolved", func(t *testing.T, dir string) {
			startConflictingMerge(t, dir)
			runGitTest(t, gitTestEnv(dir), dir, "checkout", "--ours", "shared.txt")
			runGitTest(t, gitTestEnv(dir), dir, "add", "shared.txt")
		}, MergeInProgress},
		// git am stops in rebase-apply, the same directory a rebase stops in.
		// Reading the directory alone called it a rebase, and git refuses
		// git rebase --continue while an am is in progress.
		{"an am stopped on a conflict", startConflictingAm, AmInProgress},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			dir := newRepo(t)
			c.set(t, dir)

			got, err := OperationInProgress(mustGitDir(t, dir))
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Errorf("OperationInProgress = %v (%q), want %v (%q)",
					got, got.Verb(), c.want, c.want.Verb())
			}
			if c.want == NothingInProgress {
				return
			}
			// A reader stuck mid-merge needs the command, not the name of a
			// file under .git.
			if got.Finish() == "" || got.Abandon() == "" {
				t.Errorf("%q names no way out: finish=%q abandon=%q",
					got.Verb(), got.Finish(), got.Abandon())
			}
		})
	}
}

// A rebase that stops on a conflict leaves MERGE_HEAD as well as its own
// directory. Reading MERGE_HEAD first would tell the reader to run git commit,
// which is not what continues a rebase.
func TestARebaseIsNotReportedAsAMerge(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("touches a repository")
	}
	dir := newRepo(t)
	gitDir := mustGitDir(t, dir)
	for _, name := range []string{"MERGE_HEAD", "rebase-merge"} {
		path := filepath.Join(gitDir, name)
		if name == "rebase-merge" {
			if err := os.Mkdir(path, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.WriteFile(path, []byte("0000000\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := OperationInProgress(mustGitDir(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	if got != RebaseInProgress {
		t.Errorf("OperationInProgress = %q, want rebase", got.Verb())
	}
}

// The commands mirugit prints have to be ones git will accept. git refuses
// "git rebase --continue" and "git rebase --abort" while an am is in progress,
// so naming the operation wrong makes both lines on the footer dead.
func TestTheCommandsForAnAmAreGitAm(t *testing.T) {
	t.Parallel()
	if got := AmInProgress.Finish(); got != "git am --continue" {
		t.Errorf("Finish = %q", got)
	}
	if got := AmInProgress.Abandon(); got != "git am --abort" {
		t.Errorf("Abandon = %q", got)
	}
	if got := AmInProgress.Verb(); got != "am" {
		t.Errorf("Verb = %q", got)
	}
}

func startConflictingAm(t *testing.T, dir string) {
	t.Helper()
	env := gitTestEnv(dir)
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "shared.txt"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("one\ntwo\nthree\n")
	runGitTest(t, env, dir, "add", "shared.txt")
	runGitTest(t, env, dir, "commit", "-q", "-m", "base")
	runGitTest(t, env, dir, "checkout", "-q", "-b", "side")
	write("one\nside\nthree\n")
	runGitTest(t, env, dir, "commit", "-q", "-am", "side")
	runGitTest(t, env, dir, "format-patch", "-1", "-q", "-o", filepath.Join(dir, "patches"))
	runGitTest(t, env, dir, "checkout", "-q", "-")
	write("one\nmain\nthree\n")
	runGitTest(t, env, dir, "commit", "-q", "-am", "main")
	// The apply is expected to fail, so its exit code is not checked.
	_, _ = runWrite(context.Background(), dir, "am", filepath.Join(dir, "patches", "0001-side.patch"))
}

// startConflictingMerge leaves dir part way through a merge that git could not
// finish on its own.

func startConflictingMerge(t *testing.T, dir string) {
	t.Helper()
	env := gitTestEnv(dir)
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "shared.txt"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("one\ntwo\nthree\n")
	runGitTest(t, env, dir, "add", "shared.txt")
	runGitTest(t, env, dir, "commit", "-q", "-m", "base")
	runGitTest(t, env, dir, "checkout", "-q", "-b", "side")
	write("one\nside\nthree\n")
	runGitTest(t, env, dir, "commit", "-q", "-am", "side")
	runGitTest(t, env, dir, "checkout", "-q", "-")
	write("one\nmain\nthree\n")
	runGitTest(t, env, dir, "commit", "-q", "-am", "main")
	// The merge is expected to fail, so its exit code is not checked.
	_, _ = runWrite(context.Background(), dir, "merge", "side")
}

// Load is what the pane reads, so the state has to arrive through it. Checking
// OperationInProgress on its own leaves the wiring untested, and a Load that
// never asks draws a clean tree over an unfinished merge.
func TestLoadCarriesTheUnfinishedOperation(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	before, err := Load(context.Background(), dir, mustGitDir(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	if before.Unfinished != NothingInProgress {
		t.Fatalf("a fresh repository reports %q", before.Unfinished.Verb())
	}

	startConflictingMerge(t, dir)
	after, err := Load(context.Background(), dir, mustGitDir(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	if after.Unfinished != MergeInProgress {
		t.Errorf("Load reports %q during a merge, want merge", after.Unfinished.Verb())
	}
}

// mustGitDir resolves what Load now takes as an argument. Production resolves
// it once at startup; a test that loads twice would otherwise have to carry it.
func mustGitDir(t *testing.T, dir string) string {
	t.Helper()
	gitDir, err := GitDir(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	return gitDir
}
