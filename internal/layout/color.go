package layout

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/catpotd/mirugit/internal/state"
)

// Palette is passed in rather than read from the environment, because the
// layout layer has no terminal in reach and a test must be able to ask for
// both answers.
type Palette struct {
	Enabled bool
	Theme   Theme
}

// Theme names the eight colors the pane draws with. Every distinction on screen
// is carried by a character as well, so a theme changes how the pane feels and
// never what it says.
type Theme struct {
	Add   string
	Del   string
	Mark  string
	Verb  string
	Faint string
	Dim   string
	// Dir colors directory names. File names keep the terminal default, so the
	// pane tells them apart by hue rather than by brightness alone.
	Dir      string
	Rule     string
	CursorFg string
	CursorBg string
	// BlockGap fills the row between two blocks. Empty leaves it blank.
	BlockGap string
	// RuleEdge, when set, is the color the rules fade to at both ends. A flat
	// line reads as the edge of a box; one that falls off reads as a surface
	// with the content sitting on it. Empty keeps the rule one color.
	RuleEdge string
}

// slate is the palette the design was drawn in.
var slate = Theme{
	Add: "#57a86e", Del: "#d8695c", Mark: "#79a7f2", Verb: "#79a7f2",
	Faint: "#4d5665", Dim: "#79818f", Dir: "#79a7f2", Rule: "#2a313d",
	CursorFg: "#e8edf5", CursorBg: "#1d2635",
}

// cozmic runs the accents at neon chroma and tints the quiet tiers violet, so
// the whole surface reads as lit rather than as grey with colored parts. Added
// and deleted stay green and red-ish because a diff is read that way
// everywhere else, and this pane is not the place to relearn it.
//
// Deleted sits lighter than a saturated red would: at full chroma it measured
// 5.35 against the ground where every other tier cleared 6.6, and removed lines
// are read in long stretches.
var cozmic = Theme{
	Add: "#00f5a0", Del: "#ff7a8a", Mark: "#c77dff", Verb: "#00e5ff",
	Faint: "#4a3f7a", Dim: "#9d8fd6", Dir: "#c77dff", Rule: "#2b1f52",
	CursorFg: "#ffffff", CursorBg: "#4c1d95",
	RuleEdge: "#161029",
	BlockGap: "┈",
}

func ThemeByName(name string) Theme {
	if name == "cozmic" {
		return cozmic
	}
	return slate
}

func (p Palette) theme() Theme {
	if p.Theme.Add == "" {
		return slate
	}
	return p.Theme
}

func (p Palette) wrap(style lipgloss.Style, s string) string {
	if !p.Enabled || s == "" {
		return s
	}
	return style.Render(s)
}

func (p Palette) fg(hex, s string) string {
	return p.wrap(lipgloss.NewStyle().Foreground(lipgloss.Color(hex)), s)
}

func (p Palette) Add(s string) string   { return p.fg(p.theme().Add, s) }
func (p Palette) Del(s string) string   { return p.fg(p.theme().Del, s) }
func (p Palette) Verb(s string) string  { return p.fg(p.theme().Verb, s) }
func (p Palette) Faint(s string) string { return p.fg(p.theme().Faint, s) }
func (p Palette) Dim(s string) string   { return p.fg(p.theme().Dim, s) }
func (p Palette) Dir(s string) string   { return p.fg(p.theme().Dir, s) }
func (p Palette) Rule(s string) string  { return p.fg(p.theme().Rule, s) }
func (p Palette) Tab(s string) string   { return p.fg(p.theme().Verb, s) }

func (p Palette) Mark(s string) string {
	return p.wrap(lipgloss.NewStyle().
		Foreground(lipgloss.Color(p.theme().Mark)).Bold(true), s)
}

func (p Palette) Cursor(s string) string {
	th := p.theme()
	return p.wrap(lipgloss.NewStyle().
		Foreground(lipgloss.Color(th.CursorFg)).
		Background(lipgloss.Color(th.CursorBg)), s)
}

func replaceOnce(line, plain, styled string) string {
	if plain == "" || plain == styled {
		return line
	}
	i := strings.Index(line, plain)
	if i < 0 {
		return line
	}
	return line[:i] + styled + line[i+len(plain):]
}

func colorCheckbox(p Palette, box string) string {
	switch box {
	case "[✓]", "[-]":
		return p.Mark(box)
	default:
		return box
	}
}

func (w Renderer) colorGutter(r Row) string {
	p := w.Palette
	plain := w.gutter(r)
	if !p.Enabled {
		return plain
	}
	line := plain
	if r.State.Cursor {
		line = replaceOnce(line, "▌", p.Cursor("▌"))
	}
	if r.State.Selected {
		line = replaceOnce(line, "[✓]", p.Mark("[✓]"))
	}
	switch r.State.Read {
	case state.Unread:
		line = replaceOnce(line, "·", p.Mark("·"))
	case state.Changed:
		line = replaceOnce(line, "●", p.Mark("●"))
	case state.Read:
		// A read row draws no mark, so there is nothing to paint.
	}
	return line
}

func (w Renderer) colorStatusWords(line string) string {
	if !w.Enabled {
		return line
	}
	p := w.Palette
	line = replaceOnce(line, "conflicts", p.Del("conflicts"))
	line = replaceOnce(line, "merges", p.Add("merges"))
	line = replaceOnce(line, "applies", p.Add("applies"))
	line = replaceOnce(line, "unrelated", p.Dim("unrelated"))
	return line
}

func (w Renderer) colorStashFileConflicts() string {
	return w.colorStatusWords("conflicts")
}

func (w Renderer) colorFigures(r Row) string {
	p := w.Palette
	plain := w.figures(r)
	if !p.Enabled || r.Count.Binary {
		return plain
	}
	line := replaceOnce(plain, state.Plus(r.Count.Added), p.Add(state.Plus(r.Count.Added)))
	line = replaceOnce(line, state.Minus(r.Count.Deleted), p.Del(state.Minus(r.Count.Deleted)))
	return line
}

func (w Renderer) colorCursorLeft(left string) string {
	if !w.Enabled {
		return left
	}
	return replaceOnce(left, "▌", w.Cursor("▌"))
}

func (w Renderer) colorVerbField(verbs []state.VerbName, fieldWidth int) string {
	plain := w.verbField(verbs, fieldWidth)
	if !w.Enabled {
		return plain
	}
	return w.colorVerbs(plain, verbs)
}

func (w Renderer) colorCounts(s string) string {
	p := w.Palette
	if !p.Enabled {
		return s
	}
	out := s
	for _, token := range strings.Fields(s) {
		if strings.HasPrefix(token, "+") {
			out = replaceOnce(out, token, p.Add(token))
		}
		if strings.HasPrefix(token, "−") {
			out = replaceOnce(out, token, p.Del(token))
		}
	}
	return out
}

func (w Renderer) colorRuleLine(s string) string {
	if !w.Enabled {
		return s
	}
	return w.gradedRule(s)
}

// gradedRule draws the line brightest in the middle and falling to RuleEdge at
// both ends, in eight bands. Fewer bands step visibly; more cost escape
// sequences on a line redrawn on every frame.
func (p Palette) gradedRule(s string) string {
	th := p.theme()
	if th.RuleEdge == "" {
		return p.Rule(s)
	}
	runes := []rune(s)
	const bands = 8
	var b strings.Builder
	for i := range bands {
		lo := i * len(runes) / bands
		hi := (i + 1) * len(runes) / bands
		if lo >= hi {
			continue
		}
		// Distance from the middle, 0 at the center and 1 at either end.
		away := float64(bands-1-2*i) / float64(bands-1)
		if away < 0 {
			away = -away
		}
		b.WriteString(p.fg(mix(th.Rule, th.RuleEdge, away), string(runes[lo:hi])))
	}
	return b.String()
}

func mix(from, to string, amount float64) string {
	f, t2 := hexRGB(from), hexRGB(to)
	out := [3]int{}
	for i := range out {
		out[i] = f[i] + int(float64(t2[i]-f[i])*amount)
	}
	return fmt.Sprintf("#%02x%02x%02x", out[0], out[1], out[2])
}

func hexRGB(h string) [3]int {
	var r, g, b int
	if _, err := fmt.Sscanf(strings.TrimPrefix(h, "#"), "%02x%02x%02x", &r, &g, &b); err != nil {
		return [3]int{}
	}
	return [3]int{r, g, b}
}

// colorTabRule keeps the active tab's underline at full strength and lets the
// rest fall off toward the ends, so the bar reads as the same surface the other
// rules sit on. Runs of one color share an escape, because this line is
// redrawn on every frame.
func (w Renderer) colorTabRule(rule string) string {
	if !w.Enabled {
		return rule
	}
	runes := []rune(rule)
	th := w.theme()
	colorAt := func(i int, r rune) string {
		if r == '\u2501' {
			return th.Verb
		}
		if th.RuleEdge == "" || len(runes) < 2 {
			return th.Rule
		}
		away := float64(2*i-(len(runes)-1)) / float64(len(runes)-1)
		if away < 0 {
			away = -away
		}
		return mix(th.Rule, th.RuleEdge, away)
	}
	var b strings.Builder
	start := 0
	for i := 1; i <= len(runes); i++ {
		if i < len(runes) && colorAt(i, runes[i]) == colorAt(start, runes[start]) {
			continue
		}
		b.WriteString(w.fg(colorAt(start, runes[start]), string(runes[start:i])))
		start = i
	}
	return b.String()
}

func (w Renderer) colorDots(s string) string {
	p := w.Palette
	if !p.Enabled {
		return s
	}
	return strings.ReplaceAll(s, "·", p.Mark("·"))
}

// colorDirHeading dims the fold arrow and the trailing file count and paints
// the path with Dir so the name reads apart from file rows by hue, not merely
// by brightness.
func (w Renderer) colorDirHeading(line, arrow string) string {
	if !w.Enabled {
		return line
	}
	arrowIdx := strings.Index(line, arrow)
	if arrowIdx < 0 {
		return line
	}
	pathStart := arrowIdx + len(arrow) + 1
	count := trailingCount(line)
	pathEnd := len(line)
	if count != "" {
		pathEnd = len(line) - len(count)
	}
	path := strings.TrimRight(line[pathStart:pathEnd], " ")
	lineOut := line
	if count != "" {
		lineOut = w.paintTail(lineOut, w.Of(line), w.Dim(count), w.Of(count))
	}
	if path != "" {
		lineOut = lineOut[:pathStart] + w.Dir(path) + lineOut[pathStart+len(path):]
	}
	lineOut = lineOut[:arrowIdx] + w.Dim(arrow) + lineOut[arrowIdx+len(arrow):]
	return lineOut
}

func trailingCount(line string) string {
	s := strings.TrimRight(line, " ")
	i := len(s)
	for i > 0 && s[i-1] >= '0' && s[i-1] <= '9' {
		i--
	}
	return s[i:]
}

// colorVerbs paints each word where the footer actually offers it. It takes
// plain words rather than VerbName because it also paints "cancels", which no
// key runs. The word is searched as "<key> <verb>" first, because a bare search
// finds the first match anywhere: "select" hits the word inside "1 selected"
// and leaves the pressable "space select" plain, with the reset landing between
// "select" and "ed".
func (w Renderer) colorVerbs(s string, verbs []state.VerbName, extras ...string) string {
	p := w.Palette
	if !p.Enabled {
		return s
	}
	line := s
	for _, v := range verbs {
		word := v.String()
		if key, ok := footerKeys[v]; ok {
			token := key + " " + word
			if strings.Contains(line, token) {
				line = replaceOnce(line, token, key+" "+p.Verb(word))
				continue
			}
		}
		line = replaceOnce(line, word, p.Verb(word))
	}
	// extras are words the footer prints beside a verb that no key runs, such
	// as "cancels" in a confirmation line.
	for _, word := range extras {
		line = replaceOnce(line, word, p.Verb(word))
	}
	return line
}

func (w Renderer) colorDiffBlockHead(line string, marked bool, right string) string {
	p := w.Palette
	if !p.Enabled {
		return line
	}
	if right != "" {
		line = w.paintTail(line, w.Of(line), w.colorCounts(right), w.Of(right))
	}
	if marked {
		line = replaceOnce(line, "▌", p.Add("▌"))
	}
	return w.colorVerbs(line, []state.VerbName{state.VerbNameStage})
}

func (w Renderer) colorFooterLine(line string, verbs []state.VerbName, extras ...string) string {
	p := w.Palette
	if !p.Enabled {
		return line
	}
	line = w.colorVerbs(line, verbs, extras...)
	return w.colorDots(line)
}

func (w Renderer) colorSummaryRight(barVerbs []state.VerbName, syncBtn, commitBtn, rightPlain string) string {
	p := w.Palette
	coloredRight := rightPlain
	if syncBtn != "" {
		coloredRight = replaceOnce(coloredRight, syncBtn, p.Verb(syncBtn))
	}
	if commitBtn != "" {
		coloredRight = replaceOnce(coloredRight, commitBtn, p.Verb(commitBtn))
	}
	return w.colorVerbs(coloredRight, barVerbs)
}

func (w Renderer) colorDiffLine(padded, numPlain, raw string) string {
	p := w.Palette
	if !p.Enabled {
		return padded
	}
	if raw == "" {
		return padded
	}
	if len(numPlain) > len(padded) {
		return padded
	}
	prefix := padded[:len(numPlain)]
	suffix := padded[len(numPlain):]
	var styled string
	switch raw[0] {
	case '+':
		styled = replaceOnce(suffix, raw, p.Add(raw))
	case '-':
		styled = replaceOnce(suffix, raw, p.Del(raw))
	default:
		styled = replaceOnce(suffix, raw, p.Dim(raw))
	}
	return prefix + styled
}

// blockGap is the row between two blocks. A theme can mark it, which turns the
// gap into a graduation the eye can count; otherwise it stays the blank row the
// design was drawn with.
func (w Renderer) blockGap(width int) string {
	mark := w.theme().BlockGap
	if mark == "" || !w.Enabled {
		return strings.Repeat(" ", width)
	}
	body := strings.Repeat(mark, width/w.Of(mark))
	for w.Of(body) < width {
		body += " "
	}
	return w.fg(w.theme().Faint, body)
}
