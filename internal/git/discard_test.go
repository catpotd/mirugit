package git

import (
	"reflect"
	"testing"
)

func TestClassifyDiscardTargetsSplitsByEntryState(t *testing.T) {
	t.Parallel()
	beforeByPath := map[string]Entry{
		"new.txt": {Path: "new.txt", Worktree: Untracked},
		"tracked.txt": {
			Path:     "tracked.txt",
			Index:    Modified,
			Worktree: Modified,
		},
		"protected.txt": {
			Path:     "protected.txt",
			Index:    Modified,
			Worktree: Modified,
		},
	}
	protectedSet := []string{"other"}

	stagedOnly, fullRestore, cleanPaths := classifyDiscardTargets(
		[]Entry{
			{Path: "new.txt"},
			{Path: "tracked.txt"},
			{Path: "missing.txt"},
			{Path: "tracked.txt"},
		},
		beforeByPath,
		protectedSet,
	)

	wantClean := []string{":(literal)new.txt"}
	if !reflect.DeepEqual(cleanPaths, wantClean) {
		t.Errorf("cleanPaths = %v, want %v", cleanPaths, wantClean)
	}

	wantFull := []string{":(literal)tracked.txt"}
	if !reflect.DeepEqual(fullRestore, wantFull) {
		t.Errorf("fullRestore = %v, want %v", fullRestore, wantFull)
	}

	if len(stagedOnly) != 0 {
		t.Errorf("stagedOnly = %v, want empty without protected collision", stagedOnly)
	}

	colliding := []string{"protected.txt"}
	stagedOnly, fullRestore, cleanPaths = classifyDiscardTargets(
		[]Entry{{Path: "protected.txt"}},
		beforeByPath,
		colliding,
	)
	if len(cleanPaths) != 0 || len(fullRestore) != 0 {
		t.Fatalf("protected entry should not be clean or full restore: clean=%v full=%v", cleanPaths, fullRestore)
	}
	wantStaged := []string{":(literal)protected.txt"}
	if !reflect.DeepEqual(stagedOnly, wantStaged) {
		t.Errorf("stagedOnly = %v, want %v", stagedOnly, wantStaged)
	}
}
