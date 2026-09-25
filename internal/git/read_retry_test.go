package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// runRead asks again when a signal ended git before it answered, because a read
// changes nothing and a git that never answered asked nothing. Every other
// outcome is answered once. Asking twice there runs a second process on every
// poll, and on a read whose context was canceled it restarts work the caller
// has already given up on.
//
// git is made to leave a mark each time it starts, so the count is what ran
// rather than what the code looks like it does.
func TestARunReadThatAnsweredIsNotAskedAgain(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	tally := filepath.Join(t.TempDir(), "starts")
	alias := "!sh -c 'printf x >> " + tally + "'"

	if _, err := runRead(context.Background(), dir, "-c", "alias.tally="+alias, "tally"); err != nil {
		t.Fatal(err)
	}

	body, err := os.ReadFile(tally)
	if err != nil {
		t.Fatalf("git left no mark, so this proves nothing: %v", err)
	}
	if len(body) != 1 {
		t.Errorf("git ran %d times for one read, want 1", len(body))
	}
}
