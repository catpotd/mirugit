package layout

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/catpotd/mirugit/internal/state"
)

// replaceOnce puts a painted word back where the plain one sat. It has two
// reasons to do nothing, and either alone is enough: an empty word matches at
// the front of every line, and a word that is its own painted form would be
// written back unchanged after a search that costs a scan of the line.
//
// Joining them with and makes an empty word reach strings.Index, which finds it
// at 0 and splices the painted form in before the first character.
func TestReplaceOnceLeavesTheLineAloneWhenThereIsNothingToPaint(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, line, plain, styled, want string
	}{
		{"a word that is painted", "a b c", "b", "<b>", "a <b> c"},
		{"an empty word", "a b c", "", "<>", "a b c"},
		{"a word that is its own painted form", "a b c", "b", "b", "a b c"},
		{"an empty word that is its own painted form", "a b c", "", "", "a b c"},
		{"a word the line does not hold", "a b c", "z", "<z>", "a b c"},
		{"a word that appears twice", "b a b", "b", "<b>", "<b> a b"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := replaceOnce(c.line, c.plain, c.styled); got != c.want {
				t.Errorf("replaceOnce(%q, %q, %q) = %q, want %q",
					c.line, c.plain, c.styled, got, c.want)
			}
		})
	}
}

// A subject in a column of its own keeps that column whatever sits beside it,
// so the columns line up down the list. The gap that holds a subject clear of
// the column beside it is taken out of the room the subject has, and a subject
// with a column of its own has no such room to give: taking it anyway makes the
// column one cell narrower than the rows around it, and the region the subject
// carries ends one cell short of where it is drawn.
func TestASubjectWithAColumnOfItsOwnKeepsTheWholeColumn(t *testing.T) {
	t.Parallel()
	var w Renderer
	const subjectWidth, width = 20, 32

	in := subjectRowInput{
		Subject:      "",
		SubjectWidth: subjectWidth,
		Cursor:       true,
		Verbs:        []state.VerbName{state.VerbNameSelect},
	}
	_, regions := w.subjectRow(in, width)

	// The row's own region carries the cursor rather than a click target, and
	// it is the one that spans the subject's column.
	var subject *Region
	for i, r := range regions {
		if r.Target.Kind == TargetNone {
			subject = &regions[i]
		}
	}
	if subject == nil {
		t.Fatalf("the row carries no region for its subject: %+v", regions)
	}
	if got := subject.ColEnd - subject.ColStart; got != subjectWidth {
		t.Errorf("the subject's column is %d cells, want the %d it was given",
			got, subjectWidth)
	}
}

// The bar's two buttons are painted only when the bar has them. An empty button
// reaches replaceOnce as an empty word, and the guard there is what keeps it
// from splicing a color code in before the first character — this is the guard
// above it, which keeps the search from being made at all.
func TestTheBarPaintsOnlyTheButtonsItHas(t *testing.T) {
	t.Parallel()
	w := Renderer{Palette: Palette{Enabled: true}}

	for _, c := range []struct {
		name, sync, commit, right string
	}{
		{"both buttons", "S sync", "c commit", "S sync   c commit"},
		{"only sync", "S sync", "", "S sync"},
		{"only commit", "", "c commit", "c commit"},
		{"neither", "", "", "nothing to press"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := w.colorSummaryRight(nil, c.sync, c.commit, c.right)
			if plain := ansi.Strip(got); plain != c.right {
				t.Errorf("painting changed the row from %q to %q", c.right, plain)
			}
			for _, btn := range []string{c.sync, c.commit} {
				if btn == "" {
					continue
				}
				if !strings.Contains(got, w.Verb(btn)) {
					t.Errorf("%q is on the row and was not painted: %q", btn, got)
				}
			}
		})
	}
}

// The tab rule is graded from the middle out when the theme names an edge
// color, and drawn flat when it does not. Either reason alone means there is no
// grade to compute: a theme with no edge color has nothing to grade towards,
// and a rule one cell wide has no distance to grade across — the divisor is the
// width less one, which is zero there.
func TestTheTabRuleIsDrawnFlatWhenThereIsNoGradeToCompute(t *testing.T) {
	t.Parallel()
	flat := Renderer{Palette: Palette{Enabled: true}}
	graded := Renderer{Palette: Palette{Enabled: true, Theme: ThemeByName("cozmic")}}
	if graded.theme().RuleEdge == "" {
		t.Fatal("the graded theme names no edge color, so this proves nothing")
	}
	if flat.theme().RuleEdge != "" {
		t.Fatal("the flat theme names an edge color, so this proves nothing")
	}

	for _, c := range []struct {
		name   string
		w      Renderer
		rule   string
		graded bool
	}{
		{"a theme with no edge color", flat, strings.Repeat("─", 20), false},
		{"a rule one cell wide", graded, "─", false},
		{"a rule of no cells", graded, "", false},
		{"a theme with an edge color and room to grade", graded, strings.Repeat("─", 20), true},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := c.w.colorTabRule(c.rule)
			if plain := ansi.Strip(got); plain != c.rule {
				t.Errorf("coloring changed the rule from %q to %q", c.rule, plain)
			}
			// A flat rule is one color from end to end; a graded one is not.
			// Counting the distinct colors is what tells them apart.
			if got := distinctColors(got); (got > 1) != c.graded {
				t.Errorf("the rule is drawn in %d colors, want it graded = %v: %q",
					got, c.graded, c.rule)
			}
		})
	}
}

// distinctColors counts the foreground colors a line is drawn in.
// Two cells are the two ends of the rule, and the ends are where the edge color
// belongs: the distance from the middle is the whole of it at both. Drawn in
// the middle color instead, the narrowest pane is the one place the rule does
// not match the theme it was asked for.
func TestARuleTwoCellsWideIsDrawnInTheEdgeColor(t *testing.T) {
	t.Parallel()
	graded := Renderer{Palette: Palette{Enabled: true, Theme: ThemeByName("cozmic")}}
	if graded.theme().RuleEdge == "" {
		t.Fatal("the graded theme names no edge color, so this proves nothing")
	}

	wide := firstColorOf(graded.colorTabRule(strings.Repeat("─", 20)))
	narrow := firstColorOf(graded.colorTabRule("──"))
	middle := firstColorOf(graded.fg(graded.theme().Rule, "─"))
	if wide == middle {
		t.Fatal("the end of a wide rule is the middle color, so this proves nothing")
	}
	if narrow != wide {
		t.Errorf("a rule two cells wide starts in %q; the end of a wide one is %q",
			narrow, wide)
	}
}

func firstColorOf(s string) string {
	for _, part := range strings.Split(s, "\x1b[") {
		if i := strings.IndexByte(part, 'm'); i >= 0 && strings.HasPrefix(part, "38;2;") {
			return part[:i]
		}
	}
	return ""
}

func distinctColors(s string) int {
	seen := map[string]bool{}
	for _, part := range strings.Split(s, "\x1b[") {
		if i := strings.IndexByte(part, 'm'); i >= 0 && strings.HasPrefix(part, "38;2;") {
			seen[part[:i]] = true
		}
	}
	return len(seen)
}
