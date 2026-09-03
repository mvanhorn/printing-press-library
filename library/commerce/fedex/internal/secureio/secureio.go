// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

// Package secureio provides owner-only, descriptor-relative filesystem
// primitives for credentials, shipment state, ledgers, labels, and exports.
package secureio

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	PrivateDirMode  = 0o700
	PrivateFileMode = 0o600
)

// EnsurePrivateDir creates path without accepting symlink components and
// tightens every opened directory component to owner-only access.
func EnsurePrivateDir(path string) error {
	root, err := openPrivateDir(path, true)
	if err != nil {
		return err
	}
	return root.Close()
}

// SecureExistingFile rejects symlinks/non-regular targets and tightens the
// opened file descriptor to mode 0600.
func SecureExistingFile(path string) error {
	root, base, err := openParent(path, false)
	if err != nil {
		return err
	}
	defer root.Close()
	file, err := openExistingRegular(root, base, os.O_RDWR)
	if err != nil {
		return err
	}
	return file.Close()
}

// PreparePrivateFile creates a missing regular file or secures an existing
// one. The parent is kept open while the final path is inspected and opened.
func PreparePrivateFile(path string) error {
	root, base, err := openParent(path, true)
	if err != nil {
		return err
	}
	defer root.Close()

	file, err := openExistingRegular(root, base, os.O_RDWR)
	if errors.Is(err, os.ErrNotExist) {
		file, err = openRegularNoFollow(filepath.Join(root.Name(), base), os.O_CREATE|os.O_EXCL|os.O_RDWR, PrivateFileMode)
	}
	if err != nil {
		return err
	}
	if err := secureOpenedRegular(file, path); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("syncing private file %s: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("closing private file %s: %w", path, err)
	}
	return syncRoot(root)
}

// ReadFile opens the target relative to its already-opened private parent,
// validates the opened descriptor, and reads it without path-based reopening.
func ReadFile(path string) ([]byte, error) {
	root, base, err := openParent(path, false)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	file, err := openExistingRegular(root, base, os.O_RDONLY)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(file)
}

// WriteFileAtomic writes to a same-directory 0600 temporary file, fsyncs it,
// renames it relative to the held parent descriptor, then fsyncs the parent.
func WriteFileAtomic(path string, data []byte) error {
	root, base, err := openParent(path, true)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := rejectUnsafeExisting(root, base, path); err != nil {
		return err
	}

	tempName, err := randomTempName(base)
	if err != nil {
		return err
	}
	file, err := openRegularNoFollow(filepath.Join(root.Name(), tempName), os.O_CREATE|os.O_EXCL|os.O_WRONLY, PrivateFileMode)
	if err != nil {
		return fmt.Errorf("creating private temp file for %s: %w", path, err)
	}
	cleanup := func() {
		_ = file.Close()
		_ = root.Remove(tempName)
	}
	if err := secureOpenedRegular(file, path); err != nil {
		cleanup()
		return err
	}
	if _, err := file.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("writing private temp file for %s: %w", path, err)
	}
	if err := file.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("syncing private temp file for %s: %w", path, err)
	}
	if err := file.Close(); err != nil {
		_ = root.Remove(tempName)
		return fmt.Errorf("closing private temp file for %s: %w", path, err)
	}
	if err := root.Rename(tempName, base); err != nil {
		_ = root.Remove(tempName)
		return fmt.Errorf("installing private file %s: %w", path, err)
	}
	return syncRoot(root)
}

// OpenFile opens a private regular file relative to a held parent descriptor.
func OpenFile(path string, flags int) (*os.File, error) {
	root, base, err := openParent(path, flags&os.O_CREATE != 0)
	if err != nil {
		return nil, err
	}
	defer root.Close()

	file, err := openExistingRegular(root, base, flags&^os.O_CREATE)
	if errors.Is(err, os.ErrNotExist) && flags&os.O_CREATE != 0 {
		file, err = openRegularNoFollow(filepath.Join(root.Name(), base), flags|os.O_EXCL, PrivateFileMode)
	}
	if err != nil {
		return nil, err
	}
	if err := secureOpenedRegular(file, path); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

func openParent(path string, create bool) (*os.Root, string, error) {
	path = filepath.Clean(path)
	base := filepath.Base(path)
	if base == "." || base == string(filepath.Separator) {
		return nil, "", fmt.Errorf("invalid private file path %q", path)
	}
	root, err := openPrivateDir(filepath.Dir(path), create)
	return root, base, err
}

func openPrivateDir(path string, create bool) (*os.Root, error) {
	path = filepath.Clean(path)
	start := "."
	rel := path
	if filepath.IsAbs(path) {
		volume := filepath.VolumeName(path)
		start = volume + string(filepath.Separator)
		rel = strings.TrimPrefix(path, start)
	}
	root, err := os.OpenRoot(start)
	if err != nil {
		return nil, fmt.Errorf("opening private directory root %s: %w", start, err)
	}
	if rel == "." || rel == "" {
		return root, nil
	}

	components := make([]string, 0)
	for _, component := range strings.Split(rel, string(filepath.Separator)) {
		if component != "" && component != "." {
			components = append(components, component)
		}
	}
	for index, component := range components {
		info, err := root.Lstat(component)
		if errors.Is(err, os.ErrNotExist) && create {
			if err := root.Mkdir(component, PrivateDirMode); err != nil && !errors.Is(err, os.ErrExist) {
				root.Close()
				return nil, fmt.Errorf("creating private directory component %s: %w", component, err)
			}
			info, err = root.Lstat(component)
		}
		if err != nil {
			root.Close()
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			root.Close()
			return nil, fmt.Errorf("unsafe private directory component %s", component)
		}
		if index == len(components)-1 {
			dirFile, err := openDirectoryNoFollow(filepath.Join(root.Name(), component))
			if err != nil {
				root.Close()
				return nil, err
			}
			if err := dirFile.Chmod(PrivateDirMode); err != nil {
				dirFile.Close()
				root.Close()
				return nil, fmt.Errorf("securing private directory %s: %w", path, err)
			}
			if err := dirFile.Close(); err != nil {
				root.Close()
				return nil, err
			}
		}
		next, err := root.OpenRoot(component)
		if err != nil {
			root.Close()
			return nil, fmt.Errorf("opening private directory component %s: %w", component, err)
		}
		root.Close()
		root = next
	}
	return root, nil
}

func openExistingRegular(root *os.Root, base string, flags int) (*os.File, error) {
	info, err := root.Lstat(base)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("unsafe private file target %s", base)
	}
	file, err := openRegularNoFollow(filepath.Join(root.Name(), base), flags, PrivateFileMode)
	if err != nil {
		return nil, err
	}
	if err := secureOpenedRegular(file, base); err != nil {
		file.Close()
		return nil, err
	}
	return file, nil
}

func secureOpenedRegular(file *os.File, displayPath string) error {
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("statting private file %s: %w", displayPath, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("unsafe private file target %s: not a regular file", displayPath)
	}
	if err := file.Chmod(PrivateFileMode); err != nil {
		return fmt.Errorf("securing private file %s: %w", displayPath, err)
	}
	return nil
}

func rejectUnsafeExisting(root *os.Root, base, displayPath string) error {
	info, err := root.Lstat(base)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("unsafe private file target %s", displayPath)
	}
	return nil
}

func randomTempName(base string) (string, error) {
	var raw [12]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generating private temp name: %w", err)
	}
	return "." + base + ".tmp-" + hex.EncodeToString(raw[:]), nil
}

func syncRoot(root *os.Root) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	file, err := root.Open(".")
	if err != nil {
		return fmt.Errorf("opening private directory for sync: %w", err)
	}
	defer file.Close()
	if err := file.Sync(); err != nil {
		return fmt.Errorf("syncing private directory: %w", err)
	}
	return nil
}
