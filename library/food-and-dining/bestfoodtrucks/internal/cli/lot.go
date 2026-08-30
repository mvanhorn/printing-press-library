// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/bestfoodtrucks/internal/graphqlclient"
	"github.com/spf13/cobra"
)

// GqlID handles both JSON numbers and JSON strings for IDs seamlessly.
type GqlID string

func (id *GqlID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*id = GqlID(s)
		return nil
	}
	var n int
	if err := json.Unmarshal(data, &n); err == nil {
		*id = GqlID(strconv.Itoa(n))
		return nil
	}
	*id = GqlID(strings.Trim(string(data), `"`))
	return nil
}

func (id GqlID) MarshalJSON() ([]byte, error) {
	if n, err := strconv.Atoi(string(id)); err == nil {
		return json.Marshal(n)
	}
	return json.Marshal(string(id))
}

func (id GqlID) String() string {
	return string(id)
}

func (id GqlID) Int() int {
	n, _ := strconv.Atoi(string(id))
	return n
}

func newNovelLotCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "lot",
		Short:       "Look up lot details and schedules",
		Example:     "  bestfoodtrucks-pp-cli lot get playa-district\n  bestfoodtrucks-pp-cli lot schedule playa-district",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelLotGetCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelLotScheduleCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelLotDigestCmd(flags))
	return cmd
}

type LotResult struct {
	ID          GqlID  `json:"id"`
	Name        string `json:"name"`
	FullAddress string `json:"fullAddress"`
	Facebook    string `json:"facebook"`
	Instagram   string `json:"instagram"`
	Twitter     string `json:"twitter"`
	Tiktok      string `json:"tiktok"`
	Website     string `json:"website"`
	Subscribed  bool   `json:"subscribed"`
	Active      bool   `json:"active"`
}

func newNovelLotGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <seoName>",
		Short: "Get basic information about a lot by its slug",
		Long:  "Get detailed metadata about a food truck lot, including its address, social media links, and subscription status.",
		Example: strings.Trim(`
  bestfoodtrucks-pp-cli lot get playa-district
  bestfoodtrucks-pp-cli lot get playa-district --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "lot get")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("seoName is required"))
			}
			seoName := args[0]

			client := graphqlclient.New(flags.timeout)
			query := `
				query GetLot($seoName: String!) {
					lot(seoName: $seoName) {
						id
						name
						fullAddress
						facebook
						instagram
						twitter
						tiktok
						website
						subscribed
						active
					}
				}
			`
			var result struct {
				Lot *LotResult `json:"lot"`
			}
			err := client.Query(ctx, query, map[string]any{"seoName": seoName}, &result)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), fmt.Errorf("fetching lot: %w", err), flags)
			}
			if result.Lot == nil {
				return notFoundErr(fmt.Errorf("lot not found: %s", seoName))
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				w := cmd.OutOrStdout()
				fmt.Fprintf(w, "%s (%s)\n", bold(result.Lot.Name), result.Lot.ID.String())
				fmt.Fprintf(w, "Address:   %s\n", result.Lot.FullAddress)
				fmt.Fprintf(w, "Active:    %t\n", result.Lot.Active)
				fmt.Fprintf(w, "Subscribed: %t\n", result.Lot.Subscribed)
				if result.Lot.Website != "" {
					fmt.Fprintf(w, "Website:   %s\n", result.Lot.Website)
				}
				if result.Lot.Facebook != "" {
					fmt.Fprintf(w, "Facebook:  %s\n", result.Lot.Facebook)
				}
				if result.Lot.Instagram != "" {
					fmt.Fprintf(w, "Instagram: %s\n", result.Lot.Instagram)
				}
				if result.Lot.Twitter != "" {
					fmt.Fprintf(w, "Twitter:   %s\n", result.Lot.Twitter)
				}
				if result.Lot.Tiktok != "" {
					fmt.Fprintf(w, "TikTok:    %s\n", result.Lot.Tiktok)
				}
				return nil
			}

			return printJSONFiltered(cmd.OutOrStdout(), result.Lot, flags)
		},
	}
	return cmd
}

type GqlLotScheduleResult struct {
	ID               GqlID                 `json:"id"`
	Name             string                `json:"name"`
	LocationSchedule []GqlLocationSchedule `json:"locationSchedule"`
}

type GqlLocationSchedule struct {
	ID        GqlID         `json:"id"`
	DateAlias string        `json:"dateAlias"`
	Locations []GqlLocation `json:"locations"`
}

type GqlLocation struct {
	ID              GqlID     `json:"id"`
	StartTime       string    `json:"startTime"`
	EndTime         string    `json:"endTime"`
	WorkStatusHuman string    `json:"workStatusHuman"`
	AllowOrders     bool      `json:"allowOrders"`
	CustomerUrl     string    `json:"customerUrl"`
	Truck           *GqlTruck `json:"truck"`
}

type GqlTruck struct {
	ID   GqlID  `json:"id"`
	Name string `json:"name"`
}

func newNovelLotScheduleCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{
		Use:   "schedule <seoName>",
		Short: "Get upcoming schedule for a lot",
		Long:  "List all food trucks scheduled to visit a lot over the next N days.",
		Example: strings.Trim(`
  bestfoodtrucks-pp-cli lot schedule playa-district
  bestfoodtrucks-pp-cli lot schedule playa-district --days 7
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "lot schedule")
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
				return classifyAPIError(cmd.OutOrStdout(), fmt.Errorf("fetching lot: %w", err), flags)
			}
			if result.Lot == nil {
				return notFoundErr(fmt.Errorf("lot not found: %s", seoName))
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				w := cmd.OutOrStdout()
				fmt.Fprintf(w, "Schedule for %s:\n", bold(result.Lot.Name))
				fmt.Fprintln(w, strings.Repeat("-", 40))
				count := 0
				for _, sch := range result.Lot.LocationSchedule {
					if len(sch.Locations) == 0 {
						continue
					}
					fmt.Fprintf(w, "%s:\n", bold(sch.DateAlias))
					for _, loc := range sch.Locations {
						truckName := "Unknown Truck"
						if loc.Truck != nil {
							truckName = loc.Truck.Name
						}
						timeRange := formatTimeRange(loc.StartTime, loc.EndTime)
						fmt.Fprintf(w, "  - %s (%s) [%s]\n", green(truckName), timeRange, loc.WorkStatusHuman)
						count++
					}
				}
				if count == 0 {
					fmt.Fprintf(w, "No trucks currently scheduled.\n")
				}
				return nil
			}

			return printJSONFiltered(cmd.OutOrStdout(), result.Lot, flags)
		},
	}
	cmd.Flags().IntVar(&days, "days", 5, "Number of days of schedule to fetch")
	return cmd
}

func formatTimeRange(startStr, endStr string) string {
	start, err1 := time.Parse(time.RFC3339, startStr)
	end, err2 := time.Parse(time.RFC3339, endStr)
	if err1 != nil || err2 != nil {
		// Try parsing without timezone offset or fallback
		return fmt.Sprintf("%s - %s", startStr, endStr)
	}
	return fmt.Sprintf("%s - %s", start.Format("3:04 PM"), end.Format("3:04 PM"))
}
