// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

//go:build windows

package cli

import (
	"errors"
	"fmt"
	"runtime"
	"sync/atomic"

	"golang.org/x/sys/windows"
)

var placementInProcess atomic.Bool

// A global named mutex has the same scope as the HKCU registry reservation.
// USERPROFILE can differ between processes under one account, so a lock file
// beneath that directory would allow concurrent paid POSTs.
func acquirePlacementLock(string) (func(), error) {
	// Windows mutexes are recursive for the owning thread. The process guard
	// also rejects a second checkout from the same process.
	if !placementInProcess.CompareAndSwap(false, true) {
		return nil, fmt.Errorf("another checkout is already in progress (checkout mutex held); wait for it to finish")
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		placementInProcess.Store(false)
		return nil, err
	}
	name, err := windows.UTF16PtrFromString(`Global\PrintingPress.OrderToGo.Checkout.` + user.User.Sid.String())
	if err != nil {
		placementInProcess.Store(false)
		return nil, err
	}
	handle, err := windows.CreateMutex(nil, false, name)
	// x/sys/windows returns ERROR_ALREADY_EXISTS together with a valid
	// handle when another process created the mutex first. Waiting on that
	// same handle is what gives us the cross-process exclusion.
	if handle == 0 || (err != nil && !errors.Is(err, windows.ERROR_ALREADY_EXISTS)) {
		if handle != 0 {
			_ = windows.CloseHandle(handle)
		}
		placementInProcess.Store(false)
		if err == nil {
			return nil, fmt.Errorf("Windows checkout mutex returned an invalid handle")
		}
		return nil, err
	}
	// A Windows mutex is owned by an OS thread. Keep this goroutine on that
	// thread until ReleaseMutex, including while the provider POST is in flight.
	runtime.LockOSThread()
	state, err := windows.WaitForSingleObject(handle, 0)
	if err != nil {
		_ = windows.CloseHandle(handle)
		runtime.UnlockOSThread()
		placementInProcess.Store(false)
		return nil, err
	}
	if state != windows.WAIT_OBJECT_0 && state != windows.WAIT_ABANDONED {
		_ = windows.CloseHandle(handle)
		runtime.UnlockOSThread()
		placementInProcess.Store(false)
		return nil, fmt.Errorf("another checkout is already in progress (checkout mutex held); wait for it to finish")
	}
	return func() {
		_ = windows.ReleaseMutex(handle)
		_ = windows.CloseHandle(handle)
		runtime.UnlockOSThread()
		placementInProcess.Store(false)
	}, nil
}
