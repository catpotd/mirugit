package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func repoWithBinaryChange(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	env := []string{
		"HOME=" + dir,
		"GIT_CONFIG_GLOBAL=" + dir + "/.gitconfig",
		"GIT_CONFIG_SYSTEM=/dev/null",
		// git forks a repack of its own once a repository crosses a
		// threshold, and that child outlives the test: it races
		// t.TempDir's removal, and it takes the repository's locks
		// while another test is reading it.
		"GIT_CONFIG_COUNT=2",
		"GIT_CONFIG_KEY_0=gc.auto", "GIT_CONFIG_VALUE_0=0",
		"GIT_CONFIG_KEY_1=maintenance.auto", "GIT_CONFIG_VALUE_1=false",
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
	}
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	if err := os.WriteFile(dir+"/img.png", []byte{0, 1, 2, 3, 4}, 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "img.png")
	run("commit", "-q", "-m", "base")
	if err := os.WriteFile(dir+"/img.png", []byte{0, 1, 2, 3, 4, 5}, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestBinaryDiffHasNoBlocksAndReportsSizeAndMode(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := repoWithBinaryChange(t)
	d, err := Diff(context.Background(), dir, "img.png", false)
	if err != nil {
		t.Fatal(err)
	}
	if !d.Binary {
		t.Fatal("want a binary diff")
	}
	if len(d.Blocks) != 0 {
		t.Fatalf("want no blocks, got %d", len(d.Blocks))
	}
	if d.Size != 6 {
		t.Errorf("size = %d, want 6", d.Size)
	}
	if d.Mode != "100644" {
		t.Errorf("mode = %q, want 100644", d.Mode)
	}
}

func repoWithTwoBlocks(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	env := []string{
		"HOME=" + dir,
		"GIT_CONFIG_GLOBAL=" + dir + "/.gitconfig",
		"GIT_CONFIG_SYSTEM=/dev/null",
		// git forks a repack of its own once a repository crosses a
		// threshold, and that child outlives the test: it races
		// t.TempDir's removal, and it takes the repository's locks
		// while another test is reading it.
		"GIT_CONFIG_COUNT=2",
		"GIT_CONFIG_KEY_0=gc.auto", "GIT_CONFIG_VALUE_0=0",
		"GIT_CONFIG_KEY_1=maintenance.auto", "GIT_CONFIG_VALUE_1=false",
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
	}
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	content := "one\ntwo\nthree\n"
	for i := 4; i <= 20; i++ {
		content += fmt.Sprintf("line %d\n", i)
	}
	content += "twenty\n"
	if err := os.WriteFile(dir+"/a.txt", []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "a.txt")
	run("commit", "-q", "-m", "base")
	changed := strings.Replace(content, "two\n", "two\nadded\n", 1)
	changed = strings.Replace(changed, "twenty\n", "twenty\ntwentyone\n", 1)
	if err := os.WriteFile(dir+"/a.txt", []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestStageBlockLeavesOtherBlocksUnstaged(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := repoWithTwoBlocks(t)
	d, err := Diff(context.Background(), dir, "a.txt", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Blocks) != 2 {
		t.Fatalf("want 2 blocks, got %d", len(d.Blocks))
	}
	if err := StageBlock(context.Background(), dir, d, 0); err != nil {
		t.Fatal(err)
	}
	staged, err := Diff(context.Background(), dir, "a.txt", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(staged.Blocks) != 1 {
		t.Fatalf("staged side = %d blocks, want 1", len(staged.Blocks))
	}
	unstaged, err := Diff(context.Background(), dir, "a.txt", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(unstaged.Blocks) != 1 {
		t.Fatalf("worktree side = %d blocks, want 1", len(unstaged.Blocks))
	}
}

func repoWithUntrackedAndDeletedFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	env := []string{
		"HOME=" + dir,
		"GIT_CONFIG_GLOBAL=" + dir + "/.gitconfig",
		"GIT_CONFIG_SYSTEM=/dev/null",
		// git forks a repack of its own once a repository crosses a
		// threshold, and that child outlives the test: it races
		// t.TempDir's removal, and it takes the repository's locks
		// while another test is reading it.
		"GIT_CONFIG_COUNT=2",
		"GIT_CONFIG_KEY_0=gc.auto", "GIT_CONFIG_VALUE_0=0",
		"GIT_CONFIG_KEY_1=maintenance.auto", "GIT_CONFIG_VALUE_1=false",
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
	}
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	if err := os.WriteFile(dir+"/d.md", []byte("# doc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "d.md")
	run("commit", "-q", "-m", "base")
	if err := os.Remove(dir + "/d.md"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/b.txt", []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestStageBlockStagesUntrackedFile(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := repoWithUntrackedAndDeletedFile(t)
	d, err := Diff(context.Background(), dir, "b.txt", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Blocks) != 1 {
		t.Fatalf("want 1 block, got %d", len(d.Blocks))
	}
	if err := StageBlock(context.Background(), dir, d, 0); err != nil {
		t.Fatal(err)
	}
	out, err := runRead(context.Background(), dir, "ls-files")
	if err != nil {
		t.Fatal(err)
	}
	listed := string(out)
	if !strings.Contains(listed, "b.txt") {
		t.Errorf("ls-files = %q, want b.txt", listed)
	}
	if strings.Contains(listed, "dev/null") {
		t.Errorf("ls-files = %q, want no dev/null", listed)
	}
}

func TestStageBlockStagesDeletedFile(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := repoWithUntrackedAndDeletedFile(t)
	d, err := Diff(context.Background(), dir, "d.md", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Blocks) != 1 {
		t.Fatalf("want 1 block, got %d", len(d.Blocks))
	}
	if err := StageBlock(context.Background(), dir, d, 0); err != nil {
		t.Fatal(err)
	}
	out, err := runRead(context.Background(), dir, "ls-files")
	if err != nil {
		t.Fatal(err)
	}
	listed := string(out)
	if strings.Contains(listed, "d.md") {
		t.Errorf("ls-files = %q, want d.md gone from index", listed)
	}
	if strings.Contains(listed, "dev/null") {
		t.Errorf("ls-files = %q, want no dev/null", listed)
	}
	status, err := runRead(context.Background(), dir, "status", "--porcelain")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(status), "D  d.md") {
		t.Errorf("status = %q, want D  d.md", string(status))
	}
}

// The block a key names is read out of the list the frame drew, and the list
// can be shorter by the time the key arrives: a reload replaces the diff while
// the reader is looking at it. An index one past the end has to be refused
// before the patch is built, because building it reads the block that is not
// there.
func TestStagingABlockRefusesAnIndexTheDiffDoesNotHave(t *testing.T) {
	t.Parallel()
	d := FileDiff{
		Path:   "a.txt",
		Header: []string{"diff --git a/a.txt b/a.txt", "--- a/a.txt", "+++ b/a.txt"},
		Blocks: []Block{
			{Header: "@@ -1,1 +1,2 @@", Lines: []string{" a", "+b"}},
			{Header: "@@ -9,1 +10,2 @@", Lines: []string{" c", "+d"}},
		},
	}
	// The directory is never reached: the refusal happens before git starts.
	for _, index := range []int{-1, -9, len(d.Blocks), len(d.Blocks) + 1, 99} {
		if err := StageBlock(context.Background(), "", d, index); err == nil {
			t.Errorf("block %d was accepted for a diff of %d blocks",
				index, len(d.Blocks))
		}
	}
	// A diff with no blocks has no block to stage either.
	if err := StageBlock(context.Background(), "", FileDiff{Path: "a.txt"}, 0); err == nil {
		t.Error("a diff with no blocks accepted block 0")
	}
}
