package git

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestStashListReadsMessageBranchAndFileCount(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/a.txt", []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "a.txt")
	runGit(t, dir, "commit", "-q", "-m", "add")
	if err := os.WriteFile(dir+"/a.txt", []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "stash", "push", "-q", "-m", "hold this")
	rows := stashListFilled(t, dir, Head{Branch: "main"})
	if len(rows) != 1 {
		t.Fatalf("want one stash, got %+v", rows)
	}
	row := rows[0]
	if row.Message != "hold this" {
		t.Errorf("message = %q", row.Message)
	}
	if row.Branch != "main" {
		t.Errorf("branch = %q", row.Branch)
	}
	if row.FileCount != 1 {
		t.Errorf("file count = %d", row.FileCount)
	}
	if row.Ref == "" || row.SHA == "" || row.Age == "" {
		t.Errorf("missing fields: %+v", row)
	}
}

func TestStashListIsNewestFirst(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/a.txt", []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "a.txt")
	runGit(t, dir, "commit", "-q", "-m", "add")
	for _, msg := range []string{"first", "second"} {
		if err := os.WriteFile(dir+"/a.txt", []byte(msg+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		runGit(t, dir, "stash", "push", "-q", "-m", msg)
	}
	rows := stashListFilled(t, dir, Head{Branch: "main"})
	if len(rows) != 2 {
		t.Fatalf("want two stashes, got %d", len(rows))
	}
	if rows[0].Message != "second" || rows[1].Message != "first" {
		t.Fatalf("order = %q then %q", rows[0].Message, rows[1].Message)
	}
}

// stash pop, branch and drop name a stash, not paths. Sending them through the
// pathspec-from-file wrapper makes git reject the option and the verb never
// runs at all.
func TestStashVerbsRunWithoutAPathspec(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	for _, c := range []struct {
		name string
		run  func(ctx context.Context, dir, ref string) error
		want func(t *testing.T, dir string)
	}{
		{"pop", StashPop, func(t *testing.T, dir string) {
			b, err := os.ReadFile(dir + "/a.txt")
			if err != nil || string(b) != "two\n" {
				t.Errorf("a.txt = %q err=%v, want the stash applied", b, err)
			}
		}},
		{"drop", stashDrop,
			func(t *testing.T, dir string) {
				rows := stashListFilled(t, dir, Head{})
				if len(rows) != 0 {
					t.Errorf("%d stashes left, want none", len(rows))
				}
			}},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := newRepo(t)
			if err := os.WriteFile(dir+"/a.txt", []byte("one\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			runGit(t, dir, "add", "-A")
			runGit(t, dir, "commit", "-q", "-m", "base")
			if err := os.WriteFile(dir+"/a.txt", []byte("two\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			runGit(t, dir, "stash", "-q")
			if err := c.run(context.Background(), dir, "stash@{0}"); err != nil {
				t.Fatalf("%s: %v", c.name, err)
			}
			c.want(t, dir)
		})
	}
}

func TestStashBranchRunsWithoutAPathspec(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/a.txt", []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "base")
	if err := os.WriteFile(dir+"/a.txt", []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "stash", "-q")
	if err := StashBranch(context.Background(), dir, "stash@{0}", "recovered"); err != nil {
		t.Fatalf("StashBranch: %v", err)
	}
	if b, _ := os.ReadFile(dir + "/a.txt"); string(b) != "two\n" {
		t.Errorf("a.txt = %q, want the stash applied on the new branch", b)
	}
}

// git stash branch exits 0 whether or not it applied anything. With the stash's
// own paths edited in the tree it creates the branch, applies nothing, keeps the
// stash, and says so only in prose. The pane routes conflicting stashes here, so
// silence would be a dead end that reads like success.
func TestStashBranchReportsAStashItCouldNotApply(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/a.txt", []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "base")
	if err := os.WriteFile(dir+"/a.txt", []byte("base\nstashed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "stash", "-q")
	if err := os.WriteFile(dir+"/a.txt", []byte("base\nlocal\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := StashBranch(context.Background(), dir, "stash@{0}", "recovered"); err == nil {
		t.Fatal("no error, but the stash was not applied")
	}
	rows := stashListFilled(t, dir, Head{})
	if len(rows) != 1 {
		t.Errorf("%d stashes, want the unapplied one still there", len(rows))
	}
}

func TestStashDropBySHADropsThatStashAfterTheListShifts(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/a.txt", []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "a.txt")
	runGit(t, dir, "commit", "-q", "-m", "add")
	for _, msg := range []string{"first", "second"} {
		if err := os.WriteFile(dir+"/a.txt", []byte(msg+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		runGit(t, dir, "stash", "push", "-q", "-m", msg)
	}
	rows := stashListFilled(t, dir, Head{Branch: "main"})
	if len(rows) != 2 {
		t.Fatalf("want two stashes, got %d", len(rows))
	}
	firstSHA := rows[1].SHA
	if rows[1].Message != "first" {
		t.Fatalf("oldest stash = %q, want first", rows[1].Message)
	}
	if err := os.WriteFile(dir+"/a.txt", []byte("third\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "stash", "push", "-q", "-m", "third")
	if err := StashDropBySHA(context.Background(), dir, firstSHA); err != nil {
		t.Fatal(err)
	}
	rows = stashListFilled(t, dir, Head{Branch: "main"})
	if len(rows) != 2 {
		t.Fatalf("want two stashes, got %d", len(rows))
	}
	if rows[0].Message != "third" || rows[1].Message != "second" {
		t.Fatalf("order = %q then %q", rows[0].Message, rows[1].Message)
	}
	for _, row := range rows {
		if row.SHA == firstSHA {
			t.Fatal("first stash still present")
		}
	}
}

func TestStashDropBySHAReportsAStashThatIsGone(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/a.txt", []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "a.txt")
	runGit(t, dir, "commit", "-q", "-m", "add")
	if err := os.WriteFile(dir+"/a.txt", []byte("hold\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "stash", "push", "-q", "-m", "hold")
	gone := strings.Repeat("0", 40)
	if err := StashDropBySHA(context.Background(), dir, gone); err == nil {
		t.Fatal("expected error for missing stash")
	}
	rows := stashListFilled(t, dir, Head{})
	if len(rows) != 1 {
		t.Fatalf("want one stash left, got %d", len(rows))
	}
}

func TestStashFilesMarksThePathEditedInTheTree(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/a.txt", []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/b.txt", []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "base")
	if err := os.WriteFile(dir+"/a.txt", []byte("base\nstashed-a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/b.txt", []byte("base\nstashed-b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "stash", "push", "-q", "-m", "both")
	if err := os.WriteFile(dir+"/a.txt", []byte("base\nlocal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rows := stashListFilled(t, dir, Head{})
	if len(rows) == 0 {
		t.Fatal("no stash was listed")
	}
	if !rows[0].CollidesWith("a.txt") {
		t.Errorf("a.txt should collide; the stash names %v", rows[0].Collides)
	}
	if rows[0].CollidesWith("b.txt") {
		t.Errorf("b.txt should not collide; the stash names %v", rows[0].Collides)
	}
}

func TestStashFilesListUntrackedAndDeleted(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/a.txt", []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir+"/docs", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/docs/d.md", []byte("doc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "base")
	if err := os.WriteFile(dir+"/a.txt", []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/b.txt", []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(dir + "/docs/d.md"); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "stash", "push", "-q", "-u", "-m", "wip")
	files, err := StashFiles(context.Background(), dir, "stash@{0}")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 {
		t.Fatalf("want 3 files, got %+v", files)
	}
	want := []struct {
		path       string
		kind       Kind
		add        int
		del        int
		checkCount bool
	}{
		{"a.txt", Modified, 0, 0, false},
		{"b.txt", Untracked, 1, 0, true},
		{"docs/d.md", Deleted, 0, 1, true},
	}
	for i, w := range want {
		f := files[i]
		if f.Path != w.path {
			t.Errorf("files[%d].Path = %q, want %q", i, f.Path, w.path)
		}
		if f.Worktree != w.kind {
			t.Errorf("files[%d].Worktree = %q, want %q", i, f.Worktree, w.kind)
		}
		if w.checkCount && (f.WorktreeCount.Added != w.add || f.WorktreeCount.Deleted != w.del) {
			t.Errorf("files[%d].WorktreeCount = %+v, want added=%d deleted=%d", i, f.WorktreeCount, w.add, w.del)
		}
	}
}

func TestStashFileCountIncludesUntracked(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/a.txt", []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir+"/docs", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/docs/d.md", []byte("doc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "base")
	if err := os.WriteFile(dir+"/a.txt", []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/b.txt", []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(dir + "/docs/d.md"); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "stash", "push", "-q", "-u", "-m", "wip")
	rows := stashListFilled(t, dir, Head{Branch: "main"})
	if len(rows) != 1 {
		t.Fatalf("want one stash, got %d", len(rows))
	}
	if rows[0].FileCount != 3 {
		t.Errorf("file count = %d, want 3", rows[0].FileCount)
	}
}

func TestStashDiffIsTheStashAgainstItsParentNotHEAD(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	const path = "held.txt"
	if err := os.WriteFile(dir+"/"+path, []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", path)
	runGit(t, dir, "commit", "-q", "-m", "base")
	if err := os.WriteFile(dir+"/"+path, []byte("stashed line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "stash", "push", "-q", "-m", "hold")
	rows := stashListFilled(t, dir, Head{Branch: "main"})
	if len(rows) != 1 {
		t.Fatalf("want one stash, got %d", len(rows))
	}
	sha := rows[0].SHA
	if err := os.WriteFile(dir+"/"+path, []byte("after stash\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", path)
	runGit(t, dir, "commit", "-q", "-m", "move head")

	diff, err := StashDiff(context.Background(), dir, sha, path)
	if err != nil {
		t.Fatal(err)
	}
	if !diffLineContains(diff, "stashed line") {
		t.Fatalf("stash diff = %+v, want the stashed line", diff)
	}
	if diffLineContains(diff, "after stash") {
		t.Fatalf("stash diff = %+v, must not be against HEAD", diff)
	}
}

func TestStashDiffIncludesUntracked(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	const path = "new.txt"
	if err := os.WriteFile(dir+"/a.txt", []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "a.txt")
	runGit(t, dir, "commit", "-q", "-m", "base")
	if err := os.WriteFile(dir+"/"+path, []byte("untracked body\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "stash", "push", "-q", "-u", "-m", "hold")
	rows := stashListFilled(t, dir, Head{Branch: "main"})
	if len(rows) != 1 {
		t.Fatalf("want one stash, got %d", len(rows))
	}
	diff, err := StashDiff(context.Background(), dir, rows[0].SHA, path)
	if err != nil {
		t.Fatal(err)
	}
	if !diffLineContains(diff, "untracked body") {
		t.Fatalf("stash diff = %+v, want untracked body", diff)
	}
}

func diffLineContains(d FileDiff, want string) bool {
	for _, b := range d.Blocks {
		for _, line := range b.Lines {
			if strings.Contains(line, want) {
				return true
			}
		}
	}
	return false
}

// A stash subject reads "WIP on main: the message". The branch is what comes
// before the colon, and a subject whose colon is at the front names no branch —
// the whole of it after the colon is the message.
func TestAStashSubjectSplitsAtTheFirstColon(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		subject, branch, message string
	}{
		{"WIP on main: changed a thing", "main", "changed a thing"},
		{"On main: changed a thing", "main", "changed a thing"},
		{"WIP on : no branch name", "", "no branch name"},
		{"WIP on main", "", "WIP on main"},
		{"whatever the reader typed", "", "whatever the reader typed"},
		{"", "", ""},
	} {
		t.Run(c.subject, func(t *testing.T) {
			t.Parallel()
			branch, message := parseStashSubject(c.subject)
			if branch != c.branch || message != c.message {
				t.Errorf("split %q into %q and %q, want %q and %q",
					c.subject, branch, message, c.branch, c.message)
			}
		})
	}
}
