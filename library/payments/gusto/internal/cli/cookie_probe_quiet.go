// Copyright 2026 david and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"io"
	"os"
	"path/filepath"
)

// pythonImportProbe reports whether bin can import a module. It discards the
// interpreter's stderr: a failed probe is an expected "tool not installed"
// answer, and its traceback must not leak into `doctor --json` callers that
// read combined output.
func pythonImportProbe(bin string, leading []string, module string) bool {
	dir, err := os.MkdirTemp("", "pp-cookie-py-")
	if err != nil {
		return false
	}
	defer os.RemoveAll(dir)
	scriptPath := filepath.Join(dir, "helper.py")
	if err := os.WriteFile(scriptPath, []byte("import "+module), 0o600); err != nil {
		return false
	}
	cmd, err := execNamed(bin, append(append([]string{}, leading...), scriptPath)...)
	if err != nil {
		return false
	}
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}
