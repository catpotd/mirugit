package layout

import (
	"strings"
	"testing"

	"github.com/catpotd/mirugit/internal/git"
)

func TestTheCommitColumnShowsTheGitAuthor(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	for _, c := range []struct {
		name   string
		author string
		want   string
	}{
		{"no author", "", ""},
		{"an author", "Taro", "Taro"},
		{"a Japanese author", "山田太郎", "山田太郎"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := w.authorColumn(git.CommitInfo{Author: c.author})
			if got != c.want {
				t.Errorf("the column is %q, want %q", got, c.want)
			}
		})
	}
}

func TestALongAuthorIsCut(t *testing.T) {
	t.Parallel()
	w := Renderer{}
	got := w.authorColumn(git.CommitInfo{Author: "Claude Opus 5 (1M context)"})
	if n := w.Of(got); n > authorWidth {
		t.Errorf("the column is %d cells, want at most %d: %q", n, authorWidth, got)
	}
}

func TestEveryCommitRowStartsItsShaInTheSameColumn(t *testing.T) {
	t.Parallel()
	for _, eastAsian := range []bool{false, true} {
		w := Renderer{Widths: Widths{EastAsian: eastAsian}}
		var want int
		for i, author := range []string{
			"",
			"Taro",
			"山田太郎",
			"Claude Opus 5 (1M context)",
		} {
			meta := w.commitMeta(git.CommitInfo{ShortSHA: "abc1234", Age: "2m", Author: author})
			byteAt := strings.Index(meta, "abc1234")
			if byteAt < 0 {
				t.Fatalf("no sha in %q", meta)
			}
			at := w.Of(meta[:byteAt])
			if i == 0 {
				want = at
				continue
			}
			if at != want {
				t.Errorf("eastAsian %v: the sha starts at %d with %q and at %d without: %q", eastAsian, at, author, want, meta)
			}
		}
	}
}

func TestAuthorColumnMakesControlCharactersPrintable(t *testing.T) {
	t.Parallel()
	got := (Renderer{}).authorColumn(git.CommitInfo{Author: "Taro\x1b[31m"})
	if strings.ContainsRune(got, '\x1b') {
		t.Fatalf("column contains an escape character: %q", got)
	}
	if !strings.ContainsRune(got, controlReplacement) {
		t.Fatalf("column = %q, want the control-character replacement", got)
	}
}
