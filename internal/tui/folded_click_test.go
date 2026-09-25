package tui

import (
	"testing"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
)

func TestClickingAFileAfterAFoldedDirectoryKeepsTheCursorOnThatFile(t *testing.T) {
	t.Parallel()
	m, foldedDirectory := foldedDirectoryModel(t)
	m.state.Cursor = foldedDirectory
	target := clickTarget(t, m, layout.TargetFile, "other/b.go", state.SectionUnstaged)

	next, _ := m.click(target, 0)
	m = next.(*Model)
	row, ok := state.CursorRow(m.state)
	if !ok || row.Kind() != state.RowFile || row.Path() != target.Path {
		t.Fatalf("cursor row = %+v, want clicked file %q", row, target.Path)
	}
}

func TestClickingADirectoryAfterAFoldedDirectoryKeepsTheCursorOnThatDirectory(t *testing.T) {
	t.Parallel()
	m, foldedDirectory := foldedDirectoryModel(t)
	m.state.Cursor = foldedDirectory
	target := clickTarget(t, m, layout.TargetDirectory, "other", state.SectionUnstaged)

	next, _ := m.click(target, 0)
	m = next.(*Model)
	row, ok := state.CursorRow(m.state)
	if !ok || row.Kind() != state.RowDirectory || row.DirPath() != target.Path {
		t.Fatalf("cursor row = %+v, want clicked directory %q", row, target.Path)
	}
}

func TestClickingASectionHeadingAfterAFoldedDirectoryKeepsTheCursorOnThatHeading(t *testing.T) {
	t.Parallel()
	m := fixed(t)
	m.state = state.Apply(m.state, state.StatusLoaded{Head: git.Head{Branch: "main"}, Rows: []git.Entry{
		{Path: "staged/a.go", Index: git.Modified},
		{Path: "app/a.go", Worktree: git.Modified},
		{Path: "other/b.go", Worktree: git.Modified},
		{Path: "z/after.go", Worktree: git.Modified},
	}})
	m.state = state.Apply(m.state, state.DirectoryFolded{
		Path: "app", Section: state.SectionUnstaged,
	})
	for i, row := range m.state.Rows {
		if row.Kind() == state.RowFile && row.Path() == "z/after.go" {
			m.state.Cursor = i
		}
	}
	target := clickTarget(t, m, layout.TargetSectionHeading, "", state.SectionUnstaged)

	next, _ := m.click(target, 0)
	m = next.(*Model)
	row, ok := state.CursorRow(m.state)
	if !ok || row.Kind() != state.RowSectionHeading || row.Section() != target.Section {
		t.Fatalf("cursor row = %+v, want section heading %v", row, target.Section)
	}
}

func clickTarget(t *testing.T, m *Model, kind layout.TargetKind, path string, section state.Section) layout.Target {
	t.Helper()
	m.View()
	for _, region := range m.frame.Regions {
		target := region.Target
		if target.Kind != kind || target.Path != path {
			continue
		}
		if kind == layout.TargetSectionHeading && target.Section != section {
			continue
		}
		return target
	}
	t.Fatalf("no visible %v target for %q in %v", kind, path, section)
	return layout.Target{}
}
