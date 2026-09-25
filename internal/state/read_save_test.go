package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveUsesPrivateStatePermissions(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "repo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "read.json")
	r := &ReadState{path: file, Finished: map[string]bool{}, Blocks: map[string]bool{}}

	if err := r.Save(); err != nil {
		t.Fatal(err)
	}

	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Errorf("directory mode = %v, want 0700", got)
	}
	fileInfo, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if got := fileInfo.Mode().Perm(); got != 0o600 {
		t.Errorf("file mode = %v, want 0600", got)
	}
}

// Writing in place leaves a half-written file if the process ends mid-write,
// and LoadRead drops every mark when it cannot parse the file. Rename replaces
// it in one step, so an interrupted save leaves the previous marks readable.
func TestSaveReplacesTheFileInOneStep(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	file := filepath.Join(dir, "read.json")
	r := &ReadState{
		path:     file,
		Finished: map[string]bool{"changes:a.txt": true},
		Blocks:   map[string]bool{},
	}
	if err := r.Save(); err != nil {
		t.Fatal(err)
	}
	// A save leaves nothing behind but the file itself.
	found, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(found))
	for _, e := range found {
		names = append(names, e.Name())
	}
	if len(names) != 1 || names[0] != "read.json" {
		t.Errorf("保存後に残ったもの = %v, want [read.json]", names)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "changes:a.txt") {
		t.Errorf("保存した内容が読めない: %s", data)
	}
	info, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", info.Mode().Perm())
	}

	// The file is never opened for writing in place, so a reader that opens it
	// while a save runs sees either the old bytes or the new ones. Writing in
	// place truncates first, and a crash there left LoadRead nothing to parse.
	big := &ReadState{path: file, Finished: map[string]bool{}, Blocks: map[string]bool{}}
	for i := range 2000 {
		big.Finished[strings.Repeat("x", 40)+string(rune('a'+i%26))+string(rune(i))] = true
	}
	if err := big.Save(); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(file); err != nil || info.Size() == 0 {
		t.Fatalf("大きな保存の後にファイルが空: err=%v", err)
	}
}

// Save on the update goroutine stops input while the file is written. The model
// takes the bytes with Snapshot and hands them to a command, so a later change
// to the marks cannot alter what is being written.
func TestSnapshotFixesTheBytesBeforeWriting(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	file := filepath.Join(dir, "read.json")
	r := &ReadState{
		path:     file,
		Finished: map[string]bool{"changes:a.txt": true},
		Blocks:   map[string]bool{},
	}
	encoded, path, err := r.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	// The model marks another file while the command is still queued.
	r.Finished["changes:b.txt"] = true
	if err := WriteSnapshot(encoded, path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "b.txt") {
		t.Error("Snapshot の後の変更が書き込まれた")
	}
	if !strings.Contains(string(data), "a.txt") {
		t.Error("Snapshot 時点の内容が書かれていない")
	}
}

// A save that cannot finish has to say so. Reporting success leaves the reader
// believing the marks are written, and the next run of mirugit shows every file
// unread with nothing explaining why.
func TestASaveThatCannotReplaceTheFileReportsIt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// A directory with something in it cannot be replaced by a file.
	file := filepath.Join(dir, "read.json")
	if err := os.MkdirAll(filepath.Join(file, "occupied"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := &ReadState{
		path:     file,
		Finished: map[string]bool{"changes:a.txt": true},
		Blocks:   map[string]bool{},
	}

	if err := r.Save(); err == nil {
		t.Fatal("a save that replaced nothing reported success")
	}

	// The half-written file goes with the failure: left behind, one builds up
	// per save in the directory the marks are read from.
	found, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range found {
		if strings.HasPrefix(e.Name(), ".read-") {
			t.Errorf("%q was left behind by a save that failed", e.Name())
		}
	}
}
