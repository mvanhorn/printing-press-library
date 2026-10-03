//go:build windows

// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package hostelworld

import (
	"golang.org/x/sys/windows"
	"os"
)

func cacheLinkCount(path string, _ os.FileInfo) (uint64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(windows.Handle(f.Fd()), &info); err != nil {
		return 0, err
	}
	return uint64(info.NumberOfLinks), nil
}
