//go:build windows

// Copyright 2026 Jet Sng and contributors. Licensed under Apache-2.0.
package store

import (
	"os"

	"golang.org/x/sys/windows"
)

// Windows Stat omits NumberOfLinks; request only read access to file attributes.
func wheelogFileHasSingleLink(path string, _ os.FileInfo) (bool, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false, err
	}
	handle, err := windows.CreateFile(name, windows.FILE_READ_ATTRIBUTES, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return false, err
	}
	var info windows.ByHandleFileInformation
	err = windows.GetFileInformationByHandle(handle, &info)
	closeErr := windows.CloseHandle(handle)
	if err != nil {
		return false, err
	}
	if closeErr != nil {
		return false, closeErr
	}
	return info.NumberOfLinks == 1, nil
}
