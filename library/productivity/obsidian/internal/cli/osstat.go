package cli

import "os"

// osStat is a thin alias the doctor uses so vault-path validation can be
// stubbed in future tests without dragging os.Stat into the test setup.
func osStat(p string) (os.FileInfo, error) {
	return os.Stat(p)
}
