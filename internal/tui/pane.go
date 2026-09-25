package tui

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/layout"
	"github.com/catpotd/mirugit/internal/state"
)

// This file holds the keys and commands that work the same on every tab:
// reading, scrolling, fetching, reloading, and the diff block cursor.
// changes.go, history.go, stash.go and worktree.go each answer for their own tab
// alone, which TestTabNamedFilesDoNotBranchOnTheTab keeps true.

func (m *Model) requestRead() (tea.Model, tea.Cmd) {
	if !state.Facts[m.state.Tab].RequestReadOnTab {
		return m, nil
	}
	row, ok := state.CursorRow(m.state)
	if !ok {
		return m, nil
	}
	if row.Kind() != state.RowFile {
		return m, nil
	}
	path := row.Path()
	if path == "" {
		return m, nil
	}
	origin, ok := state.OriginOf(m.state, m.state.Cursor)
	if !ok {
		return m, nil
	}
	// The block hashes behind a read mark belong to the row, not to the diff on
	// screen: following the cursor opens a diff, and scoping the mark to that
	// read had the open cancel it.
	return m, loadReadMarks(m.readsForTab(), m.dir, path, m.sectionFor(path) == state.SectionStaged, origin)
}

// rebindTo reads what pointing the model at another worktree needs. The
// directory alone is not enough: the watcher is watching the old one, the read
// marks are keyed by repository path, and each linked worktree keeps its state
// somewhere of its own. Moving without all three wrote the new worktree's marks
// into the old one's file and read the old one's merge state.
//
// The three are read here and swapped together in the handler, because the
// reads can fail. Writing the directory first and asking for the other two
// after left the pane pointed at one worktree while writing the marks of
// another — the accident above, reached through the error path rather than by
// forgetting a line.
func (m *Model) rebindTo(dir string) tea.Cmd {
	ctx, stateDir := m.rootContext(), m.stateDir
	return func() tea.Msg {
		gitDir, err := git.GitDir(ctx, dir)
		if err != nil {
			return reboundMsg{err: err}
		}
		read, err := state.LoadRead(ctx, stateDir, dir)
		if err != nil {
			return reboundMsg{err: err}
		}
		return reboundMsg{dir: dir, gitDir: gitDir, read: read}
	}
}

// nestedFileDiff answers where a nested file's diff comes from. It is the one
// place a tab is named for this: a stash reads from its own commit, a worktree
// from its own directory, and no table can hold either because both are
// commands rather than values.
func (m *Model) nestedFileDiff(path string) (dir string, cmd tea.Cmd) {
	return commandsFor(m.state.Tab).NestedDiff(m, path)
}

func (m *Model) stashNestedDiff(path string) (string, tea.Cmd) {
	sha := m.stashSHAFor(path)
	if sha == "" {
		return "", nil
	}
	return m.dir, loadStashDiff(m.startDiffRead(), m.dir, sha, path)
}

func (m *Model) worktreeNestedDiff(path string) (string, tea.Cmd) {
	wtDir := m.worktreeDirFor(path)
	row, ok := state.CursorRow(m.state)
	if wtDir == "" || !ok {
		return "", nil
	}
	return wtDir, loadWorktreeDiff(m.startDiffRead(), wtDir, path, worktreeFileStaged(row))
}

// listShape is how many lines the list has and where its rows sit. It lays out
// nothing: the cursor and the scroll are placed from positions, and building
// the text to answer them cost the whole repository on every keystroke.
func (m *Model) listShape() (lines int, regions []layout.Region) {
	return layout.TabListShape(m.state, m.read, m.render)
}

// windowRows answers how many rows the list and the diff each get, and how many
// lines the list holds.
func (m *Model) windowRows() (listRows, diffRows, listLines int) {
	lines := layout.TabListLineCount(m.state, m.read, m.render)
	body := m.state.Height - layout.HeaderRowsOf(m.state) - 1
	listRows, diffRows = layout.ListRowsFor(m.state, lines, body)
	return listRows, diffRows, lines
}

// syncScroll keeps the cursor row inside the list window when the list scrolls.
func (m *Model) syncScroll() {
	lines, regions := m.listShape()
	body := m.state.Height - layout.HeaderRowsOf(m.state) - 1
	cursorLine := layout.CursorLineInList(regions, m.state.Cursor)
	listRows, _ := layout.ListRowsFor(m.state, lines, body)
	m.state = state.Apply(m.state, state.ScrollSynced{SetList: true,
		ListTop: layout.ScrollTopFor(m.state.ScrollTop, lines, listRows, cursorLine)})
}

// The guard sits here rather than at each caller because the button, the S key
// and the help line all arrive at it, and the click on the button used to reach
// sync without asking at all.
func (m *Model) requestSync() (tea.Model, tea.Cmd) {
	if !layout.SyncOffered(m.state.Head) {
		return m, nil
	}
	return m, runSync(m.rootContext(), m.dir)
}

func (m *Model) requestFetch() (tea.Model, tea.Cmd) {
	return m, runFetch(m.rootContext(), m.dir)
}

// listRowsNow is how far one page moves: the rows the list has on screen, so a
// page ends where the eye left off rather than at a fixed count. ListRowsFor
// never answers less than one, so a pane too short for a list still moves.
// reloadRepo is the one place that pairs the worktree with the directory git
// keeps its state in. Ten call sites naming both was ten chances to pass one
// worktree's path with another's state.
func (m *Model) reloadRepo() tea.Cmd {
	return loadRepo(m.readsForTab(), m.dir, m.gitDir)
}

func (m *Model) listRowsNow() int {
	listRows, _, _ := m.windowRows()
	return listRows
}

func (m *Model) diffRowsNow() int {
	if m.state.Open.Path == "" {
		return 0
	}
	_, diffRows, _ := m.windowRows()
	return diffRows
}

func (m *Model) recordShownBlocks() {
	if m.state.Open.Path == "" || len(m.state.Open.Diff.Blocks) == 0 {
		return
	}
	key := fmt.Sprintf("%v:%s", m.state.Open.Origin, m.state.Open.Path)
	if key != m.shown.key {
		m.shown.key = key
		m.shown.hashes = map[string]bool{}
	}
	for _, hash := range layout.ShownBlockHashes(m.state.Open.Diff, m.state.Open.Scroll, m.state.Open.BlockLine, m.diffRowsNow()) {
		m.shown.hashes[hash] = true
	}
}

func (m *Model) moveBlockCursor(by int) (tea.Model, tea.Cmd) {
	// Whether the open diff has blocks to step through is answered in Apply,
	// which leaves the block cursor alone when it has none.
	if m.state.Open.Path == "" {
		return m, nil
	}
	m.state = state.Apply(m.state, state.BlockCursorMoved{By: by})
	m.state = state.Apply(m.state, state.ScrollSynced{SetDiff: true,
		DiffTop: layout.DiffScrollFor(m.state.Open.Diff, m.state.Open.Scroll,
			m.state.Open.BlockCursor, m.diffRowsNow())})
	m.recordShownBlocks()
	return m, nil
}

func (m *Model) moveBlockLine(by int) bool {
	next := layout.DiffLineScrollFor(m.state.Open.Diff, m.state.Open.Scroll,
		m.state.Open.BlockLine+by, m.diffRowsNow())
	if next == m.state.Open.BlockLine {
		return false
	}
	m.state = state.Apply(m.state, state.ScrollSynced{SetDiffBlockLine: true,
		DiffBlockLine: next})
	m.recordShownBlocks()
	return true
}

// diffStale reports whether the open diff still matches the repository. A
// failed read answers stale: the guard withholds stage until the reader
// reloads, and answering "fresh" let a stage run against blocks git could not
// confirm.
func diffStale(ctx context.Context, dir, path string, staged bool, shown git.FileDiff) (bool, error) {
	if len(shown.Blocks) == 0 {
		return false, nil
	}
	current, err := git.Diff(ctx, dir, path, staged)
	if err != nil {
		return true, err
	}
	if len(current.Blocks) != len(shown.Blocks) {
		return true, nil
	}
	for i := range shown.Blocks {
		if shown.Blocks[i].Hash != current.Blocks[i].Hash {
			return true, nil
		}
	}
	return false, nil
}

// checkStale asks whether the open diff still matches the repository. It runs
// as a command because git costs about 20 ms and a frame cannot wait for it;
// View used to call syncStale directly, which put one git process on every
// mouse motion.
func (m *Model) checkStale() tea.Cmd {
	if !m.state.Open.Origin.IsWorkingTree() || m.state.Open.Path == "" {
		return nil
	}
	dir, path := m.dir, m.state.Open.Path
	staged := m.state.Open.Origin.Side() == state.SectionStaged
	shown := m.state.Open.Diff
	ctx := m.readsForTab()
	return func() tea.Msg {
		stale, err := diffStale(ctx, dir, path, staged, shown)
		return staleMsg{path: path, stale: stale, err: err}
	}
}

// noBlockYet says the stale check was started by a key that has not chosen a
// block. Which block is staged is decided after the answer, by the same rules
// the key path uses.
const noBlockYet = -1

// checkStaleFirst asks whether the open diff still matches the file. Staging
// what the pane no longer shows stages something the reader never saw, so the
// answer comes before the write. It is nil when no working-tree diff is open,
// because then there is nothing on screen that could be out of date.
func (m *Model) checkStaleFirst(block int) tea.Cmd {
	if !m.state.Open.Origin.IsWorkingTree() || m.state.Open.Path == "" {
		return nil
	}
	ctx, dir := m.readsForTab(), m.dir
	path := m.state.Open.Path
	staged := m.state.Open.Origin.Side() == state.SectionStaged
	shown := m.state.Open.Diff
	return func() tea.Msg {
		stale, err := diffStale(ctx, dir, path, staged, shown)
		return staleCheckMsg{path: path, block: block, stale: stale, err: err}
	}
}

func (m *Model) blockStageReady() bool {
	if m.state.Open.Path == "" || len(m.state.Open.Diff.Blocks) == 0 {
		return false
	}
	row, ok := state.CursorRow(m.state)
	if !ok {
		return false
	}
	return row.Kind() == state.RowFile &&
		row.Path() == m.state.Open.Path &&
		!row.Entry().IsConflicted() &&
		row.Section() == state.SectionUnstaged
}

func (m *Model) requestBlockStage(block int) (tea.Model, tea.Cmd) {
	if !m.blockStageReady() {
		return m, nil
	}
	m.state = state.Apply(m.state, state.BlockCursorSet{Block: block})
	if cmd := m.checkStaleFirst(block); cmd != nil {
		return m, cmd
	}
	return m.stageBlockNow(block)
}

func (m *Model) stageBlockNow(block int) (tea.Model, tea.Cmd) {
	if m.state.Changes.Stale[m.state.Open.Path] {
		return m, nil
	}
	path := m.state.Open.Path
	m.state = state.Apply(m.state, state.VerbRequested{
		Verb: state.VerbStage, Targets: []string{path}, Block: block,
	})
	return m, runStageBlock(m.rootContext(), m.dir, m.state.Open.Diff, block)
}

// reloadCurrentTab rereads what the pane draws. The worktree list comes with it
// for the same reason Init reads it: the tab is hidden while its count is zero,
// so a tree added beside this pane has no row to click and no tab to open. The
// bar's other three counts arrive with the tab reload; this is the fourth, and
// leaving it to the reader's R would make the bar tell the truth about three
// numbers and not the fourth.
//
// git worktree list --porcelain measured at 10 ms, against a poll every five
// seconds.
func (m *Model) reloadCurrentTab() tea.Cmd {
	tabCmd, fetchCmd := m.reloadTabCmds()
	return tea.Batch(tabCmd, fetchCmd, loadWorktrees(m.readsForTab(), m.dir))
}

func (m *Model) reloadTabCmds() (tea.Cmd, tea.Cmd) {
	return commandsFor(m.state.Tab).Reload(m), loadFetched(m.readsForTab(), m.dir)
}

func (m *Model) requestReload() (tea.Model, tea.Cmd) {
	cmds := []tea.Cmd{m.reloadCurrentTab()}
	if path := m.state.Open.Path; path != "" {
		if sha, fromCommit := m.state.Open.Origin.CommitSHA(); fromCommit {
			cmds = append(cmds, loadCommitDiff(m.readsForDiff(), m.dir, sha, path))
		} else if _, nested := m.nestedFileDiff(path); nested != nil {
			cmds = append(cmds, nested)
		} else {
			cmds = append(cmds, loadDiff(m.readsForDiff(), m.dir, path, m.state.Open.Origin.Side() == state.SectionStaged))
		}
	}
	return m, tea.Batch(cmds...)
}

// requestUndo asks which of the discarded paths have been written since. The
// answer decides between putting them back and asking first, and it costs a git
// call, so it is read off the update loop like every other repository call.
func (m *Model) requestUndo() (tea.Model, tea.Cmd) {
	if m.state.LastUndo == nil {
		return m, nil
	}
	undo := *m.state.LastUndo
	ctx, dir := m.rootContext(), m.dir
	return m, func() tea.Msg {
		edited, err := git.PathsEditedAfterDiscard(ctx, dir, undo)
		if err != nil {
			return undoCheckMsg{err: err}
		}
		return undoCheckMsg{undo: undo, edited: len(edited)}
	}
}

func (m *Model) requestCommit() (tea.Model, tea.Cmd) {
	if m.state.Changes.Message == "" {
		return m, nil
	}
	files := m.state.StagedFileCount()
	if files == 0 {
		m.state = state.Apply(m.state, state.CommitBlocked{})
		return m, nil
	}
	return m, runCommit(m.rootContext(), m.dir, m.state.Changes.Message, files)
}

// wheelStep is how many rows one notch of the wheel moves. Terminals send one
// message per notch and give no distance, so the pane picks one.
const wheelStep = 3

// handleMouseWheel scrolls whichever side the pointer is over. A click already
// decides by coordinate; the wheel following a different rule would make the
// same pointer mean two things.
func (m *Model) handleMouseWheel(msg tea.MouseWheelMsg) (tea.Model, tea.Cmd, bool) {
	by := wheelStep
	switch msg.Button {
	case tea.MouseWheelUp:
		by = -wheelStep
	case tea.MouseWheelDown:
	default:
		return m, nil, true
	}
	listRows, _, listLines := m.windowRows()
	// The diff side moves the block cursor rather than the scroll alone. Only
	// the cursor's block carries the "s stage" label and only it is what s
	// stages, so scrolling past it offered a key that acted on a block off the
	// screen. Moving the cursor also records what the reader passed, which a
	// bare scroll left unread.
	if m.state.Open.Path != "" && msg.Y >= layout.HeaderRowsOf(m.state)+listRows {
		if m.moveBlockLine(by) {
			return m, nil, true
		}
		step := 1
		if by < 0 {
			step = -1
		}
		model, cmd := m.moveBlockCursor(step)
		return model, cmd, true
	}
	nextTop := clampScroll(m.state.ScrollTop+by, listLines, listRows)
	m.state = state.Apply(m.state, state.ScrollSynced{SetList: true, ListTop: nextTop})
	if by > 0 && nextTop == clampScroll(listLines, listLines, listRows) {
		return m, m.requestHistoryMore(), true
	}
	return m, nil, true
}

func clampScroll(top, lines, rows int) int {
	max := lines - rows
	if max < 0 {
		max = 0
	}
	if top > max {
		top = max
	}
	if top < 0 {
		top = 0
	}
	return top
}
