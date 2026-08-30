// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/bestfoodtrucks/internal/graphqlclient"
	"github.com/spf13/cobra"
)

func newNovelTruckCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "truck",
		Short:       "Look up food truck metadata and schedules",
		Example:     "  bestfoodtrucks-pp-cli truck get 11869\n  bestfoodtrucks-pp-cli truck schedule 11869",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelTruckGetCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelTruckScheduleCmd(flags))
	return cmd
}

type GqlTruckGetResult struct {
	ID            GqlID             `json:"id"`
	Name          string            `json:"name"`
	AverageRating float64           `json:"averageRating"`
	RatingInfo    *GqlGetRatingInfo `json:"ratingInfo"`
}

type GqlGetRatingInfo struct {
	AverageRating float64 `json:"averageRating"`
	RatingsCount  int     `json:"ratingsCount"`
}

func newNovelTruckGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get basic metadata for a food truck by its numeric ID",
		Long:  "Get basic metadata for a food truck by its numeric ID, including rating statistics.",
		Example: strings.Trim(`
  bestfoodtrucks-pp-cli truck get 11869
  bestfoodtrucks-pp-cli truck get 11869 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "truck get")
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

			client := graphqlclient.New(flags.timeout)
			query := `
				query GetTruck($id: Int!) {
					truck(id: $id) {
						id
						name
						averageRating
						ratingInfo {
							averageRating
							ratingsCount
						}
					}
				}
			`
			var result struct {
				Truck *GqlTruckGetResult `json:"truck"`
			}
			err = client.Query(ctx, query, map[string]any{"id": id}, &result)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), fmt.Errorf("fetching truck: %w", err), flags)
			}
			if result.Truck == nil {
				return notFoundErr(fmt.Errorf("truck not found: %d", id))
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				w := cmd.OutOrStdout()
				fmt.Fprintf(w, "%s (%s)\n", bold(result.Truck.Name), result.Truck.ID.String())
				ratingVal := result.Truck.AverageRating
				ratingsCount := 0
				if result.Truck.RatingInfo != nil {
					ratingVal = result.Truck.RatingInfo.AverageRating
					ratingsCount = result.Truck.RatingInfo.RatingsCount
				}
				if ratingsCount > 0 {
					fmt.Fprintf(w, "Rating: %.1f★ (%d reviews)\n", ratingVal, ratingsCount)
				} else {
					fmt.Fprintf(w, "Rating: N/A\n")
				}
				return nil
			}

			return printJSONFiltered(cmd.OutOrStdout(), result.Truck, flags)
		},
	}
	return cmd
}
