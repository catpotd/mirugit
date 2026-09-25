package state

// Origin names where a file's diff comes from and which one it is. Section used
// to carry this, and it had no room for the second half: every commit's version
// of a path shared one read mark, so reading a file in one commit marked it read
// in every other commit that touched it. Two stashes holding the same path
// shared a mark the same way.
//
// The kind and the name travel together in one value, so a caller cannot pass a
// commit's SHA while saying the file came from the working tree.
type Origin struct {
	kind originKind
	name string
}

type originKind int

const (
	// originWorkingTree is a file in this repository. Its name is the side of
	// the index it sits on, which is the only part of a working-tree file that
	// can hold two different diffs at once.
	originWorkingTree originKind = iota
	originCommit
	originParent
)

// WorkingTree is a file on one side of this repository's index.
func WorkingTree(side Section) Origin {
	return Origin{kind: originWorkingTree, name: sectionPrefix(side)}
}

// FromCommit is a file as one commit left it.
func FromCommit(sha string) Origin {
	return Origin{kind: originCommit, name: sha}
}

// FromParent is a file held by a stash or by another worktree, named by the
// stash's ref or the tree's path.
func FromParent(ref string) Origin {
	return Origin{kind: originParent, name: ref}
}

// Side is the index side for a working-tree file, and SectionNone for a file
// that is not in this working tree.
func (o Origin) Side() Section {
	if o.kind != originWorkingTree {
		return SectionNone
	}
	for _, sec := range []Section{SectionStaged, SectionUnstaged, SectionNone} {
		if sectionPrefix(sec) == o.name {
			return sec
		}
	}
	return SectionNone
}

// CommitSHA is the commit a file came from, and false for a file that came from
// anywhere else.
func (o Origin) CommitSHA() (string, bool) {
	return o.name, o.kind == originCommit
}

// IsWorkingTree reports a file this repository holds, which is the only kind
// whose diff the index and the working tree can both answer for.
func (o Origin) IsWorkingTree() bool { return o.kind == originWorkingTree }

// key is what a read mark is filed under. The kind is in the string so that a
// stash ref and a commit SHA cannot collide.
func (o Origin) key() string {
	switch o.kind {
	case originWorkingTree:
		return o.name
	case originCommit:
		return "commit:" + o.name + "\x00"
	case originParent:
		return "parent:" + o.name + "\x00"
	}
	return "none:"
}

// OriginOf answers where the file row at i reads its diff from. A row that is
// not a file, or one whose parent has gone, answers false.
func OriginOf(s State, i int) (Origin, bool) {
	row, ok := RowAt(s, i)
	if !ok || row.Kind() != RowFile {
		return Origin{}, false
	}
	switch Facts[s.Tab].OpenFile {
	case OpenFileFromCommit:
		parent, _, ok := ParentAbove(s.Rows, i, RowCommit)
		if !ok {
			return Origin{}, false
		}
		return FromCommit(parent.Commit().SHA), true
	case OpenFileFromParent:
		parent, _, ok := Facts[s.Tab].ParentRow(s)
		if !ok {
			return Origin{}, false
		}
		return FromParent(parent.Path()), true
	case OpenFileFromWorkingTree:
	}
	return WorkingTree(row.Section()), true
}
