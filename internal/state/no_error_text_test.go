package state_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This package decided what a failure said by taking Error() apart. Error() is
// written for a person reading a log; its shape is not a promise, and the layer
// furthest from the screen is the wrong place to decide what the screen says.
//
// The conversion lives in tui now, where errors.As can ask the type. This keeps
// it from coming back: the check is on the source rather than on behavior,
// because the behavior would only differ once someone had already written it.
func TestThisPackageDoesNotReadTheTextOfAnError(t *testing.T) {
	t.Parallel()
	for _, file := range goFilesIn(t, ".") {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(body), "\n") {
			if strings.Contains(line, ".Error()") {
				t.Errorf("%s:%d reads the text of an error; the type decides the "+
					"sentence, and the types belong to the packages that make them:\n\t%s",
					filepath.Base(file), i+1, strings.TrimSpace(line))
			}
		}
	}
}
