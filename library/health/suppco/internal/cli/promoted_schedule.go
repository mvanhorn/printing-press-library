// Copyright 2026 Felix Banuchi and contributors. Licensed under Apache-2.0. See LICENSE.
// Generated command body replaced by a retained provider projection.

package cli

import (
	"encoding/json"

	"github.com/mvanhorn/printing-press-library/library/health/suppco/internal/provider"

	"github.com/spf13/cobra"
)

func newSchedulePromotedCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "schedule <date>",
		Short:       "Show minimized provider activities and scheduled products for one ISO calendar date.",
		Example:     "  suppco-pp-cli schedule 2026-01-15",
		Args:        suppCoDateArgs(flags),
		Annotations: map[string]string{"pp:endpoint": "schedule.show", "pp:method": "GET", "pp:path": "/api/schedules/{date}", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := provider.ValidateDate(args[0]); err != nil {
				return usageErr(err)
			}
			service, err := newSuppCoProvider(flags)
			if err != nil {
				return err
			}
			projection, err := service.Schedule(cmd.Context(), args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(projection)
		},
	}
}
