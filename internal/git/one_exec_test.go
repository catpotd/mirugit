package git

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// Starting git in the wrong directory is the worst thing this package can do:
// exec with an empty Dir runs in the process's own working directory, so
// git clean -f there deletes untracked files in whatever repository the binary
// was started in. execGit turns that away.
//
// execGitIn starts the process and does not check, because TopLevel and IsBare
// have no directory to run in yet and ask git where the repository is. Every
// other caller goes through execGit, which checks first. A function added later
// that calls execGitIn, or reaches os/exec on its own, would be past the check
// with nothing to say so.
func TestOnlyTheNamedFunctionsStartGitWithoutTheDirectoryCheck(t *testing.T) {
	t.Parallel()
	mayCallUnchecked := map[string]string{
		"runNetworkWithin": "checks dir itself before it calls",
		"execGit":          "is the check, and calls this to run what it passed",
		"TopLevel":         "has no directory to run in yet and asks git for one",
		"IsBare":           "asks the same question about the same directory",
	}
	const startsTheProcess = "execGitIn"

	callers := whoCalls(t, startsTheProcess)
	if len(callers) == 0 {
		t.Fatalf("nothing calls %s, so this checks nothing", startsTheProcess)
	}
	for name := range callers {
		if _, ok := mayCallUnchecked[name]; !ok {
			t.Errorf("%s calls %s, which starts git without checking the repository "+
				"directory. Call execGit, or add %s to mayCallUnchecked saying what "+
				"it checks instead", name, startsTheProcess, name)
		}
	}

	// A name here that nothing calls is a permission for a caller that has gone,
	// and the entry stays as cover for whatever is written next under that name.
	for name, why := range mayCallUnchecked {
		if callers[name] == 0 {
			t.Errorf("mayCallUnchecked names %s (%s) and nothing by that name calls %s",
				name, why, startsTheProcess)
		}
	}

	execs := whoReachesOsExec(t)
	if execs[startsTheProcess] == 0 {
		t.Errorf("%s does not start a process, so this checks nothing", startsTheProcess)
	}
	for name := range execs {
		if name != startsTheProcess {
			t.Errorf("%s starts a process of its own, past every check in this package", name)
		}
	}
}

func whoCalls(t *testing.T, callee string) map[string]int {
	t.Helper()
	return walkFuncs(t, func(fn *ast.FuncDecl, found map[string]int) {
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if id, ok := call.Fun.(*ast.Ident); ok && id.Name == callee {
				found[fn.Name.Name]++
			}
			return true
		})
	})
}

func whoReachesOsExec(t *testing.T) map[string]int {
	t.Helper()
	return walkFuncs(t, func(fn *ast.FuncDecl, found map[string]int) {
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if ok && pkg.Name == "exec" && strings.HasPrefix(sel.Sel.Name, "Command") {
				found[fn.Name.Name]++
			}
			return true
		})
	})
}

func walkFuncs(t *testing.T, look func(*ast.FuncDecl, map[string]int)) map[string]int {
	t.Helper()
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	found, read := map[string]int{}, 0
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		read++
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil {
				look(fn, found)
			}
		}
	}
	if read == 0 {
		t.Fatal("no source file was read, so this checks nothing")
	}
	return found
}
