// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Library patch: keep provider-derived local outputs private even when replacing an existing file.

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/sarvam/internal/cliutil"
)

func privateOutputPath(dir, name string) (string, error) {
	if name == "" || name == "." || name == ".." || filepath.IsAbs(name) || filepath.Base(name) != name || strings.ContainsAny(name, `/\\`) {
		return "", fmt.Errorf("unsafe output filename %q", name)
	}
	return filepath.Join(dir, name), nil
}

func writePrivateOutputFile(path string, data []byte) error {
	// Write beside the destination, then replace it. Opening path directly
	// would follow an existing symlink and could write private data elsewhere.
	f, err := createPrivateOutputTempFile(path)
	if err != nil {
		return err
	}
	tempPath := f.Name()
	defer os.Remove(tempPath)
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return cliutil.RenamePrivateFileWithRetry(tempPath, path)
}

func createPrivateOutputTempFile(path string) (*os.File, error) {
	f, err := os.CreateTemp(filepath.Dir(path), ".sarvam-private-*")
	if err != nil {
		return nil, err
	}
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return nil, err
	}
	return f, nil
}
