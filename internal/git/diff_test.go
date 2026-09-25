package git

import (
	"strings"
	"testing"
)

const twoBlocks = `diff --git a/a.txt b/a.txt
index 111..222 100644
--- a/a.txt
+++ b/a.txt
@@ -1,3 +1,4 @@
 one
+two
 three
@@ -20,2 +21,3 @@
 twenty
+twentyone
`

func TestParseDiffSplitsBlocksAndCountsThem(t *testing.T) {
	t.Parallel()
	files := parseDiff([]byte(twoBlocks), "a.txt")
	if len(files) != 1 {
		t.Fatalf("want 1 file, got %d", len(files))
	}
	f := files[0]
	if f.Path != "a.txt" {
		t.Errorf("path = %q", f.Path)
	}
	if len(f.Blocks) != 2 {
		t.Fatalf("want 2 blocks, got %d", len(f.Blocks))
	}
	if f.Blocks[0].Added != 1 || f.Blocks[0].Deleted != 0 {
		t.Errorf("block 0 = %+v", f.Blocks[0])
	}
}

func TestParseDiffReadsBlockStartFromFullHeader(t *testing.T) {
	t.Parallel()
	const hunk = `diff --git a/a.txt b/a.txt
index 111..222 100644
--- a/a.txt
+++ b/a.txt
@@ -40,3 +42,5 @@
 one
+two
 three
`
	files := parseDiff([]byte(hunk), "a.txt")
	if len(files) != 1 || len(files[0].Blocks) != 1 {
		t.Fatalf("got %+v", files)
	}
	b := files[0].Blocks[0]
	if b.OldStart != 40 || b.NewStart != 42 {
		t.Errorf("OldStart=%d NewStart=%d, want 40 42", b.OldStart, b.NewStart)
	}
}

func TestParseDiffReadsBlockStartFromAbbreviatedHeader(t *testing.T) {
	t.Parallel()
	const hunk = `diff --git a/a.txt b/a.txt
index 111..222 100644
--- a/a.txt
+++ b/a.txt
@@ -40 +42 @@
 one
`
	files := parseDiff([]byte(hunk), "a.txt")
	if len(files) != 1 || len(files[0].Blocks) != 1 {
		t.Fatalf("got %+v", files)
	}
	b := files[0].Blocks[0]
	if b.OldStart != 40 || b.NewStart != 42 {
		t.Errorf("OldStart=%d NewStart=%d, want 40 42", b.OldStart, b.NewStart)
	}
}

func TestParseDiffReadsNewSideFromCombinedHeader(t *testing.T) {
	t.Parallel()
	const hunk = `diff --cc f.txt
index 1234567,89abcde..0000000
--- a/f.txt
+++ b/f.txt
@@@ -1,2 -1,2 +1,6 @@@
 one
`
	files := parseDiff([]byte(hunk), "f.txt")
	if len(files) != 1 || len(files[0].Blocks) != 1 {
		t.Fatalf("got %+v", files)
	}
	b := files[0].Blocks[0]
	if b.NewStart != 1 {
		t.Errorf("NewStart=%d, want 1", b.NewStart)
	}
	if b.LinePrefixLength != 2 {
		t.Errorf("LinePrefixLength=%d, want 2", b.LinePrefixLength)
	}
}

func TestParseDiffCountsCombinedAgainstFirstParent(t *testing.T) {
	t.Parallel()
	const hunk = "diff --cc a.txt\n" +
		"index f4c4712,b129931..0000000\n" +
		"--- a/a.txt\n" +
		"+++ b/a.txt\n" +
		"@@@ -1,3 -1,4 +1,8 @@@\n" +
		"  one\n" +
		"++<<<<<<< HEAD\n" +
		" +MAIN\n" +
		"++=======\n" +
		"+ OTHER\n" +
		"++>>>>>>> f6aa27b (other)\n" +
		"  three\n" +
		"+ FOUR\n"
	files := parseDiff([]byte(hunk), "a.txt")
	if len(files) != 1 || len(files[0].Blocks) != 1 {
		t.Fatalf("got %+v", files)
	}
	b := files[0].Blocks[0]
	if b.Added != 5 {
		t.Errorf("Added=%d, want 5", b.Added)
	}
}

func TestParseDiffDoesNotCountContextStartingWithPlus(t *testing.T) {
	t.Parallel()
	const hunk = `diff --git a/a.txt b/a.txt
index 111..222 100644
--- a/a.txt
+++ b/a.txt
@@ -1,2 +1,3 @@
 +foo
+bar
`
	files := parseDiff([]byte(hunk), "a.txt")
	if len(files) != 1 || len(files[0].Blocks) != 1 {
		t.Fatalf("got %+v", files)
	}
	b := files[0].Blocks[0]
	if b.Added != 1 {
		t.Errorf("Added=%d, want 1", b.Added)
	}
}

func TestBlockHashIgnoresLineNumbers(t *testing.T) {
	t.Parallel()
	// The same change further down the file must hash the same, so that read
	// state survives an unrelated edit above it.
	a := parseDiff([]byte(twoBlocks), "a.txt")[0].Blocks[0]
	moved := strings.Replace(twoBlocks, "@@ -1,3 +1,4 @@", "@@ -41,3 +41,4 @@", 1)
	b := parseDiff([]byte(moved), "a.txt")[0].Blocks[0]
	if a.Hash != b.Hash {
		t.Errorf("hash changed with the line number: %s vs %s", a.Hash, b.Hash)
	}
}

func TestBlockHashChangesWithTheContent(t *testing.T) {
	t.Parallel()
	a := parseDiff([]byte(twoBlocks), "a.txt")[0].Blocks[0]
	edited := strings.Replace(twoBlocks, "+two", "+TWO", 1)
	b := parseDiff([]byte(edited), "a.txt")[0].Blocks[0]
	if a.Hash == b.Hash {
		t.Error("hash did not change when the content did")
	}
}

func TestParseDiffKeepsFileHeadersForApply(t *testing.T) {
	t.Parallel()
	files := parseDiff([]byte(twoBlocks), "a.txt")
	if len(files) != 1 {
		t.Fatalf("got %d files", len(files))
	}
	want := []string{
		"diff --git a/a.txt b/a.txt",
		"index 111..222 100644",
		"--- a/a.txt",
		"+++ b/a.txt",
	}
	if len(files[0].Header) != len(want) {
		t.Fatalf("header = %v, want %v", files[0].Header, want)
	}
	for i := range want {
		if files[0].Header[i] != want[i] {
			t.Errorf("header[%d] = %q, want %q", i, files[0].Header[i], want[i])
		}
	}
}

func TestParseDiffKeepsNewFileModeHeaderForApply(t *testing.T) {
	t.Parallel()
	const newFile = `diff --git a/b.txt b/b.txt
new file mode 100644
index 0000000..3e75765
--- /dev/null
+++ b/b.txt
@@ -0,0 +1 @@
+new
`
	files := parseDiff([]byte(newFile), "b.txt")
	if len(files) != 1 {
		t.Fatalf("got %d files", len(files))
	}
	want := []string{
		"diff --git a/b.txt b/b.txt",
		"new file mode 100644",
		"index 0000000..3e75765",
		"--- /dev/null",
		"+++ b/b.txt",
	}
	if len(files[0].Header) != len(want) {
		t.Fatalf("header = %v, want %v", files[0].Header, want)
	}
	for i := range want {
		if files[0].Header[i] != want[i] {
			t.Errorf("header[%d] = %q, want %q", i, files[0].Header[i], want[i])
		}
	}
}

func TestParseDiffKeepsDeletedFileModeHeaderForApply(t *testing.T) {
	t.Parallel()
	const deletedFile = `diff --git a/d.md b/d.md
deleted file mode 100644
index d98d442..0000000
--- a/d.md
+++ /dev/null
@@ -1 +0,0 @@
-# doc
`
	files := parseDiff([]byte(deletedFile), "d.md")
	if len(files) != 1 {
		t.Fatalf("got %d files", len(files))
	}
	want := []string{
		"diff --git a/d.md b/d.md",
		"deleted file mode 100644",
		"index d98d442..0000000",
		"--- a/d.md",
		"+++ /dev/null",
	}
	if len(files[0].Header) != len(want) {
		t.Fatalf("header = %v, want %v", files[0].Header, want)
	}
	for i := range want {
		if files[0].Header[i] != want[i] {
			t.Errorf("header[%d] = %q, want %q", i, files[0].Header[i], want[i])
		}
	}
}

func TestParseDiffMarksBinary(t *testing.T) {
	t.Parallel()
	const bin = `diff --git a/img.png b/img.png
index 111..222 100644
Binary files a/img.png and b/img.png differ
`
	files := parseDiff([]byte(bin), "img.png")
	if len(files) != 1 || !files[0].Binary {
		t.Fatalf("got %+v", files)
	}
	if len(files[0].Blocks) != 0 {
		t.Error("a binary file has no blocks to stage")
	}
	// ModeOnly starts true and every line that proves otherwise clears it. The
	// screen reads Binary first, so leaving it set changes nothing that is
	// drawn today, and a record that says a picture changed only its mode is
	// still wrong for whoever reads FileDiff next.
	if files[0].ModeOnly {
		t.Error("the bytes changed and the record says only the mode did")
	}
}

// Each of the three lines that say a file's contents changed clears the
// ModeOnly that parseDiff sets when it opens the record. A diff that reaches
// none of them is the mode-only case that flag is named for.
func TestOnlyADiffWithNoContentChangeIsModeOnly(t *testing.T) {
	t.Parallel()
	const head = "diff --git a/f b/f\nold mode 100644\nnew mode 100755\n"
	for _, c := range []struct {
		name string
		body string
		want bool
	}{
		{"nothing but a mode change", "", true},
		{"bytes that are not text", "Binary files a/f and b/f differ\n", false},
		{"a rename", "rename from g\nrename to f\n", false},
		{"a block of lines", "@@ -1 +1 @@\n-a\n+b\n", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			files := parseDiff([]byte(head+c.body), "f")
			if len(files) != 1 {
				t.Fatalf("read %d files, want 1: %+v", len(files), files)
			}
			if files[0].ModeOnly != c.want {
				t.Errorf("ModeOnly = %v, want %v: %+v", files[0].ModeOnly, c.want, files[0])
			}
		})
	}
}

func TestParseDiffMarksAModeOnlyChange(t *testing.T) {
	t.Parallel()
	const mode = `diff --git a/s.sh b/s.sh
old mode 100644
new mode 100755
`
	files := parseDiff([]byte(mode), "s.sh")
	if len(files) != 1 || !files[0].ModeOnly {
		t.Fatalf("got %+v", files)
	}
}

func TestParseDiffKeepsBothSectionsOfATypeChange(t *testing.T) {
	t.Parallel()
	// A symlink replaced by a file writes two diff headers for one path.
	const typechange = `diff --git a/link b/link
deleted file mode 120000
--- a/link
+++ /dev/null
@@ -1 +0,0 @@
-target.txt
diff --git a/link b/link
new file mode 100644
--- /dev/null
+++ b/link
@@ -0,0 +1 @@
+now a file
`
	files := parseDiff([]byte(typechange), "link")
	if len(files) != 2 {
		t.Fatalf("want 2 sections for one path, got %d", len(files))
	}
	if files[0].Path != "link" || files[1].Path != "link" {
		t.Errorf("paths = %q, %q", files[0].Path, files[1].Path)
	}
}

// A combined header carries two old sides and one new one, and the new side is
// the last field. A header cut down to one field names both with that field,
// which is the answer a truncated read should give: the number it did print is
// better than a hunk that says it starts at line zero, and the gutter draws
// zero as a line the file does not have.
//
// The suite's combined headers all carry three fields, so the bound that lets a
// single field through is not reached by them.
func TestACombinedHeaderWithOneFieldStillNamesAStart(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name             string
		header           string
		wantOld, wantNew int
	}{
		{"the whole header", "@@@ -1,3 -1,4 +7,8 @@@", 1, 7},
		{"one field", "@@@ +7,8 @@@", 7, 7},
		{"one field with no count", "@@@ +7 @@@", 7, 7},
		{"no field at all", "@@@ @@@", 0, 0},
		{"nothing between the marks", "@@@@@@", 0, 0},
		// A field with no sign in front of it is the whole number: reading it
		// as a sign drops the first digit, so 12 becomes 2 and the gutter
		// numbers every line of the hunk from the wrong place.
		{"a field with no sign", "@@@ -1,3 -1,4 12,8 @@@", 1, 12},
		{"one field with no sign", "@@@ 7 @@@", 7, 7},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			oldStart, newStart := parseBlockHeader(c.header)
			if oldStart != c.wantOld || newStart != c.wantNew {
				t.Errorf("parseBlockHeader(%q) = %d, %d; want %d, %d",
					c.header, oldStart, newStart, c.wantOld, c.wantNew)
			}
		})
	}
}
