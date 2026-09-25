package git

// Kind uses the zero value for git's dot, which means nothing happened on that
// side.
type Kind byte

const (
	Unchanged   Kind = 0
	Modified    Kind = 'M'
	Added       Kind = 'A'
	Deleted     Kind = 'D'
	Renamed     Kind = 'R'
	Copied      Kind = 'C'
	TypeChanged Kind = 'T'
	Untracked   Kind = '?'
	Unmerged    Kind = 'U'
)

// Entry holds both paths of a rename or a copy, because a verb given only the
// new one leaves the old one staged for deletion.
type Entry struct {
	Path     string
	OldPath  string
	Index    Kind
	Worktree Kind
	// IndexCount and WorktreeCount take the names of their sides, so that one
	// word means one side throughout.
	IndexCount    Count
	WorktreeCount Count
	Submodule     bool
}

func (e Entry) IsStaged() bool {
	return e.Index != Unchanged && e.Index != Untracked && e.Index != Unmerged
}

func (e Entry) IsUnstaged() bool {
	return e.Worktree != Unchanged && e.Worktree != Unmerged
}

func (e Entry) IsConflicted() bool {
	return e.Index == Unmerged || e.Worktree == Unmerged
}

// CanStash is false for a staged rename: passing the old path fails because it
// is not in the working tree, and passing only the new one leaves the old one
// staged for deletion.
func (e Entry) CanStash() bool {
	if e.OldPath == "" {
		return true
	}
	return e.Index != Renamed && e.Index != Copied
}

func (e Entry) Pathspecs() []string {
	if e.OldPath == "" {
		return []string{pathspec(e.Path)}
	}
	return []string{pathspec(e.Path), pathspec(e.OldPath)}
}

// pathspec marks a path literal. One beginning with a colon reads as a magic
// prefix otherwise, and no amount of quoting or NUL separation prevents it.
func pathspec(path string) string {
	return ":(literal)" + path
}
