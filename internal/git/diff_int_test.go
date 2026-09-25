package git

import (
	"context"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

func repoWithManyChangedFiles(t *testing.T, n int) string {
	t.Helper()
	dir := newRepo(t)
	for i := range n {
		name := "f" + string(rune('a'+i)) + ".txt"
		if err := os.WriteFile(dir+"/"+name, []byte("one\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		runGit(t, dir, "add", name)
	}
	runGit(t, dir, "commit", "-q", "-m", "seed")
	for i := range n {
		name := "f" + string(rune('a'+i)) + ".txt"
		if err := os.WriteFile(dir+"/"+name, []byte("two\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// Diffs issues one git diff per side (--numstat in Load is separate), not one
// call per changed file.
func TestDiffsReadsEveryChangedFileInOneCall(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	const n = 5
	dir := repoWithManyChangedFiles(t, n)
	files, err := Diffs(context.Background(), dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != n {
		t.Fatalf("want %d files, got %d", n, len(files))
	}
	for i := range n {
		name := "f" + string(rune('a'+i)) + ".txt"
		if _, ok := files[name]; !ok {
			t.Errorf("missing %q", name)
		}
	}
}

func TestDiffsParsesEachPathFromTheHeader(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/a.txt", []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/b.txt", []byte("b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "a.txt", "b.txt")
	runGit(t, dir, "commit", "-q", "-m", "seed")
	if err := os.WriteFile(dir+"/a.txt", []byte("A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/b.txt", []byte("B\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, err := Diffs(context.Background(), dir, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.txt", "b.txt"} {
		if _, ok := files[name]; !ok {
			t.Errorf("missing %q in %v", name, fileNames(files))
		}
	}
}

func fileNames(files map[string]FileDiff) []string {
	out := make([]string, 0, len(files))
	for p := range files {
		out = append(out, p)
	}
	return out
}

// A conflict makes git write `diff --cc` with a three-@ header and two columns
// of markers, which nothing else in the repository produces.
func TestDiffParsesCombinedDiffFromMergeConflict(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := conflictedRepo(t)
	d, err := Diff(context.Background(), dir, "f.txt", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Blocks) == 0 {
		t.Fatal("want at least one block from the combined diff")
	}
}

func TestCombinedBlockCountsMatchNumstat(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := conflictedRepo(t)
	d, err := Diff(context.Background(), dir, "f.txt", false)
	if err != nil {
		t.Fatal(err)
	}
	var blockAdded, blockDeleted int
	for _, b := range d.Blocks {
		blockAdded += b.Added
		blockDeleted += b.Deleted
	}
	counts, err := Counts(context.Background(), dir, false)
	if err != nil {
		t.Fatal(err)
	}
	c, ok := counts["f.txt"]
	if !ok {
		t.Fatalf("f.txt missing from counts: %v", counts)
	}
	if blockAdded != c.Added || blockDeleted != c.Deleted {
		t.Errorf("blocks added=%d deleted=%d, numstat added=%d deleted=%d",
			blockAdded, blockDeleted, c.Added, c.Deleted)
	}
}

// git writes the header as `diff --git a/<old> b/<new>`, which cannot be split
// back apart when a name holds " b/", and quotes a name that holds non-ASCII
// bytes. Both shapes reach Diffs through the same map key.
func TestDiffsKeysPathsGitCannotWriteInAHeader(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	names := []string{"plain.txt", "x b/y.txt", "日本語.txt"}
	if err := os.MkdirAll(dir+"/x b", 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		if err := os.WriteFile(dir+"/"+n, []byte("one\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "seed")
	for _, n := range names {
		if err := os.WriteFile(dir+"/"+n, []byte("two\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	m, err := Diffs(context.Background(), dir, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		if _, ok := m[n]; !ok {
			t.Errorf("no diff keyed %q; got keys %v", n, fileNames(m))
		}
	}
}

// git can list a path in --name-only with no diff body behind it, and it
// lists a conflicted path once per merge stage. Treating either as an error
// blanked the whole file list.
func TestDiffsSurvivesAConflictedPath(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := conflictedRepo(t)
	for _, staged := range []bool{false, true} {
		if _, err := Diffs(context.Background(), dir, staged); err != nil {
			t.Fatalf("staged=%v: %v", staged, err)
		}
	}
	m, err := Diffs(context.Background(), dir, false)
	if err != nil {
		t.Fatal(err)
	}
	d, ok := m["f.txt"]
	if !ok {
		t.Fatalf("f.txt missing, got %v", fileNames(m))
	}
	if len(d.Blocks) == 0 {
		t.Error("the conflicted path came back with no blocks")
	}
}

func conflictedRepo(t *testing.T) string {
	t.Helper()
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/f.txt", []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "f.txt")
	runGit(t, dir, "commit", "-q", "-m", "seed")
	runGit(t, dir, "checkout", "-q", "-b", "other")
	if err := os.WriteFile(dir+"/f.txt", []byte("one\nOTHER\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "f.txt")
	runGit(t, dir, "commit", "-q", "-m", "other")
	runGit(t, dir, "checkout", "-q", "main")
	if err := os.WriteFile(dir+"/f.txt", []byte("one\nMAIN\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Committing before the merge is what makes git report the path once per
	// merge stage; merging over an uncommitted edit does not.
	runGit(t, dir, "commit", "-q", "-am", "main")
	cmd := exec.Command("git", "merge", "other")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("merge: want conflict, got success\n%s", out)
	}
	return dir
}

// git names a conflicted path once per merge stage. The duplicate is harmless
// to the map but costs the per-path fallback, so it is dropped here.
func TestDiffNamesAreUnique(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	names, err := diffNames(context.Background(), conflictedRepo(t), false)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, n := range names {
		if seen[n] {
			t.Errorf("%q listed twice in %v", n, names)
		}
		seen[n] = true
	}
}

// A repository with no commits is where an agent starts, and git log exits 128
// there. Rule 8 in docs/git-behavior.md covers this for the write verbs; the
// read that prunes stale marks has to answer it too.
func TestKnownPathsWorksBeforeTheFirstCommit(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")
	if err := os.WriteFile(dir+"/a.txt", []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "a.txt")

	alive, err := KnownPaths(context.Background(), dir)
	if err != nil {
		t.Fatalf("KnownPaths before the first commit: %v", err)
	}
	if !alive["a.txt"] {
		t.Errorf("a.txt is in the index but not reported alive: %v", alive)
	}
}

// A file the agent just created is the most common thing in this pane, and
// git diff says nothing about a path it has never seen. Without this the diff
// area is blank for exactly the file the reader most wants to look at.
func TestDiffShowsAnUntrackedFile(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	runGit(t, dir, "commit", "-q", "--allow-empty", "-m", "base")
	body := "one\ntwo\nthree\n"
	if err := os.WriteFile(dir+"/fresh.txt", []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	d, err := Diff(context.Background(), dir, "fresh.txt", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Blocks) == 0 {
		t.Fatal("no blocks for an untracked file")
	}
	var added int
	for _, b := range d.Blocks {
		added += b.Added
	}
	if added != 3 {
		t.Errorf("added = %d, want 3", added)
	}
}

func TestDiffIsEmptyForATrackedFileFullyStaged(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/a.txt", []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "a.txt")
	runGit(t, dir, "commit", "-q", "-m", "init")
	if err := os.WriteFile(dir+"/a.txt", []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "a.txt")

	d, err := Diff(context.Background(), dir, "a.txt", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Blocks) != 0 {
		t.Fatalf("unstaged diff has %d blocks, want 0", len(d.Blocks))
	}

	d, err = Diff(context.Background(), dir, "a.txt", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Blocks) != 1 {
		t.Fatalf("staged diff has %d blocks, want 1", len(d.Blocks))
	}
}

func TestUnreadableUntrackedFileErrorsInDiffAndCounts(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	runGit(t, dir, "commit", "-q", "--allow-empty", "-m", "base")
	path := dir + "/unreadable.txt"
	if err := os.WriteFile(path, []byte("secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	if _, err := os.ReadFile(path); err == nil {
		t.Skip("os.ReadFile succeeded; cannot test unreadable file")
	}

	if _, err := Diff(context.Background(), dir, "unreadable.txt", false); err == nil {
		t.Error("Diff: want error for unreadable untracked file")
	}
	if _, err := Counts(context.Background(), dir, false); err == nil {
		t.Error("Counts: want error for unreadable untracked file")
	}
}

// blobSize asks git for the size of what is staged, because the file on disk
// may have moved on since. A read that failed is not a size: reporting it as
// one puts zero where the number belongs, and the diff is then held back or let
// through on a figure nothing measured.
func TestBlobSizeReadsTheStagedSizeAndReportsAReadItCouldNotMake(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	const body = "0123456789\n"
	if err := os.WriteFile(dir+"/a.txt", []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "a.txt")
	// The worktree moves on; what is staged does not.
	if err := os.WriteFile(dir+"/a.txt", []byte("longer than what was staged\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := blobSize(context.Background(), dir, "a.txt", true)
	if err != nil {
		t.Fatal(err)
	}
	if got != int64(len(body)) {
		t.Errorf("the staged size is %d, want %d", got, len(body))
	}

	if _, err := blobSize(context.Background(), dir, "not-staged.txt", true); err == nil {
		t.Error("a path with nothing staged for it was answered with a size and no error")
	}
}

// IgnoredDirs is what keeps the watcher off node_modules. An answer of "none"
// spends the descriptor budget there instead of on the working tree, and a read
// that failed is not an answer of none.
func TestIgnoredDirsNamesTheIgnoredOnesAndReportsAReadItCouldNotMake(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/.gitignore", []byte("node_modules/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir+"/node_modules/pkg", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/node_modules/pkg/a.js", []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	dirs, err := IgnoredDirs(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(dirs, "node_modules") {
		t.Errorf("the ignored directory is not in %v", dirs)
	}

	if _, err := IgnoredDirs(context.Background(), t.TempDir()); err == nil {
		t.Error("a directory that is not a repository was answered with no ignored dirs and no error")
	}
}

// The diff bodies are paired with the -z list by position, which holds only
// while there is one body per name. A path in a conflict that has been staged
// is listed with no body behind it: measured, a repository mid-merge with one
// more file staged listed two names and printed one body. Pairing by position
// there hands the second file's diff to the first file's name, and the reader
// opens one file and reads another's changes.
func TestDiffsKeepEachDiffWithItsOwnPathWhenABodyIsMissing(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := conflictedRepo(t)
	if err := os.WriteFile(dir+"/g.txt", []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "g.txt")

	names, err := diffNames(context.Background(), dir, true)
	if err != nil {
		t.Fatal(err)
	}
	out, err := runRead(context.Background(), dir, "diff", "--cached")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) == len(parseDiff(out, "")) {
		t.Fatal("this state has one body per name, so it proves nothing")
	}

	files, err := Diffs(context.Background(), dir, true)
	if err != nil {
		t.Fatal(err)
	}
	added := func(d FileDiff) string {
		for _, b := range d.Blocks {
			for _, line := range b.Lines {
				if strings.HasPrefix(line, "+") {
					return line
				}
			}
		}
		return ""
	}
	if got := added(files["g.txt"]); got != "+new" {
		t.Errorf("g.txt holds %q, want the line staged into it", got)
	}
	if got := added(files["f.txt"]); got == "+new" {
		t.Error("f.txt holds the line that was staged into g.txt")
	}
	for path, d := range files {
		if d.Path != path {
			t.Errorf("the diff under %q says it is %q", path, d.Path)
		}
	}
}
