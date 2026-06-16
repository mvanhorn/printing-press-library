// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-ad-manager/internal/cliutil"
	"github.com/spf13/cobra"
)

// schedulePaceFraction returns the fraction of a line item's flight that has
// elapsed at now: 0.0 before it starts, 1.0 after it ends, and elapsed/total in
// between. This is SCHEDULE pace from the line item's own start/end metadata —
// "where on the calendar should delivery be" — with no delivery data joined in.
// A non-positive duration (end <= start) returns 0 to avoid a divide-by-zero and
// signals a degenerate flight to the caller.
func schedulePaceFraction(start, end, now time.Time) float64 {
	total := end.Sub(start)
	if total <= 0 {
		return 0
	}
	if now.Before(start) {
		return 0
	}
	if !now.Before(end) {
		return 1
	}
	return now.Sub(start).Seconds() / total.Seconds()
}

// lineItemPace is the per-line-item schedule view emitted by the command.
type lineItemPace struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	GoalUnits        int64   `json:"goal_units"`
	Start            string  `json:"start"`
	End              string  `json:"end"`
	ScheduleFraction float64 `json:"schedule_fraction"`
	// LineItemType is the GAM line-item type enum (e.g. STANDARD, SPONSORSHIP).
	// The LineItem schema exposes no delivery status field, so this stands in
	// for the line item's classification rather than a serving state.
	LineItemType string `json:"line_item_type"`
}

// lineItemFetchFailure tags a line item that could not be turned into a pace row
// (e.g. missing/unparseable start or end time) so it is surfaced explicitly
// rather than silently dropped or counted as a zero-progress phantom.
type lineItemFetchFailure struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// rawLineItem mirrors the subset of GoogleAdsAdmanagerV1__LineItem this command
// reads. startTime/endTime are google-datetime (RFC3339); goal.units is an
// int64 serialized as a string; name is the resource path
// networks/{code}/lineItems/{id}.
type rawLineItem struct {
	Name         string       `json:"name"`
	DisplayName  string       `json:"displayName"`
	StartTime    string       `json:"startTime"`
	EndTime      string       `json:"endTime"`
	LineItemType string       `json:"lineItemType"`
	Goal         lineItemGoal `json:"goal"`
}

// lineItemGoal mirrors GoogleAdsAdmanagerV1__Goal: units is an int64 serialized
// as a string.
type lineItemGoal struct {
	Units    string `json:"units"`
	GoalType string `json:"goalType"`
	UnitType string `json:"unitType"`
}

// classifyLineItems converts raw line items into ranked pace rows plus a
// fetch-failure list, evaluated at now. Rows are ranked by how far each is from
// being on schedule is left to the caller's display; here they are sorted by
// schedule fraction descending (closest to flight end first) for a stable,
// meaningful default order. A line item with an unparseable start or end time
// becomes a failure, never a phantom zero. Pure: no I/O, testable directly.
func classifyLineItems(items []rawLineItem, now time.Time) ([]lineItemPace, []lineItemFetchFailure) {
	var paces []lineItemPace
	var failures []lineItemFetchFailure
	for _, li := range items {
		id := lastPathSegment(li.Name)
		if li.StartTime == "" || li.EndTime == "" {
			failures = append(failures, lineItemFetchFailure{ID: id, Name: li.DisplayName, Reason: "missing start or end time"})
			continue
		}
		start, serr := time.Parse(time.RFC3339, li.StartTime)
		if serr != nil {
			failures = append(failures, lineItemFetchFailure{ID: id, Name: li.DisplayName, Reason: "unparseable startTime: " + li.StartTime})
			continue
		}
		end, eerr := time.Parse(time.RFC3339, li.EndTime)
		if eerr != nil {
			failures = append(failures, lineItemFetchFailure{ID: id, Name: li.DisplayName, Reason: "unparseable endTime: " + li.EndTime})
			continue
		}
		var units int64
		if li.Goal.Units != "" {
			if u, err := strconv.ParseInt(li.Goal.Units, 10, 64); err == nil {
				units = u
			}
		}
		paces = append(paces, lineItemPace{
			ID:               id,
			Name:             li.DisplayName,
			GoalUnits:        units,
			Start:            li.StartTime,
			End:              li.EndTime,
			ScheduleFraction: round4(schedulePaceFraction(start, end, now)),
			LineItemType:     li.LineItemType,
		})
	}
	sort.SliceStable(paces, func(i, j int) bool {
		return paces[i].ScheduleFraction > paces[j].ScheduleFraction
	})
	return paces, failures
}

// round4 trims a fraction to 4 decimal places for stable, readable output.
func round4(f float64) float64 { return math.Round(f*10000) / 10000 }

// lastPathSegment returns the final "/"-delimited segment, e.g. the line-item id
// out of networks/123/lineItems/456. Returns the input unchanged if there is no
// slash.
func lastPathSegment(s string) string {
	if i := strings.LastIndex(s, "/"); i >= 0 && i < len(s)-1 {
		return s[i+1:]
	}
	return s
}

// pp:data-source live -- fetches the order's line items directly from the GAM
// API and computes schedule pace from their metadata; not read from the mirror.
func newNovelLineitemPaceCmd(flags *rootFlags) *cobra.Command {
	var flagOrder string
	var flagDateRange string
	var flagNetwork string
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "pace",
		Short: "Rank an order's line items by how far each is from on-schedule, using line-item metadata only.",
		Long: `Fetch the line items under an order and rank them by SCHEDULE pace: the fraction
of each line item's flight (startTime..endTime) that has elapsed right now.

Scope: this is schedule pace computed purely from line-item metadata — start and
end times and the primary goal units. It does NOT join a delivery report, so it
answers "where on the calendar should this be" and not "how many impressions has
it actually served". A line item whose start/end can't be parsed is reported in
fetch_failures rather than shown as zero progress.

Output: {order, line_items:[{id,name,goal_units,start,end,schedule_fraction,line_item_type}],
fetch_failures:[{id,name,reason}]}.`,
		Example:     "  google-ad-manager-pp-cli lineitem pace --order 9876543",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch line items for --order and rank by schedule pace")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if strings.TrimSpace(flagOrder) == "" {
				return usageErr(fmt.Errorf("--order <order-id> is required"))
			}
			code, err := resolveNetworkCode(flagNetwork)
			if err != nil {
				return err
			}

			limit := flagLimit
			if cliutil.IsDogfoodEnv() && limit > 25 {
				limit = 25
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			parent := networkParent(code)
			orderName := parent + "/orders/" + strings.TrimSpace(flagOrder)

			// Filterable fields per spec include `order` (the full resource path);
			// there is no `orderId` filter field.
			params := map[string]string{
				"filter":   fmt.Sprintf("order = \"%s\"", orderName),
				"pageSize": fmt.Sprintf("%d", limit),
			}
			data, err := c.Get(ctx, "/v1/"+parent+"/lineItems", params)
			if err != nil {
				return apiErr(fmt.Errorf("listing line items for order %q: %w", flagOrder, err))
			}
			var resp struct {
				LineItems []rawLineItem `json:"lineItems"`
			}
			if err := json.Unmarshal(data, &resp); err != nil {
				return apiErr(fmt.Errorf("decoding line items: %w", err))
			}
			if len(resp.LineItems) > limit {
				resp.LineItems = resp.LineItems[:limit]
			}

			paces, failures := classifyLineItems(resp.LineItems, time.Now().UTC())

			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"order":          strings.TrimSpace(flagOrder),
				"line_items":     paces,
				"fetch_failures": failures,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&flagOrder, "order", "", "Order ID whose line items to pace. Required.")
	cmd.Flags().StringVar(&flagDateRange, "date-range", "", "Reserved for a future delivery-report join; ignored by schedule pace.")
	cmd.Flags().StringVar(&flagNetwork, "network", "", "GAM network code (else $GOOGLE_AD_MANAGER_NETWORK_CODE).")
	cmd.Flags().IntVar(&flagLimit, "limit", 100, "Maximum number of line items to fetch.")
	return cmd
}
