// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored waitlist mutation gate. Live join/check-in/cancel must not
// POST unless --yes is set. --dry-run previews the request and does not POST.
// Default (neither flag) refuses. Matches the printed-CLI --yes/--dry-run
// gate used by sibling CLIs (see clarify confirmDestructive).

package cli

import (
	"fmt"
)

// confirmWaitlistMutation gates live waitlist POSTs behind --yes.
// --dry-run is allowed through so the client can print the body without sending.
func confirmWaitlistMutation(flags *rootFlags) error {
	if flags.dryRun || flags.yes {
		return nil
	}
	return usageErr(fmt.Errorf("destructive operation requires --yes (or --dry-run to preview the request)"))
}
