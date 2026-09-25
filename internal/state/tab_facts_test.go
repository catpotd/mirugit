package state_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/catpotd/mirugit/internal/state"
)

func TestTabNameLiteralsLiveOnlyInTabFacts(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
	pkgs := []string{
		filepath.Join(root, "internal", "state"),
		filepath.Join(root, "internal", "layout"),
		filepath.Join(root, "internal", "tui"),
	}
	want := map[string]bool{"history": true, "stashed": true, "worktrees": true}
	fset := token.NewFileSet()
	for _, dir := range pkgs {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			if filepath.Base(path) == "tab_facts.go" {
				return nil
			}
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			ast.Inspect(file, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				val, err := strconvUnquote(lit.Value)
				if err != nil || !want[val] {
					return true
				}
				t.Errorf("%s:%d: tab name literal %q must live only in tab_facts.go", path, fset.Position(lit.Pos()).Line, val)
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func strconvUnquote(s string) (string, error) {
	if len(s) < 2 {
		return "", os.ErrInvalid
	}
	return s[1 : len(s)-1], nil
}

// TabFacts のゼロ値は、そのタブの本当の答えとして読まれる。フィールドを足して
// 書き忘れると、false や 0 が「そう決めた」ことになって黙って通る。
//
// 行はコンストラクタで作られるようになったので、検査対象は
// changesFacts と parentFacts が返すリテラルの2つ。どちらも全フィールドを
// 名指ししていれば、どのタブにもゼロ値は入らない。
func TestEveryTabAnswersEveryFact(t *testing.T) {
	t.Parallel()
	typ := reflect.TypeOf(state.TabFacts{})
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "tab_facts.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, fn := range []string{"changesFacts", "parentFacts"} {
		lit := returnedFactsLiteral(t, file, fn)
		written := map[string]bool{}
		for _, v := range lit.Elts {
			kv, keyed := v.(*ast.KeyValueExpr)
			if !keyed {
				t.Fatalf("%s has a positional value; write the field name", fn)
			}
			written[kv.Key.(*ast.Ident).Name] = true
		}
		for f := 0; f < typ.NumField(); f++ {
			if name := typ.Field(f).Name; !written[name] {
				t.Errorf("%s does not answer %s", fn, name)
			}
		}
	}
}

// Every field of parentTabFacts has to reach the TabFacts it builds. One that
// does not is a value a tab writes and nobody reads, which is the shape the
// dead RestoreCursorByKey entry had.
func TestParentFactsSpendsEveryParameter(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "tab_facts.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	lit := returnedFactsLiteral(t, file, "parentFacts")
	used := map[string]bool{}
	for _, v := range lit.Elts {
		ast.Inspect(v, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "p" {
				used[sel.Sel.Name] = true
			}
			return true
		})
	}
	for _, name := range structFieldNames(t, file, "parentTabFacts") {
		if !used[name] {
			t.Errorf("parentFacts never reads %s, so every tab writes it for nothing", name)
		}
	}
}

// The parameter struct is unexported, so its fields are read from the source
// rather than through reflection.
func structFieldNames(t *testing.T, file *ast.File, name string) []string {
	t.Helper()
	var out []string
	ast.Inspect(file, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok || ts.Name.Name != name {
			return true
		}
		st, ok := ts.Type.(*ast.StructType)
		if !ok {
			return false
		}
		for _, f := range st.Fields.List {
			for _, id := range f.Names {
				out = append(out, id.Name)
			}
		}
		return false
	})
	if len(out) == 0 {
		t.Fatalf("%s has no fields, so this checks nothing", name)
	}
	return out
}

func returnedFactsLiteral(t *testing.T, file *ast.File, fn string) *ast.CompositeLit {
	t.Helper()
	var lit *ast.CompositeLit
	ast.Inspect(file, func(n ast.Node) bool {
		decl, ok := n.(*ast.FuncDecl)
		if !ok || decl.Name.Name != fn || decl.Body == nil {
			return true
		}
		for _, stmt := range decl.Body.List {
			ret, ok := stmt.(*ast.ReturnStmt)
			if !ok || len(ret.Results) != 1 {
				continue
			}
			if cl, ok := ret.Results[0].(*ast.CompositeLit); ok {
				lit = cl
				return false
			}
		}
		return false
	})
	if lit == nil {
		t.Fatalf("%s does not return a TabFacts literal", fn)
	}
	return lit
}

// A nil function in Facts is the same silent hole a false was: the tab draws
// nothing and nothing says why. The literal test above only checks that the
// field is written, not that it holds something callable.
func TestEveryTabHasItsFunctions(t *testing.T) {
	t.Parallel()
	for i := range int(state.TabCount) {
		f := state.Facts[state.Tab(i)]
		if f.Rows == nil {
			t.Errorf("Facts[%s].Rows is nil", f.Name)
		}
	}
}
