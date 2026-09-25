package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// A package variable holding a function is a seam every test in the package
// shares. Swapping it in one test swaps it for the tests running beside it: the
// race detector caught clipboardCommand being written by one parallel test
// while another read it, and in between, a copy that was supposed to reach the
// clipboard ran the stub instead.
//
// A seam belongs to the Model, which one test owns, or to the call, which the
// caller hands in. m.delay and m.clipboardCommand are both.
func TestNoPackageVariableHoldsAFunction(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	for _, file := range goFilesHere(t) {
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range parsed.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok || !holdsAFunction(value) {
					continue
				}
				for _, name := range value.Names {
					t.Errorf("%s: the package variable %q holds a function; a test "+
						"that swaps it swaps it for every test beside it, so put it "+
						"on the Model or pass it in",
						fset.Position(value.Pos()), name.Name)
				}
			}
		}
	}
}

// holdsAFunction reports a var whose type or value is a function: either
// written out as one, or assigned from a name that is one.
func holdsAFunction(value *ast.ValueSpec) bool {
	if _, ok := value.Type.(*ast.FuncType); ok {
		return true
	}
	for _, v := range value.Values {
		switch v.(type) {
		case *ast.FuncLit, *ast.Ident, *ast.SelectorExpr:
			return value.Type == nil
		}
	}
	return false
}
