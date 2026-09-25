package state

import (
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/catpotd/mirugit/internal/git"
)

// everyEvent carries one value per Event implementation. Field values are set
// only where the zero value would return before reaching a write.
func everyEvent() []Event {
	entries := []git.Entry{{Path: "a.txt", Worktree: git.Modified}}
	return []Event{
		CursorMoved{By: 1},
		CursorMovedTo{Row: 1},
		BlockCursorMoved{By: 1},
		TabChanged{Tab: TabHistory},
		DiffOpened{Path: "a.txt", Origin: WorkingTree(SectionUnstaged)},
		DiffClosed{For: "a.txt"},
		DirectoryFolded{Section: SectionUnstaged, Path: "dir"},
		StatusLoaded{Rows: entries, Head: git.Head{Branch: "main"}},
		HistoryLoaded{Commits: []git.CommitInfo{{SHA: "c1", Subject: "s"}}},
		HistoryMoreRequested{},
		HistoryMoreLoaded{Offset: 1, Commits: []git.CommitInfo{{SHA: "c0", Subject: "older"}}},
		HistoryMoreFailed{},
		WorktreeFilesLoaded{Path: "/w0", Files: []git.Entry{{Path: "a.txt"}}},
		StashFilesLoaded{Ref: "stash@{0}", Files: []git.Entry{{Path: "a.txt"}}},
		StashedLoaded{Stashes: []git.StashRow{{Ref: "stash@{0}", SHA: "s0"}}},
		StashBranchFinished{Branch: "b"},
		StashRestoreFinished{Files: 1},
		StashRowUpdated{Index: 0, Row: git.StashRow{Ref: "stash@{0}"}},
		StashRefsMoved{},
		WorktreesLoaded{Worktrees: []git.WorktreeRow{{Path: "/w0", Name: "w0"}}, Base: "main"},
		WorktreeRowUpdated{Index: 0, Row: git.WorktreeRow{Path: "/w0", Name: "w0", Ahead: 3}},
		WorktreeGo{Here: "/w0"},
		CommitExpanded{SHA: "c1", Files: entries},
		RowCollapsed{},
		UndoFinished{},
		DiffLoaded{Diff: git.FileDiff{Path: "a.txt"}},
		HelpScrollClamped{Scroll: 1},
		ScrollSynced{SetList: true, ListTop: 1, SetDiff: true, DiffTop: 1},
		BlockCursorSet{Block: 0},
		FetchedAgoUpdated{Label: "3m ago"},
		Resized{Width: 80, Height: 24},
		SelectionToggled{Section: SectionUnstaged, Path: "a.txt"},
		SelectionRangeExtended{From: 0, To: 1},
		SectionToggled{Section: SectionUnstaged},
		DirectorySelectionToggled{Section: SectionUnstaged, Path: "dir"},
		TabSelectionToggled{},
		SelectionCleared{},
		VerbRequested{Verb: VerbStage, Targets: []string{"a.txt"}},
		VerbFinished{Verb: VerbStage},
		DiscardConfirmationShown{Confirm: DiscardConfirm{Targets: []string{"dir/a.txt"}}},
		DiscardCancelled{},
		DiscardConfirmed{},
		StashDropConfirmationShown{Confirm: StashDropConfirm{SHA: "s0", Message: "m0"}},
		WorktreeRemoveConfirmationShown{Confirm: WorktreeRemoveConfirm{Path: "/wt", Name: "wt"}},
		WorktreeRemoveCancelled{},
		WorktreeRemoveConfirmed{},
		StashDropCancelled{},
		StashDropConfirmed{},
		UndiscardConfirmationShown{},
		UndiscardCancelled{},
		UndiscardConfirmed{},
		MessageFocused{},
		MessageBlurred{},
		MessageEdited{Text: "m"},
		CommitFinished{},
		CommitBlocked{},
		DiffBlocked{},
		DiscardFinished{Applied: 1, Undo: git.Undo{Ref: "stash@{0}"}},
		HelpOpened{},
		HelpClosed{},
		HelpScrolled{By: 1},
		StaleChanged{Path: "a.txt", Stale: true},
		Failed{Line: "boom"},
	}
}

// TestEveryEventTypeHasAPurityCase keeps everyEvent from drifting behind
// event.go, because a missing entry silently narrows the purity check below.
func TestEveryEventTypeHasAPurityCase(t *testing.T) {
	t.Parallel()
	want := eventReceiverTypes(t)
	got := make([]string, 0, len(want))
	for _, e := range everyEvent() {
		got = append(got, reflect.TypeOf(e).Name())
	}
	sort.Strings(want)
	sort.Strings(got)
	if !reflect.DeepEqual(want, got) {
		t.Errorf("everyEvent() = %v\nevent.go = %v", got, want)
	}
}

// Apply takes State by value, but a value copy still shares every slice's
// backing array and every map's buckets. A handler that writes an element
// changes the State its caller is still holding, which is what "a transition
// can be asserted directly" was supposed to rule out.
func TestApplyLeavesTheCallersStateAlone(t *testing.T) {
	t.Parallel()
	for _, e := range everyEvent() {
		name := reflect.TypeOf(e).Name()
		s := populatedState()
		snap := snapshotShared(reflect.ValueOf(s), "State")
		Apply(s, e)
		for _, sh := range snap {
			if !reflect.DeepEqual(sh.live.Interface(), sh.copy) {
				t.Errorf("%s: %s が書き換わった\n前: %v\n後: %v",
					name, sh.path, sh.copy, sh.live.Interface())
			}
		}
	}
}

func populatedState() State {
	s := State{
		Tab:      TabChanges,
		Width:    80,
		Height:   24,
		Rows:     nil,
		LastUndo: &git.Undo{Ref: "stash@{9}"},
		Changes: Changes{
			Folded:   map[string]bool{"k": true},
			Selected: map[string]bool{"k": true},
			Stale:    map[string]bool{"k": true},
		},
	}
	s = Apply(s, StatusLoaded{
		Rows: []git.Entry{
			{Path: "dir/a.txt", Worktree: git.Modified},
			{Path: "dir/b.txt", Index: git.Modified},
		},
		Head: git.Head{Branch: "main"},
	})
	s = Apply(s, HistoryLoaded{Commits: []git.CommitInfo{
		{SHA: "c1", Subject: "one"}, {SHA: "c2", Subject: "two"}}})
	s = Apply(s, StashedLoaded{Stashes: []git.StashRow{
		{Ref: "stash@{0}", SHA: "s0", Message: "m0"},
		{Ref: "stash@{1}", SHA: "s1", Message: "m1"}}})
	s = Apply(s, WorktreesLoaded{Worktrees: []git.WorktreeRow{
		{Path: "/w0", Name: "w0", Main: true}, {Path: "/w1", Name: "w1"}}, Base: "main"})
	s = Apply(s, StashFilesLoaded{Ref: "stash@{0}", Files: []git.Entry{{Path: "a.txt"}}})
	s = Apply(s, WorktreeFilesLoaded{Path: "/w0", Files: []git.Entry{{Path: "a.txt"}}})
	s = Apply(s, DiffOpened{Path: "dir/a.txt", Origin: WorkingTree(SectionUnstaged)})
	s = Apply(s, DiffLoaded{Diff: git.FileDiff{
		Path:   "dir/a.txt",
		Blocks: []git.Block{{Header: "@@", Lines: []string{"+x"}, Hash: "h1"}}}})
	return s
}

// shared names one slice, map, or pointed-to value that a State copy still
// reaches, paired with the contents it held before Apply ran.
type shared struct {
	path string
	live reflect.Value
	copy any
}

func snapshotShared(v reflect.Value, path string) []shared {
	var out []shared
	switch v.Kind() {
	case reflect.Slice, reflect.Map:
		if v.IsNil() {
			return nil
		}
		out = append(out, shared{path: path, live: v, copy: deepCopy(v)})
		if v.Kind() == reflect.Slice {
			for i := range v.Len() {
				out = append(out, snapshotShared(v.Index(i), fmt.Sprintf("%s[%d]", path, i))...)
			}
		}
	case reflect.Pointer:
		if v.IsNil() {
			return nil
		}
		out = append(out, shared{path: path, live: v.Elem(), copy: deepCopy(v.Elem())})
		out = append(out, snapshotShared(v.Elem(), path+".*")...)
	case reflect.Struct:
		for i := range v.NumField() {
			f := v.Type().Field(i)
			if !f.IsExported() {
				// reflect refuses to read it, so nothing below it can be
				// compared. The slice or map that holds this struct is still
				// snapshotted whole.
				continue
			}
			out = append(out, snapshotShared(v.Field(i), path+"."+f.Name)...)
		}
	case reflect.Array:
		for i := range v.Len() {
			out = append(out, snapshotShared(v.Index(i), fmt.Sprintf("%s[%d]", path, i))...)
		}
	}
	return out
}

// deepCopy detaches the value from every array and bucket the original shares,
// so a later write to the original is visible as a difference.
func deepCopy(v reflect.Value) any {
	out := reflect.New(v.Type()).Elem()
	copyInto(out, v)
	return out.Interface()
}

func copyInto(dst, src reflect.Value) {
	switch src.Kind() {
	case reflect.Slice:
		if src.IsNil() {
			return
		}
		dst.Set(reflect.MakeSlice(src.Type(), src.Len(), src.Len()))
		for i := range src.Len() {
			copyInto(dst.Index(i), src.Index(i))
		}
	case reflect.Map:
		if src.IsNil() {
			return
		}
		dst.Set(reflect.MakeMap(src.Type()))
		for _, k := range src.MapKeys() {
			val := reflect.New(src.Type().Elem()).Elem()
			copyInto(val, src.MapIndex(k))
			dst.SetMapIndex(k, val)
		}
	case reflect.Pointer:
		if src.IsNil() {
			return
		}
		dst.Set(reflect.New(src.Type().Elem()))
		copyInto(dst.Elem(), src.Elem())
	case reflect.Struct:
		if hasUnexportedField(src.Type()) {
			// reflect cannot read an unexported field on its own, but a whole
			// struct assignment copies every field. Row keeps its payload
			// unexported, so this is the only way to hold a before-picture of
			// one.
			dst.Set(src)
			return
		}
		for i := range src.NumField() {
			copyInto(dst.Field(i), src.Field(i))
		}
	case reflect.Array:
		for i := range src.Len() {
			copyInto(dst.Index(i), src.Index(i))
		}
	default:
		dst.Set(src)
	}
}

func hasUnexportedField(t reflect.Type) bool {
	for i := range t.NumField() {
		if !t.Field(i).IsExported() {
			return true
		}
	}
	return false
}
