package state_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// CONTRIBUTING.md states these three. A rule a document states and no test
// checks holds until the first person who has not read the document, and every
// one of these was true by hand this morning.

const modulePath = "github.com/catpotd/mirugit/"

// layerRank is the order imports may point in: git and update ← state ← layout
// ← tui. A package may import one to its left and nothing to its right.
var layerRank = map[string]int{
	"git": 0, "osproc": 0, "update": 0, "state": 1, "layout": 2, "tui": 3,
}

func TestImportsPointOneWay(t *testing.T) {
	t.Parallel()
	for pkg, rank := range layerRank {
		for _, file := range goFilesIn(t, filepath.Join(root(), "internal", pkg)) {
			for _, imp := range importsOf(t, file) {
				dep, ok := strings.CutPrefix(imp, modulePath+"internal/")
				if !ok {
					continue
				}
				depRank, known := layerRank[dep]
				if !known {
					t.Errorf("%s imports internal/%s, which is in no layer", pkg, dep)
					continue
				}
				if depRank >= rank && dep != pkg {
					t.Errorf("%s imports %s: imports may only point left in git ← state ← layout ← tui",
						pkg, dep)
				}
			}
		}
	}
}

// stateWrite matches an assignment straight into the model's state. Apply is
// the only place state changes, and seventeen of these were once written
// around it.
var stateWrite = regexp.MustCompile(`^\s+m\.state\.[A-Z][A-Za-z.]*\s*=[^=]`)

func TestOnlyApplyChangesState(t *testing.T) {
	t.Parallel()
	for _, file := range goFilesIn(t, filepath.Join(root(), "internal", "tui")) {
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(body), "\n") {
			if stateWrite.MatchString(line) {
				t.Errorf("%s:%d writes state directly; add an Event and go through Apply\n\t%s",
					filepath.Base(file), i+1, strings.TrimSpace(line))
			}
		}
	}
}

// rowBound matches a hand-written row-index check, in either spelling. Twenty-
// three of these existed and fourteen could be broken without a test noticing,
// so state.RowAt holds the one copy — but this pattern only looked for the
// reject spelling, and four checks written the other way round survived the
// change that was supposed to remove them.
var rowBound = regexp.MustCompile(`<\s*0\s*\|\|.*>=\s*len\(|>=\s*0\s*&&.*<\s*len\(`)

func TestRowIndexIsCheckedInOnePlace(t *testing.T) {
	t.Parallel()
	for _, pkg := range []string{"tui", "layout"} {
		for _, file := range goFilesIn(t, filepath.Join(root(), "internal", pkg)) {
			body, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			for i, line := range strings.Split(string(body), "\n") {
				if rowBound.MatchString(line) && strings.Contains(line, "Rows") {
					t.Errorf("%s:%d checks a row index by hand; use state.RowAt or state.CursorRow\n\t%s",
						filepath.Base(file), i+1, strings.TrimSpace(line))
				}
			}
		}
	}
}

func root() string { return filepath.Join("..", "..") }

func goFilesIn(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		out = append(out, filepath.Join(dir, name))
	}
	return out
}

func importsOf(t *testing.T, file string) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(f.Imports))
	for _, spec := range f.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, path)
	}
	return out
}

// rowsWrite matches a write into the drawn list. Rows is what the payloads
// project to rather than state the program keeps, and eleven places used to
// rebuild it, each settling the cursor its own way. refreshRows holds the one
// copy, and the field the table grew to describe one of those differences was
// read by nothing.
var rowsWrite = regexp.MustCompile(`^\s+s\.Rows\s*=[^=]`)

func TestTheDrawnListIsRebuiltInOnePlace(t *testing.T) {
	t.Parallel()
	for _, file := range goFilesIn(t, filepath.Join(root(), "internal", "state")) {
		if filepath.Base(file) == "rows.go" {
			continue
		}
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(body), "\n") {
			if rowsWrite.MatchString(line) {
				t.Errorf("%s:%d rebuilds the drawn list; call refreshRows\n\t%s",
					filepath.Base(file), i+1, strings.TrimSpace(line))
			}
		}
	}
}
