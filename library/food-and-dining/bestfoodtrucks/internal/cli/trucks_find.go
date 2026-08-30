// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/bestfoodtrucks/internal/graphqlclient"
	"github.com/spf13/cobra"
)

type ShiftFoodTypesResult struct {
	LocationID int
	FoodTypes  []GqlFoodType
	Error      error
}

type CuisineMatchResult struct {
	ID        int    `json:"id"`
	Date      string `json:"date"`
	TruckName string `json:"truckName"`
	Cuisine   string `json:"cuisine"`
	TimeRange string `json:"timeRange"`
}

type FindCuisineEnvelope struct {
	ScannedShifts int                  `json:"scanned_shifts"`
	FetchFailures []string             `json:"fetch_failures,omitempty"`
	Results       []CuisineMatchResult `json:"results"`
	Note          string               `json:"note,omitempty"`
}

func newNovelTrucksFindCmd(flags *rootFlags) *cobra.Command {
	var flagCuisine string
	var flagLot string
	var days int

	cmd := &cobra.Command{
		Use:   "find",
		Short: "Finds every upcoming shift at a lot matching a cuisine, without opening each shift page one at a time.",
		Long:  "Finds every upcoming shift at a lot matching a cuisine, walking the schedule window and checking nested menu food types.",
		Example: strings.Trim(`
  bestfoodtrucks-pp-cli trucks find --cuisine Thai --lot playa-district
  bestfoodtrucks-pp-cli trucks find --cuisine Thai --lot playa-district --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "trucks find")
			}
			if flagCuisine == "" || flagLot == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("both --cuisine and --lot are required"))
			}
			if days < 1 || days > 30 {
				return usageErr(fmt.Errorf("--days must be between 1 and 30, got %d", days))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			client := graphqlclient.New(flags.timeout)

			// Step 1: Get the lot's schedule
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
			var schedRes struct {
				Lot *GqlLotScheduleResult `json:"lot"`
			}
			err := client.Query(ctx, query, map[string]any{"seoName": flagLot, "days": days}, &schedRes)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), fmt.Errorf("fetching lot schedule: %w", err), flags)
			}
			if schedRes.Lot == nil {
				return notFoundErr(fmt.Errorf("lot not found: %s", flagLot))
			}

			// Gather all location/shift records from schedule
			var shiftIDs []int
			shiftsMap := make(map[int]GqlLocation)
			var shiftDatesMap = make(map[int]string)

			for _, sch := range schedRes.Lot.LocationSchedule {
				for _, loc := range sch.Locations {
					locID := loc.ID.Int()
					shiftIDs = append(shiftIDs, locID)
					shiftsMap[locID] = loc
					shiftDatesMap[locID] = sch.DateAlias
				}
			}

			if len(shiftIDs) == 0 {
				note := fmt.Sprintf("No upcoming shifts scheduled at %s in the next %d days to scan.", schedRes.Lot.Name, days)
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), FindCuisineEnvelope{
						ScannedShifts: 0,
						Results:       []CuisineMatchResult{},
						Note:          note,
					}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), note)
				return nil
			}

			// Step 2: Bounded-concurrency fan-out to fetch menu foodTypes for each shift
			concurrencyLimit := 5
			sem := make(chan struct{}, concurrencyLimit)
			resultsChan := make(chan ShiftFoodTypesResult, len(shiftIDs))

			for _, id := range shiftIDs {
				go func(shiftID int) {
					sem <- struct{}{}
					defer func() { <-sem }()

					qMenu := `
						query GetLocationMenu($id: Int!) {
							location(id: $id) {
								id
								menu {
									id
									foodTypes {
										id
										name
									}
								}
							}
						}
					`
					var res struct {
						Location *struct {
							ID   int `json:"id"`
							Menu *struct {
								ID        int           `json:"id"`
								FoodTypes []GqlFoodType `json:"foodTypes"`
							} `json:"menu"`
						} `json:"location"`
					}

					err := client.Query(ctx, qMenu, map[string]any{"id": shiftID}, &res)
					if err != nil {
						resultsChan <- ShiftFoodTypesResult{LocationID: shiftID, Error: err}
						return
					}
					if res.Location == nil || res.Location.Menu == nil {
						resultsChan <- ShiftFoodTypesResult{LocationID: shiftID, FoodTypes: []GqlFoodType{}}
						return
					}
					resultsChan <- ShiftFoodTypesResult{LocationID: shiftID, FoodTypes: res.Location.Menu.FoodTypes}
				}(id)
			}

			var fetchFailures []string
			matches := make([]CuisineMatchResult, 0)
			scanned := 0

			for i := 0; i < len(shiftIDs); i++ {
				res := <-resultsChan
				scanned++

				if res.Error != nil {
					fetchFailures = append(fetchFailures, fmt.Sprintf("shift %d: %v", res.LocationID, res.Error))
					continue
				}

				locInfo := shiftsMap[res.LocationID]
				dateAlias := shiftDatesMap[res.LocationID]

				for _, ft := range res.FoodTypes {
					if strings.Contains(strings.ToLower(ft.Name), strings.ToLower(flagCuisine)) {
						truckName := "Unknown Truck"
						if locInfo.Truck != nil {
							truckName = locInfo.Truck.Name
						}
						matches = append(matches, CuisineMatchResult{
							ID:        res.LocationID,
							Date:      dateAlias,
							TruckName: truckName,
							Cuisine:   ft.Name,
							TimeRange: formatTimeRange(locInfo.StartTime, locInfo.EndTime),
						})
						break
					}
				}
			}

			// Handle partial failure warning on stderr in human mode
			if len(fetchFailures) > 0 && !flags.asJSON {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to fetch menu for %d shifts\n", len(fetchFailures))
			}

			var note string
			if len(matches) == 0 {
				note = fmt.Sprintf("No upcoming shifts found matching cuisine %q at %s in the %d-day window. Try increasing --days (e.g. --days 10) to search a wider schedule window.", flagCuisine, schedRes.Lot.Name, days)
			}

			if flags.asJSON {
				envelope := FindCuisineEnvelope{
					ScannedShifts: scanned,
					FetchFailures: fetchFailures,
					Results:       matches,
					Note:          note,
				}
				return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
			}

			w := cmd.OutOrStdout()
			if len(matches) == 0 {
				fmt.Fprintln(w, note)
				return nil
			}

			fmt.Fprintf(w, "Matching upcoming %q shifts at %s:\n", flagCuisine, bold(schedRes.Lot.Name))
			fmt.Fprintln(w, strings.Repeat("-", 60))
			for _, m := range matches {
				fmt.Fprintf(w, "  - %s: %s (Cuisine: %s, Shift ID: %d, Time: %s)\n", bold(m.Date), green(m.TruckName), yellow(m.Cuisine), m.ID, m.TimeRange)
			}

			return nil
		},
	}
	cmd.Flags().StringVar(&flagCuisine, "cuisine", "", "Cuisine name to search for (case-insensitive)")
	cmd.Flags().StringVar(&flagLot, "lot", "", "Lot seoName slug")
	cmd.Flags().IntVar(&days, "days", 5, "Number of days of schedule to scan")
	return cmd
}
