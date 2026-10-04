//go:build !windows

package update

import "os"

// replaceFile atomically puts src at dest (same-directory rename).
func replaceFile(src, dest string) error {
	return os.Rename(src, dest)
}
