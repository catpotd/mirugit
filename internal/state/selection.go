package state

import (
	"fmt"
	"strconv"
)

// copyBools returns a writable copy. maps.Clone is not used because it returns
// nil for a nil map, and Apply's callers write into these without checking.
func copyBools(m map[string]bool) map[string]bool {
	out := make(map[string]bool, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func ensureSelected(s *State) {
	if s.Changes.Selected == nil {
		s.Changes.Selected = map[string]bool{}
	}
}

// SelectedIn reports whether one row is ticked. A path staged and then edited
// again shows in both sections, and the two rows carry different work, so the
// tick belongs to the row rather than to the path.
func SelectedIn(s State, sec Section, path string) bool {
	return s.Changes.Selected[selectionKey(sec, path)]
}

// SelectionCount is how many rows are ticked, which is what the summary says.
func SelectionCount(s State) int { return len(s.Changes.Selected) }

func selectionKey(sec Section, path string) string {
	return strconv.Itoa(int(sec)) + "\x00" + path
}

// selectedPaths returns the ticked paths of one section, without duplicates.
func selectedPaths(s State, sec Section) []string {
	var out []string
	for _, row := range s.Rows {
		if row.Kind() == RowFile && row.Section() == sec &&
			SelectedIn(s, sec, row.Path()) {
			out = append(out, row.Path())
		}
	}
	return out
}

func togglePath(s *State, sec Section, path string) {
	ensureSelected(s)
	key := selectionKey(sec, path)
	if s.Changes.Selected[key] {
		delete(s.Changes.Selected, key)
	} else {
		s.Changes.Selected[key] = true
	}
}

func addPathRange(s *State, from, to int) {
	if len(s.Rows) == 0 {
		return
	}
	lo, hi := from, to
	if lo > hi {
		lo, hi = hi, lo
	}
	lo = clampIndex(lo, len(s.Rows)-1)
	hi = clampIndex(hi, len(s.Rows)-1)
	ensureSelected(s)
	for i := lo; i <= hi; i++ {
		if s.Rows[i].Kind() != RowFile || rowHiddenByFold(*s, s.Rows[i]) {
			continue
		}
		s.Changes.Selected[selectionKey(s.Rows[i].Section(), s.Rows[i].Path())] = true
	}
}

func toggleSection(s *State, sec Section) {
	setAll(s, sec, sectionPaths(*s, sec))
}

// setAll ticks every row of a group, or clears them when they are all ticked.
func setAll(s *State, sec Section, paths []string) {
	if len(paths) == 0 {
		return
	}
	all := true
	for _, p := range paths {
		if !SelectedIn(*s, sec, p) {
			all = false
			break
		}
	}
	ensureSelected(s)
	for _, p := range paths {
		if all {
			delete(s.Changes.Selected, selectionKey(sec, p))
		} else {
			s.Changes.Selected[selectionKey(sec, p)] = true
		}
	}
}

func sectionPaths(s State, sec Section) []string {
	var paths []string
	for _, row := range s.Rows {
		if row.Kind() != RowFile || row.Section() != sec {
			continue
		}
		paths = append(paths, row.Path())
	}
	return paths
}

func toggleDirectory(s *State, dir string, sec Section) {
	setAll(s, sec, directoryPaths(*s, dir, sec))
}

func directoryPaths(s State, dir string, sec Section) []string {
	var paths []string
	for _, row := range s.Rows {
		if row.Kind() != RowFile || row.Section() != sec {
			continue
		}
		if IsBelowDirectory(row.Path(), dir) {
			paths = append(paths, row.Path())
		}
	}
	return paths
}

func clearSelected(s *State) {
	s.Changes.Selected = map[string]bool{}
}

// A tab switch drops the selection without confirmation, so the notice is the only sign it happened.
func clearSelectedWithNotice(s *State) {
	if n := len(s.Changes.Selected); n > 0 {
		setNotice(s, fmt.Sprintf("%d deselected · tab changed", n))
	}
	clearSelected(s)
}

// toggleTab ticks every file row in the tab, or clears them all. Each row is
// keyed on its own, so a path in both sections counts twice here as it does on
// screen.
func toggleTab(s *State) {
	rows := tabRows(*s)
	if len(rows) == 0 {
		return
	}
	all := true
	for _, r := range rows {
		if !SelectedIn(*s, r.Section(), r.Path()) {
			all = false
			break
		}
	}
	ensureSelected(s)
	for _, r := range rows {
		if all {
			delete(s.Changes.Selected, selectionKey(r.Section(), r.Path()))
		} else {
			s.Changes.Selected[selectionKey(r.Section(), r.Path())] = true
		}
	}
}

func tabRows(s State) []Row {
	var out []Row
	for _, row := range s.Rows {
		if row.Kind() == RowFile {
			out = append(out, row)
		}
	}
	return out
}

// consumeSelection drops ticked paths a verb just finished on. Leaving them
// selected makes the next reload report still here for paths the reader
// removed themselves.
func consumeSelection(s *State, req *VerbRequested, ignored []string) {
	if req == nil {
		return
	}
	if req.Verb == VerbStage && req.Block >= 0 {
		return
	}
	skip := make(map[string]bool, len(ignored))
	for _, p := range ignored {
		skip[p] = true
	}
	for _, p := range req.Targets {
		if skip[p] {
			continue
		}
		switch req.Verb {
		case VerbStage:
			delete(s.Changes.Selected, selectionKey(SectionUnstaged, p))
		case VerbUnstage:
			delete(s.Changes.Selected, selectionKey(SectionStaged, p))
		case VerbStash, VerbDiscard:
			delete(s.Changes.Selected, selectionKey(SectionUnstaged, p))
			delete(s.Changes.Selected, selectionKey(SectionStaged, p))
		case VerbUndo:
			// Undo puts a discarded file back rather than spending a selection,
			// so it leaves the ticks where they are.
		}
	}
}

func applySelection(s State, e Event) (State, bool) {
	switch e := e.(type) {
	case SelectionToggled:
		return applySelectionToggled(s, e), true
	case SelectionRangeExtended:
		return applySelectionRangeExtended(s, e), true
	case SectionToggled:
		return applySectionToggled(s, e), true
	case DirectorySelectionToggled:
		return applyDirectorySelectionToggled(s, e), true
	case TabSelectionToggled:
		return applyTabSelectionToggled(s), true
	case SelectionCleared:
		return applySelectionCleared(s), true
	}
	return s, false
}

func applySelectionToggled(s State, e SelectionToggled) State {
	togglePath(&s, e.Section, e.Path)
	return s
}

func applySelectionRangeExtended(s State, e SelectionRangeExtended) State {
	addPathRange(&s, e.From, e.To)
	return s
}

func applySectionToggled(s State, e SectionToggled) State {
	toggleSection(&s, e.Section)
	return s
}

func applyDirectorySelectionToggled(s State, e DirectorySelectionToggled) State {
	toggleDirectory(&s, e.Path, e.Section)
	return s
}

func applyTabSelectionToggled(s State) State {
	toggleTab(&s)
	return s
}

func applySelectionCleared(s State) State {
	clearSelected(&s)
	return s
}
