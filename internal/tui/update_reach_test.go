package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"
	"testing"
)

// This package "issues a command for every repository call", which its own doc
// comment says. A call written straight into Update runs on the goroutine that
// draws, so the pane stops answering for as long as git takes. One had been
// there unnoticed: the reload handler refreshed the read marks' block hashes in
// place, and that is two git diffs on every poll.
//
// The rule is checked rather than stated, because nothing about a synchronous
// call looks wrong at the call site.
//
// Commands are closures — the body of a func literal runs later, on a goroutine
// of bubbletea's. So the walk stops at every func literal. A closure called
// where it is written would pass this check; none is written that way here, and
// the shape to watch for is a call the reader can see returning a tea.Cmd.
func TestUpdateReachesNoRepositoryCall(t *testing.T) {
	t.Parallel()
	// The two functions outside internal/git that reach git. They are named
	// because they live in internal/state, whose other functions are pure, so
	// no package name separates them. internal/state's own doc comment says
	// these are the exception.
	repositoryCallsElsewhere := map[string]string{
		"LoadRead":        "prunes the read marks against the paths git reports",
		"ReadBlockHashes": "runs git diff on both sides",
	}
	calls, reachesGit := callGraphOfThisPackage(t, repositoryCallsElsewhere)

	const entry = "Model.Update"
	if _, ok := calls[entry]; !ok {
		t.Fatalf("no %s in this package, so this test is watching nothing", entry)
	}
	if len(reachesGit) == 0 {
		t.Fatal("nothing here reaches the repository, so this test is watching nothing")
	}
	if path := pathToRepository(entry, calls, reachesGit); path != nil {
		t.Errorf("%s reaches the repository on the update goroutine: %s",
			entry, strings.Join(path, " → "))
	}
}

func pathToRepository(name string, calls map[string][]string, reaches map[string]bool) []string {
	type step struct {
		name string
		path []string
	}
	seen := map[string]bool{name: true}
	queue := []step{{name, []string{name}}}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if reaches[cur.name] {
			return cur.path
		}
		for _, callee := range calls[cur.name] {
			if seen[callee] {
				continue
			}
			seen[callee] = true
			queue = append(queue, step{callee, append(append([]string{}, cur.path...), callee)})
		}
	}
	return nil
}

// callGraphOfThisPackage reads the production files only. A test helper may call
// git where it likes; the rule is about what the update loop does.
func callGraphOfThisPackage(t *testing.T, elsewhere map[string]string) (
	calls map[string][]string, reachesGit map[string]bool) {
	t.Helper()
	calls = map[string][]string{}
	reachesGit = map[string]bool{}
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			key := funcKeyOf(fn)
			walkOutsideClosures(fn.Body, func(call *ast.CallExpr) {
				switch fun := call.Fun.(type) {
				case *ast.Ident:
					calls[key] = append(calls[key], fun.Name)
				case *ast.SelectorExpr:
					if pkg, ok := fun.X.(*ast.Ident); ok && pkg.Name == "git" {
						reachesGit[key] = true
					}
					if _, named := elsewhere[fun.Sel.Name]; named {
						reachesGit[key] = true
					}
					calls[key] = append(calls[key], fun.Sel.Name)
					calls[key] = append(calls[key], funcKeyOfSelector(fun))
				}
			})
			if _, ok := calls[key]; !ok {
				calls[key] = nil
			}
		}
	}
	sortEach(calls)
	return calls, reachesGit
}

// walkOutsideClosures visits every call except the ones inside a func literal.
func walkOutsideClosures(body ast.Node, visit func(*ast.CallExpr)) {
	ast.Inspect(body, func(n ast.Node) bool {
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		if call, ok := n.(*ast.CallExpr); ok {
			visit(call)
		}
		return true
	})
}

// funcKeyOfSelector names m.handleRepoMsg as Model.handleRepoMsg, which is how
// the method is declared. Without it a method call reaches nothing and the walk
// stops at the first handler.
func funcKeyOfSelector(sel *ast.SelectorExpr) string {
	if id, ok := sel.X.(*ast.Ident); ok && id.Name == "m" {
		return "Model." + sel.Sel.Name
	}
	return sel.Sel.Name
}

func funcKeyOf(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	t := fn.Recv.List[0].Type
	if star, ok := t.(*ast.StarExpr); ok {
		t = star.X
	}
	if id, ok := t.(*ast.Ident); ok {
		return id.Name + "." + fn.Name.Name
	}
	return fn.Name.Name
}

func sortEach(calls map[string][]string) {
	for name := range calls {
		sort.Strings(calls[name])
	}
}
