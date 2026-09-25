package state

import (
	"sort"

	"github.com/catpotd/mirugit/internal/git"
)

// displayOrder puts rows in the order the tree draws them, so that one index
// means one line. Moving the cursor by one moved it several lines up the screen
// while git's order and the drawn order disagreed.
func displayOrder(entries []git.Entry) []Row {
	rows := expandRows(entries)
	staged := filterSection(rows, SectionStaged)
	changes := filterSection(rows, SectionUnstaged)

	var out []Row
	if len(staged) > 0 {
		out = append(out, SectionHeadingRow(SectionStaged))
		out = append(out, directoryOrder(staged)...)
	}
	if len(changes) > 0 {
		out = append(out, SectionHeadingRow(SectionUnstaged))
		out = append(out, directoryOrder(changes)...)
	}
	return out
}

func directoryOrder(files []Row) []Row {
	byDir := map[string][]Row{}
	directories := map[string]bool{}
	for _, r := range files {
		d := ParentDir(r.Path())
		byDir[d] = append(byDir[d], r)
		directories[d] = true
	}
	orderedDirectories := make([]string, 0, len(directories))
	for dir := range directories {
		orderedDirectories = append(orderedDirectories, dir)
	}
	sort.Strings(orderedDirectories)

	out := make([]Row, 0, len(files)+len(directories))
	for _, dir := range orderedDirectories {
		out = append(out, DirectoryRow(dir, files[0].Section()))
		out = append(out, byDir[dir]...)
	}
	return out
}

// FileRowCount matches how many file rows expandRows would draw, so the changes
// tab count stays aligned with the summary line's Files total. It counts rather
// than builds: the tab bar asks on every frame, and building the rows to take
// their length copied every entry twice.
func FileRowCount(entries []git.Entry) int {
	n := 0
	for _, e := range entries {
		switch {
		case e.IsStaged() && e.IsUnstaged():
			n += 2
		default:
			// A file on neither side still draws one row, on the unstaged side.
			n++
		}
	}
	return n
}

func expandRows(entries []git.Entry) []Row {
	out := make([]Row, 0, len(entries))
	for _, e := range entries {
		if e.IsStaged() {
			out = append(out, FileRow(e, SectionStaged))
		}
		if e.IsUnstaged() {
			out = append(out, FileRow(e, SectionUnstaged))
		}
		if !e.IsStaged() && !e.IsUnstaged() {
			out = append(out, FileRow(e, SectionUnstaged))
		}
	}
	return out
}

func filterSection(rows []Row, sec Section) []Row {
	var out []Row
	for _, r := range rows {
		if r.Section() == sec {
			out = append(out, r)
		}
	}
	return out
}
