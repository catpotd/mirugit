package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var prePushGitLocalEnvVars = []string{
	"GIT_ALTERNATE_OBJECT_DIRECTORIES",
	"GIT_CONFIG",
	"GIT_CONFIG_PARAMETERS",
	"GIT_CONFIG_COUNT",
	"GIT_OBJECT_DIRECTORY",
	"GIT_DIR",
	"GIT_WORK_TREE",
	"GIT_IMPLICIT_WORK_TREE",
	"GIT_GRAFT_FILE",
	"GIT_INDEX_FILE",
	"GIT_NO_REPLACE_OBJECTS",
	"GIT_REPLACE_REF_BASE",
	"GIT_PREFIX",
	"GIT_SHALLOW_FILE",
	"GIT_COMMON_DIR",
}

func TestPrePushClearsGitLocalEnvironment(t *testing.T) {
	t.Run("clears variables before make", func(t *testing.T) {
		output, makeCalled, err := runPrePushHook(t, false)
		if err != nil {
			t.Fatalf("pre-push hook: %v\n%s", err, output)
		}
		if !makeCalled {
			t.Fatal("pre-push did not run make")
		}
	})

	t.Run("stops when git cannot list variables", func(t *testing.T) {
		output, makeCalled, err := runPrePushHook(t, true)
		if err == nil {
			t.Fatalf("pre-push succeeded after git failed to list variables:\n%s", output)
		}
		if makeCalled {
			t.Fatal("pre-push ran make after git failed to list variables")
		}
	})
}

func runPrePushHook(t *testing.T, gitFails bool) ([]byte, bool, error) {
	t.Helper()

	directory := t.TempDir()
	binDirectory := filepath.Join(directory, "bin")
	if err := os.Mkdir(binDirectory, 0o755); err != nil {
		t.Fatal(err)
	}

	makeCalledFile := filepath.Join(directory, "make-called")
	names := strings.Join(prePushGitLocalEnvVars, " ")
	fakeGit := `#!/bin/sh
if [ "$#" -ne 2 ] || [ "$1" != "rev-parse" ] || [ "$2" != "--local-env-vars" ]; then
	printf 'unexpected git arguments: %s\n' "$*" >&2
	exit 2
fi
if [ "$FAIL_GIT" = 1 ]; then exit 7; fi
printf '%s\n' ` + names + `
`
	fakeMake := `#!/bin/sh
: > "$MAKE_CALLED_FILE"
if [ "$#" -ne 1 ] || [ "$1" != "check" ]; then
	printf 'unexpected make arguments: %s\n' "$*" >&2
	exit 2
fi
for name in ` + names + `; do
	eval "value=\$$name"
	if [ -n "$value" ]; then
		printf '%s remains set\n' "$name" >&2
		exit 1
	fi
done
`

	for name, body := range map[string]string{"git": fakeGit, "make": fakeMake} {
		path := filepath.Join(binDirectory, name)
		if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	environment := []string{
		"PATH=" + binDirectory,
		"MAKE_CALLED_FILE=" + makeCalledFile,
	}
	for _, name := range prePushGitLocalEnvVars {
		environment = append(environment, name+"=sentinel")
	}
	if gitFails {
		environment = append(environment, "FAIL_GIT=1")
	}

	hook := exec.Command(filepath.Join("..", "..", ".githooks", "pre-push"))
	hook.Env = environment
	output, err := hook.CombinedOutput()
	_, markerErr := os.Stat(makeCalledFile)
	return output, markerErr == nil, err
}
