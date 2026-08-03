// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0.

//go:build windows

package granola

import "os"

// checkOwnerOnly is a no-op on Windows. Go's os.FileMode does not carry NTFS
// ACLs, so the Unix permission bits are not meaningful here and enforcing them
// would reject every legitimate session file. Windows access control for this
// path is left to the user profile directory's own ACLs.
func checkOwnerOnly(info os.FileInfo, path string) error { return nil }

// syncDir is a no-op on Windows: directory handles are not syncable the way
// they are on Unix, and the rename above is already atomic on NTFS.
func syncDir(dir string) {}
