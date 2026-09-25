package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// probeGlyph is East Asian Ambiguous; the terminal decides one or two columns.
const probeGlyph = "·"

// widthProbeTimeout caps how long startup waits for a silent terminal. Five
// hundred milliseconds is noticeable but beats drawing the whole pane twice.
const widthProbeTimeout = 500 * time.Millisecond

// widthProbe holds the measurement in flight. The terminal is asked rather
// than inferred because a locale variable can disagree with the terminal that
// is actually drawing.
type widthProbe struct {
	before  int
	settled bool
	replied bool
}

type widthProbeTimeoutMsg struct{}

type widthProbeAfterDrawMsg struct{}

func beginWidthProbe() tea.Cmd {
	return tea.Batch(requestCursorPosition(), widthProbeDeadline())
}

func requestCursorPosition() tea.Cmd {
	return tea.RequestCursorPosition
}

func widthProbeDeadline() tea.Cmd {
	return tea.Tick(widthProbeTimeout, func(t time.Time) tea.Msg {
		return widthProbeTimeoutMsg{}
	})
}

func afterProbeDraw() tea.Cmd {
	return func() tea.Msg {
		return widthProbeAfterDrawMsg{}
	}
}

func (m *Model) handleCursorPosition(msg tea.CursorPositionMsg) (tea.Model, tea.Cmd) {
	if m.probe.settled {
		return m, nil
	}
	if !m.probe.replied {
		m.probe.before = msg.X
		m.probe.replied = true
		return m, afterProbeDraw()
	}
	m.render.EastAsian = msg.X-m.probe.before == 2
	m.render.IsWidthAssumed = false
	m.probe.settled = true
	return m, nil
}

func (m *Model) handleWidthProbeTimeout() (tea.Model, tea.Cmd) {
	if m.probe.settled {
		return m, nil
	}
	m.render.EastAsian = false
	m.render.IsWidthAssumed = true
	m.probe.settled = true
	return m, nil
}

func (m *Model) probeViewContent() string {
	if m.probe.replied && !m.probe.settled {
		return probeGlyph
	}
	return ""
}
