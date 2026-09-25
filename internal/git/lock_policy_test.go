package git

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

// readOnlySubcommands name git commands that change neither the index, the
// refs, nor the object store. Reaching one through a write wrapper takes the
// index lock and blocks an agent writing to the same repository, which is
// exactly what runRead exists to avoid.
var readOnlySubcommands = map[string]bool{
	"cat-file":      true,
	"diff":          true,
	"diff-tree":     true,
	"for-each-ref":  true,
	"log":           true,
	"ls-files":      true,
	"ls-tree":       true,
	"merge-tree":    true,
	"name-rev":      true,
	"rev-list":      true,
	"rev-parse":     true,
	"show":          true,
	"stash list":    true,
	"stash show":    true,
	"status":        true,
	"symbolic-ref":  true,
	"var":           true,
	"verify-pack":   true,
	"check-ignore":  true,
	"check-attr":    true,
	"config --get":  true,
	"worktree list": true,
}

var writeWrappers = map[string]bool{"runWrite": true, "runWritePaths": true}

// A short unmarked wrapper named run used to sit next to runRead, and six
// read-only commands were written through it, including hasHEAD — which the
// five-second poll reaches on every tick through Log. Naming both wrappers for
// what they do is what keeps the policy from drifting again.
func TestNoReadOnlyCommandTakesTheIndexLock(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	names, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range names {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			fn, ok := call.Fun.(*ast.Ident)
			if !ok || !writeWrappers[fn.Name] {
				return true
			}
			sub := leadingLiterals(call.Args)
			for prefix := range readOnlySubcommands {
				if sub == prefix || strings.HasPrefix(sub, prefix+" ") {
					t.Errorf("%s: 読み取り専用の %q を %s で呼んでいる。runRead を使う",
						fset.Position(call.Pos()), sub, fn.Name)
				}
			}
			return true
		})
	}
}

// leadingLiterals joins the run of string literals a call passes, skipping the
// directory and any path list ahead of them. "stash" alone does not say whether
// the run writes, so the second word has to be read too.
func leadingLiterals(args []ast.Expr) string {
	var words []string
	for _, a := range args {
		lit, ok := a.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			if len(words) > 0 {
				break
			}
			continue
		}
		v, err := strconv.Unquote(lit.Value)
		if err != nil {
			break
		}
		words = append(words, v)
	}
	return strings.Join(words, " ")
}
