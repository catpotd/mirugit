package layout

import (
	"fmt"
	"strings"
	"time"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// TabsOf builds the tab bar from state. changes stays visible at zero because
// the commit box and summary line live on that tab (spec 335); hiding it would
// leave them nowhere to go.
func TabsOf(s state.State, r ReadMarks) []Tab {
	var out []Tab
	for i := range int(state.TabCount) {
		tab := state.Tab(i)
		count, unread := renderFor(tab).Bar(s, r)
		if !state.TabVisible(tab, count) {
			continue
		}
		out = append(out, Tab{
			Logical: tab,
			Name:    state.Facts[tab].Name,
			Count:   count,
			Unread:  unread,
			Number:  i + 1,
		})
	}
	return out
}

// The changed files are read from Changes.Entries whichever tab is showing.
// Reading them from Rows instead is only right while the changes tab is the one
// drawn: the stashed and worktrees tabs put the files of the expanded row into
// Rows as file rows, and counting those lit the changes tab's dot for files
// that are not in the working tree at all.
func changesTabUnread(s state.State, r ReadMarks) bool {
	for _, e := range s.Changes.Entries {
		if e.IsStaged() && fileMark(r, state.WorkingTree(state.SectionStaged), e.Path, s) == state.Unread {
			return true
		}
		if e.IsUnstaged() && fileMark(r, state.WorkingTree(state.SectionUnstaged), e.Path, s) == state.Unread {
			return true
		}
	}
	return false
}

func stashTabUnread(s state.State, r ReadMarks) bool {
	if !marksPresent(r) {
		return false
	}
	for _, row := range s.Stashed.Stashes {
		if r.StashMark(row.SHA) == state.Unread {
			return true
		}
	}
	return false
}

func worktreeTabUnread(s state.State, r ReadMarks) bool {
	if !marksPresent(r) {
		return false
	}
	for _, row := range s.Worktrees.List {
		if row.SHA == "" {
			continue
		}
		if r.WorktreeMark(row.Path, row.SHA) == state.Unread {
			return true
		}
	}
	return false
}

// worktreeTabCount comes from s.Worktrees; a lone main tree is omitted because
// most repositories have nothing else to show on that tab.
func worktreeTabCount(s state.State) int {
	n := len(s.Worktrees.List)
	if n <= 1 {
		return 0
	}
	return n
}

// summary holds the counts drawn on the header's bottom line.
type summary struct {
	Unread  int
	Files   int
	Added   int
	Deleted int
	Behind  int
	Ahead   int
	Staged  int
	// Selection summary fields are filled when anything is selected.
	Selected          int
	SelectedStaged    int
	SelectedAdded     int
	SelectedDeleted   int
	Notice            string
	DiscardConfirm    *state.DiscardConfirm
	SelectionBarVerbs []state.VerbName
	// NoUpstream hides the sync button when set, because counts alone cannot
	// distinguish a synced branch from one with no upstream.
	NoUpstream bool
}

// summaryOf counts from the state what the summary line needs.
func summaryOf(s state.State, r ReadMarks) summary {
	sum := summary{
		Behind: s.Head.Behind, Ahead: s.Head.Ahead,
		NoUpstream: !s.Head.HasUpstream,
	}
	selected := len(s.Changes.Selected) > 0
	for _, row := range s.Rows {
		// Headings and directories share the row list with files but carry an
		// empty Entry, so counting them inflates every total on this line.
		if row.Kind() != state.RowFile {
			continue
		}
		sum.Files++
		count := row.Entry().WorktreeCount
		if row.Section() == state.SectionStaged {
			count = row.Entry().IndexCount
			sum.Staged++
		}
		sum.Added += count.Added
		sum.Deleted += count.Deleted
		if fileMark(r, state.WorkingTree(row.Section()), row.Entry().Path, s) == state.Unread {
			sum.Unread++
		}
		if selected && state.SelectedIn(s, row.Section(), row.Entry().Path) {
			sum.SelectedAdded += count.Added
			sum.SelectedDeleted += count.Deleted
			if row.Section() == state.SectionStaged {
				sum.SelectedStaged++
			}
		}
	}
	if selected {
		sum.Selected = len(s.Changes.Selected)
		sum.SelectionBarVerbs = state.SelectionVerbs(s)
	}
	sum.Notice = s.Notice
	sum.DiscardConfirm = s.Changes.DiscardConfirm
	return sum
}

func summaryLeft(sum summary) string {
	if sum.DiscardConfirm != nil {
		c := sum.DiscardConfirm
		// No way back is offered yet: nothing has run, so U would reach the
		// discard before this one.
		return fmt.Sprintf("Discard %d %s · %s %s",
			c.Files, state.FileWord(c.Files), state.Plus(c.Added), state.Minus(c.Deleted))
	}
	if sum.Selected > 0 {
		left := []string{fmt.Sprintf("%d selected", sum.Selected)}
		if sum.SelectedStaged > 0 {
			left = append(left, fmt.Sprintf("%d staged", sum.SelectedStaged))
		}
		left = append(left, fmt.Sprintf("%s %s", state.Plus(sum.SelectedAdded), state.Minus(sum.SelectedDeleted)))
		return strings.Join(left, " · ")
	}
	var left []string
	if sum.Unread > 0 {
		left = append(left, fmt.Sprintf("%d unread", sum.Unread))
	}
	left = append(left, fmt.Sprintf("%d %s", sum.Files, state.FileWord(sum.Files)))
	left = append(left, fmt.Sprintf("%s %s", state.Plus(sum.Added), state.Minus(sum.Deleted)))
	return strings.Join(left, " · ")
}

func activeTab(logical state.Tab, tabs []Tab) int {
	for i, t := range tabs {
		if t.Logical == logical {
			return i
		}
	}
	return -1
}

// CommitBox draws the field even when empty, because a commit message typed
// into a box that appears only once something is staged is a message typed
// twice.
// CommitBox draws the message field. focused decides whether the caret is
// shown: typing only reaches this box while it holds the focus, and a caret is
// a character rather than a color, so it reads on a sixteen-color terminal and
// under NO_COLOR.
func (w Renderer) CommitBox(text string, focused bool, width int) (string, Region) {
	const open = "[ "
	const close = " ]"
	const caret = "▏"
	inner := width - w.Of(open) - w.Of(close)
	content := text
	if focused {
		// The prompt would read as text already typed once the caret is there.
		if w.Of(content) > inner-w.Of(caret) {
			content = w.TruncateFront(content, inner-w.Of(caret))
		}
		content += caret
	} else {
		if content == "" {
			content = "commit message"
		}
		if w.Of(content) > inner {
			content = w.Truncate(content, inner)
		}
	}
	gap := max(inner-w.Of(content), 0)
	line := open + content + strings.Repeat(" ", gap) + close
	if focused && w.Enabled {
		line = w.paintTail(line, width, w.Verb(close), w.Of(close))
		caretAt := len(open) + len(content) - len(caret)
		line = line[:caretAt] + w.Mark(caret) + line[caretAt+len(caret):]
		line = w.Verb(open) + line[len(open):]
	}
	return line, Region{
		Target:   Target{Kind: TargetCommitBox},
		ColStart: 0,
		ColEnd:   width,
	}
}

// VisibleTabs names the tabs the bar draws. A tab that is not drawn cannot be
// switched to, or the bar would stop saying where the reader is.
func VisibleTabs(s state.State, r ReadMarks) []string {
	tabs := TabsOf(s, r)
	names := make([]string, 0, len(tabs))
	for _, t := range tabs {
		names = append(names, t.Name)
	}
	return names
}

// FormatFetchedAgo labels how long ago the last fetch ran. now is passed in so
// TabBar stays a pure function of strings. Zero fetchedAt means fetch never
// ran, and the label says so rather than guessing an elapsed time.
func FormatFetchedAgo(fetchedAt, now time.Time) string {
	if fetchedAt.IsZero() {
		return "never fetched"
	}
	return "fetched " + git.FormatAge(now.Sub(fetchedAt)) + " ago"
}
