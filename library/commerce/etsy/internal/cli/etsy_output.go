package cli

import (
	"fmt"
	"os"
)

// etsyOpenOutput never follows a destination symlink. Exclusive creation also
// protects against a symlink planted between the force check and recreation.
func etsyOpenOutput(path string, force bool) (*os.File, error) {
	if force {
		info, err := os.Lstat(path)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		if err == nil {
			if !info.Mode().IsRegular() {
				return nil, fmt.Errorf("refusing non-regular output %q, even with --force", path)
			}
			if err := os.Remove(path); err != nil {
				return nil, err
			}
		}
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL|etsyNoFollow, 0o600) // #nosec G304 -- operator-supplied --output path; O_EXCL+O_NOFOLLOW, regular files only.
	if os.IsExist(err) {
		return nil, fmt.Errorf("output %q already exists; use --force to replace a regular file", path)
	}
	return f, err
}
