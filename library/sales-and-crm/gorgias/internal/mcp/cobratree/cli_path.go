// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cobratree

import (
	"os"
	"os/exec"
	"path/filepath"
)

// SiblingCLIPath resolves the companion CLI via sibling-of-executable,
// GORGIAS_CLI_PATH env var, then PATH.
func SiblingCLIPath() (string, error) {
	const cliName = "gorgias-pp-cli"
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), cliName)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	if v := os.Getenv("GORGIAS_CLI_PATH"); v != "" {
		return v, nil
	}
	return exec.LookPath(cliName)
}
