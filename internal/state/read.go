// Package state holds what the program believes and what the reader is doing
// with it. Apply is the one place that changes it, and every transition is a
// pure function of the state and one event.
//
// Two things here are not that, and both are here because splitting them would
// put one question in two packages.
//
// The read marks are written to disk. They are state that outlives the process,
// and the file they go in is keyed by the same repository path this package
// already keys them by; a writer somewhere else would have to be told the
// layout of a record it does not own.
//
// Plus, Minus, FileWord and ParentDir turn values into the words a notice is
// built from. Apply writes those notices, so moving the words to the drawing
// package would point this package at it, and the imports run the other way.
package state

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/catpotd/mirugit/internal/git"
)

// Mark is the read state shown in a file row gutter. Unread is the zero value,
// so a field left unset cannot pass for read or changed.
type Mark int

const (
	// Unread means the reader has not finished this row.
	Unread Mark = iota
	// Changed means the reader finished this row and new content appeared since.
	Changed
	// Read means the reader finished this row and it has not changed since.
	Read
)

// ReadState keys on the path and the hash of a block's content. The hash alone
// would mark nine files read when an agent applies one edit to ten; the path
// alone would not survive the commit that renumbers every line.
type ReadState struct {
	path string
	Repo string `json:"repo"`
	// Version is the key layout the file was written with. Changing how a mark
	// is keyed leaves every old entry unmatched, which reads as ● on files the
	// reader has read; a mismatch drops the record instead.
	Version int `json:"version"`
	// Finished separates ● from ·: a file abandoned after three lines is still
	// unread, and only one that was read through can be said to have changed.
	// The spec calls this opened; Finished is the name in the record because r
	// read must set the same flag opening does, not introduce a parallel one.
	Finished map[string]bool `json:"finished"`
	Blocks   map[string]bool `json:"read"`
	// worktreeHashes and stagedHashes mirror git in memory only. Persisting them
	// would make every commit look unread after a restart.
	worktreeHashes map[string][]string
	stagedHashes   map[string][]string
}

func LoadRead(ctx context.Context, stateDir, repo string) (*ReadState, error) {
	sum := sha256.Sum256([]byte(repo))
	file := filepath.Join(stateDir, hex.EncodeToString(sum[:])[:16], "read.json")

	encoded, err := os.ReadFile(file)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// Unmarshal leaves a field the file does not mention alone, so the target
	// starts at version zero rather than at the current one.
	loaded := empty(file, repo)
	loaded.Version = 0
	if json.Unmarshal(encoded, loaded) == nil && loaded.Version == readVersion {
		if err := loaded.Prune(ctx, repo); err != nil {
			return nil, err
		}
		return loaded, nil
	}
	// An unreadable or corrupt file costs the reader their marks, not their
	// session, so it is treated as an absent one.
	rs := empty(file, repo)
	if err := rs.Prune(ctx, repo); err != nil {
		return nil, err
	}
	return rs, nil
}

// readVersion goes up whenever a key changes shape.
const readVersion = 2

func empty(file, repo string) *ReadState {
	return &ReadState{
		path:     file,
		Repo:     repo,
		Version:  readVersion,
		Finished: map[string]bool{},
		Blocks:   map[string]bool{},
	}
}

// Snapshot returns what Save would write. The caller hands it to a command, so
// the bytes are fixed before the model is free to change the marks again.
func (r *ReadState) Snapshot() ([]byte, string, error) {
	encoded, err := json.Marshal(r)
	if err != nil {
		return nil, "", err
	}
	return encoded, r.path, nil
}

// WriteSnapshot puts bytes taken by Snapshot where they belong. It takes no
// receiver so it can run on a command goroutine without racing the model.
func WriteSnapshot(encoded []byte, path string) error {
	return writeAtomic(encoded, path)
}

func (r *ReadState) Save() error {
	encoded, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return writeAtomic(encoded, r.path)
}

// writeAtomic replaces path in one step. Writing in place leaves a half-written
// file if the process ends mid-write, and LoadRead then drops every mark.
func writeAtomic(encoded []byte, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".read-*.json")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(encoded); err != nil {
		_ = tmp.Close()
		_ = os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	if err := os.Chmod(name, 0o600); err != nil {
		_ = os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		_ = os.Remove(name)
		return err
	}
	return nil
}

// NUL separates because it is the one byte a path cannot contain. A colon can,
// and then Rename would rewrite the wrong entries.
func key(path, hash string) string { return path + "\x00" + hash }

// The record has two layers. Blocks holds content: a block hash is the same
// lines wherever they sit, so staging what was read keeps it read. Finished
// holds a row: only the row the reader read through can later show ●, and a
// path staged and edited again has a row on each side.
func fileKey(o Origin, path string) string { return o.key() + path }

func sectionPrefix(sec Section) string {
	switch sec {
	case SectionStaged:
		return "staged:"
	case SectionUnstaged:
		return "changes:"
	case SectionNone:
	}
	// Go wants a statement after a switch even when every value is named, and
	// the linter has already checked that every Section is.
	return "none:"
}

// finishedPath undoes fileKey. A key written before the rows were separated, or
// before the origin carried which commit or stash it came from, belongs to no
// row and is dropped.
func finishedPath(k string) (string, bool) {
	for _, p := range []string{"staged:", "changes:", "none:"} {
		if rest, ok := strings.CutPrefix(k, p); ok {
			return rest, true
		}
	}
	for _, p := range []string{"commit:", "parent:"} {
		if rest, ok := strings.CutPrefix(k, p); ok {
			if _, path, found := strings.Cut(rest, "\x00"); found {
				return path, true
			}
		}
	}
	return "", false
}

func (r *ReadState) MarkBlock(path, hash string) {
	r.Blocks[key(path, hash)] = true
}

// MarkFileIn is the only thing that finishes a row, so reading part of one and
// closing it leaves it unread rather than inventing a fourth state.
func (r *ReadState) MarkFileIn(o Origin, path string, hashes []string) {
	for _, h := range hashes {
		r.MarkBlock(path, h)
	}
	r.Finished[fileKey(o, path)] = true
}

// MarkShownIn records only the blocks the reader actually saw. When every block
// in the row is marked, the row is finished the same way MarkFileIn would.
func (r *ReadState) MarkShownIn(o Origin, path string, shown, all []string) {
	for _, h := range shown {
		r.MarkBlock(path, h)
	}
	if len(all) == 0 {
		return
	}
	for _, h := range all {
		if !r.BlockRead(path, h) {
			return
		}
	}
	r.Finished[fileKey(o, path)] = true
}

func (r *ReadState) BlockRead(path, hash string) bool {
	if r == nil {
		return false
	}
	return r.Blocks[key(path, hash)]
}

// MarkStash records that a stash commit was seen. The key is the stash SHA,
// not path plus block hash, because a stash row has no diff blocks and the SHA
// names its contents until pop removes the entry.
func (r *ReadState) MarkStash(sha string) {
	r.mark(stashKey(sha))
}

func (r *ReadState) StashMark(sha string) Mark {
	return r.markOf(stashKey(sha))
}

// MarkWorktree records that a worktree tip was seen. The key is path and SHA,
// like stash uses SHA alone, because a worktree row has no diff blocks.
func (r *ReadState) MarkWorktree(path, sha string) {
	r.mark(worktreeKey(path, sha))
}

// mark and markOf are what every row kind that has no diff blocks does with its
// key. Written per kind, the nil check that markOf carries was in one of them
// and not the other.
func (r *ReadState) mark(key string) {
	r.Finished[key] = true
}

func (r *ReadState) markOf(key string) Mark {
	if r == nil || !r.Finished[key] {
		return Unread
	}
	return Read
}

func stashKey(sha string) string {
	return "stash:" + sha
}

func (r *ReadState) WorktreeMark(path, sha string) Mark {
	return r.markOf(worktreeKey(path, sha))
}

func worktreeKey(path, sha string) string {
	return "worktree:" + path + ":" + sha
}

// MarkIn reads both layers for one row. Every block seen means read, wherever
// the reader saw them; ● needs this row to be the one they read through, or a
// path staged and edited again would claim the untouched side changed.
func (r *ReadState) MarkIn(o Origin, path string, hashes []string) Mark {
	// No record means unread. BlockRead and HashesIn answer the same way, and
	// without this a caller holding a typed nil would fault on the map read.
	if r == nil {
		return Unread
	}
	seen := 0
	for _, h := range hashes {
		if r.BlockRead(path, h) {
			seen++
		}
	}
	// Reading through on one side and then staging moves the same lines to the
	// other row, so seeing every block counts there too. Without the finished
	// test a file abandoned halfway would turn read the moment its last block
	// happened to be shown.
	if len(hashes) > 0 && seen == len(hashes) && r.finishedAnySide(path) {
		return Read
	}
	if !r.Finished[fileKey(o, path)] {
		return Unread
	}
	if len(hashes) == 0 {
		return Read
	}
	return Changed
}

// finishedAnySide answers for the two rows one working-tree path can occupy.
// Reading through on one side and then staging moves the same lines to the
// other row. A commit or a stash is a different file, so neither counts here.
func (r *ReadState) finishedAnySide(path string) bool {
	for _, sec := range []Section{SectionStaged, SectionUnstaged} {
		if r.Finished[fileKey(WorkingTree(sec), path)] {
			return true
		}
	}
	return false
}

// BlockHashes is the block hash of every changed file on both sides. Each side
// costs one git diff call regardless of how many paths changed.
//
// It is a function rather than a method so the caller can read it off the
// update loop and hand the answer to SetHashes, the way Snapshot and
// WriteSnapshot split the write. Reading it in place put two git processes on
// the goroutine that draws.
type BlockHashes struct {
	Worktree map[string][]string
	Staged   map[string][]string
}

func ReadBlockHashes(ctx context.Context, dir string) (BlockHashes, error) {
	wt, err := git.Diffs(ctx, dir, false)
	if err != nil {
		return BlockHashes{}, err
	}
	st, err := git.Diffs(ctx, dir, true)
	if err != nil {
		return BlockHashes{}, err
	}
	return BlockHashes{Worktree: blockHashes(wt), Staged: blockHashes(st)}, nil
}

func (r *ReadState) SetHashes(h BlockHashes) {
	r.worktreeHashes = h.Worktree
	r.stagedHashes = h.Staged
}

func blockHashes(files map[string]git.FileDiff) map[string][]string {
	m := make(map[string][]string, len(files))
	for path, f := range files {
		hs := make([]string, len(f.Blocks))
		for i, b := range f.Blocks {
			hs[i] = b.Hash
		}
		m[path] = hs
	}
	return m
}

// HashesIn returns the block hashes git reports for one side of a path. The
// two sides are separate diffs and a row shows only its own. A commit's blocks
// are not cached here, so the history section has none to give.
func (r *ReadState) HashesIn(o Origin, path string) []string {
	if r == nil {
		return nil
	}
	switch o.Side() {
	case SectionStaged:
		return r.stagedHashes[path]
	case SectionUnstaged:
		return r.worktreeHashes[path]
	case SectionNone:
	}
	// A commit and a stash cache no hashes: their diffs are not the working
	// tree's, and the marks for them are keyed by which commit or stash it was.
	return nil
}

// Prune drops read marks whose path left the working tree and the last hundred
// commits, so the on-disk record does not grow without bound.
func (r *ReadState) Prune(ctx context.Context, repo string) error {
	alive, err := git.KnownPaths(ctx, repo)
	if err != nil {
		return err
	}
	changed := false
	for k := range r.Finished {
		if strings.HasPrefix(k, "stash:") || strings.HasPrefix(k, "worktree:") {
			continue
		}
		if path, ok := finishedPath(k); ok && alive[path] {
			continue
		}
		delete(r.Finished, k)
		changed = true
	}
	for k := range r.Blocks {
		path, _, ok := strings.Cut(k, "\x00")
		if !ok || alive[path] {
			continue
		}
		delete(r.Blocks, k)
		changed = true
	}
	if changed {
		return r.Save()
	}
	return nil
}

func (r *ReadState) Rename(oldPath, newPath string) {
	// Whether a key added during range is visited is unspecified, and with the
	// same path on both sides the new key would match the prefix and delete
	// itself.
	if oldPath == newPath {
		return
	}
	for _, sec := range []Section{SectionStaged, SectionUnstaged} {
		from, to := fileKey(WorkingTree(sec), oldPath), fileKey(WorkingTree(sec), newPath)
		if r.Finished[from] {
			r.Finished[to] = true
			delete(r.Finished, from)
		}
	}
	for k := range r.Blocks {
		if hash, ok := strings.CutPrefix(k, oldPath+"\x00"); ok {
			r.Blocks[key(newPath, hash)] = true
			delete(r.Blocks, k)
		}
	}
}
