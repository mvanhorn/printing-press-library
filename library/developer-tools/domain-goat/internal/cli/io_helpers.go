package cli

import (
	"errors"
	"io"
	"os"
)

// openFileOrStdin opens path, or stdin when path == "-".
func openFileOrStdin(path string) (io.ReadCloser, error) {
	if path == "-" {
		return io.NopCloser(os.Stdin), nil
	}
	if path == "" {
		return nil, errors.New("empty path")
	}
	return os.Open(path)
}
