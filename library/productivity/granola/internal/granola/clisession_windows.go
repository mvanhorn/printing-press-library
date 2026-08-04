// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0.

//go:build windows

package granola

import "os"

// checkOwnerOnly is a no-op on Windows. Go's os.FileMode does not carry NTFS
// ACLs, so the Unix permission bits are not meaningful here and enforcing them
// would reject every legitimate session file. Windows access control for this
// path is left to the user profile directory's own ACLs.
func checkOwnerOnly(info os.FileInfo, path string) error { return nil }

// syncDir is a no-op on Windows and reports success. Directory handles are not
// syncable there the way they are on Unix; durability of a newly created entry
// comes from the file's own FlushFileBuffers, which callers perform before
// reaching here. Returning nil is therefore accurate rather than optimistic --
// there is no pending directory flush left outstanding.
func syncDir(dir string) error { return nil }
