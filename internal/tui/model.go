// Package tui is the only package that talks to bubbletea. It maps input to
// state events and issues a command for every repository call.
package tui

import (
	"context"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/layout"
	"os/exec"

	"github.com/catpotd/mirugit/internal/osproc"
	"github.com/catpotd/mirugit/internal/state"
	"github.com/catpotd/mirugit/internal/update"
	"github.com/charmbracelet/colorprofile"
	"github.com/fsnotify/fsnotify"
)

type Model struct {
	dir string
	// startDir is where mirugit was started. dir moves when the reader goes to
	// another worktree; this does not, so a worktree can be removed by a git
	// run from a directory the removal does not take with it.
	startDir string
	// gitDir is where the current worktree keeps its state. Finding it costs a
	// git call and the answer holds until the pane is pointed elsewhere, so it
	// is resolved on rebind rather than on every reload.
	gitDir string
	// stateDir is kept because moving to another worktree has to load that
	// worktree's own read marks; LoadRead keys the file by repository path.
	stateDir  string
	render    layout.Renderer
	probe     widthProbe
	watch     watchState
	asked     filesAsked
	shown     shownBlocks
	state     state.State
	read      *state.ReadState
	frame     layout.Frame
	fetchedAt time.Time
	// base is what every git this pane starts descends from. main cancels it on
	// the way out, and the two below are derived from it.
	base context.Context
	// tabCtx and tabCancel govern the reads that fill the current tab. A
	// repository big enough for a read to be slow is the one where the reader
	// moves on before the answer arrives, and the answer to a tab nobody is
	// looking at is work nobody wanted.
	tabCtx    context.Context
	tabCancel context.CancelFunc
	// diffCtx and diffCancel govern the reads for the diff on screen.
	diffCtx    context.Context
	diffCancel context.CancelFunc
	// clipboard is how y reaches the clipboard. It is a field rather than a
	// package variable because a test that swaps a package variable swaps it for
	// every test in the package: the race detector caught one parallel test
	// writing it while another read it, and in between, a copy that was supposed
	// to work ran the stub. m.delay is the same shape for the same reason.
	clipboard func(string) *exec.Cmd
	// browser is how o reaches the browser, a field for the reason clipboard is
	// one. Without it a test that presses o starts a browser on the machine
	// running the tests: measured, one run of the suite opened a tab at the
	// address the fixture's remote builds.
	browser func(string) *exec.Cmd
	// updateCheck is injected so the optional release check can be tested without
	// starting a network request.
	updateCheck func(context.Context) (string, error)
}

// clipboardCommand is what y runs. A Model built as a literal leaves the field
// nil and gets the real clipboard, the same way delay hands back the real
// interval.
func (m *Model) clipboardCommand() func(string) *exec.Cmd {
	if m.clipboard != nil {
		return m.clipboard
	}
	return osproc.ClipboardCommand
}

// browserCommand is what o runs, and it answers the same way clipboardCommand
// does for the same reason.
func (m *Model) browserCommand() func(string) *exec.Cmd {
	if m.browser != nil {
		return m.browser
	}
	return osproc.BrowserCommand
}

// watchState is everything the file watcher owns. It was seven fields on Model,
// and watch.go was the only file that read most of them.
type watchState struct {
	// focused is false while the terminal window is not, and a blurred pane
	// does not reload: the reader is not looking at it.
	focused bool
	fs      *fsnotify.Watcher
	// debounceGen rises on every event so a debounce that fires late for an
	// older burst is dropped.
	debounceGen int
	// Exactly one waitWatchEvent and one poll tick may be outstanding. Each
	// blocks a goroutine until it fires, so issuing one without consuming one
	// leaks: focus and blur used to add without taking, and a focus within five
	// seconds of a blur started a second poll chain that doubled the reload rate.
	waitingForEvent bool
	waitingForPoll  bool
	// debounceDelay and pollDelay let a test shorten its own waits. Zero means
	// the real interval, so a Model built as a literal behaves like the
	// program.
	debounceDelay time.Duration
	pollDelay     time.Duration
}

// filesAsked records what has already been asked for, so that a keypress that
// changes nothing costs no git call and no write.
//
// One key per question, not one per tab: expandRow was two fields answering the
// same thing, and clearing one without the other left a stash's file list gone
// for good at that cursor.
type filesAsked struct {
	// expandRow is the row whose files were last requested. It records the
	// request rather than the answer, so a second keypress does not stack a
	// duplicate git call while the first is still running.
	expandRow string
	// readRow is the row last marked read, so that holding a key down does not
	// write the read file once per repeat.
	readRow string
}

// shownBlocks records which blocks of the open diff have been on screen, so
// closing the file can mark exactly those read.
type shownBlocks struct {
	key    string
	hashes map[string]bool
}

func New(ctx context.Context, dir, stateDir string) (*Model, error) {
	if err := git.Prune(ctx, dir); err != nil {
		return nil, err
	}
	read, err := state.LoadRead(ctx, stateDir, dir)
	if err != nil {
		return nil, err
	}
	gitDir, err := git.GitDir(ctx, dir)
	if err != nil {
		return nil, err
	}
	return &Model{
		base:      ctx,
		dir:       dir,
		startDir:  dir,
		gitDir:    gitDir,
		stateDir:  stateDir,
		read:      read,
		watch:     watchState{focused: true},
		clipboard: osproc.ClipboardCommand,
		render: layout.Renderer{
			Clipboard: osproc.ClipboardReady(),
			Browser:   osproc.BrowserReady(),
		},
		state: state.State{Changes: state.Changes{
			Folded:   map[string]bool{},
			Selected: map[string]bool{},
			Stale:    map[string]bool{},
		},
			Width: 80, Height: 24,
			// The label starts at its zero-time answer so the bar reads right
			// before the first fetchedMsg arrives; View no longer computes it.
			Fetched: layout.FormatFetchedAgo(time.Time{}, time.Now()),
		},
	}, nil
}

// NewWithVersion builds the reader and enables the optional release check.
// MIRUGIT_NO_UPDATE_CHECK disables the check when the variable is present.
func NewWithVersion(ctx context.Context, dir, stateDir, version string) (*Model, error) {
	m, err := New(ctx, dir, stateDir)
	if err != nil {
		return nil, err
	}
	if _, disabled := os.LookupEnv("MIRUGIT_NO_UPDATE_CHECK"); disabled {
		return m, nil
	}
	m.updateCheck = func(checkCtx context.Context) (string, error) {
		return update.Check(checkCtx, stateDir, version)
	}
	return m, nil
}

func (m *Model) Init() tea.Cmd {
	// The palette is decided once. NO_COLOR and MIRUGIT_THEME do not change
	// while the program runs, and colorprofile.Detect allocates os.Environ()
	// on every call.
	m.render.Palette = colorPalette(colorprofile.Detect(os.Stdout, os.Environ()))
	// Worktrees are loaded here rather than when the tab opens, because the
	// tab is hidden while its count is zero and would never offer the click
	// that loads it.
	return tea.Batch(m.checkForUpdate(), m.reloadRepo(), loadWorktrees(m.readsForTab(), m.dir), loadFetched(m.readsForTab(), m.dir),
		beginWidthProbe(), beginWatch(m.base, m.dir), m.schedulePoll())
}

// followCursor re-reads the diff when the cursor lands on a different file.
// Moving the cursor releases the diff snapshot, so scattering the check
// across each key would leave a path where the diff never updates.
func (m *Model) followCursor() tea.Cmd {
	if !state.Facts[m.state.Tab].DiffFollowsCursor {
		return nil
	}
	// Which side a file's diff comes from is a fact about the tab. Naming the
	// tab here instead re-derived it, and the two could disagree.
	if state.Facts[m.state.Tab].OpenFile == state.OpenFileFromCommit {
		return m.followHistoryCursor()
	}
	path := m.cursorPath()
	if path == "" {
		return nil
	}
	if m.state.Open.Closed && path == m.state.Open.ClosedFor {
		return nil
	}
	sec := m.sectionFor(path)
	// The path alone does not say which diff is open. A file staged and edited
	// again has a row on each side holding different content, and skipping the
	// load left the reader on the other side's diff.
	if path == m.state.Open.Path && sec == m.state.Open.Origin.Side() {
		return nil
	}
	save := m.finishReading()
	// The reader did not ask for this diff; the cursor landed on it. Whatever
	// room the open diff has is the room this one gets, so a list longer than
	// the pane does not take the diff away on the next keypress.
	m.state = state.Apply(m.state, state.DiffOpened{Path: path, Peek: m.state.Open.Peek,
		Origin: state.WorkingTree(sec)})
	return tea.Batch(save, loadDiff(m.startDiffRead(), m.dir, path, sec == state.SectionStaged))
}

// markCursorRead records that the reader has looked at the row they are on.
// Looking is what reading is, so this one follows the cursor. What the row
// holds does not: a row opens because the reader opened it.
//
// It answers nil while the cursor has not left the row it last marked. Every
// keypress comes through here, and a save on each one wrote the read file
// again for a cursor that had not moved.
func (m *Model) markCursorRead() tea.Cmd {
	key := commandsFor(m.state.Tab).KeyAtCursor(m)
	if key == "" || key == m.asked.readRow {
		return nil
	}
	m.asked.readRow = key
	return commandsFor(m.state.Tab).MarkCursorRead(m)
}

// The profile is a parameter rather than detected here so that the answer for
// each one can be read back. Detect needs a terminal, so a test that cannot
// reach this decision leaves it unchecked: a mutation that turned color on for
// an Ascii terminal survived the suite.
//
// Ascii is a terminal that renders text and nothing else, so it and everything
// below it get no escape codes.
func colorPalette(profile colorprofile.Profile) layout.Palette {
	if os.Getenv("NO_COLOR") != "" {
		return layout.Palette{Enabled: false}
	}
	return layout.Palette{
		Enabled: profile > colorprofile.Ascii,
		Theme:   layout.ThemeByName(os.Getenv("MIRUGIT_THEME")),
	}
}

func (m *Model) View() tea.View {
	if !m.probe.settled {
		v := tea.NewView(m.probeViewContent())
		v.AltScreen = true
		v.ReportFocus = true
		v.MouseMode = tea.MouseModeCellMotion
		return v
	}
	m.frame = layout.Pane(m.state, m.read, m.render)

	v := tea.NewView(strings.Join(m.frame.Lines, "\n"))
	v.AltScreen = true
	v.ReportFocus = true
	// Clicks, releases and the wheel, and no report for a pointer that is only
	// passing over. Asking for every move woke the pane on each one and the
	// answer was always to do nothing.
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

// reopenExpandedRow re-reads what the open row holds after its list is reloaded.
// The list is rebuilt from scratch, so the files under the open row are the one
// part of it that no reload carries.
func (m *Model) reopenExpandedRow() tea.Cmd {
	return commandsFor(m.state.Tab).ExpandRow(m, m.expandedKey())
}

// expandedKey names the row the reader has open, in the key ExpandRow takes.
func (m *Model) expandedKey() string {
	return state.Facts[m.state.Tab].ExpandedKey(m.state)
}

// rootContext is what every scope below descends from. A Model built field by
// field rather than by New has none, which 37 tests in this package do, and
// context.WithCancel panics on a nil parent.
func (m *Model) rootContext() context.Context {
	if m.base == nil {
		return context.Background()
	}
	return m.base
}

// readsForTab is the context this tab's reads run under. Several of them go out
// in one batch, so the cancel is not here: canceling per command would end the
// reads issued beside it.
func (m *Model) readsForTab() context.Context {
	if m.tabCtx == nil {
		m.startTabReads()
	}
	return m.tabCtx
}

// startTabReads ends the reads the previous tab asked for. What makes a read
// unwanted is the tab changing, which is why the decision sits at the tab
// change rather than at each command.
func (m *Model) startTabReads() {
	if m.tabCancel != nil {
		m.tabCancel()
	}
	m.tabCtx, m.tabCancel = context.WithCancel(m.rootContext())
}

// startDiffRead ends the read for the diff that was open and returns the
// context for the new one. It is separate from the tab's because opening a file
// does not make the list stale. Moving the cursor down five rows started five
// reads, and all five ran to the end.
//
// Only opening another diff calls this. One diff is read by more than one
// command — the body and the block hashes behind the read mark — and starting a
// scope for the second ended the first.
func (m *Model) startDiffRead() context.Context {
	if m.diffCancel != nil {
		m.diffCancel()
	}
	m.diffCtx, m.diffCancel = context.WithCancel(m.rootContext())
	return m.diffCtx
}

// readsForDiff is the context the diff on screen is being read under. Re-reading
// the same diff after a reload belongs to the read already in flight, not to a
// new one.
func (m *Model) readsForDiff() context.Context {
	if m.diffCtx == nil {
		return m.startDiffRead()
	}
	return m.diffCtx
}
