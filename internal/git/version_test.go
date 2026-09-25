package git

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// The forms git prints differ by build. Reading the wrong field turns a working
// git into a refusal to start, which is worse than the failure the check exists
// to explain.
func TestParseVersionReadsWhatGitPrints(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		line string
		want Version
		ok   bool
	}{
		{"git version 2.41.0\n", Version{2, 41}, true},
		{"git version 2.50.1 (Apple Git-155)\n", Version{2, 50}, true},
		{"git version 2.39.5 (Apple Git-154)", Version{2, 39}, true},
		{"git version 3.0.0", Version{3, 0}, true},
		{"git version 2.41.0.windows.1", Version{2, 41}, true},
		// A build that prints two numbers still names the pair this reads, and
		// refusing it turns a working git into a refusal to start.
		{"git version 2.41", Version{2, 41}, true},
		{"git version 2.41 (Apple Git-155)", Version{2, 41}, true},
		{"", Version{}, false},
		{"git version", Version{}, false},
		{"git version two.point.five", Version{}, false},
		{"git version 2", Version{}, false},
	} {
		t.Run(c.line, func(t *testing.T) {
			t.Parallel()
			got, ok := ParseVersion(c.line)
			if ok != c.ok {
				t.Fatalf("ok = %v, want %v", ok, c.ok)
			}
			if ok && got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestVersionOrdersByMajorThenMinor(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		a, b Version
		want bool
	}{
		{Version{2, 40}, Version{2, 41}, true},
		{Version{2, 41}, Version{2, 41}, false},
		{Version{2, 42}, Version{2, 41}, false},
		// A minor above the floor's minor does not make an older major newer.
		{Version{1, 99}, Version{2, 41}, true},
		{Version{3, 0}, Version{2, 41}, false},
	} {
		if got := c.a.OlderThan(c.b); got != c.want {
			t.Errorf("%v older than %v = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

// A git without the fields this program reads answers "unknown field name" and
// prints nothing a reader can act on: the pane shows the command line it ran,
// truncated to the pane width. The refusal turns that into one sentence naming
// the version to install.
func TestVersionRefusalNamesTheVersionToInstall(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name   string
		prints string
		// names is what the refusal has to contain, and nil means the version
		// is accepted. The reader needs both numbers: the one they have and the
		// one to install.
		names []string
	}{
		{"one minor below the floor", "git version 2.40.0", []string{"2.40", "2.41"}},
		{"an older major", "git version 1.99.0", []string{"1.99", "2.41"}},
		{"the floor itself", "git version 2.41.0", nil},
		{"far above the floor", "git version 2.50.1 (Apple Git-155)", nil},
		// Refusing to start on a version string this program cannot read would
		// turn a cosmetic difference into a failure to run.
		{"a build that names itself some other way", "git is great", nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			err := VersionRefusal(c.prints)
			if (err != nil) != (c.names != nil) {
				t.Fatalf("VersionRefusal = %v, want a refusal: %v", err, c.names != nil)
			}
			for _, want := range c.names {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("the refusal does not name %q: %v", want, err)
				}
			}
		})
	}
}

// CheckVersion has to reach git, not just judge a string. A git ahead of the
// floor is the one on this machine, so the wiring is checked against it.
func TestCheckVersionAcceptsTheGitOnThisMachine(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := CheckVersion(context.Background(), dir); err != nil {
		out, _ := runRead(context.Background(), dir, "--version")
		t.Errorf("CheckVersion refused %q: %v", strings.TrimSpace(string(out)), err)
	}
}

// git is the only program this package runs, so its absence is not a fault of
// the arguments that were about to take it. The exec error names the arguments
// and buries the cause; a reader sees a rev-parse and looks at their
// repository.
func TestAMissingGitSaysSoRatherThanNamingTheArguments(t *testing.T) {
	if testing.Short() {
		t.Skip("runs a command")
	}
	t.Setenv("PATH", t.TempDir())
	_, err := runRead(context.Background(), t.TempDir(), "rev-parse", "--show-toplevel")
	if !errors.Is(err, ErrGitMissing) {
		t.Fatalf("err = %v, want ErrGitMissing", err)
	}
	if strings.Contains(err.Error(), "rev-parse") {
		t.Errorf("the message names the arguments rather than the missing program: %v", err)
	}
}

// ExitError.Error is what a caller sees when it prints the error rather than
// asking its type. cmd/mirugit does that for the top-level failure, so the
// three parts have to be there: what ran, what it answered, and why.
func TestAFailedGitSaysWhatRanAndWhatItAnswered(t *testing.T) {
	t.Parallel()
	err := &ExitError{
		Args:   []string{"stash", "pop"},
		Code:   1,
		Stderr: "error: could not restore\n",
	}
	got := err.Error()
	for _, want := range []string{"stash pop", "exit 1", "could not restore"} {
		if !strings.Contains(got, want) {
			t.Errorf("Error() = %q, want it to say %q", got, want)
		}
	}
	// The trailing newline of git's message would break the line it lands on.
	if strings.HasSuffix(got, "\n") {
		t.Errorf("Error() ends with a newline: %q", got)
	}
}
