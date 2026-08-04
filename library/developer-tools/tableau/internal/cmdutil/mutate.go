package cmdutil

import (
	"fmt"
	"os"
)

// ResolveOutputPath requires either --output or --in-place for mutations.
// Prefer not defaulting to overwrite: callers must pass one of the flags.
func ResolveOutputPath(input, output string, inPlace bool) (string, error) {
	if output != "" && inPlace {
		return "", fmt.Errorf("use either --output or --in-place, not both")
	}
	if output == "" && !inPlace {
		return "", fmt.Errorf("mutation requires --output PATH or --in-place")
	}
	if inPlace {
		return input, nil
	}
	return output, nil
}

// WriteFile writes data to path, creating parent directories as needed.
func WriteFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644)
}
