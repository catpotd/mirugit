package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A tea.Cmd runs on a goroutine of its own while the update loop keeps going.
// The Model belongs to the update loop: a command that reaches into it reads
// fields another goroutine is writing, and the race detector only says so when
// the two happen to overlap.
//
// The rule is not about the signature. Functions that take *Model and return a
// tea.Cmd are correct and there are dozens: they run on the update goroutine
// and only the closure they return runs elsewhere. What has to hold is that the
// closure captures values, not the Model.
//
// saveRead (commands.go) is the shape: it takes the snapshot first and the
// closure holds only the bytes and the path.
func TestNoCommandReachesIntoTheModel(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	for _, file := range goFilesHere(t) {
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(parsed, func(n ast.Node) bool {
			ret, ok := n.(*ast.ReturnStmt)
			if !ok {
				return true
			}
			for _, result := range ret.Results {
				lit, ok := result.(*ast.FuncLit)
				if !ok {
					continue
				}
				if !returnsTeaMsg(lit) {
					continue
				}
				for _, name := range modelNamesIn(lit) {
					t.Errorf("%s: a command's body names %q; the update loop owns "+
						"the model, so a command has to close over values instead",
						fset.Position(lit.Pos()), name)
				}
			}
			return true
		})
	}
}

// returnsTeaMsg reports a literal shaped like a tea.Cmd: no arguments, one
// tea.Msg result.
func returnsTeaMsg(lit *ast.FuncLit) bool {
	if lit.Type.Params != nil && len(lit.Type.Params.List) > 0 {
		return false
	}
	results := lit.Type.Results
	if results == nil || len(results.List) != 1 {
		return false
	}
	sel, ok := results.List[0].Type.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "tea" && sel.Sel.Name == "Msg"
}

// modelNamesIn collects the identifiers in the literal's body that name the
// model. The receiver is m by convention throughout this package; a command
// that names it is reaching into state the update loop owns.
func modelNamesIn(lit *ast.FuncLit) []string {
	var found []string
	seen := map[string]bool{}
	ast.Inspect(lit.Body, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := sel.X.(*ast.Ident)
		if !ok || ident.Name != "m" || seen[ident.Name+"."+sel.Sel.Name] {
			return true
		}
		seen[ident.Name+"."+sel.Sel.Name] = true
		found = append(found, ident.Name+"."+sel.Sel.Name)
		return true
	})
	return found
}

func goFilesHere(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		files = append(files, filepath.Join(".", name))
	}
	return files
}
