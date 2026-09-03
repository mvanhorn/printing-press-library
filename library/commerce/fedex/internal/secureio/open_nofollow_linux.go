// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

//go:build linux

package secureio

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func openRegularNoFollow(path string, flags int, perm os.FileMode) (*os.File, error) {
	parentPath := filepath.Dir(filepath.Clean(path))
	parentHow := &unix.OpenHow{
		Flags:   unix.O_PATH | unix.O_DIRECTORY | unix.O_CLOEXEC,
		Resolve: unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_NO_MAGICLINKS,
	}
	parentFD, err := unix.Openat2(unix.AT_FDCWD, parentPath, parentHow)
	if err != nil {
		return nil, fmt.Errorf("opening no-follow parent %s: %w", parentPath, err)
	}
	defer unix.Close(parentFD)

	mode := uint64(0)
	if flags&os.O_CREATE != 0 {
		mode = uint64(perm.Perm())
	}
	how := &unix.OpenHow{
		Flags:   uint64(flags | unix.O_CLOEXEC | unix.O_NOFOLLOW),
		Mode:    mode,
		Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_NO_MAGICLINKS,
	}
	fd, err := unix.Openat2(parentFD, filepath.Base(path), how)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), path), nil
}

func openDirectoryNoFollow(path string) (*os.File, error) {
	how := &unix.OpenHow{
		Flags:   unix.O_RDONLY | unix.O_DIRECTORY | unix.O_CLOEXEC | unix.O_NOFOLLOW,
		Resolve: unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_NO_MAGICLINKS,
	}
	fd, err := unix.Openat2(unix.AT_FDCWD, filepath.Clean(path), how)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), path), nil
}
