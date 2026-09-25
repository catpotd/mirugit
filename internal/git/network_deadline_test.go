package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A network call that hangs stops the pane answering: f blocks until the TCP
// layer gives up, which is minutes on some networks. Local reads keep no
// deadline, because a slow read is a slow frame and the poll picks it up.
func TestAContextDeadlineStopsGit(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	// A sleep long enough that only the deadline can end it.
	_, err := execGitIn(ctx, t.TempDir(), nil, nil,
		[]string{"-c", "alias.wait=!sleep 10", "wait"})
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("期限を過ぎても git が戻らなかった")
	}
	if elapsed > 3*time.Second {
		t.Errorf("%v かかった。50ms の期限で止まるべき", elapsed.Round(time.Millisecond))
	}
	if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Errorf("ctx.Err() = %v, want DeadlineExceeded", ctx.Err())
	}
}

// A deadline that is never applied costs nothing until a host stops answering,
// and then it costs the whole pane. Asserting the constant is positive says
// nothing about whether the call uses it.
func TestRunNetworkStopsGitAtItsDeadline(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	start := time.Now()
	err := runNetworkWithin(context.Background(), 50*time.Millisecond, t.TempDir(),
		"-c", "alias.wait=!sleep 10", "wait")
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("git outlived the deadline and reported success")
	}
	if !strings.Contains(err.Error(), context.DeadlineExceeded.Error()) {
		t.Errorf("err = %v, want it to name the deadline", err)
	}
	if elapsed > 3*time.Second {
		t.Errorf("took %v; the 50ms deadline did not end it",
			elapsed.Round(time.Millisecond))
	}
}

// Fetch, pull and push leave this machine and are the calls a deadline is for.
// A local read behind one fails on a slow disk instead of an unreachable host,
// so the list is written out here rather than derived from the calls.
func TestOnlyNetworkCallsUseRunNetwork(t *testing.T) {
	t.Parallel()
	callers := map[string]bool{}
	for _, file := range goFilesIn(t, ".") {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		fn := ""
		for _, line := range strings.Split(string(body), "\n") {
			if strings.HasPrefix(line, "func ") {
				fn = line
			}
			if strings.Contains(line, "runNetwork(") && !strings.HasPrefix(line, "func ") {
				callers[funcName(fn)] = true
			}
		}
	}
	// runNetwork is what the three of them pass through, and the deadline it
	// carries is only a deadline while it is short enough to end a frame the
	// reader is waiting on.
	if networkDeadline < time.Second || networkDeadline > time.Minute {
		t.Errorf("networkDeadline is %v; a pane redrawing cannot wait that long", networkDeadline)
	}
	body, err := os.ReadFile("run.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "runNetworkWithin(ctx, networkDeadline,") {
		t.Error("runNetwork no longer passes networkDeadline, so the constant above " +
			"bounds nothing")
	}

	want := map[string]bool{"Fetch": true, "pull": true, "push": true}
	for name := range callers {
		if !want[name] {
			t.Errorf("%s calls runNetwork; a deadline there fails a slow disk, "+
				"not an unreachable host", name)
		}
	}
	for name := range want {
		if !callers[name] {
			t.Errorf("%s no longer calls runNetwork, so it leaves with no deadline", name)
		}
	}
}

// funcName is the identifier in a "func Name(" or "func (r T) Name(" line.
func funcName(line string) string {
	rest := strings.TrimPrefix(line, "func ")
	if strings.HasPrefix(rest, "(") {
		if i := strings.Index(rest, ") "); i >= 0 {
			rest = rest[i+2:]
		}
	}
	if i := strings.IndexAny(rest, "(["); i >= 0 {
		return rest[:i]
	}
	return rest
}

// goFilesIn lists the Go files of one directory, so a convention test reads the
// package as it is rather than a list that has to be kept up to date.
func goFilesIn(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	return files
}
