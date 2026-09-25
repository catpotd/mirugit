package docscheck_test

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Keep CONTRIBUTING's local CI command block synchronized with ci.yml.
// Contributors need the complete set before opening a pull request.
func TestContributingListsTheCommandsCIRuns(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")

	// Every single-line run step, whatever it runs. Matching only the ones that
	// start with make counted a step that did not as no step at all: adding
	// `- run: go vet ./...` left this green — measured — and the contributor
	// was never told to run it.
	//
	// A step written as `run: |` with a block under it is still not seen.
	// Reading that form would mean parsing YAML to know where a step ends, and
	// a job added to CI is a change its author is reading this file for anyway.
	// What is bought here is the drift that happens without anyone looking.
	steps := makeTargets(t, filepath.Join(root, ".github", "workflows", "ci.yml"),
		regexp.MustCompile(`(?m)^\s*- run: (.+)$`))

	// ci.yml says why every gate goes through a make target: "Calling it here
	// rather than listing the same gates again keeps one answer to what has to
	// pass." A step that runs something else is a second answer, and the block
	// a contributor pastes cannot hold it.
	inCI := make([]string, 0, len(steps))
	for _, step := range steps {
		if !strings.HasPrefix(step, "make") {
			t.Errorf("ci.yml runs %q, which is not a make target; a gate that is "+
				"not one cannot be handed to a contributor as a command to run",
				step)
			continue
		}
		inCI = append(inCI, step)
	}
	// The one block the sentence above it points at, found by that sentence
	// rather than by the shape of its lines: CONTRIBUTING holds other bare
	// `make` lines — `make quick` while editing, `make tools` for the hooks —
	// and a rule about line shape counted those too.
	inDoc := makeTargets(t, filepath.Join(root, "CONTRIBUTING.md"),
		regexp.MustCompile("(?s)Individual CI commands:\n+```sh\n(.*?)```"))

	for _, want := range inCI {
		if !contains(inDoc, want) {
			t.Errorf("CI runs %q and CONTRIBUTING does not list it among the commands "+
				"a contributor runs when the checks do not appear", want)
		}
	}
	// The other direction: a job removed from CI leaves the contributor running
	// what nobody asks for. The comment promised both and only one was checked.
	for _, listed := range inDoc {
		if !contains(inCI, listed) {
			t.Errorf("CONTRIBUTING sends a contributor to run %q and CI does not "+
				"run it", listed)
		}
	}
	if len(inCI) == 0 {
		t.Fatal("no make targets found in ci.yml, so this proves nothing")
	}
	if len(inDoc) == 0 {
		t.Fatal("expected a fenced CI command block after the CONTRIBUTING anchor")
	}
}

func makeTargets(t *testing.T, path string, pattern *regexp.Regexp) []string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	found := pattern.FindAllStringSubmatch(string(body), -1)
	out := make([]string, 0, len(found))
	for _, m := range found {
		// A match is one command in the workflow and a block of them in the
		// document, so both are read line by line.
		for _, line := range strings.Split(m[1], "\n") {
			if line = strings.TrimSpace(line); line != "" {
				out = append(out, line)
			}
		}
	}
	sort.Strings(out)
	return out
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// make ci-local runs the same gates here, because GitHub is not running the
// workflow this month. A gate added to ci.yml and not to CI_GATES leaves the
// local run short of what the workflow would have caught, and the workflow is
// disabled, so nothing else would say so.
//
// It compares against CI_GATES rather than against the block CONTRIBUTING
// holds: that block is what a contributor pastes, and this is what a command
// runs. The two are checked separately so a change to one cannot be excused by
// the other.
func TestTheLocalRunCoversEveryGateCIRuns(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")

	steps := makeTargets(t, filepath.Join(root, ".github", "workflows", "ci.yml"),
		regexp.MustCompile(`(?m)^\s*- run: (.+)$`))
	inCI := make([]string, 0, len(steps))
	for _, step := range steps {
		if strings.HasPrefix(step, "make") {
			inCI = append(inCI, step)
		}
	}
	if len(inCI) == 0 {
		t.Fatal("no make targets found in ci.yml, so this proves nothing")
	}

	body, err := os.ReadFile(filepath.Join(root, "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^CI_GATES := (.+)$`).FindSubmatch(body)
	if m == nil {
		t.Fatal("the Makefile has no CI_GATES line; make ci-local names its gates there")
	}
	local := strings.Fields(string(m[1]))

	for _, step := range inCI {
		// "make fuzz FUZZTIME=30s" names the gate in its second word; the
		// setting beside it is passed by ci-local through CI_FUZZTIME.
		target := strings.Fields(step)[1]
		if !contains(local, target) {
			t.Errorf("ci.yml runs %q and CI_GATES does not hold %q, so make ci-local "+
				"does not run it", step, target)
		}
	}
	for _, gate := range local {
		found := false
		for _, step := range inCI {
			if strings.Fields(step)[1] == gate {
				found = true
			}
		}
		if !found {
			t.Errorf("CI_GATES holds %q and ci.yml does not run it", gate)
		}
	}
}
