package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// Every message a command answers with can carry a repository error, and
// updateResult reports it before the handler runs. A message type that gains an
// error field without the method drops back to the old shape: its handler sees
// a message that failed and applies it as if it had not.
func TestEveryMessageThatCanFailSaysSo(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "messages.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	carries := map[string]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}
		st, ok := ts.Type.(*ast.StructType)
		if !ok {
			return true
		}
		for _, f := range st.Fields.List {
			ident, ok := f.Type.(*ast.Ident)
			if !ok || ident.Name != "error" {
				continue
			}
			carries[ts.Name.Name] = true
		}
		return true
	})
	if len(carries) == 0 {
		t.Fatal("no message carries an error, so this checks nothing")
	}

	says := map[string]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "failed" || fn.Recv == nil || len(fn.Recv.List) != 1 {
			return true
		}
		if ident, ok := fn.Recv.List[0].Type.(*ast.Ident); ok {
			says[ident.Name] = true
		}
		return true
	})

	for name := range carries {
		if !says[name] {
			t.Errorf("%s carries an error and has no failed method, so updateResult "+
				"hands it to its handler as if the command had worked", name)
		}
	}
}
