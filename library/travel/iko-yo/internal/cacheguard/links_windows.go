//go:build windows

// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cacheguard

import (
	"os"

	"golang.org/x/sys/windows"
)

func linkCount(path string, _ os.FileInfo) (uint64, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	handle, err := windows.CreateFile(name, windows.FILE_READ_ATTRIBUTES, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return 0, err
	}
	var info windows.ByHandleFileInformation
	infoErr := windows.GetFileInformationByHandle(handle, &info)
	closeErr := windows.CloseHandle(handle)
	if infoErr != nil {
		return 0, infoErr
	}
	return uint64(info.NumberOfLinks), closeErr
}
