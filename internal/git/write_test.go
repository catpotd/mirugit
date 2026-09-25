package git

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

// runWritePaths appends --pathspec-from-file, so it belongs only to commands
// that take pathspecs. stash pop, worktree remove and their kind are rejected
// outright with exit 129, and the verb silently never runs.
func TestOnlyPathspecCommandsUseTheSafePathWrapper(t *testing.T) {
	t.Parallel()
	takesPathspecs := map[string]bool{
		"add": true, "restore": true, "rm": true, "commit": true,
		"reset": true, "stash": true,
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		body, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(body), "\n") {
			i := strings.Index(line, "runWritePaths(")
			if i < 0 || strings.Contains(line, "func runWritePaths") {
				continue
			}
			verb := firstQuoted(line[i:])
			if verb == "" {
				continue
			}
			if !takesPathspecs[verb] {
				t.Errorf("%s: runWritePaths with %q, which takes no pathspec: %s",
					name, verb, strings.TrimSpace(line))
			}
		}
	}
}

func firstQuoted(s string) string {
	start := strings.IndexByte(s, '"')
	if start < 0 {
		return ""
	}
	rest := s[start+1:]
	end := strings.IndexByte(rest, '"')
	if end < 0 {
		return ""
	}
	return rest[:end]
}

func TestStageOutcomeSplitsAppliedAndIgnored(t *testing.T) {
	t.Parallel()
	entries := []Entry{
		{Path: "a.txt"},
		{Path: "b.txt"},
		{Path: "c.txt"},
	}

	t.Run("stage", func(t *testing.T) {
		before := map[string]bool{"b.txt": true}
		after := map[string]bool{"a.txt": true, "b.txt": true}
		out := stageOutcome(entries, before, after, true)

		wantApplied := []Entry{{Path: "a.txt"}}
		if !reflect.DeepEqual(out.Applied, wantApplied) {
			t.Errorf("Applied = %v, want %v", out.Applied, wantApplied)
		}
		wantIgnored := []Entry{{Path: "b.txt"}, {Path: "c.txt"}}
		if !reflect.DeepEqual(out.Ignored, wantIgnored) {
			t.Errorf("Ignored = %v, want %v", out.Ignored, wantIgnored)
		}
	})

	t.Run("unstage", func(t *testing.T) {
		before := map[string]bool{"a.txt": true, "b.txt": true}
		after := map[string]bool{"b.txt": true}
		out := stageOutcome(entries, before, after, false)

		wantApplied := []Entry{{Path: "a.txt"}}
		if !reflect.DeepEqual(out.Applied, wantApplied) {
			t.Errorf("Applied = %v, want %v", out.Applied, wantApplied)
		}
		wantIgnored := []Entry{{Path: "b.txt"}, {Path: "c.txt"}}
		if !reflect.DeepEqual(out.Ignored, wantIgnored) {
			t.Errorf("Ignored = %v, want %v", out.Ignored, wantIgnored)
		}
	})
}

func TestStageOutcomeDedupsTheSamePath(t *testing.T) {
	t.Parallel()
	dup := []Entry{{Path: "a.txt"}, {Path: "a.txt"}}

	t.Run("stage", func(t *testing.T) {
		out := stageOutcome(dup, map[string]bool{}, map[string]bool{"a.txt": true}, true)
		if len(out.Applied) != 1 {
			t.Fatalf("Applied = %d, want 1", len(out.Applied))
		}
	})

	t.Run("unstage", func(t *testing.T) {
		out := stageOutcome(dup, map[string]bool{"a.txt": true}, map[string]bool{}, false)
		if len(out.Applied) != 1 {
			t.Fatalf("Applied = %d, want 1", len(out.Applied))
		}
	})
}
