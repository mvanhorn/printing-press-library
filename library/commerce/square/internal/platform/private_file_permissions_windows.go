//go:build windows

// Copyright 2026 matthew.martin and contributors. Licensed under Apache-2.0. See LICENSE.

package platform

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// securePrivateFile applies the Windows equivalent of mode 0600: the current
// user owns the file and inherited access-control entries are removed.
func securePrivateFile(path string) error {
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return fmt.Errorf("open process token: %w", err)
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		return fmt.Errorf("get token user: %w", err)
	}
	sid := user.User.Sid.String()
	descriptor, err := windows.SecurityDescriptorFromString(fmt.Sprintf("O:%sD:PAI(A;;FA;;;%s)", sid, sid))
	if err != nil {
		return fmt.Errorf("build owner-only security descriptor: %w", err)
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		return fmt.Errorf("read owner-only DACL: %w", err)
	}
	owner, _, err := descriptor.Owner()
	if err != nil {
		return fmt.Errorf("read file owner: %w", err)
	}
	if err := windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.OWNER_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		owner,
		nil,
		dacl,
		nil,
	); err != nil {
		return fmt.Errorf("apply owner-only file permissions: %w", err)
	}
	return nil
}
