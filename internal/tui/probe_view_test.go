package tui

import "testing"

// The probe draws one wide glyph and asks the terminal where the cursor
// landed. Drawing it before the reply, or after the answer is in, would leave
// the glyph on the screen. The glyph is written out rather than read from the
// constant: taking it from there passes whatever the constant says, and a
// glyph the terminal reports as one column measures nothing.
func TestProbeViewDrawsTheGlyphOnlyWhileWaiting(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		replied, settled bool
		want             string
	}{
		{false, false, ""},
		{true, false, "\u00b7"},
		{true, true, ""},
		{false, true, ""},
	} {
		m := &Model{}
		m.probe.replied, m.probe.settled = c.replied, c.settled
		if got := m.probeViewContent(); got != c.want {
			t.Errorf("replied=%v settled=%v: got %q, want %q",
				c.replied, c.settled, got, c.want)
		}
	}
}
