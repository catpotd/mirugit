package git

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"
	"testing"
)

// This package holds two kinds of function: ones that turn git's bytes into
// values, and ones that start git. Splitting them into two packages was
// proposed and turned down — the two kinds share Entry, CommitInfo and
// FileDiff, so whichever package the types went to, the other would import it —
// and the reason was written in a plan document that has since been deleted.
// Without it, the next reader of these eighty-odd symbols reaches the same
// proposal and does the same work again.
//
// So the reason lives here instead, as the guarantee the split was wanted for:
// a fuzz target feeds arbitrary bytes to a function, and none of the functions
// it can reach may start a process. The package boundary would have been the
// compiler's way of saying that. This is the same sentence, checked.
//
// It walks the call graph rather than naming the parsing functions, because a
// list of names has to be kept and a function added next to one of them is not
// on it.
func TestNoFuzzTargetCanReachAProcess(t *testing.T) {
	t.Parallel()
	calls, startsProcess := callGraphHere(t)

	var targets []string
	for name := range calls {
		if strings.HasPrefix(name, "Fuzz") {
			targets = append(targets, name)
		}
	}
	sort.Strings(targets)
	if len(targets) == 0 {
		t.Fatal("no fuzz targets found; this test is watching nothing")
	}

	for _, target := range targets {
		if path := pathToProcess(target, calls, startsProcess); path != nil {
			t.Errorf("%s reaches a process: %s", target, strings.Join(path, " → "))
		}
	}
}

// pathToProcess returns the chain from name to the first function that starts a
// process, or nil. The chain rather than the name alone, because "parseDiff
// reaches a process" leaves the reader to find out how.
func pathToProcess(name string, calls map[string][]string, startsProcess map[string]bool) []string {
	type step struct {
		name string
		path []string
	}
	seen := map[string]bool{name: true}
	queue := []step{{name, []string{name}}}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if startsProcess[cur.name] {
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

// callGraphHere reads every file of this package, test files included: the fuzz
// targets are in one. Build tags are ignored, so a function written once per
// platform contributes both bodies. Every such approximation adds edges, and an
// edge that is not really there can only make this test complain about a path
// that is safe — never let a real one through.
func callGraphHere(t *testing.T) (calls map[string][]string, startsProcess map[string]bool) {
	t.Helper()
	calls = map[string][]string{}
	startsProcess = map[string]bool{}
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		f, err := parser.ParseFile(fset, entry.Name(), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			name := funcKey(fn)
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				switch fun := call.Fun.(type) {
				case *ast.Ident:
					calls[name] = append(calls[name], fun.Name)
				case *ast.SelectorExpr:
					// os/exec is how a process starts. Naming the package
					// rather than this program's wrappers keeps the check
					// working when a wrapper is renamed or a new one is added.
					if pkg, ok := fun.X.(*ast.Ident); ok && pkg.Name == "exec" {
						startsProcess[name] = true
					}
					calls[name] = append(calls[name], fun.Sel.Name)
				}
				return true
			})
			if _, ok := calls[name]; !ok {
				calls[name] = nil
			}
		}
	}
	return calls, startsProcess
}

// funcKey names a method by its type as well, because two types here each have
// an Error method and merging them would report a path no call makes. Plain
// functions keep their bare name: the pair that repeats it is one function
// written twice behind build tags, and merging those two adds edges rather than
// hiding any.
func funcKey(fn *ast.FuncDecl) string {
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
