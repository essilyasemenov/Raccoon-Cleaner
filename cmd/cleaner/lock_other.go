//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"syscall"
)

func acquireRunLock(root string) (*os.File, error) {
	lock, err := os.OpenFile(filepath.Join(root, ".cleaner.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lock.Close()
		return nil, err
	}
	return lock, nil
}
