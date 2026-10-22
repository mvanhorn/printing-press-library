// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

//go:build linux

package secureio

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// openat2 (Linux 5.6+) resolves a whole path without following symlinks. On
// older kernels, or where a seccomp policy blocks it, fall back to walking
// the path one component at a time with openat(O_NOFOLLOW|O_DIRECTORY), which
// refuses symlinked components just the same.
func openat2Unavailable(err error) bool {
	return errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.EPERM)
}

func openRegularNoFollow(path string, flags int, perm os.FileMode) (*os.File, error) {
	parentPath := filepath.Dir(filepath.Clean(path))
	parentFD, err := openDirFD(parentPath, unix.O_PATH)
	if err != nil {
		return nil, fmt.Errorf("opening no-follow parent %s: %w", parentPath, err)
	}
	defer unix.Close(parentFD)

	mode := uint32(0)
	if flags&os.O_CREATE != 0 {
		mode = uint32(perm.Perm())
	}
	how := &unix.OpenHow{
		Flags:   uint64(flags | unix.O_CLOEXEC | unix.O_NOFOLLOW),
		Mode:    uint64(mode),
		Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_NO_MAGICLINKS,
	}
	base := filepath.Base(path)
	fd, err := unix.Openat2(parentFD, base, how)
	if err != nil && openat2Unavailable(err) {
		fd, err = unix.Openat(parentFD, base, flags|unix.O_CLOEXEC|unix.O_NOFOLLOW, mode)
	}
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), path), nil
}

func openDirectoryNoFollow(path string) (*os.File, error) {
	fd, err := openDirFD(filepath.Clean(path), unix.O_RDONLY)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), path), nil
}

// openDirFD opens a directory without following symlinks in any component.
func openDirFD(path string, access int) (int, error) {
	how := &unix.OpenHow{
		Flags:   uint64(access | unix.O_DIRECTORY | unix.O_CLOEXEC | unix.O_NOFOLLOW),
		Resolve: unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_NO_MAGICLINKS,
	}
	fd, err := unix.Openat2(unix.AT_FDCWD, path, how)
	if err == nil || !openat2Unavailable(err) {
		return fd, err
	}
	return walkDirNoFollow(path, access)
}

func walkDirNoFollow(path string, access int) (int, error) {
	start := "."
	if filepath.IsAbs(path) {
		start = "/"
	}
	dirFlags := unix.O_DIRECTORY | unix.O_CLOEXEC | unix.O_NOFOLLOW
	fd, err := unix.Open(start, unix.O_PATH|dirFlags, 0)
	if err != nil {
		return -1, err
	}
	var components []string
	for _, component := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		if component != "" && component != "." {
			components = append(components, component)
		}
	}
	for index, component := range components {
		flags := unix.O_PATH | dirFlags
		if index == len(components)-1 {
			flags = access | dirFlags
		}
		next, err := unix.Openat(fd, component, flags, 0)
		unix.Close(fd)
		if err != nil {
			return -1, err
		}
		fd = next
	}
	if len(components) == 0 && access != unix.O_PATH {
		// The path was just "/" or ".".
		reopened, err := unix.Openat(fd, ".", access|dirFlags, 0)
		unix.Close(fd)
		if err != nil {
			return -1, err
		}
		fd = reopened
	}
	return fd, nil
}
