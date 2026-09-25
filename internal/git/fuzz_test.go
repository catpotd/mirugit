package git

import (
	"strings"
	"testing"
)

// Every parser here reads bytes git printed, and git's output depends on the
// repository: a branch name, a path, a stash message. A repository is data
// someone else wrote, so a parser that panics on unusual bytes takes the pane
// down when the reader opens a clone.
//
// The property is the same for all of them: no panic, and the values that leave
// are ones the rest of the program can hold.

func FuzzParseStatus(f *testing.F) {
	f.Add([]byte("1 .M N... 100644 100644 100644 abc def a.txt\x00"))
	f.Add([]byte("2 R. N... 100644 100644 100644 abc def R100 new.txt\x00old.txt\x00"))
	f.Add([]byte("# branch.head main\x00# branch.upstream origin/main\x00"))
	f.Add([]byte("? untracked.txt\x00"))
	f.Add([]byte("u UU N... 100644 100644 100644 100644 a b c both.txt\x00"))
	f.Add([]byte(""))
	f.Fuzz(func(t *testing.T, out []byte) {
		entries, _, err := parseStatus(out)
		if err != nil {
			return
		}
		for _, e := range entries {
			if e.Path == "" {
				t.Fatalf("an entry with no path: %+v", e)
			}
		}
	})
}

func FuzzParseDiff(f *testing.F) {
	f.Add([]byte("diff --git a/a.txt b/a.txt\n@@ -1,2 +1,3 @@\n l\n+new\n"), "a.txt")
	f.Add([]byte("@@ -0,0 +1 @@\n+only\n"), "b.txt")
	f.Add([]byte("Binary files a/x and b/x differ\n"), "x")
	f.Add([]byte(""), "")
	f.Fuzz(func(t *testing.T, out []byte, path string) {
		for _, d := range parseDiff(out, path) {
			for i, b := range d.Blocks {
				// The read marks are keyed by hash, so a block without one is a
				// row that can never be marked read.
				if b.Hash == "" {
					t.Fatalf("block %d of %q has no hash", i, d.Path)
				}
				if b.Added < 0 || b.Deleted < 0 {
					t.Fatalf("block %d of %q counts %d/%d", i, d.Path, b.Added, b.Deleted)
				}
			}
		}
	})
}

func FuzzParseNumstat(f *testing.F) {
	f.Add([]byte("3\t1\ta.txt\x00"))
	f.Add([]byte("-\t-\tbinary.png\x00"))
	f.Add([]byte("1\t0\t\x00old.txt\x00new.txt\x00"))
	f.Add([]byte(""))
	f.Fuzz(func(t *testing.T, out []byte) {
		for path, c := range parseNumstat(out) {
			if c.Added < 0 || c.Deleted < 0 {
				t.Fatalf("%q counts %d/%d", path, c.Added, c.Deleted)
			}
		}
	})
}

func FuzzParseStashSubject(f *testing.F) {
	f.Add("WIP on main: abc1234 subject")
	f.Add("On main: held work")
	f.Add("")
	f.Fuzz(func(t *testing.T, subject string) {
		branch, message := parseStashSubject(subject)
		// Every byte of the subject has to come out one side or the other: the
		// row prints the message and the meta column prints the branch, and a
		// byte in neither is a byte the reader cannot see.
		//
		// What the message may hold is not this parser's business. A reader can
		// type a control character into git stash push -m, and layout.Printable
		// is what makes it safe to draw.
		if !strings.Contains(subject, branch) || !strings.Contains(subject, message) {
			t.Fatalf("subject=%q lost text: branch=%q message=%q", subject, branch, message)
		}
	})
}

func FuzzParseBlockHeader(f *testing.F) {
	f.Add("@@ -1,2 +3,4 @@")
	f.Add("@@ -0,0 +1 @@ func x()")
	f.Add("@@")
	f.Add("")
	f.Fuzz(func(t *testing.T, header string) {
		oldStart, newStart := parseBlockHeader(header)
		// Line numbers are drawn in a four-digit field. A negative one would
		// widen the gutter and push every column right.
		if oldStart < 0 || newStart < 0 {
			t.Fatalf("%q gave %d/%d", header, oldStart, newStart)
		}
	})
}

func FuzzParseTwoCounts(f *testing.F) {
	f.Add("3\t7\n")
	f.Add("not numbers")
	f.Add("")
	f.Fuzz(func(t *testing.T, s string) {
		first, second := parseTwoCounts(s)
		if first < 0 || second < 0 {
			t.Fatalf("%q gave %d/%d", s, first, second)
		}
	})
}

func FuzzMergeTreeConflictPaths(f *testing.F) {
	f.Add("100644 abc 1\tf.txt\n100644 def 2\tf.txt\n")
	f.Add("tree-oid\n\nAuto-merging f.txt\nCONFLICT (content)\n")
	f.Add("")
	f.Fuzz(func(t *testing.T, text string) {
		paths, seen := mergeTreeConflictPaths(text)
		if len(paths) != len(seen) {
			t.Fatalf("%d paths but %d seen; the row count and the list disagree",
				len(paths), len(seen))
		}
		for _, p := range paths {
			if p == "" {
				t.Fatal("an empty conflicting path")
			}
		}
	})
}
