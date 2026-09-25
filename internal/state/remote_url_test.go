package state

import "testing"

// remote.origin.url belongs to whoever the repository was cloned from, so it is
// input from outside. What this package builds out of it goes to a program that
// opens it, and a string that is not a URL has no business getting there.
func TestOnlyASomethingABrowserCanOpenComesOut(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name   string
		remote string
		want   string
	}{
		{"an https remote", "https://github.com/owner/repo.git", "https://github.com/owner/repo"},
		{"an ssh remote", "git@github.com:owner/repo.git", "https://github.com/owner/repo"},
		{"an ssh remote on a port-looking host", "git@host:1234/owner/repo", "https://host/1234/owner/repo"},
		{"a scheme a browser would run", "javascript:alert(1)", ""},
		{"a local file", "file:///etc/passwd", ""},
		{"something that reads as a flag", "-n", ""},
		{"an ssh remote with no host", "git@:80", ""},
		// git@host with nothing after it names no path. The colon is what
		// separates the two, and a remote without one cannot be split.
		{"an ssh remote with no colon", "git@github.com", ""},
		{"a newline in the middle", "https://x\nhttps://y", ""},
		{"an escape sequence", "https://x\x1b[2Jy", ""},
		{"nothing at all", "", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, ok := HTTPSRemoteBase(c.remote)
			if ok != (c.want != "") {
				t.Fatalf("ok = %v with %q, want %v", ok, got, c.want != "")
			}
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// The two callers of webURL both hand it a string starting with "https://", so
// no remote reaches its scheme check: "javascript:alert(1)" above is turned away
// earlier, by the test for a git@ prefix. Measured — the check can be inverted
// and every case above stays green.
//
// It is the check itself, named in webURL's comment as the last thing before a
// browser is started, so it is asked directly rather than deleted.
func TestOnlyASchemeABrowserOpensGetsPast(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name  string
		built string
		want  bool
	}{
		{"https", "https://host/owner/repo", true},
		{"http", "http://host/owner/repo", true},
		{"a scheme a browser would run", "javascript:alert(1)", false},
		{"a local file", "file:///etc/passwd", false},
		{"a scheme that opens another program", "ftp://host/x", false},
		{"no scheme at all", "host/owner/repo", false},
		{"the scheme spelled in capitals", "HTTPS://host/owner/repo", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, ok := webURL(c.built)
			if ok != c.want {
				t.Fatalf("webURL(%q) = %q, %v; want ok = %v", c.built, got, ok, c.want)
			}
			if ok && got != c.built {
				t.Errorf("webURL(%q) changed it to %q", c.built, got)
			}
		})
	}
}

// The string this hands back is given to a program that opens a browser, and a
// control character in it can end the line in whatever reads it next. The check
// is asked directly, as the scheme check above is: no remote reaches it, because
// net/url refuses a control character before webURL's own line looks for one.
func TestNoControlCharacterReachesTheBrowser(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name  string
		built string
		want  bool
	}{
		{"an ordinary address", "https://host/owner/repo", true},
		{"a newline at the end", "https://host/owner/repo\n", false},
		{"a carriage return in the path", "https://host/owner/re\rpo", false},
		{"a null in the host", "https://ho\x00st/owner/repo", false},
		{"an escape in the path", "https://host/owner/\x1brepo", false},
		{"a delete in the path", "https://host/owner/\x7frepo", false},
		{"a tab in the path", "https://host/owner/re\tpo", false},
		// A space is not a control character. A repository whose path holds
		// one still has a page, and the address is handed to the browser as
		// one argument rather than as a command line to be split.
		{"a space in the path", "https://host/owner/re po", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, ok := webURL(c.built)
			if ok != c.want {
				t.Fatalf("webURL(%q) = %q, %v; want ok = %v", c.built, got, ok, c.want)
			}
			if ok && got != c.built {
				t.Errorf("webURL(%q) changed it to %q", c.built, got)
			}
		})
	}
}
