package layout

import (
	"reflect"
	"testing"

	"github.com/catpotd/mirugit/internal/state"
)

func TestHitReturnsTheRegionUnderTheCoordinate(t *testing.T) {
	t.Parallel()
	f := Frame{
		Lines: []string{"a", "b"},
		Regions: []Region{
			{Target: Target{Kind: TargetFile, Path: "a.txt"}, Row: 1, ColStart: 4, ColEnd: 12},
			{Target: Target{Kind: TargetButton, Verb: state.VerbNameSync}, Row: 1, ColStart: 60, ColEnd: 66},
		},
	}
	region, _, _ := f.Hit(1, 5)
	if region.Path != "a.txt" {
		t.Errorf("inside the first region: got %q", region.Path)
	}
	region, _, _ = f.Hit(1, 61)
	if region.Verb != state.VerbNameSync {
		t.Errorf("inside the second region: got %q", region.Name)
	}
	region, _, _ = f.Hit(1, 40)
	if region.Kind != TargetNone {
		t.Errorf("between regions: got %v, want TargetNone", region.Kind)
	}
	region, _, _ = f.Hit(0, 5)
	if region.Kind != TargetNone {
		t.Errorf("wrong row: got %v, want TargetNone", region.Kind)
	}
}

func TestTargetHasNoIndexField(t *testing.T) {
	t.Parallel()
	if _, ok := reflect.TypeOf(Target{}).FieldByName("Index"); ok {
		t.Error("Target still has Index")
	}
}

func TestHitTreatsColEndAsExclusive(t *testing.T) {
	t.Parallel()
	f := Frame{Regions: []Region{
		{Target: Target{Kind: TargetFile, Path: "a"}, Row: 0, ColStart: 0, ColEnd: 3},
	}}
	region, _, _ := f.Hit(0, 2)
	if region.Kind != TargetFile {
		t.Error("the last column inside the region should hit")
	}
	region, _, _ = f.Hit(0, 3)
	if region.Kind != TargetNone {
		t.Error("ColEnd itself is outside the region")
	}
}

func TestFrameReturnsTheColumnRegionWhenOneExists(t *testing.T) {
	t.Parallel()
	f := Frame{Regions: []Region{
		{Target: Target{Kind: TargetFile, Path: "a.go", Row: 2}, Row: 7, ColStart: 9, ColEnd: 13},
		{Target: Target{Kind: TargetVerb, Verb: state.VerbNameStage, Row: 2}, Row: 7, ColStart: 54, ColEnd: 59},
	}}
	region, row, hasRow := f.Hit(7, 55)
	if region.Kind != TargetVerb || region.Verb != state.VerbNameStage {
		t.Errorf("region = %+v, want stage verb", region)
	}
	if !hasRow || row.Row != 2 || row.Path != "a.go" {
		t.Errorf("row = %+v ok=%v, want a.go row 2", row, hasRow)
	}
}

func TestFrameFallsBackToTheRowTargetWhenTheColumnMisses(t *testing.T) {
	t.Parallel()
	f := Frame{Regions: []Region{
		{Target: Target{Kind: TargetFile, Path: "a.go", Row: 2}, Row: 7, ColStart: 9, ColEnd: 13},
		{Target: Target{Kind: TargetFile, Path: "b.go", Row: 3}, Row: 8, ColStart: 9, ColEnd: 13},
	}}
	for _, col := range []int{0, 30, 76} {
		region, row, hasRow := f.Hit(7, col)
		if region.Kind != TargetNone {
			t.Errorf("col %d: region kind %v, want TargetNone", col, region.Kind)
		}
		if !hasRow || row.Row != 2 {
			t.Errorf("col %d: row %+v ok=%v, want row 2", col, row, hasRow)
		}
	}
	region, row, hasRow := f.Hit(8, 30)
	if region.Kind != TargetNone {
		t.Errorf("region kind %v, want TargetNone", region.Kind)
	}
	if !hasRow || row.Row != 3 {
		t.Errorf("row %+v ok=%v, want row 3", row, hasRow)
	}
	_, _, hasRow = f.Hit(0, 5)
	if hasRow {
		t.Error("row 0 has no row target and should report none")
	}
}
