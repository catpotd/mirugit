package state

import "testing"

// The keys p, b, g, y and o run a verb on the tab whose rows carry it. A tab
// that answers a key its rows do not carry runs a verb against a row of another
// kind: p on the changes tab has no stash to restore.
func TestATabAnswersOnlyTheKeysItsOwnRowsCarry(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		tab  Tab
		verb VerbName
		want bool
	}{
		{TabStashed, VerbNameRestore, true},
		{TabStashed, VerbNameBranch, true},
		{TabChanges, VerbNameRestore, false},
		{TabHistory, VerbNameRestore, false},
		{TabWorktrees, VerbNameRestore, false},
		{TabHistory, VerbNameSHA, true},
		{TabHistory, VerbNameOpen, true},
		{TabChanges, VerbNameSHA, false},
		{TabStashed, VerbNameSHA, false},
		{TabWorktrees, VerbNameGo, true},
		{TabChanges, VerbNameGo, false},
		{TabHistory, VerbNameGo, false},
	} {
		t.Run(Facts[c.tab].Name+" "+c.verb.String(), func(t *testing.T) {
			t.Parallel()
			if got := Facts[c.tab].AnswersKey(c.verb); got != c.want {
				t.Errorf("the %s tab answers %q = %v, want %v",
					Facts[c.tab].Name, c.verb, got, c.want)
			}
		})
	}
}
