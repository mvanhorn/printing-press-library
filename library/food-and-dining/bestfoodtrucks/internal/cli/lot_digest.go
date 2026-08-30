// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/bestfoodtrucks/internal/graphqlclient"
	"github.com/spf13/cobra"
)

func newNovelLotDigestCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{
		Use:   "digest [seoName]",
		Short: "Turns a lot's upcoming schedule into ready-to-paste announcement text instead of raw structured data.",
		Long:  "Turns a lot's upcoming schedule into ready-to-paste announcement text instead of raw structured data.",
		Example: strings.Trim(`
  bestfoodtrucks-pp-cli lot digest playa-district
  bestfoodtrucks-pp-cli lot digest playa-district --days 7
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "seoName=playa-district"},
		Args:        cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "lot digest")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("seoName is required"))
			}
			if days < 1 || days > 30 {
				return usageErr(fmt.Errorf("--days must be between 1 and 30, got %d", days))
			}
			seoName := args[0]

			return runLotDigest(ctx, cmd, flags, seoName, days)
		},
	}
	cmd.Flags().IntVar(&days, "days", 5, "Number of days of schedule to fetch")
	return cmd
}

func runLotDigest(ctx context.Context, cmd *cobra.Command, flags *rootFlags, seoName string, days int) error {
	client := graphqlclient.New(flags.timeout)
	query := `
		query GetLotSchedule($seoName: String!, $days: Int!) {
			lot(seoName: $seoName) {
				id
				name
				locationSchedule(days: $days) {
					id
					dateAlias
					locations {
						id
						startTime
						endTime
						workStatusHuman
						allowOrders
						customerUrl
						truck {
							id
							name
						}
					}
				}
			}
		}
	`
	var result struct {
		Lot *GqlLotScheduleResult `json:"lot"`
	}
	err := client.Query(ctx, query, map[string]any{"seoName": seoName, "days": days}, &result)
	if err != nil {
		return classifyAPIError(cmd.OutOrStdout(), fmt.Errorf("fetching lot schedule: %w", err), flags)
	}
	if result.Lot == nil {
		return notFoundErr(fmt.Errorf("lot not found: %s", seoName))
	}

	if flags.asJSON {
		return printJSONFiltered(cmd.OutOrStdout(), result.Lot, flags)
	}

	fmt.Fprint(cmd.OutOrStdout(), formatLotDigest(result.Lot.Name, seoName, result.Lot.LocationSchedule))
	return nil
}

func formatLotDigest(lotName, seoName string, schedules []GqlLocationSchedule) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("This week at %s (%s):\n", lotName, seoName))

	count := 0
	for _, sch := range schedules {
		for _, loc := range sch.Locations {
			truckName := "Unknown Truck"
			if loc.Truck != nil {
				truckName = loc.Truck.Name
			}
			timeRange := formatTimeRange(loc.StartTime, loc.EndTime)
			sb.WriteString(fmt.Sprintf("  %s: %s (%s)\n", sch.DateAlias, truckName, timeRange))
			count++
		}
	}

	if count == 0 {
		return fmt.Sprintf("No trucks currently scheduled at %s (%s).\n", lotName, seoName)
	}
	return sb.String()
}
