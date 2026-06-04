// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newGorgiasJobsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "gorgias-jobs",
		Short:  "Schedule and track async Gorgias jobs (bulk exports, macro applies)",
		Hidden: true,
	}

	cmd.AddCommand(newGorgiasJobsCreateCmd(flags))
	cmd.AddCommand(newGorgiasJobsDeleteCmd(flags))
	cmd.AddCommand(newGorgiasJobsGetCmd(flags))
	cmd.AddCommand(newGorgiasJobsListCmd(flags))
	cmd.AddCommand(newGorgiasJobsUpdateCmd(flags))
	return cmd
}
