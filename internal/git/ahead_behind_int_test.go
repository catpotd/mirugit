package git

import (
	"context"
	"testing"
)

func TestAheadBehindWithoutUpstreamReturnsZero(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	behind, ahead, err := aheadBehind(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if behind != 0 || ahead != 0 {
		t.Errorf("got behind=%d ahead=%d, want 0", behind, ahead)
	}
}
