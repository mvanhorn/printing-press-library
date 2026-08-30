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

type GqlShiftResult struct {
	ID              GqlID             `json:"id"`
	StartTime       string            `json:"startTime"`
	EndTime         string            `json:"endTime"`
	WorkStatusHuman string            `json:"workStatusHuman"`
	AllowOrders     bool              `json:"allowOrders"`
	CustomerUrl     string            `json:"customerUrl"`
	Address         string            `json:"address"`
	Active          bool              `json:"active"`
	Truck           *GqlShiftTruck    `json:"truck"`
	Menu            *GqlShiftMenu     `json:"menu"`
	LocationItems   []GqlLocationItem `json:"locationItems"`
}

type GqlShiftTruck struct {
	ID            GqlID          `json:"id"`
	Name          string         `json:"name"`
	AverageRating float64        `json:"averageRating"`
	RatingInfo    *GqlRatingInfo `json:"ratingInfo"`
}

type GqlRatingInfo struct {
	AverageRating float64 `json:"averageRating"`
	RatingsCount  int     `json:"ratingsCount"`
}

type GqlShiftMenu struct {
	ID        GqlID         `json:"id"`
	FoodTypes []GqlFoodType `json:"foodTypes"`
}

type GqlFoodType struct {
	ID   GqlID  `json:"id"`
	Name string `json:"name"`
}

type GqlLocationItem struct {
	ID     GqlID        `json:"id"`
	Status GqlID        `json:"status"`
	Item   *GqlItemInfo `json:"item"`
}

type GqlItemInfo struct {
	ID                     GqlID        `json:"id"`
	Name                   string       `json:"name"`
	Description            string       `json:"description"`
	Active                 bool         `json:"active"`
	Tags                   []GqlItemTag `json:"tags"`
	Price                  *GqlPrice    `json:"price"`
	HasSpecialInstructions bool         `json:"hasSpecialInstructions"`
}

type GqlPrice struct {
	Cents     int    `json:"cents"`
	Formatted string `json:"formatted"`
}

type GqlItemTag struct {
	ID    GqlID  `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

func newNovelShiftCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "shift",
		Short:       "Look up shift/location details, menus, and item tags",
		Example:     "  bestfoodtrucks-pp-cli shift get 179609",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelShiftGetCmd(flags))
	return cmd
}

func newNovelShiftGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get details and menu for a specific shift",
		Long:  "Get full details for a scheduled shift, including its hours, the operating food truck's rating, and the full menu with prices and item tags.",
		Example: strings.Trim(`
  bestfoodtrucks-pp-cli shift get 179609
  bestfoodtrucks-pp-cli shift get 179609 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "shift get")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("id is required"))
			}

			id, err := strconv.Atoi(args[0])
			if err != nil {
				return usageErr(fmt.Errorf("invalid shift id %q: must be an integer", args[0]))
			}

			client := graphqlclient.New(flags.timeout)
			query := `
				query GetShift($id: Int!) {
					location(id: $id) {
						id
						startTime
						endTime
						workStatusHuman
						allowOrders
						customerUrl
						address
						active
						truck {
							id
							name
							averageRating
							ratingInfo {
								averageRating
								ratingsCount
							}
						}
						menu {
							id
							foodTypes {
								id
								name
							}
						}
						locationItems {
							id
							status
							item {
								id
								name
								description
								active
								tags {
									id
									name
									color
								}
								price {
									cents
									formatted
								}
								hasSpecialInstructions
							}
						}
					}
				}
			`
			var result struct {
				Location *GqlShiftResult `json:"location"`
			}
			err = client.Query(ctx, query, map[string]any{"id": id}, &result)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), fmt.Errorf("fetching shift: %w", err), flags)
			}
			if result.Location == nil {
				return notFoundErr(fmt.Errorf("shift not found: %d", id))
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				w := cmd.OutOrStdout()
				loc := result.Location
				fmt.Fprintf(w, "%s (Shift ID: %s)\n", bold("SHIFT DETAILS"), loc.ID.String())
				fmt.Fprintln(w, strings.Repeat("-", 40))

				if loc.Truck != nil {
					fmt.Fprintf(w, "Truck:        %s\n", green(loc.Truck.Name))
					ratingVal := loc.Truck.AverageRating
					ratingsCount := 0
					if loc.Truck.RatingInfo != nil {
						ratingVal = loc.Truck.RatingInfo.AverageRating
						ratingsCount = loc.Truck.RatingInfo.RatingsCount
					}
					if ratingsCount > 0 {
						fmt.Fprintf(w, "Rating:       %.1f★ (%d reviews)\n", ratingVal, ratingsCount)
					} else {
						fmt.Fprintf(w, "Rating:       N/A\n")
					}
				}

				fmt.Fprintf(w, "Hours:        %s\n", formatTimeRange(loc.StartTime, loc.EndTime))
				fmt.Fprintf(w, "Work Status:  %s\n", loc.WorkStatusHuman)
				fmt.Fprintf(w, "Allow Orders: %t\n", loc.AllowOrders)
				if loc.CustomerUrl != "" {
					fmt.Fprintf(w, "Customer URL: %s\n", loc.CustomerUrl)
				}
				fmt.Println()

				if loc.Menu != nil && len(loc.Menu.FoodTypes) > 0 {
					var ftNames []string
					for _, ft := range loc.Menu.FoodTypes {
						ftNames = append(ftNames, ft.Name)
					}
					fmt.Fprintf(w, "Food Types: %s\n", strings.Join(ftNames, ", "))
				}

				fmt.Fprintln(w, bold("\nMENU ITEMS:"))
				if len(loc.LocationItems) == 0 {
					fmt.Fprintln(w, "  No items available for this shift.")
				} else {
					for _, li := range loc.LocationItems {
						if li.Item == nil {
							continue
						}
						it := li.Item
						priceStr := "N/A"
						if it.Price != nil {
							priceStr = it.Price.Formatted
						}
						fmt.Fprintf(w, "  - %s (%s)\n", bold(it.Name), green(priceStr))
						if len(it.Tags) > 0 {
							var tagNames []string
							for _, t := range it.Tags {
								tagNames = append(tagNames, t.Name)
							}
							fmt.Fprintf(w, "    Tags: %s\n", yellow(strings.Join(tagNames, ", ")))
						}
						if it.Description != "" {
							fmt.Fprintf(w, "    Description: %s\n", it.Description)
						}
					}
				}
				return nil
			}

			return printJSONFiltered(cmd.OutOrStdout(), result.Location, flags)
		},
	}
	return cmd
}
