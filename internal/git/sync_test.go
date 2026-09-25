package git

import "testing"

// RemoteURL tells "this repository has no origin" apart from "this directory
// answered nothing", and the list of remotes is what separates them. Reading a
// listed name as absent turns a repository whose origin cannot be read into one
// with no remote at all, and the history tab then hides the key that opens a
// commit on the web instead of reporting the failure.
func TestNamesRemoteFindsAListedName(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		list string
		want bool
	}{
		{"the only remote", "origin\n", true},
		{"one of several", "upstream\norigin\nfork\n", true},
		{"no list at all", "", false},
		{"another name entirely", "upstream\n", false},
		{"a name that starts the same", "origin-mirror\n", false},
		{"a name that ends the same", "my-origin\n", false},
		{"a line with spaces around it", "  origin  \n", true},
		{"a line ending in a carriage return", "origin\r\n", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := namesRemote(c.list, "origin"); got != c.want {
				t.Errorf("namesRemote(%q) = %v, want %v", c.list, got, c.want)
			}
		})
	}
}
