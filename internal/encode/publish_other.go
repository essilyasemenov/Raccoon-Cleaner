//go:build !windows

package encode

import "os"

// PublishFile installs tmpPath at dest only if dest does not exist.
func PublishFile(tmpPath, dest string) error {
	return os.Link(tmpPath, dest)
}
