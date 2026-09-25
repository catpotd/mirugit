package state

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestApplyHandlesEveryEventType(t *testing.T) {
	t.Parallel()
	eventTypes := eventReceiverTypes(t)
	handled := handledEventTypes(t)
	for _, name := range eventTypes {
		if !handled[name] {
			t.Errorf("Apply has no case for %s", name)
		}
	}
}

func eventReceiverTypes(t *testing.T) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "event.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || len(fn.Recv.List) != 1 {
			continue
		}
		if fn.Name.Name != "event" {
			continue
		}
		recv := fn.Recv.List[0].Type
		ident, ok := recv.(*ast.Ident)
		if !ok {
			t.Fatalf("unexpected receiver type %T", recv)
		}
		names = append(names, ident.Name)
	}
	return names
}

func handledEventTypes(t *testing.T) map[string]bool {
	t.Helper()
	files := []string{"apply.go", "apply_load.go", "apply_verb.go", "selection.go"}
	switchFuncs := map[string]bool{
		"applyNavigation":    true,
		"applyLoaded":        true,
		"applySelection":     true,
		"applyVerb":          true,
		"applyCommitMessage": true,
	}
	handled := make(map[string]bool)
	fset := token.NewFileSet()
	for _, file := range files {
		f, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !switchFuncs[fn.Name.Name] {
				continue
			}
			for _, name := range typeSwitchCases(fn) {
				handled[name] = true
			}
		}
	}
	return handled
}

func typeSwitchCases(fn *ast.FuncDecl) []string {
	var names []string
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		stmt, ok := n.(*ast.TypeSwitchStmt)
		if !ok {
			return true
		}
		for _, clause := range stmt.Body.List {
			cc, ok := clause.(*ast.CaseClause)
			if !ok || len(cc.List) == 0 {
				continue
			}
			for _, typ := range cc.List {
				if ident, ok := typ.(*ast.Ident); ok {
					names = append(names, ident.Name)
				}
			}
		}
		return false
	})
	return names
}
