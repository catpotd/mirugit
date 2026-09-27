package update

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

const (
	cacheSchemaVersion = 1
	cacheFileName      = "update.json"
	lockFileName       = "update.lock"
	checkInterval      = 24 * time.Hour
)

type cache struct {
	LastCheckedAt time.Time `json:"last_checked_at"`
	SchemaVersion int       `json:"schema_version"`
	LatestVersion string    `json:"latest_version"`
}

func cachePath(stateDir string) string {
	return filepath.Join(stateDir, cacheFileName)
}

func lockPath(stateDir string) string {
	return filepath.Join(stateDir, lockFileName)
}

func loadCache(stateDir string) (cache, bool, error) {
	encoded, err := os.ReadFile(cachePath(stateDir))
	if errors.Is(err, os.ErrNotExist) {
		return cache{}, false, nil
	}
	if err != nil {
		return cache{}, false, err
	}

	var record cache
	if err := json.Unmarshal(encoded, &record); err != nil {
		return cache{}, false, nil //nolint:nilerr // invalid cache is absent state
	}
	if record.SchemaVersion != cacheSchemaVersion || record.LastCheckedAt.IsZero() {
		return cache{}, false, nil
	}
	if record.LatestVersion != "" {
		if _, ok := parseVersion(record.LatestVersion); !ok {
			return cache{}, false, nil
		}
	}
	return record, true, nil
}

func writeCache(stateDir string, record cache) error {
	if err := ensureStateDir(stateDir); err != nil {
		return err
	}
	record.SchemaVersion = cacheSchemaVersion
	encoded, err := json.Marshal(record)
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(stateDir, ".update-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = os.Remove(tmpName)
		}
	}()
	if err := os.Chmod(tmpName, 0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(encoded); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, cachePath(stateDir)); err != nil {
		return err
	}
	removeTemp = false
	return nil
}

func ensureStateDir(stateDir string) error {
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return err
	}
	return os.Chmod(stateDir, 0o700)
}

func cacheFresh(record cache, now time.Time) bool {
	age := now.Sub(record.LastCheckedAt)
	return age >= 0 && age < checkInterval
}

func acquireLock(stateDir string, now time.Time) (func(), bool, error) {
	if err := ensureStateDir(stateDir); err != nil {
		return nil, false, err
	}
	path := lockPath(stateDir)
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, false, err
	}
	acquired, err := tryLockFile(file)
	if err != nil {
		_ = file.Close()
		return nil, false, err
	}
	if !acquired {
		_ = file.Close()
		return nil, false, nil
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = unlockFile(file)
		_ = file.Close()
		return nil, false, err
	}
	if err := os.Chtimes(path, now, now); err != nil {
		_ = unlockFile(file)
		_ = file.Close()
		return nil, false, err
	}
	return func() {
		_ = unlockFile(file)
		_ = file.Close()
	}, true, nil
}
