package encode

import "os"

// ReplaceFile renames tmpPath onto dest without deleting dest first.
func ReplaceFile(tmpPath, dest string) error {
	return os.Rename(tmpPath, dest)
}
