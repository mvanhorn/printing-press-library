// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
// pp:client-call
// Shared Haneda configuration calls the bounded first-party client at execution.
package cli

import (
	"github.com/mvanhorn/printing-press-library/library/travel/haneda-airport/internal/haneda"
	"github.com/spf13/cobra"
)

func newNovelPlanCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "plan [flight-or-id]"}
	q := haneda.Query{}
	hanedaQueryFlags(cmd, &q, true)
	return configureHanedaDetailCmd(cmd, flags, true, &q)
}
