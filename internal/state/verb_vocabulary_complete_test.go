package state

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// AllVerbNames says it is the whole vocabulary, and three sweeps in two other
// packages check their tables against it rather than against each other. A name
// declared without being added here is in none of them: the sweeps still pass,
// with one fewer name in them and nothing to say which.
func TestAllVerbNamesHoldsEveryNameThatIsDeclared(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "verbname.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	declared, listed := map[string]bool{}, map[string]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		for i, name := range spec.Names {
			if i >= len(spec.Values) {
				continue
			}
			v, ok := spec.Values[i].(*ast.CompositeLit)
			if !ok {
				continue
			}
			if id, ok := v.Type.(*ast.Ident); ok && id.Name == "VerbName" {
				declared[name.Name] = true
			}
			if name.Name == "AllVerbNames" {
				for _, elt := range v.Elts {
					if id, ok := elt.(*ast.Ident); ok {
						listed[id.Name] = true
					}
				}
			}
		}
		return true
	})

	if len(declared) == 0 || len(listed) == 0 {
		t.Fatalf("read %d declared names and %d listed, so this checks nothing",
			len(declared), len(listed))
	}
	for name := range declared {
		if !listed[name] {
			t.Errorf("%s is declared and is not in AllVerbNames, so the sweeps that "+
				"read it never see that name", name)
		}
	}
	for name := range listed {
		if !declared[name] {
			t.Errorf("AllVerbNames holds %s and verbname.go declares no such name", name)
		}
	}
	if len(AllVerbNames) != len(listed) {
		t.Errorf("AllVerbNames has %d entries and %d distinct names, so one is in it twice",
			len(AllVerbNames), len(listed))
	}
	for _, v := range AllVerbNames {
		if v.IsZero() {
			t.Error("AllVerbNames holds a name with no word in it")
		}
	}
}
