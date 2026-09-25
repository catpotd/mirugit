package layout

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

const helpKeyWidth = 18

type helpRow struct {
	keys string
	desc string
	verb state.VerbName
}

var helpRows = []helpRow{
	{desc: "commands"},
	{},
	{keys: "j  k  ↑  ↓", desc: "move the cursor", verb: state.VerbNameDown},
	{keys: "pgup  pgdn", desc: "move the cursor a page", verb: state.VerbNamePage},
	{keys: "home  end", desc: "first · last row", verb: state.VerbNameEnds},
	{keys: "space", desc: "select · deselect", verb: state.VerbNameSelect},
	{keys: "a", desc: "select everything in the tab", verb: state.VerbNameSelectAll},
	{keys: "shift + j  k", desc: "extend the selection", verb: state.VerbNameExtend},
	{keys: "←  →", desc: "fold · unfold the directory", verb: state.VerbNameFold},
	{keys: "esc", desc: "clear the selection, or close the diff", verb: state.VerbNameClear},
	{},
	{keys: "s", desc: state.VerbNameStage.String(), verb: state.VerbNameStage},
	{keys: "u", desc: state.VerbNameUnstage.String(), verb: state.VerbNameUnstage},
	{keys: "x", desc: "discard the change", verb: state.VerbNameDiscard},
	{keys: "z", desc: state.VerbNameStash.String(), verb: state.VerbNameStash},
	{keys: "U", desc: "undo the last discard", verb: state.VerbNameUndo},
	{keys: "u", desc: "move the last commit back to the staged area", verb: state.VerbNameUncommit},
	{keys: "p", desc: "restore the stash to the working tree", verb: state.VerbNameRestore},
	{keys: "b", desc: "turn the stash into a branch", verb: state.VerbNameBranch},
	{keys: "g", desc: "go to the worktree", verb: state.VerbNameGo},
	{},
	{keys: "tab  1-4", desc: "switch tabs", verb: state.VerbNameNextTab},
	{keys: "d  enter", desc: "open the diff", verb: state.VerbNameDiff},
	{keys: "n  N", desc: "next · previous diff block", verb: state.VerbNameNextBlock},
	{keys: "r", desc: "mark read without opening", verb: state.VerbNameRead},
	{keys: "y", desc: "copy the commit sha", verb: state.VerbNameSHA},
	{keys: "o", desc: "open the commit in a browser", verb: state.VerbNameOpen},
	{keys: "R", desc: state.VerbNameReload.String(), verb: state.VerbNameReload},
	{keys: "f", desc: state.VerbNameFetch.String(), verb: state.VerbNameFetch},
	{keys: "S", desc: "sync · pull then push", verb: state.VerbNameSync},
	{keys: "c", desc: "write a commit message", verb: state.VerbNameCommit},
	{},
	{keys: "q  ctrl+c", desc: state.VerbNameQuit.String(), verb: state.VerbNameQuit},
}

var helpAlwaysVerbs = map[state.VerbName]bool{
	state.VerbNameDown:    true,
	state.VerbNamePage:    true,
	state.VerbNameEnds:    true,
	state.VerbNameNextTab: true,
	state.VerbNameQuit:    true,
	state.VerbNameReload:  true,
	state.VerbNameFetch:   true,
	state.VerbNameSync:    true,
	state.VerbNameUndo:    true,
}

// helpRowFor names the row that explains a verb the help has no row of its own
// for. The fold row lists both arrow keys and both words, so a reader looks up
// either key there. The rows a tab draws are decided by the verb each row
// carries, which is fold: a tab that prints the unfold key prints the fold key
// too, and TestEveryFooterKeyHasAHelpRowForThatKey is what says so if one ever
// prints only the second.
var helpRowFor = map[state.VerbName]state.VerbName{
	state.VerbNameUnfold: state.VerbNameFold,
}

func helpTabHasCommitBox(tab state.Tab) bool {
	return state.Facts[tab].HasCommitBox
}

func helpTabHasDiffPanel(tab state.Tab, w Renderer) bool {
	return footerVerbUnionForTab(tab, w)[state.VerbNameDiff]
}

func footerVerbUnionForTab(tab state.Tab, w Renderer) map[state.VerbName]bool {
	allowed := make(map[state.VerbName]bool)
	add := func(verbs []state.VerbName) {
		for _, v := range verbs {
			allowed[v] = true
		}
	}
	switch tab {
	case state.TabHistory:
		tools := state.HostTools{Clipboard: w.Clipboard, Browser: w.Browser}
		add(state.CommitVerbs(git.CommitInfo{Unpushed: true}, 0, tools))
		add(state.CommitVerbs(git.CommitInfo{}, 1, tools))
		add([]state.VerbName{state.VerbNameDiff})
	case state.TabStashed:
		for _, status := range []git.StashStatus{git.StashApplies, git.StashConflicts} {
			add(state.StashVerbs(status))
		}
		allowed[state.VerbNameDiff] = true
	case state.TabWorktrees:
		other := git.WorktreeRow{Path: "/wt", Dirty: 0, Main: false}
		add(state.WorktreeVerbs(other, false))
		add(state.WorktreeVerbs(other, true))
		allowed[state.VerbNameRemove] = true
		allowed[state.VerbNameDiff] = true
	case state.TabChanges:
		unionChangesFooterVerbs(add, w)
	}
	return allowed
}

func unionChangesFooterVerbs(add func([]state.VerbName), w Renderer) {
	folded := map[string]bool{}
	tab := state.TabChanges

	add(footerVerbs(state.State{Tab: tab, Changes: state.Changes{Folded: folded}}, w))

	rows := []state.Row{
		state.SectionHeadingRow(state.SectionUnstaged),
		state.DirectoryRow("app", state.SectionUnstaged),
		state.FileRow(git.Entry{Path: "a.txt", Worktree: git.Modified}, state.SectionUnstaged),
		state.FileRow(git.Entry{Path: "b.txt", Index: git.Modified}, state.SectionStaged),
		state.FileRow(git.Entry{Path: "c.txt", Index: git.Unmerged, Worktree: git.Unmerged}, state.SectionUnstaged),
	}
	selected := map[string]bool{
		strconv.Itoa(int(state.SectionUnstaged)) + "\x00" + "a.txt": true,
		strconv.Itoa(int(state.SectionStaged)) + "\x00" + "b.txt":   true,
	}

	for cursor := range rows {
		add(footerVerbs(state.State{Tab: tab, Cursor: cursor, Rows: rows, Changes: state.Changes{Folded: folded}}, w))
		add(footerVerbs(state.State{Tab: tab, Cursor: cursor, Rows: rows, Changes: state.Changes{Folded: folded, Selected: selected}}, w))
	}
	add(footerVerbs(state.State{Tab: tab, Cursor: -1, Rows: rows, Changes: state.Changes{Folded: folded}}, w))
	add(footerVerbs(state.State{Tab: tab, Cursor: -1, Rows: rows, Changes: state.Changes{Folded: folded, Selected: selected}}, w))

	// The cursor sits on the directory in the list above, which is where a
	// folded one prints "unfold". A one-row list with the cursor at 1 put it
	// past the only row, and the verb the reader sees on a closed directory
	// was in nothing this gathers.
	foldedApp := map[string]bool{strconv.Itoa(int(state.SectionUnstaged)) + "\x00" + "app": true}
	add(footerVerbs(state.State{Tab: tab, Cursor: 1, Rows: rows, Changes: state.Changes{Folded: foldedApp}}, w))

	add(footerVerbs(state.State{Tab: tab, Cursor: 2, Rows: rows[:3], Changes: state.Changes{Folded: folded}, Open: state.OpenDiff{Path: "a.txt", Diff: git.FileDiff{Blocks: []git.Block{{Hash: "a"}, {Hash: "b"}}}}}, w))
}

func helpRowAllowedForTab(tab state.Tab, row helpRow, allowedFooter map[state.VerbName]bool, w Renderer) bool {
	if row.verb.IsZero() || row.verb == state.VerbNameClear {
		return true
	}
	if row.verb == state.VerbNameCommit {
		return helpTabHasCommitBox(tab)
	}
	if row.verb == state.VerbNameNextBlock {
		return helpTabHasDiffPanel(tab, w)
	}
	if helpAlwaysVerbs[row.verb] {
		return true
	}
	switch row.verb {
	case state.VerbNameSelect, state.VerbNameSelectAll, state.VerbNameExtend:
		return allowedFooter[state.VerbNameSelect]
	case state.VerbNameDiscard:
		return allowedFooter[state.VerbNameDiscard] || allowedFooter[state.VerbNameDrop] || allowedFooter[state.VerbNameRemove]
	default:
		// Every remaining verb is one the footer prints, so the tabs whose
		// footer prints it are the tabs that explain it. A fallback for a
		// fourth kind of verb showed a row on every tab without anything
		// saying the key does something there.
		return allowedFooter[row.verb]
	}
}

// discardDescFor names what the x key removes on this tab, in that tab's own
// word: the footer prints the same word.
func discardDescFor(tab state.Tab) string {
	switch state.Facts[tab].DiscardVerb {
	case state.VerbNameDrop:
		return "drop the stash"
	case state.VerbNameRemove:
		return "remove the worktree"
	}
	return "discard the change"
}

func helpRowsForTab(tab state.Tab, w Renderer) []helpRow {
	allowed := footerVerbUnionForTab(tab, w)
	var out []helpRow
	for _, row := range helpRows {
		if !helpRowAllowedForTab(tab, row, allowed, w) {
			continue
		}
		if state.Facts[tab].HelpClearClosesDiff && row.verb == state.VerbNameClear {
			row = helpRow{
				keys: row.keys,
				desc: "close the diff",
				verb: row.verb,
			}
		}
		// The x key runs a different verb per tab, and the row names the one
		// this tab runs. One row listing all three words made a reader on the
		// stashed tab read about worktrees.
		if row.verb == state.VerbNameDiscard {
			row = helpRow{keys: row.keys, desc: discardDescFor(tab), verb: state.Facts[tab].DiscardVerb}
		}
		out = append(out, row)
	}
	return out
}

func (w Renderer) helpRowsForOverlay(tabs []Tab, tab state.Tab) []helpRow {
	rows := helpRowsForTab(tab, w)
	keys := tabSwitchKeys(tabs)
	for i := range rows {
		if rows[i].verb == state.VerbNameNextTab {
			rows[i].keys = keys
			break
		}
	}
	desc := "character width  measured"
	if w.IsWidthAssumed {
		desc = "character width  assumed one column — the terminal did not answer"
	}
	rows = append(rows, helpRow{}, helpRow{desc: desc})
	return rows
}

func tabSwitchKeys(tabs []Tab) string {
	if len(tabs) == 0 {
		return "tab"
	}
	nums := make([]int, len(tabs))
	for i, t := range tabs {
		nums[i] = t.Number
	}
	if len(nums) == 1 {
		return fmt.Sprintf("tab  %d", nums[0])
	}
	consecutive := nums[0] == 1
	for i := 1; consecutive && i < len(nums); i++ {
		if nums[i] != nums[i-1]+1 {
			consecutive = false
		}
	}
	if consecutive {
		return fmt.Sprintf("tab  1-%d", nums[len(nums)-1])
	}
	parts := make([]string, len(nums))
	for i, n := range nums {
		parts[i] = strconv.Itoa(n)
	}
	return "tab  " + strings.Join(parts, "  ")
}

// HelpRowCount answers for the widths the overlay will actually draw with. The
// list is shorter without a clipboard or a browser, and counting as if both were
// there let the clamp scroll past the end.
func (w Renderer) HelpRowCount(tab state.Tab) int {
	return len(helpRowsForTab(tab, w)) + 2
}

// HelpScrollClamp returns the largest scroll offset that still shows the overlay.
func (w Renderer) HelpScrollClamp(scroll int, tab state.Tab, height int) int {
	contentRows := height - 2
	if contentRows < 0 {
		contentRows = 0
	}
	maxScroll := w.HelpRowCount(tab) - contentRows
	if maxScroll < 0 {
		maxScroll = 0
	}
	if scroll < 0 {
		return 0
	}
	if scroll > maxScroll {
		return maxScroll
	}
	return scroll
}

// HelpOverlay lists commands for the current tab. Verbs the footer cannot fit
// stay reachable by mouse here; rows match what the footer may show on this tab,
// except global keys that apply on every tab.
func (w Renderer) HelpOverlay(tabs []Tab, tab state.Tab, scroll, width, height int) ([]string, []Region) {
	rows := w.helpRowsForOverlay(tabs, tab)
	contentRows := height - 2
	if contentRows < 0 {
		contentRows = 0
	}
	scroll = w.HelpScrollClamp(scroll, tab, height)

	lines, regions := w.helpOverlayContent(rows, scroll, contentRows, width)
	lines = append(lines, w.ruleLine(width))
	footerRow := height - 1
	footer := w.helpOverlayFooter(scroll, contentRows, len(rows), width)
	lines = append(lines, footer)
	if mark, ok := footerHelpRegion(footer, footerRow, w); ok {
		regions = append(regions, mark)
	}
	return lines, regions
}

func (w Renderer) helpOverlayContent(rows []helpRow, scroll, contentRows, width int) ([]string, []Region) {
	var lines []string
	var regions []Region
	end := scroll + contentRows
	if end > len(rows) {
		end = len(rows)
	}
	for i := scroll; i < end; i++ {
		lines = append(lines, w.drawHelpRow(rows[i], width))
		regions = append(regions, Region{
			Target:   Target{Kind: TargetHelpLine, Verb: rows[i].verb},
			Row:      len(lines) - 1,
			ColStart: 0,
			ColEnd:   width,
		})
	}
	for len(lines) < contentRows {
		row := len(lines)
		lines = append(lines, strings.Repeat(" ", width))
		regions = append(regions, Region{
			Target:   Target{Kind: TargetHelpLine},
			Row:      row,
			ColStart: 0,
			ColEnd:   width,
		})
	}
	return lines, regions
}

func (w Renderer) helpOverlayFooter(scroll, contentRows, totalRows, width int) string {
	right := "esc close" + strings.Repeat(" ", helpGap) + "?"
	if totalRows <= contentRows {
		return w.Pad("", right, width)
	}
	visible := contentRows
	if scroll+visible > totalRows {
		visible = totalRows - scroll
	}
	left := fmt.Sprintf("j  k  scroll   %d-%d / %d", scroll+1, scroll+visible, totalRows)
	// The way out is what stays. Both halves are fixed phrases and Pad lets an
	// overlong pair overflow: measured, a pane of 30 columns drew the two run
	// together at 37 cells, which wrapped and pushed the last row of the list
	// off the screen. A reader who cannot see how far down the list they are
	// can still scroll; one who cannot see "esc close" is stuck.
	if w.Of(left)+1+w.Of(right) > width {
		left = w.Truncate(left, max(width-w.Of(right)-1, 0))
	}
	return w.Pad(left, right, width)
}

func (w Renderer) drawHelpRow(row helpRow, width int) string {
	if row.keys == "" && row.desc == "" {
		return strings.Repeat(" ", width)
	}
	if row.keys == "" {
		return fitWidth("  "+row.desc, width, w)
	}
	prefix := "  " + row.keys
	gap := helpKeyWidth - w.Of(prefix)
	if gap < 1 {
		gap = 1
	}
	return fitWidth(prefix+strings.Repeat(" ", gap)+row.desc, width, w)
}

func fitWidth(s string, width int, w Renderer) string {
	s = w.Truncate(s, width)
	gap := width - w.Of(s)
	if gap < 0 {
		gap = 0
	}
	return s + strings.Repeat(" ", gap)
}

// footerHelpRegion locates the help mark the footer appends, because footer.go
// draws the character without a region and the pane must attach one.
// footerHelpRegion is where the mark that opens the key list can be clicked.
//
// The column is measured, not counted in bytes. The footer separates its verbs
// with ·, which is two bytes and one cell, so a byte offset ran ahead of the
// column by one for every separator on the line: measured, a footer with three
// verbs put the region past the right edge of the pane and the mark could not
// be clicked at all.
func footerHelpRegion(line string, row int, w Renderer) (Region, bool) {
	at := strings.LastIndex(line, "?")
	if at < 0 {
		// A footer cut to fit a narrow pane has no mark to click. Recording
		// one anyway put the region on whatever the cut left in that column.
		return Region{}, false
	}
	idx := w.Of(line[:at])
	return Region{
		Target:   Target{Kind: TargetHelp},
		Row:      row,
		ColStart: idx,
		ColEnd:   idx + w.Of("?"),
	}, true
}

// HelpVerbs returns the verbs the overlay lists that run in this build.
func HelpVerbs() []state.VerbName {
	// The rows a tab draws are not the base rows: x carries that tab's own
	// verb. Reading the base list alone answered for verbs the help never
	// shows and missed the ones it does.
	seen := make(map[state.VerbName]bool)
	var out []state.VerbName
	add := func(v state.VerbName) {
		if v.IsZero() || seen[v] {
			return
		}
		seen[v] = true
		out = append(out, v)
	}
	for _, row := range helpRows {
		add(row.verb)
	}
	for tab := range state.TabCount {
		for _, row := range helpRowsForTab(tab, Renderer{Clipboard: true, Browser: true}) {
			add(row.verb)
		}
	}
	return out
}
