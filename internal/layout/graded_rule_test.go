package layout

import (
	"strings"
	"testing"
)

// The rule under the tab bar is drawn on every frame, so a theme that names no
// edge color must not pay for eight bands of escapes.
func TestGradedRuleFallsBackWhenTheThemeHasNoEdge(t *testing.T) {
	t.Parallel()
	p := Palette{Enabled: true, Theme: Theme{Rule: "8"}}
	line := strings.Repeat("─", 16)
	if got, want := p.gradedRule(line), p.Rule(line); got != want {
		t.Errorf("without RuleEdge the rule should be one color:\n got %q\nwant %q", got, want)
	}
}

// Eight bands means eight color changes over the line, and every cell of the
// input has to come out the other side.
func TestGradedRuleKeepsEveryCellAndGrades(t *testing.T) {
	t.Parallel()
	p := Palette{Enabled: true, Theme: slate}
	if p.theme().RuleEdge == "" {
		t.Skip("this theme names no rule edge")
	}
	line := strings.Repeat("─", 16)
	got := p.gradedRule(line)
	if n := strings.Count(got, "─"); n != 16 {
		t.Errorf("the rule lost cells: %d of 16", n)
	}
	if n := strings.Count(got, "\x1b["); n < 8 {
		t.Errorf("%d color changes, want at least one per band", n)
	}
}
