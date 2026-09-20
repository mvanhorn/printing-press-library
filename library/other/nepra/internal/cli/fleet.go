// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// HAND-CODED. The `fleet` scaffold has been implemented; this file is now only
// the registration seam root.go:461 already calls, and the body lives in
// nepra_fleet.go / nepra_fleet_render.go so a `generate --force` cannot refresh
// it away as an untouched TODO.
// pp:data-source local
// Computed entirely from the embedded crosswalk: no request, no store read.
// `--data-source live` is refused (exit 2) rather than silently ignored.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelFleetCmd(flags *rootFlags) *cobra.Command {
	return newNepraFleetCmd(flags)
}
