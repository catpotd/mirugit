package git

import "testing"

// log prints five fields per commit with a NUL after each, so a whole log ends
// with a trailing empty record. The loop reads four past its own position: a
// log that ends part way through a commit reaches past the end, and the bound
// that stops it is the difference between dropping the half commit and a panic
// that takes the pane down.
func TestEveryWholeCommitIsReadAndAHalfOneIsNot(t *testing.T) {
	t.Parallel()
	const one = "abcdef0\x00abcdef0\x00a subject\x00An Author <a@b>\x001700000000\x00"
	const two = "1234567\x001234567\x00another\x00Someone <c@d>\x001700000001\x00"

	// A cut before the timestamp leaves fewer than five records and the half
	// commit is dropped. A cut right after the body leaves five, the last of
	// them empty, and an empty timestamp is a read this cannot make sense of:
	// the reader is told rather than shown a commit dated to the epoch.
	for _, c := range []struct {
		name    string
		out     string
		want    []string
		wantErr bool
	}{
		{"one commit", one, []string{"abcdef0"}, false},
		{"two commits", one + two, []string{"abcdef0", "1234567"}, false},
		{"nothing at all", "", nil, false},
		{"a read that ends after the subject", "abcdef0\x00abcdef0\x00s\x00", nil, false},
		{"a read that ends after the short sha", "abcdef0\x00abcdef0\x00", nil, false},
		{"a read that ends after the sha", "abcdef0\x00", nil, false},
		{"a read that ends after the body",
			"abcdef0\x00abcdef0\x00s\x00b\x00", nil, true},
		{"a whole commit and then one cut after its body",
			one + "1234567\x001234567\x00s\x00b\x00", nil, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			commits, err := parseLog([]byte(c.out), nil, false)
			if c.wantErr {
				if err == nil {
					t.Fatalf("read %+v without complaint", commits)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(commits) != len(c.want) {
				t.Fatalf("read %d commits, want %d: %+v", len(commits), len(c.want), commits)
			}
			for i := range commits {
				if commits[i].SHA != c.want[i] {
					t.Errorf("commit %d is %q, want %q", i, commits[i].SHA, c.want[i])
				}
			}
		})
	}
}

// A timestamp that is not a number is a read this cannot make sense of, and the
// reader is told rather than shown a log dated to the epoch.
func TestALogWithAnUnreadableTimestampIsAnError(t *testing.T) {
	t.Parallel()
	_, err := parseLog([]byte("abcdef0\x00abcdef0\x00s\x00b\x00not-a-number\x00"), nil, false)
	if err == nil {
		t.Error("a timestamp that is not a number was read without complaint")
	}
}

func TestParseLogReadsTheGitAuthor(t *testing.T) {
	t.Parallel()
	commits, err := parseLog([]byte("abcdef0\x00abcdef0\x00a subject\x00Mina\x001700000000\x00"), nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := commits[0].Author; got != "Mina" {
		t.Errorf("author = %q, want Mina", got)
	}
}

// The commits local to the upstream are answered once for the whole log, and
// every commit reads its own answer out of that set. Without an upstream there
// is nothing to compare against, so every commit counts as unpushed.
func TestACommitIsUnpushedWhenTheUpstreamDoesNotHoldIt(t *testing.T) {
	t.Parallel()
	const out = "abcdef0\x00abcdef0\x00a\x00b\x001700000000\x00" +
		"1234567\x001234567\x00c\x00d\x001700000001\x00"
	unpushed := map[string]bool{"1234567": true}

	withUpstream, err := parseLog([]byte(out), unpushed, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(withUpstream) != 2 {
		t.Fatalf("read %d commits, want 2", len(withUpstream))
	}
	if withUpstream[0].Unpushed {
		t.Error("a commit the upstream holds is marked unpushed")
	}
	if !withUpstream[1].Unpushed {
		t.Error("a commit the upstream does not hold is not marked unpushed")
	}

	none, err := parseLog([]byte(out), unpushed, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, commit := range none {
		if !commit.Unpushed {
			t.Errorf("%s is marked pushed with no upstream to hold it", commit.SHA)
		}
		if commit.HasUpstream {
			t.Errorf("%s says it has an upstream", commit.SHA)
		}
	}
}

// The records come from a read that can be cut anywhere.
func FuzzParseLog(f *testing.F) {
	f.Add("abcdef0\x00abcdef0\x00a subject\x00An Author <a@b>\x001700000000\x00")
	f.Add("abcdef0\x00abcdef0\x00")
	f.Add("\x00\x00\x00\x00\x00")
	f.Add("abcdef0\x00abcdef0\x00s\x00b\x00not-a-number\x00")
	f.Fuzz(func(t *testing.T, out string) {
		commits, err := parseLog([]byte(out), nil, false)
		if err != nil {
			return
		}
		for _, commit := range commits {
			if commit.SHA == "" {
				t.Errorf("a commit with no sha was read from %q", out)
			}
		}
	})
}
