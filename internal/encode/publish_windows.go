package encode

import "syscall"

// PublishFile installs tmpPath at dest only if dest does not exist.
func PublishFile(tmpPath, dest string) error {
	from, err := syscall.UTF16PtrFromString(tmpPath)
	if err != nil {
		return err
	}
	to, err := syscall.UTF16PtrFromString(dest)
	if err != nil {
		return err
	}
	return syscall.MoveFile(from, to)
}
