//go:build windows

package store

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

func snowJapanFileLinks(path string, info os.FileInfo) (count uint64, err error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	opened, err := file.Stat()
	if err != nil {
		return 0, err
	}
	if !os.SameFile(info, opened) {
		return 0, fmt.Errorf("database identity changed while checking links")
	}
	var details syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(syscall.Handle(file.Fd()), &details); err != nil {
		return 0, err
	}
	return uint64(details.NumberOfLinks), nil
}
