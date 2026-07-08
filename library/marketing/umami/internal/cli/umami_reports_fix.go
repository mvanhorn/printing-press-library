// Copyright 2026 David Barbier and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: Umami v3's report-runner endpoints require `filters` to be
// present as a JSON object (zod rejects undefined). The generated commands
// only include --filters when set, so this decorator defaults it to {} on
// every reports run-* subcommand.

package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

func fixReportRunnerFilters(reportsCmd *cobra.Command) {
	for _, child := range reportsCmd.Commands() {
		if !strings.HasPrefix(child.Name(), "run-") {
			continue
		}
		flag := child.Flags().Lookup("filters")
		if flag == nil || child.RunE == nil {
			continue
		}
		orig := child.RunE
		child.RunE = func(cmd *cobra.Command, args []string) error {
			f := cmd.Flags().Lookup("filters")
			if f != nil && !f.Changed {
				_ = f.Value.Set("{}")
			}
			return orig(cmd, args)
		}
	}
}
