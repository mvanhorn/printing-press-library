// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package learn

import (
	"sync"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/cliutil"
)

var statePaths struct {
	sync.RWMutex
	dir string
}

// BindStateDir scopes learning artifacts to one verified CLI invocation.
// MCP mirrors run in separate child processes, each bound to its own profile.
func BindStateDir(dir string) func() {
	statePaths.Lock()
	previous := statePaths.dir
	statePaths.dir = dir
	statePaths.Unlock()
	return func() {
		statePaths.Lock()
		statePaths.dir = previous
		statePaths.Unlock()
	}
}

func boundStateDir() string {
	statePaths.RLock()
	defer statePaths.RUnlock()
	return statePaths.dir
}

func StateDir() (string, error) {
	if dir := boundStateDir(); dir != "" {
		return dir, nil
	}
	return cliutil.StateDir()
}
