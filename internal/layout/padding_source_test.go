package layout

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fmt pads a string to a count of runes. Every field this package draws is
// measured in cells, and the two agree only while the text is ASCII — which the
// keys for fold and unfold (← and →) are not, nor is a Japanese branch name,
// nor is the mark that says text was dropped. Four fields were padded this way
// and all four were wrong.
//
// Renderer.padTo is what pads by cells. This reads the package rather than the
// drawing because a field padded wrongly still comes out the right width once
// Pad measures the row: the row is fine and the column beside it has moved, and
// no sweep over frames can see that on its own.
//
// What it does not catch: padding written some other way — a Repeat of spaces
// counted by len, say. It catches the one form that was there four times.
func TestNothingPadsAFieldWithFmt(t *testing.T) {
	t.Parallel()
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			if padsToAWidth(lit.Value) {
				t.Errorf("%s pads with fmt: %s — use Renderer.padTo, which counts cells",
					name, lit.Value)
			}
			return true
		})
	}
}

// padsToAWidth answers whether a format string asks fmt to pad a string to a
// width. %5s and %-4s both do; %s and %d do not.
func padsToAWidth(quoted string) bool {
	for i := 0; i+1 < len(quoted); i++ {
		if quoted[i] != '%' {
			continue
		}
		j := i + 1
		if j < len(quoted) && quoted[j] == '-' {
			j++
		}
		start := j
		for j < len(quoted) && (quoted[j] >= '0' && quoted[j] <= '9' || quoted[j] == '*') {
			j++
		}
		if j > start && j < len(quoted) && quoted[j] == 's' {
			return true
		}
	}
	return false
}
