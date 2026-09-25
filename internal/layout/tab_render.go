package layout

import "github.com/catpotd/mirugit/internal/state"

// tabRender is what this package does differently per tab. state.TabFacts holds
// the values every layer needs; these three are functions of this package's own
// types, so they cannot live there. Adding a tab means adding one arm here
// rather than finding the three switches that used to choose between them.
//
// It is a function rather than a [state.TabCount]tabRender table because a
// package-level table would name changesPane, which reaches footerPlain, which
// reads the table back: Go rejects that as an initialization cycle. The
// exhaustive linter requires the switch to name every tab, which is the same
// guarantee a table of fixed length would give.
type tabRender struct {
	Pane func(state.State, ReadMarks, Renderer, Frame) Frame
	// List lays the rows out. win says which of them reach the screen: the
	// callers that only need where a row sits pass noLines and get the shape
	// without paying to lay anything out.
	List        func(state.State, ReadMarks, Renderer, lineWindow) ([]string, []Region)
	FooterVerbs func(state.State, Renderer) []state.VerbName
	// Bar is what the tab bar prints: how many rows the tab holds and whether
	// any of them is unread.
	Bar func(state.State, ReadMarks) (count int, unread bool)
}

func renderFor(tab state.Tab) tabRender {
	switch tab {
	case state.TabHistory:
		return tabRender{
			Pane: historyPane,
			List: func(s state.State, _ ReadMarks, w Renderer, win lineWindow) ([]string, []Region) {
				return historyListOf(s, w, win)
			},
			FooterVerbs: historyFooterVerbs,
			Bar: func(s state.State, _ ReadMarks) (int, bool) {
				return len(s.History.Commits), false
			},
		}
	case state.TabStashed:
		return tabRender{
			Pane:        stashPane,
			List:        stashListOf,
			FooterVerbs: func(s state.State, _ Renderer) []state.VerbName { return stashFooterVerbs(s) },
			Bar: func(s state.State, r ReadMarks) (int, bool) {
				return len(s.Stashed.Stashes), stashTabUnread(s, r)
			},
		}
	case state.TabWorktrees:
		return tabRender{
			Pane:        worktreePane,
			List:        worktreeListOf,
			FooterVerbs: func(s state.State, _ Renderer) []state.VerbName { return worktreeFooterVerbs(s) },
			Bar: func(s state.State, r ReadMarks) (int, bool) {
				return worktreeTabCount(s), worktreeTabUnread(s, r)
			},
		}
	case state.TabChanges:
	}
	return tabRender{
		Pane:        changesPane,
		List:        sectionsOf,
		FooterVerbs: func(s state.State, _ Renderer) []state.VerbName { return changesFooterVerbs(s) },
		Bar: func(s state.State, r ReadMarks) (int, bool) {
			// The commit box and the summary line live on this tab, so it stays
			// in the bar at zero.
			return state.FileRowCount(s.Changes.Entries), changesTabUnread(s, r)
		},
	}
}

// TabList draws the list for one tab. tui asks for it by tab rather than
// naming the four builders itself.
func TabList(s state.State, r ReadMarks, w Renderer) ([]string, []Region) {
	return renderFor(s.Tab).List(s, r, w, everyLine)
}

// TabListShape is how many lines the list has and where each row sits, without
// laying any of them out. The pane asks this on every key and every pointer
// move to place the cursor and the scroll, and it needs neither the text nor
// the widths: laying the list out to answer it cost 214 ms per keystroke in a
// repository of twenty thousand changed files.
func TabListShape(s state.State, r ReadMarks, w Renderer) (lines int, regions []Region) {
	built, regions := renderFor(s.Tab).List(s, r, w, noLines)
	return len(built), regions
}

// TabListLineCount is TabListShape for a caller that reads only the number. The
// rows outside the window each carry a click target, and building twenty
// thousand of them to return a count was most of what one frame allocated.
func TabListLineCount(s state.State, r ReadMarks, w Renderer) int {
	built, _ := renderFor(s.Tab).List(s, r, w, countLines)
	return len(built)
}
