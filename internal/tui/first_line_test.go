package tui

import (
	"testing"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/catpotd/mirugit/internal/state"
)

// The notice row holds one line, and this is what cuts git's message down to
// it. A message that opens with a newline has an empty first line, and that is
// the line: taking the whole of it instead puts the rest of the message on a
// row one line tall, and everything under the row shifts.
func TestTheFirstLineIsWhatComesBeforeTheFirstNewline(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, in, want string
	}{
		{"one line", "fatal: refused", "fatal: refused"},
		{"two lines", "fatal: refused\nhint: try again", "fatal: refused"},
		{"a message that opens with a newline", "\nfatal: refused", ""},
		{"a message that is only a newline", "\n", ""},
		{"a trailing newline", "fatal: refused\n", "fatal: refused"},
		{"nothing at all", "", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := firstLine(c.in); got != c.want {
				t.Errorf("firstLine(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// A key that carries no text of its own is added to the commit message by its
// code. The code has to be a character the message can hold: below space are
// the control codes, and at or above the last code point there is no character
// a terminal sends as a keypress.
func TestOnlyACharacterGoesIntoTheCommitMessage(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name  string
		code  rune
		typed bool
	}{
		{"a letter", 'a', true},
		{"a space", ' ', true},
		{"one below the last code point", utf8.MaxRune - 1, true},
		{"the last code point", utf8.MaxRune, false},
		{"a control code", 7, false},
		{"one below space", 31, false},
		{"past the last code point", utf8.MaxRune + 1, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := fixed(t)
			before := m.state.Changes.Message

			next, _ := m.messageKey(tea.KeyPressMsg{Code: c.code})
			m = next.(*Model)

			if got := m.state.Changes.Message != before; got != c.typed {
				t.Errorf("the message changed = %v, want %v: %q → %q",
					got, c.typed, before, m.state.Changes.Message)
			}
		})
	}
}

// Backspace removes the last character of the commit message, and a character
// can be more than one byte: cutting a byte off "変更" leaves a byte sequence
// no terminal can draw. An empty message has nothing to remove.
func TestBackspaceRemovesTheLastCharacterOfTheCommitMessage(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, message, want string
	}{
		{"a letter", "fix", "fi"},
		{"a character of three bytes", "変更", "変"},
		{"the last character", "a", ""},
		{"nothing to remove", "", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := fixed(t)
			m.state = state.Apply(m.state, state.MessageEdited{Text: c.message})

			next, _ := m.messageKey(tea.KeyPressMsg{Code: tea.KeyBackspace})
			if got := next.(*Model).state.Changes.Message; got != c.want {
				t.Errorf("backspace on %q left %q, want %q", c.message, got, c.want)
			}
		})
	}
}

// The digits pick a tab by their distance from 1. A digit past the last tab
// names no tab: read as one, it indexes the table of tabs past its end.
func TestOnlyADigitThatNamesATabPicksOne(t *testing.T) {
	t.Parallel()
	for code := '0'; code <= '9'; code++ {
		tab, ok := tabFromDigit(code)
		want := code >= '1' && int(code-'1') < int(state.TabCount)
		if ok != want {
			t.Errorf("%q names a tab = %v, want %v", code, ok, want)
		}
		if ok && int(tab) >= int(state.TabCount) {
			t.Errorf("%q names tab %d, and there are %d tabs", code, tab, state.TabCount)
		}
	}
}
