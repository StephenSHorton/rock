//go:build windows

package update

import "os"

// replaceFile moves the running dest aside, then puts src in its place.
func replaceFile(src, dest string) error {
	old := dest + ".old"
	_ = os.Remove(old)
	if _, err := os.Stat(dest); err == nil {
		if err := os.Rename(dest, old); err != nil {
			return err
		}
	}
	if err := os.Rename(src, dest); err != nil {
		return err
	}
	_ = os.Remove(old)
	return nil
}
