// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "github.com/mvanhorn/printing-press-library/library/media-and-entertainment/myanimelist/internal/cliutil"

// isDogfoodEnv reports whether the live dogfood matrix set its marker.
func isDogfoodEnv() bool { return cliutil.IsDogfoodEnv() }
