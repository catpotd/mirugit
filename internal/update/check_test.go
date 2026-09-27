package update

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCheckReportsOnlyANewerStableRelease(t *testing.T) {
	for _, test := range []struct {
		name    string
		current string
		tag     string
		want    string
	}{
		{name: "newer major", current: "v1.2.3", tag: "v2.0.0", want: "v2.0.0"},
		{name: "newer minor", current: "v1.2.3", tag: "v1.3.0", want: "v1.3.0"},
		{name: "newer patch", current: "v1.2.3", tag: "v1.2.4", want: "v1.2.4"},
		{name: "same", current: "v1.2.3", tag: "v1.2.3"},
		{name: "older", current: "v1.2.3", tag: "v1.2.2"},
	} {
		t.Run(test.name, func(t *testing.T) {
			checker, closeServer := releaseChecker(t, releaseJSON(test.tag, false, false), time.Now())
			defer closeServer()

			got, err := checker.check(context.Background(), filepath.Join(t.TempDir(), "state"), test.current)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Errorf("check() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestCheckRejectsDraftPrereleaseAndMalformedReleases(t *testing.T) {
	for _, test := range []struct {
		name string
		body string
	}{
		{name: "draft", body: releaseJSON("v9.9.9", true, false)},
		{name: "prerelease", body: releaseJSON("v9.9.9", false, true)},
		{name: "malformed tag", body: releaseJSON("v9.9", false, false)},
		{name: "missing tag", body: `{"draft":false,"prerelease":false}`},
		{name: "missing draft flag", body: `{"tag_name":"v9.9.9","prerelease":false}`},
		{name: "missing prerelease flag", body: `{"tag_name":"v9.9.9","draft":false}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			checker, closeServer := releaseChecker(t, test.body, time.Now())
			defer closeServer()

			got, err := checker.check(context.Background(), filepath.Join(t.TempDir(), "state"), "v1.2.3")
			if err != nil {
				t.Fatal(err)
			}
			if got != "" {
				t.Errorf("check() = %q, want no update", got)
			}
		})
	}
}

func TestCheckSkipsAnUnknownCurrentVersion(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer server.Close()

	checker := newChecker(server.Client(), server.URL, time.Now, time.Second)
	got, err := checker.check(context.Background(), filepath.Join(t.TempDir(), "state"), "(devel)")
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Errorf("check() = %q, want no update", got)
	}
	if called {
		t.Error("unknown current version made a request")
	}
}

func TestCheckUsesTheExpectedRequestAndDoesNotSendRepositoryData(t *testing.T) {
	const secret = "repo-secret-value"
	t.Setenv("MIRUGIT_TEST_SECRET", secret)
	var request *http.Request
	var body []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request = r.Clone(r.Context())
		body, _ = io.ReadAll(r.Body)
		_, _ = io.WriteString(w, releaseJSON("v0.1.3", false, false))
	}))
	defer server.Close()

	checker := newChecker(server.Client(), server.URL+"/releases/latest", time.Now, time.Second)
	stateDir := filepath.Join(t.TempDir(), "private-state")
	if _, err := checker.check(context.Background(), stateDir, "v0.1.2"); err != nil {
		t.Fatal(err)
	}

	if request == nil {
		t.Fatal("server received no request")
	}
	if request.Method != http.MethodGet || request.URL.Path != "/releases/latest" {
		t.Errorf("request = %s %s, want GET /releases/latest", request.Method, request.URL.Path)
	}
	if got := request.Header.Get("Accept"); got != "application/vnd.github+json" {
		t.Errorf("Accept = %q, want application/vnd.github+json", got)
	}
	if got := request.Header.Get("User-Agent"); got != "mirugit/v0.1.2" {
		t.Errorf("User-Agent = %q, want mirugit/v0.1.2", got)
	}
	if request.Header.Get("Authorization") != "" || request.Header.Get("Cookie") != "" {
		t.Error("request included credentials")
	}
	if len(body) != 0 {
		t.Errorf("request body = %q, want empty", body)
	}
	if strings.Contains(request.URL.String()+request.UserAgent()+string(body), secret) {
		t.Error("request included an environment value")
	}
}

func TestReleaseClientIgnoresProxyEnvironment(t *testing.T) {
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "proxy should not receive the release request", http.StatusBadGateway)
	}))
	defer proxy.Close()
	proxyURL := strings.Replace(proxy.URL, "http://", "http://proxy-user:proxy-secret@", 1)
	t.Setenv("HTTP_PROXY", proxyURL)
	t.Setenv("HTTPS_PROXY", proxyURL)
	t.Setenv("NO_PROXY", "")
	t.Setenv("no_proxy", "")

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, releaseJSON("v0.1.3", false, false))
	}))
	defer target.Close()

	checker := newChecker(newReleaseClient(), target.URL, time.Now, time.Second)
	if got, err := checker.check(context.Background(), filepath.Join(t.TempDir(), "state"), "v0.1.2"); err != nil || got != "v0.1.3" {
		t.Fatalf("check() = %q, %v, want v0.1.3 without proxy", got, err)
	}
}

func TestReleaseClientDoesNotFollowRedirects(t *testing.T) {
	redirected := make(chan struct{}, 1)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirected <- struct{}{}
		_, _ = io.WriteString(w, releaseJSON("v9.9.9", false, false))
	}))
	defer target.Close()

	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer source.Close()

	checker := newChecker(newReleaseClient(), source.URL, time.Now, time.Second)
	if _, err := checker.check(context.Background(), filepath.Join(t.TempDir(), "state"), "v0.1.2"); err == nil {
		t.Fatal("redirect response was accepted")
	}
	select {
	case <-redirected:
		t.Fatal("release client followed a redirect")
	default:
	}
}

func TestCheckRejectsTrailingJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, releaseJSON("v0.1.3", false, false)+`{}`)
	}))
	defer server.Close()

	checker := newChecker(server.Client(), server.URL, time.Now, time.Second)
	if _, err := checker.check(context.Background(), filepath.Join(t.TempDir(), "state"), "v0.1.2"); err == nil {
		t.Fatal("response with trailing JSON was accepted")
	}
}

func TestCheckReturnsAnErrorWhenTheRequestTimesOut(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	checker := newChecker(server.Client(), server.URL, time.Now, 20*time.Millisecond)
	_, err := checker.check(context.Background(), filepath.Join(t.TempDir(), "state"), "v0.1.2")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("check() error = %v, want deadline exceeded", err)
	}
}

func TestCheckRecordsTheCompletionTime(t *testing.T) {
	started := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	finished := started.Add(time.Minute)
	completed := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(completed)
		_, _ = io.WriteString(w, releaseJSON("v0.1.3", false, false))
	}))
	defer server.Close()

	clock := func() time.Time {
		select {
		case <-completed:
			return finished
		default:
			return started
		}
	}
	stateDir := filepath.Join(t.TempDir(), "state")
	checker := newChecker(server.Client(), server.URL, clock, time.Second)
	if _, err := checker.check(context.Background(), stateDir, "v0.1.2"); err != nil {
		t.Fatal(err)
	}

	encoded, err := os.ReadFile(filepath.Join(stateDir, "update.json"))
	if err != nil {
		t.Fatal(err)
	}
	var record cache
	if err := json.Unmarshal(encoded, &record); err != nil {
		t.Fatal(err)
	}
	if !record.LastCheckedAt.Equal(finished) {
		t.Fatalf("LastCheckedAt = %s, want %s", record.LastCheckedAt, finished)
	}
}

func TestCanceledCheckDoesNotRecordFailure(t *testing.T) {
	entered := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stateDir := filepath.Join(t.TempDir(), "state")
	checker := newChecker(server.Client(), server.URL, time.Now, time.Second)
	result := make(chan error, 1)
	go func() {
		_, err := checker.check(ctx, stateDir, "v0.1.2")
		result <- err
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("check did not reach the server")
	}
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("check() error = %v, want context canceled", err)
	}
	if _, err := os.Stat(filepath.Join(stateDir, "update.json")); !os.IsNotExist(err) {
		t.Fatalf("canceled check wrote a cache: %v", err)
	}
}

func TestCheckUsesCachedReleaseBeforeThe24HourBoundary(t *testing.T) {
	var requests int
	var requestsMu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestsMu.Lock()
		requests++
		requestsMu.Unlock()
		_, _ = io.WriteString(w, releaseJSON("v0.1.3", false, false))
	}))
	defer server.Close()

	now := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	checker := newChecker(server.Client(), server.URL+"/releases/latest", func() time.Time { return now }, time.Second)
	stateDir := filepath.Join(t.TempDir(), "state")
	if got, err := checker.check(context.Background(), stateDir, "v0.1.2"); err != nil || got != "v0.1.3" {
		t.Fatalf("first check = %q, %v", got, err)
	}

	now = now.Add(23*time.Hour + 59*time.Minute)
	if got, err := checker.check(context.Background(), stateDir, "v0.1.2"); err != nil || got != "v0.1.3" {
		t.Fatalf("cached check = %q, %v", got, err)
	}
	requestsMu.Lock()
	defer requestsMu.Unlock()
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
}

func TestCheckRequestsAgainAtThe24HourBoundary(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = io.WriteString(w, releaseJSON("v0.1.3", false, false))
	}))
	defer server.Close()

	now := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	checker := newChecker(server.Client(), server.URL, func() time.Time { return now }, time.Second)
	stateDir := filepath.Join(t.TempDir(), "state")
	for i := 0; i < 2; i++ {
		if _, err := checker.check(context.Background(), stateDir, "v0.1.2"); err != nil {
			t.Fatal(err)
		}
		now = now.Add(24 * time.Hour)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
}

func TestFailedRequestRecordsTheCheckTimeAndSuppressesRetry(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		http.Error(w, "unavailable", http.StatusBadGateway)
	}))
	defer server.Close()

	now := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	checker := newChecker(server.Client(), server.URL, func() time.Time { return now }, time.Second)
	stateDir := filepath.Join(t.TempDir(), "state")
	if _, err := checker.check(context.Background(), stateDir, "v0.1.2"); err == nil {
		t.Fatal("failed request returned no error")
	}

	var record cache
	encoded, err := os.ReadFile(filepath.Join(stateDir, "update.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &record); err != nil {
		t.Fatal(err)
	}
	if record.LatestVersion != "" || !record.LastCheckedAt.Equal(now) {
		t.Fatalf("record = %+v, want failed check at %s", record, now)
	}

	now = now.Add(time.Hour)
	if _, err := checker.check(context.Background(), stateDir, "v0.1.2"); err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
}

func TestCheckReplacesInvalidCache(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, releaseJSON("v0.1.3", false, false))
	}))
	defer server.Close()

	for _, test := range []struct {
		name string
		body string
	}{
		{name: "invalid JSON", body: "{"},
		{name: "unknown schema", body: `{"schema_version":2,"last_checked_at":"2026-09-27T00:00:00Z","latest_version":"v9.9.9"}`},
		{name: "invalid timestamp", body: `{"schema_version":1,"last_checked_at":"not-time","latest_version":"v9.9.9"}`},
		{name: "invalid release", body: `{"schema_version":1,"last_checked_at":"2026-09-27T00:00:00Z","latest_version":"v9.9"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			stateDir := filepath.Join(t.TempDir(), "state")
			if err := os.MkdirAll(stateDir, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(stateDir, "update.json"), []byte(test.body), 0o600); err != nil {
				t.Fatal(err)
			}

			checker := newChecker(server.Client(), server.URL, func() time.Time {
				return time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
			}, time.Second)
			got, err := checker.check(context.Background(), stateDir, "v0.1.2")
			if err != nil {
				t.Fatal(err)
			}
			if got != "v0.1.3" {
				t.Errorf("check() = %q, want v0.1.3", got)
			}
		})
	}
}

func TestCheckWritesPrivateDirectoryAndFileAndRemovesTemporaryFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, releaseJSON("v0.1.3", false, false))
	}))
	defer server.Close()

	stateDir := filepath.Join(t.TempDir(), "state")
	checker := newChecker(server.Client(), server.URL, time.Now, time.Second)
	if _, err := checker.check(context.Background(), stateDir, "v0.1.2"); err != nil {
		t.Fatal(err)
	}

	dirInfo, err := os.Stat(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Errorf("state directory mode = %o, want 700", got)
	}
	fileInfo, err := os.Stat(filepath.Join(stateDir, "update.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := fileInfo.Mode().Perm(); got != 0o600 {
		t.Errorf("update file mode = %o, want 600", got)
	}
	entries, err := os.ReadDir(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Name() != "update.json" || entries[1].Name() != lockFileName {
		t.Fatalf("state directory entries = %v, want update.json and %s", entries, lockFileName)
	}
}

func TestCheckRecoversAnAbandonedLock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, releaseJSON("v0.1.3", false, false))
	}))
	defer server.Close()

	now := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	stateDir := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	lock := filepath.Join(stateDir, "update.lock")
	if err := os.WriteFile(lock, []byte("abandoned"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(lock, now.Add(-11*time.Second), now.Add(-11*time.Second)); err != nil {
		t.Fatal(err)
	}

	checker := newChecker(server.Client(), server.URL, func() time.Time { return now }, time.Second)
	if got, err := checker.check(context.Background(), stateDir, "v0.1.2"); err != nil || got != "v0.1.3" {
		t.Fatalf("check() = %q, %v", got, err)
	}
	if _, err := os.Stat(lock); err != nil {
		t.Fatalf("lock stat error = %v, want persistent lock file", err)
	}
}

func TestConcurrentChecksIssueOneRequest(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var requests int
	var requestsMu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestsMu.Lock()
		requests++
		requestsMu.Unlock()
		close(entered)
		<-release
		_, _ = io.WriteString(w, releaseJSON("v0.1.3", false, false))
	}))
	defer server.Close()

	now := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	checker := newChecker(server.Client(), server.URL, func() time.Time { return now }, time.Second)
	stateDir := filepath.Join(t.TempDir(), "state")
	first := make(chan error, 1)
	go func() {
		_, err := checker.check(context.Background(), stateDir, "v0.1.2")
		first <- err
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("first check did not reach the server")
	}

	second := make(chan error, 1)
	go func() {
		_, err := checker.check(context.Background(), stateDir, "v0.1.2")
		second <- err
	}()
	select {
	case err := <-second:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("second check did not skip the held lock")
	}
	close(release)
	if err := <-first; err != nil {
		t.Fatal(err)
	}

	requestsMu.Lock()
	defer requestsMu.Unlock()
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
}

func TestConcurrentChecksRecoveringAnAbandonedLockIssueOneRequest(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var requests int
	var requestsMu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestsMu.Lock()
		requests++
		requestsMu.Unlock()
		close(entered)
		<-release
		_, _ = io.WriteString(w, releaseJSON("v0.1.3", false, false))
	}))
	defer server.Close()

	now := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	stateDir := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	lock := filepath.Join(stateDir, lockFileName)
	if err := os.WriteFile(lock, []byte("abandoned"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(lock, now.Add(-11*time.Second), now.Add(-11*time.Second)); err != nil {
		t.Fatal(err)
	}

	checker := newChecker(server.Client(), server.URL, func() time.Time { return now }, time.Second)
	first := make(chan error, 1)
	go func() {
		_, err := checker.check(context.Background(), stateDir, "v0.1.2")
		first <- err
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("first check did not reach the server")
	}

	second := make(chan error, 1)
	go func() {
		_, err := checker.check(context.Background(), stateDir, "v0.1.2")
		second <- err
	}()
	select {
	case err := <-second:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("second check did not skip the recovered lock")
	}
	close(release)
	if err := <-first; err != nil {
		t.Fatal(err)
	}

	requestsMu.Lock()
	defer requestsMu.Unlock()
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
}

func releaseChecker(t *testing.T, body string, now time.Time) (checker, func()) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, body)
	}))
	return newChecker(server.Client(), server.URL, func() time.Time { return now }, time.Second), server.Close
}

func releaseJSON(tag string, draft, prerelease bool) string {
	return `{"tag_name":"` + tag + `","draft":` + boolText(draft) + `,"prerelease":` + boolText(prerelease) + `}`
}

func boolText(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
