package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"
)

var lockFileEx = syscall.NewLazyDLL("kernel32.dll").NewProc("LockFileEx")

func acquireRunLock(root string) (*os.File, error) {
	lock, err := os.OpenFile(filepath.Join(root, ".cleaner.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	var overlapped syscall.Overlapped
	locked, _, callErr := lockFileEx.Call(lock.Fd(), 0x3, 0, 1, 0, uintptr(unsafe.Pointer(&overlapped)))
	runtime.KeepAlive(&overlapped)
	if locked == 0 {
		lock.Close()
		if callErr == syscall.Errno(0) {
			callErr = errors.New("LockFileEx failed")
		}
		return nil, callErr
	}
	return lock, nil
}
