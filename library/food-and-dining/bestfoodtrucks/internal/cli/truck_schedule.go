// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/bestfoodtrucks/internal/graphqlclient"
	"github.com/spf13/cobra"
)

type GqlTruckScheduleResult struct {
	ID        GqlID               `json:"id"`
	Name      string              `json:"name"`
	Locations *GqlTruckLocWrapper `json:"locations"`
}

type GqlTruckLocWrapper struct {
	Records []GqlTruckLocRecord `json:"records"`
}

type GqlTruckLocRecord struct {
	ID              GqlID           `json:"id"`
	StartTime       string          `json:"startTime"`
	EndTime         string          `json:"endTime"`
	WorkStatusHuman string          `json:"workStatusHuman"`
	Lot             *GqlScheduleLot `json:"lot"`
}

type GqlScheduleLot struct {
	ID      GqlID  `json:"id"`
	Name    string `json:"name"`
	LotPath string `json:"lotPath"`
}

func newNovelTruckScheduleCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schedule [id]",
		Short: "Shows every lot a specific truck visits, past and future.",
		Long:  "Shows every lot a specific truck visits, past and future — a view the Best Food Trucks website itself never built.\nNote that the schedule list includes both past and future scheduled visits.",
		Example: strings.Trim(`
  bestfoodtrucks-pp-cli truck schedule 11869
  bestfoodtrucks-pp-cli truck schedule 11869 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "id=11869"},
		Args:        cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "truck schedule")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("id is required"))
			}
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return usageErr(fmt.Errorf("invalid truck id %q: must be an integer", args[0]))
			}

			return runTruckSchedule(ctx, cmd, flags, id)
		},
	}
	return cmd
}

func runTruckSchedule(ctx context.Context, cmd *cobra.Command, flags *rootFlags, id int) error {
	client := graphqlclient.New(flags.timeout)
	query := `
		query GetTruckSchedule($id: Int!) {
			truck(id: $id) {
				id
				name
				locations {
					records {
						id
						startTime
						endTime
						workStatusHuman
						lot {
							id
							name
							lotPath
						}
					}
				}
			}
		}
	`
	var result struct {
		Truck *GqlTruckScheduleResult `json:"truck"`
	}
	err := client.Query(ctx, query, map[string]any{"id": id}, &result)
	if err != nil {
		return classifyAPIError(cmd.OutOrStdout(), fmt.Errorf("fetching truck schedule: %w", err), flags)
	}
	if result.Truck == nil {
		return notFoundErr(fmt.Errorf("truck not found: %d", id))
	}

	// Sort output by startTime ascending
	if result.Truck.Locations != nil && len(result.Truck.Locations.Records) > 0 {
		sort.Slice(result.Truck.Locations.Records, func(i, j int) bool {
			t1, err1 := time.Parse(time.RFC3339, result.Truck.Locations.Records[i].StartTime)
			t2, err2 := time.Parse(time.RFC3339, result.Truck.Locations.Records[j].StartTime)
			if err1 != nil || err2 != nil {
				return result.Truck.Locations.Records[i].StartTime < result.Truck.Locations.Records[j].StartTime
			}
			return t1.Before(t2)
		})
	}

	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		w := cmd.OutOrStdout()
		fmt.Fprintf(w, "Schedule for %s (%s):\n", bold(result.Truck.Name), result.Truck.ID.String())
		fmt.Fprintln(w, strings.Repeat("-", 50))
		if result.Truck.Locations == nil || len(result.Truck.Locations.Records) == 0 {
			fmt.Fprintln(w, "No scheduled visits found.")
			return nil
		}
		for _, rec := range result.Truck.Locations.Records {
			lotName := "Unknown Lot"
			if rec.Lot != nil {
				lotName = rec.Lot.Name
			}
			// Parse human date
			dateStr := rec.StartTime
			t, err := time.Parse(time.RFC3339, rec.StartTime)
			if err == nil {
				dateStr = t.Format("Jan 02, 2006 (Mon)")
			}
			timeRange := formatTimeRange(rec.StartTime, rec.EndTime)
			fmt.Fprintf(w, "  %s: %s @ %s [%s]\n", dateStr, timeRange, green(lotName), rec.WorkStatusHuman)
		}
		return nil
	}

	return printJSONFiltered(cmd.OutOrStdout(), result.Truck, flags)
}
