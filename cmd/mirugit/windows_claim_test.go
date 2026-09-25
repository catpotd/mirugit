package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The README said the build does not compile on Windows. It does: make cross
// builds it, and the refusal is at startup. A README that describes a failure
// the program does not have sends a reader looking for the wrong thing.
func TestTheReadmeSaysWhatWindowsActuallyDoes(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	readme := string(raw)
	if !strings.Contains(readme, "Windows is not supported") {
		t.Fatal("the README does not say Windows is unsupported")
	}
	if strings.Contains(readme, "does not compile there") {
		t.Error("the README says the build fails on Windows; make cross builds it")
	}
	if !strings.Contains(readme, "refuses to run") {
		t.Error("the README does not say the program builds and refuses at startup")
	}
}
