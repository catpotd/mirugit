package docscheck_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestDeadcodeCheck(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name         string
		exitCode     int
		output       string
		isSuccessful bool
	}{
		{name: "empty analysis", isSuccessful: true},
		{name: "event methods", output: "event.go:1:1: unreachable func: Change.event\n", isSuccessful: true},
		{name: "unreachable function", output: "unused.go:1:1: unreachable func: Unused\n"},
		{name: "event and unreachable function", output: "event.go:1:1: unreachable func: Change.event\nunused.go:1:1: unreachable func: Unused\n"},
		{name: "analysis failure without stdout", exitCode: 42},
		{name: "analysis failure with stdout", exitCode: 42, output: "analysis could not load packages\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			output, err := runDeadcodeMakeTarget(t, "deadcode", test.exitCode, test.output)
			if (err == nil) != test.isSuccessful {
				t.Fatalf("make deadcode: %v, want successful=%t\n%s", err, test.isSuccessful, output)
			}
			if !strings.Contains(string(output), "analysis fixture executed") {
				t.Fatalf("the analyzer did not run:\n%s", output)
			}
			if test.exitCode != 0 && test.output != "" && !strings.Contains(string(output), test.output) {
				t.Fatalf("the failed analyzer's stdout was lost:\n%s", output)
			}
			if test.exitCode == 0 && strings.Contains(string(output), "Change.event") {
				t.Fatalf("an event method was reported as unreachable:\n%s", output)
			}
		})
	}
}

func TestSupplyChainStopsWhenDeadcodeFails(t *testing.T) {
	t.Parallel()
	output, err := runDeadcodeMakeTarget(t, "supply-chain", 42, "")
	if err == nil {
		t.Fatalf("make supply-chain succeeded after deadcode failed:\n%s", output)
	}
	if !strings.Contains(string(output), "analysis fixture executed") {
		t.Fatalf("supply-chain stopped before running the failing analyzer:\n%s", output)
	}
}

func runDeadcodeMakeTarget(t *testing.T, target string, exitCode int, analyzerOutput string) ([]byte, error) {
	t.Helper()
	directory := t.TempDir()
	makefile, err := os.ReadFile(filepath.Join("..", "..", "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(directory, "tools"), 0o755); err != nil {
		t.Fatal(err)
	}
	goCommand := `#!/bin/sh
case "$1" in
	build|mod) exit 0 ;;
	run)
		case "$2" in
			golang.org/x/vuln/cmd/govulncheck@*) exit 0 ;;
			golang.org/x/tools/cmd/deadcode@*)
				if [ "$#" -ne 4 ] || [ "$3" != -test ] || [ "$4" != ./... ]; then
					printf 'unexpected analyzer arguments: %s\n' "$*" >&2
					exit 99
				fi
				printf 'analysis fixture executed\n' >&2
				printf '%s' "$DEADCODE_TEST_OUTPUT"
				exit "$DEADCODE_TEST_STATUS"
				;;
		esac
		;;
esac
printf 'unexpected Go arguments: %s\n' "$*" >&2
exit 99
`
	fixtureFiles := map[string]string{
		"Makefile":         string(makefile),
		"go-for-test":      goCommand,
		"tools/notices.sh": "#!/bin/sh\nexit 0\n",
	}
	for name, contents := range fixtureFiles {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(contents), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	command := exec.Command("make", target, "GO="+filepath.Join(directory, "go-for-test"), "VERSION=test")
	command.Dir = directory
	command.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"DEADCODE_TEST_OUTPUT=" + analyzerOutput,
		"DEADCODE_TEST_STATUS=" + strconv.Itoa(exitCode),
	}
	return command.CombinedOutput()
}
