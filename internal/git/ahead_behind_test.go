package git

import "testing"

func TestAheadBehindParsesRevListOutput(t *testing.T) {
	t.Parallel()
	behind, ahead := parseTwoCounts("3\t7\n")
	if behind != 3 || ahead != 7 {
		t.Errorf("got behind=%d ahead=%d, want 3 7", behind, ahead)
	}
}

func TestAheadBehindReturnsZeroForGarbage(t *testing.T) {
	t.Parallel()
	behind, ahead := parseTwoCounts("not numbers")
	if behind != 0 || ahead != 0 {
		t.Errorf("got behind=%d ahead=%d, want 0", behind, ahead)
	}
}
