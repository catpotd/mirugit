package git

import (
	"testing"
	"time"
)

// The commit rows and the fetch label sit on the same screen and each had its
// own answer for how long ago something happened. They agreed above a minute
// and disagreed below it: a commit made ten seconds ago read "1m" while the
// header read "fetched 0m ago".
func TestAgeIsOneUnitAndCountsDown(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		elapsed time.Duration
		want    string
	}{
		{0, "0m"},
		{10 * time.Second, "0m"},
		{59 * time.Second, "0m"},
		{time.Minute, "1m"},
		{59 * time.Minute, "59m"},
		{time.Hour, "1h"},
		{23 * time.Hour, "23h"},
		{24 * time.Hour, "1d"},
		{72 * time.Hour, "3d"},
		// A clock that moved backwards is not an age the reader can act on.
		{-time.Hour, "0m"},
	} {
		if got := FormatAge(c.elapsed); got != c.want {
			t.Errorf("%v ago is %q, want %q", c.elapsed, got, c.want)
		}
	}
}

// formatAge is what the commit rows print. It has to be the same answer, or the
// two disagree again the next time one of them is changed.
func TestCommitAgeUsesTheSameAnswer(t *testing.T) {
	t.Parallel()
	now := time.Now().Unix()
	for _, seconds := range []int64{0, 10, 90, 3600, 90000} {
		elapsed := time.Duration(seconds) * time.Second
		if got, want := formatAge(now-seconds), FormatAge(elapsed); got != want {
			t.Errorf("%ds ago: the commit row says %q and FormatAge says %q",
				seconds, got, want)
		}
	}
}
