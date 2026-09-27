package update

import "testing"

func TestParseVersionAcceptsOnlyStableTags(t *testing.T) {
	for _, test := range []struct {
		tag string
		ok  bool
	}{
		{tag: "v1.2.3", ok: true},
		{tag: "v0.1.2", ok: true},
		{tag: "1.2.3", ok: false},
		{tag: "v1.2", ok: false},
		{tag: "v1.2.3-rc.1", ok: false},
		{tag: "v1.2.3+build", ok: false},
		{tag: "v1.-2.3", ok: false},
		{tag: "v1.two.3", ok: false},
	} {
		t.Run(test.tag, func(t *testing.T) {
			_, ok := parseVersion(test.tag)
			if ok != test.ok {
				t.Fatalf("parseVersion(%q) ok = %v, want %v", test.tag, ok, test.ok)
			}
		})
	}
}

func TestVersionOrdering(t *testing.T) {
	for _, test := range []struct {
		name  string
		newer string
		older string
	}{
		{name: "major", newer: "v2.0.0", older: "v1.9.9"},
		{name: "minor", newer: "v1.3.0", older: "v1.2.9"},
		{name: "patch", newer: "v1.2.4", older: "v1.2.3"},
	} {
		t.Run(test.name, func(t *testing.T) {
			newer, newerOK := parseVersion(test.newer)
			older, olderOK := parseVersion(test.older)
			if !newerOK || !olderOK {
				t.Fatal("test versions should parse")
			}
			if !newer.after(older) {
				t.Errorf("%s should be after %s", test.newer, test.older)
			}
			if older.after(newer) {
				t.Errorf("%s should not be after %s", test.older, test.newer)
			}
		})
	}

	equal, ok := parseVersion("v1.2.3")
	if !ok {
		t.Fatal("equal version should parse")
	}
	if equal.after(equal) {
		t.Error("a version should not be after itself")
	}
}
