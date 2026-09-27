//go:build windows

package update

import (
	"errors"
	"os"
)

func tryLockFile(*os.File) (bool, error) {
	return false, errors.New("update lock is unsupported on Windows")
}

func unlockFile(*os.File) error {
	return nil
}
