// Copyright 2026 Bob Benjy and contributors. Licensed under Apache-2.0. See LICENSE.

package store

// LASC holiday rows have no id field; the date is unique per row.
func init() { resourceIDFieldOverrides["holidays"] = "date" }
