// Package update checks the latest stable mirugit release without repository data.
package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	defaultEndpoint = "https://api.github.com/repos/catpotd/mirugit/releases/latest"
	requestTimeout  = 2 * time.Second
	maxResponseSize = 1 << 20
)

type checker struct {
	client   *http.Client
	endpoint string
	now      func() time.Time
	timeout  time.Duration
}

type release struct {
	TagName    string `json:"tag_name"`
	Draft      *bool  `json:"draft"`
	Prerelease *bool  `json:"prerelease"`
}

func Check(ctx context.Context, stateDir, currentVersion string) (string, error) {
	return newChecker(newReleaseClient(), defaultEndpoint, time.Now, requestTimeout).check(ctx, stateDir, currentVersion)
}

func newReleaseClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &http.Client{
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func newChecker(client *http.Client, endpoint string, now func() time.Time, timeout time.Duration) checker {
	return checker{client: client, endpoint: endpoint, now: now, timeout: timeout}
}

func (c checker) check(ctx context.Context, stateDir, currentVersion string) (string, error) {
	current, ok := parseVersion(currentVersion)
	if !ok {
		return "", nil
	}
	now := c.now()
	record, found, err := loadCache(stateDir)
	if err != nil {
		return "", err
	}
	if found && cacheFresh(record, now) {
		return newerVersion(record.LatestVersion, current), nil
	}

	releaseLock, acquired, err := acquireLock(stateDir, c.now())
	if err != nil {
		return "", err
	}
	if !acquired {
		return "", nil
	}
	defer releaseLock()

	record, found, err = loadCache(stateDir)
	if err != nil {
		return "", err
	}
	now = c.now()
	if found && cacheFresh(record, now) {
		return newerVersion(record.LatestVersion, current), nil
	}

	tag, err := c.fetch(ctx, currentVersion)
	checkedAt := c.now()
	if err != nil {
		if ctx.Err() != nil {
			return "", err
		}
		writeErr := writeCache(stateDir, cache{LastCheckedAt: checkedAt})
		if writeErr != nil {
			return "", errors.Join(err, writeErr)
		}
		return "", err
	}
	if err := writeCache(stateDir, cache{LastCheckedAt: checkedAt, LatestVersion: tag}); err != nil {
		return "", err
	}
	return newerVersion(tag, current), nil
}

func (c checker) fetch(ctx context.Context, currentVersion string) (string, error) {
	requestContext, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestContext, http.MethodGet, c.endpoint, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "mirugit/"+currentVersion)
	response, err := c.client.Do(request)
	if err != nil {
		return "", err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("update check returned HTTP status %d", response.StatusCode)
	}

	var latest release
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxResponseSize))
	if err := decoder.Decode(&latest); err != nil {
		return "", err
	}
	if latest.Draft == nil || latest.Prerelease == nil || *latest.Draft || *latest.Prerelease {
		return "", nil
	}
	if _, ok := parseVersion(latest.TagName); !ok {
		return "", nil
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return "", errors.New("update response contains multiple JSON values")
		}
		return "", err
	}
	return latest.TagName, nil
}

func newerVersion(tag string, current version) string {
	latest, ok := parseVersion(tag)
	if !ok || !latest.after(current) {
		return ""
	}
	return tag
}
