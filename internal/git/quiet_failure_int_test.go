package git

import (
	"context"
	"testing"
)

// Each of these used to return a normal-looking value when git failed: 0/0
// commits behind and ahead, an empty origin URL, and "the branches diverged".
// The reader saw a synced repository with no remote instead of an error.
func TestGitFailuresAreNotDrawnAsNormalValues(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	notARepo := t.TempDir()
	for _, c := range []struct {
		name string
		call func(dir string) error
	}{
		{"aheadBehind", func(dir string) error { _, _, err := aheadBehind(context.Background(), dir); return err }},
		{"RemoteURL", func(dir string) error { _, err := RemoteURL(context.Background(), dir); return err }},
		{"canFastForward", func(dir string) error { _, err := canFastForward(context.Background(), dir); return err }},
	} {
		if err := c.call(notARepo); err == nil {
			t.Errorf("%s: リポジトリでないディレクトリでエラーを返さなかった", c.name)
		}
	}
}

// A repository with no upstream and no origin is a normal state, not a
// failure. Reporting an error there would put a message on every frame.
func TestMissingUpstreamAndOriginAreNotErrors(t *testing.T) {
	t.Parallel()
	dir := newRepo(t)
	behind, ahead, err := aheadBehind(context.Background(), dir)
	if err != nil {
		t.Errorf("aheadBehind: %v", err)
	}
	if behind != 0 || ahead != 0 {
		t.Errorf("behind=%d ahead=%d, want 0 0", behind, ahead)
	}
	url, err := RemoteURL(context.Background(), dir)
	if err != nil {
		t.Errorf("RemoteURL: %v", err)
	}
	if url != "" {
		t.Errorf("RemoteURL = %q, want empty", url)
	}
}
