package git

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestStashOfAnUntrackedFileNeedsTheFlag(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/new.txt", []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	entry := Entry{Path: "new.txt", Worktree: Untracked}
	out, err := Stash(context.Background(), dir, []Entry{entry})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Applied) != 1 {
		t.Fatalf("want one applied, got %+v", out)
	}
	if _, err := os.Stat(dir + "/new.txt"); !os.IsNotExist(err) {
		t.Fatal("want file stashed away")
	}
	runGit(t, dir, "stash", "pop", "-q")
	data, err := os.ReadFile(dir + "/new.txt")
	if err != nil || string(data) != "x\n" {
		t.Fatalf("pop: got %q err=%v", data, err)
	}
}

func TestStashOfAnUntrackedFileDoesNotDropTheTrackedOnesWithIt(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to git")
	}
	dir := newRepo(t)
	if err := os.WriteFile(dir+"/tracked.txt", []byte("t\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "tracked.txt")
	runGit(t, dir, "commit", "-q", "-m", "add")
	if err := os.WriteFile(dir+"/tracked.txt", []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/new.txt", []byte("u\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tracked := Entry{Path: "tracked.txt", Worktree: Modified}
	untracked := Entry{Path: "new.txt", Worktree: Untracked}
	out, err := Stash(context.Background(), dir, []Entry{tracked, untracked})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Applied) != 2 {
		t.Fatalf("want both applied, got %+v", out)
	}
	data, err := os.ReadFile(dir + "/tracked.txt")
	if err != nil || string(data) != "t\n" {
		t.Fatalf("tracked file should match HEAD after stash: %q err=%v", data, err)
	}
	if _, err := os.Stat(dir + "/new.txt"); !os.IsNotExist(err) {
		t.Fatal("want untracked file stashed")
	}
}

func TestStashReportsFailureWhenTheEntryCountDidNotGrow(t *testing.T) {
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
	entry := Entry{Path: "a.txt"}
	before, err := stashCount(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Stash(context.Background(), dir, []Entry{entry})
	if err != nil {
		t.Fatal(err)
	}
	after, err := stashCount(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("stash count grew from %d to %d", before, after)
	}
	if len(out.Ignored) != 1 || len(out.Applied) != 0 {
		t.Fatalf("want one ignored, got %+v", out)
	}
}

func TestStashRefusesAnOversizedPathspec(t *testing.T) {
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
	before, err := stashCount(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	long := strings.Repeat("x", maxStashPathspecBytes)
	entry := Entry{Path: long + ".txt", Worktree: Modified}
	_, err = Stash(context.Background(), dir, []Entry{entry})
	if err == nil {
		t.Fatal("want error for oversized pathspec")
	}
	var want *OversizedPathspecError
	if !errors.As(err, &want) {
		t.Fatalf("got %T: %v", err, err)
	}
	after, err := stashCount(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatal("stash count should not grow when refused")
	}
}
