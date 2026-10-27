//go:build windows

// Copyright 2026 Rick van de Laar and contributors. Licensed under Apache-2.0. See LICENSE.

package ghfetch

// processUmask is zero because Windows has no process umask. StreamToFile
// still chmods the temp file to 0666, the mode a plain os.Create would
// request. os.Chmod maps that write bit onto the read-only attribute.
var processUmask = 0
