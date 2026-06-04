// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newSatisfactionSurveysCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "satisfaction-surveys",
		Short:  "CSAT survey definitions and customer ratings/comments",
		Hidden: true,
	}

	cmd.AddCommand(newSatisfactionSurveysCreateCmd(flags))
	cmd.AddCommand(newSatisfactionSurveysGetCmd(flags))
	cmd.AddCommand(newSatisfactionSurveysListCmd(flags))
	cmd.AddCommand(newSatisfactionSurveysUpdateCmd(flags))
	return cmd
}
