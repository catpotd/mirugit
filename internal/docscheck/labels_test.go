package docscheck_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// An issue template names the labels its issues get. GitHub attaches a label
// that exists and says nothing about one that does not, so a name nobody
// created is a line in a file that reads as working and never runs.
//
// This checks the names against the ones GitHub creates with every repository.
// A label the maintainer adds later belongs in this list too, next to why.
//
// What it cannot check is the list itself. Nothing here reaches GitHub, so a
// label deleted from the repository, or one added and not written down, reads
// as fine. `gh label list` is the only way to see that, and the publish
// checklist asks for it.
//
// It reads the inline form, `labels: [a, b]`, which is what these templates
// use. The block form — `labels:` and then a list — is not seen; measured, by
// rewriting one and watching this stay green. Templates are three files a
// maintainer writes by hand, so the form they are written in is a convention
// this checks rather than a shape it has to survive.
func TestIssueTemplatesNameLabelsThatExist(t *testing.T) {
	t.Parallel()
	// What GitHub puts in a new repository, plus the ones this one added.
	exist := map[string]bool{
		"bug": true, "documentation": true, "duplicate": true,
		"enhancement": true, "good first issue": true, "help wanted": true,
		"invalid": true, "question": true, "wontfix": true,
	}

	dir := filepath.Join("..", "..", ".github", "ISSUE_TEMPLATE")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	labels := regexp.MustCompile(`(?m)^labels:\s*\[(.+)\]\s*$`)
	checked, templates := 0, 0
	for _, entry := range entries {
		body, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		// config.yml is GitHub's settings for the chooser, not a template: it
		// has no form and files no issue.
		if entry.Name() == "config.yml" {
			continue
		}
		templates++
		m := labels.FindSubmatch(body)
		if m == nil {
			// A template with no labels line gets whatever the repository's
			// defaults are, which is nothing. Counting only the ones that name
			// a label let a line be deleted and read as nothing to check —
			// measured, by deleting bug.yml's and watching this stay green.
			t.Errorf("%s names no labels; an issue filed from it arrives with none",
				entry.Name())
			continue
		}
		checked++
		for _, name := range strings.Split(string(m[1]), ",") {
			name = strings.Trim(strings.TrimSpace(name), `"'`)
			if !exist[name] {
				t.Errorf("%s asks for the label %q, which this repository does not "+
					"have; GitHub attaches nothing and says nothing", entry.Name(), name)
			}
		}
	}
	if checked != templates {
		t.Errorf("%d templates and %d of them name a label", templates, checked)
	}
	if templates == 0 {
		t.Fatal("no issue templates found, so this proves nothing")
	}
}
