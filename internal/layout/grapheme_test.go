package layout

import (
	"strings"
	"testing"

	"github.com/catpotd/mirugit/internal/git"
)

// A rune loop measures each code point of a joined emoji on its own, so a
// two-cell glyph counts as four to eight and the caller cuts the line short or
// splits the cluster in half.
func TestWidthsCountGraphemeClusters(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	for _, path := range []string{
		"emoji-\U0001F468\u200d\U0001F469\u200d\U0001F467\u200d\U0001F466-family.txt",
		"flag-\U0001F1EF\U0001F1F5-jp.txt",
		"skin-\U0001F44D\U0001F3FD-tone.txt",
	} {
		for _, width := range []int{60, 77, 120} {
			line, _ := w.FileRow(Row{
				Entry: git.Entry{Path: path, Worktree: git.Modified},
				State: rowState{Cursor: true},
			}, width)
			if got := w.Of(line); got != width {
				t.Errorf("%q at width %d: %d cells", path, width, got)
			}
		}
	}
}

// A cut that lands inside a joined emoji leaves a trailing zero-width joiner,
// which the terminal draws as a lone person rather than the family.
func TestTruncateKeepsClustersWhole(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	const family = "\U0001F468\u200d\U0001F469\u200d\U0001F467\u200d\U0001F466"
	for _, max := range []int{3, 4, 5, 6, 7} {
		got := w.Truncate("ab"+family+"cd", max)
		if strings.Contains(got, "\u200d") && !strings.Contains(got, family) {
			t.Errorf("Truncate(%d) split the cluster: %q", max, got)
		}
		if cells := w.Of(got); cells > max {
			t.Errorf("Truncate(%d) returned %d cells: %q", max, cells, got)
		}
		front := w.TruncateFront("ab"+family+"cd", max)
		if strings.Contains(front, "\u200d") && !strings.Contains(front, family) {
			t.Errorf("TruncateFront(%d) split the cluster: %q", max, front)
		}
	}
}
