//go:build windows

package client

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

func lockCreditJournal(f *os.File) (func() error, error) {
	handle := windows.Handle(f.Fd())
	var overlapped windows.Overlapped
	if err := windows.LockFileEx(handle, windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, &overlapped); err != nil {
		return nil, err
	}
	return func() error { return windows.UnlockFileEx(handle, 0, 1, 0, &overlapped) }, nil
}

func creditJournalIdentity(f *os.File) (string, error) {
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(windows.Handle(f.Fd()), &info); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d:%d", info.VolumeSerialNumber, uint64(info.FileIndexHigh)<<32|uint64(info.FileIndexLow)), nil
}

// Windows' FileMode permission bits do not represent the file DACL: private
// writable files commonly report 0666. Inspect the actual access rules and
// reject any allow entry for a principal outside the owner, SYSTEM and local
// Administrators. Unknown ACE forms fail closed.
func creditJournalPrivate(f *os.File, _ os.FileInfo) error {
	var fileInfo windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(windows.Handle(f.Fd()), &fileInfo); err != nil {
		return err
	}
	if fileInfo.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return fmt.Errorf("journal is a reparse point")
	}
	sd, err := windows.GetSecurityInfo(windows.Handle(f.Fd()), windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	owner, _, err := sd.Owner()
	if err != nil || owner == nil {
		return fmt.Errorf("journal owner unavailable: %w", err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	acl, _, err := sd.DACL()
	if err != nil || acl == nil {
		return fmt.Errorf("journal DACL unavailable or unrestricted: %w", err)
	}
	rules := make([]creditACLRule, 0, acl.AceCount)
	for i := uint16(0); i < acl.AceCount; i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(acl, uint32(i), &ace); err != nil {
			return err
		}
		if ace == nil {
			return fmt.Errorf("null journal ACE")
		}
		if ace.Header.AceType == windows.ACCESS_DENIED_ACE_TYPE {
			rules = append(rules, creditACLRule{deny: true})
			continue
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			rules = append(rules, creditACLRule{})
			continue
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		rules = append(rules, creditACLRule{allow: true, trusted: sid.IsValid() &&
			(sid.Equals(owner) || sid.IsWellKnown(windows.WinLocalSystemSid) ||
				sid.IsWellKnown(windows.WinBuiltinAdministratorsSid) || sid.IsWellKnown(windows.WinCreatorOwnerSid))})
	}
	return creditACLPrivate(owner.Equals(user.User.Sid), true, rules)
}
