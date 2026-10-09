package encode

import "os"

// ReplaceFile atomically replaces dest with the contents of tmpPath (tmp is removed on success).
// On Windows, os.Rename cannot overwrite an existing dest — remove first.
func ReplaceFile(tmpPath, dest string) error {
	if err := os.Rename(tmpPath, dest); err == nil {
		return nil
	}
	// Fallback for Windows / busy targets: remove dest then rename.
	if err := os.Remove(dest); err != nil && !os.IsNotExist(err) {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, dest); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}
