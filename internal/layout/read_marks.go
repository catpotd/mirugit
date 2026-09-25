package layout

import (
	"github.com/catpotd/mirugit/internal/state"
)

// ReadMarks is what layout needs to draw read marks. It deliberately omits Save,
// Prune, and RefreshHashes so this package never reaches a repository.
type ReadMarks interface {
	MarkIn(o state.Origin, path string, hashes []string) state.Mark
	StashMark(sha string) state.Mark
	WorktreeMark(path, sha string) state.Mark
	BlockRead(path, hash string) bool
	HashesIn(o state.Origin, path string) []string
}

// marksPresent is false when no read state was loaded. LoadRead only returns
// nil together with an error, and New propagates it, so production always
// passes a live value; tests pass the untyped nil literal.
func marksPresent(r ReadMarks) bool {
	return r != nil
}

func fileMark(r ReadMarks, o state.Origin, path string, s state.State) state.Mark {
	if !marksPresent(r) {
		return state.Read
	}
	hashes := r.HashesIn(o, path)
	if s.Open.Path == path && s.Open.Origin == o && len(s.Open.Diff.Blocks) > 0 {
		hashes = make([]string, len(s.Open.Diff.Blocks))
		for i, b := range s.Open.Diff.Blocks {
			hashes[i] = b.Hash
		}
	}
	return r.MarkIn(o, path, hashes)
}
