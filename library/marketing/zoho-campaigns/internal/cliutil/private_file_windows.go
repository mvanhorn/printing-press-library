//go:build windows

// Copyright 2026 Kent Martin and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored Windows companion to AtomicWritePrivateFile. Chmod(0600) is a
// no-op for NTFS DACLs, so a freshly written credentials file inherits the
// parent directory's ACL — which VerifyCredsPerms then rejects (inherited
// ACEs for broad or non-owner principals). This helper rewrites the file's
// security descriptor to a protected, owner-only DACL so the writer and the
// verifier agree.

package cliutil

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// RestrictPrivateFile sets the file's owner to the current user and replaces
// its DACL with a single protected allow-full-access ACE for that user.
func RestrictPrivateFile(path string) error {
	tok, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return fmt.Errorf("opening process token: %w", err)
	}
	defer tok.Close()
	u, err := tok.GetTokenUser()
	if err != nil {
		return fmt.Errorf("resolving current user SID: %w", err)
	}
	sid := u.User.Sid
	// D:P(...) — protected DACL (no inheritance), one full-access ACE for the owner.
	sd, err := windows.SecurityDescriptorFromString("D:P(A;;FA;;;" + sid.String() + ")")
	if err != nil {
		return fmt.Errorf("building owner-only security descriptor: %w", err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return fmt.Errorf("extracting owner-only DACL: %w", err)
	}
	err = windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION|windows.OWNER_SECURITY_INFORMATION,
		sid, nil, dacl, nil,
	)
	if err != nil {
		return fmt.Errorf("applying owner-only ACL: %w", err)
	}
	return nil
}
