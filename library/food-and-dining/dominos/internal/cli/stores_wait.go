package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/dominos/internal/client"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/dominos/internal/cliutil"

	"github.com/spf13/cobra"
)

// storeWaitRow is one ranked store-wait line emitted by `stores wait`.
type storeWaitRow struct {
	StoreID       string  `json:"store_id"`
	Address       string  `json:"address"`
	WaitMin       float64 `json:"wait_min"`
	ServiceMethod string  `json:"service_method"`
}

// newStoresWaitCmd ranks every store within the locator radius by its
// EstimatedWaitMinutes profile field. Reverse-engineered from the GraphQL
// CartEtaMinutes operation but uses the public REST profile endpoint so it
// works without auth.
func newStoresWaitCmd(flags *rootFlags) *cobra.Command {
	var address, city, svc string
	cmd := &cobra.Command{
		Use:   "wait",
		Short: "Rank nearby stores by estimated wait time",
		Long: `Rank nearby stores by estimated wait time.

Calls the public store-locator with the supplied address, then fans out to
each store's profile endpoint and sorts by EstimatedWaitMinutes.`,
		Example:     "  dominos-pp-cli stores wait --address \"421 N 63rd St\" --city \"Seattle, WA\"",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if address == "" {
				return usageErr(fmt.Errorf("--address is required"))
			}
			if city == "" {
				return usageErr(fmt.Errorf("--city is required (city, state, zip)"))
			}
			if svc == "" {
				svc = "Delivery"
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"action":  "stores_wait",
					"dry_run": true,
					"address": address,
					"city":    city,
					"type":    svc,
				}, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			locatorParams := map[string]string{"s": address, "c": city, "type": svc}
			locatorData, err := c.Get("/power/store-locator", locatorParams)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			storeIDs := extractStoreIDs(locatorData)
			if len(storeIDs) == 0 {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"results": []storeWaitRow{},
					"hint":    "no stores returned by locator; try a broader address",
				}, flags)
			}
			ctx := context.Background()
			results, errs := cliutil.FanoutRun(ctx, storeIDs,
				func(s string) string { return s },
				func(_ context.Context, storeID string) (storeWaitRow, error) {
					return fetchStoreWait(c, storeID)
				},
			)
			cliutil.FanoutReportErrors(cmd.ErrOrStderr(), errs)
			rows := make([]storeWaitRow, 0, len(results))
			for _, r := range results {
				rows = append(rows, r.Value)
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].WaitMin < rows[j].WaitMin })
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().StringVar(&address, "address", "", "Street address (required)")
	cmd.Flags().StringVar(&city, "city", "", "City, state, zip (required)")
	cmd.Flags().StringVar(&svc, "type", "Delivery", "Service type: Delivery or Carryout")
	return cmd
}

// extractStoreIDs walks the locator response and pulls every Stores[].StoreID
// it can find. The locator returns an envelope { "Stores": [...] }; we also
// accept a bare array for resilience against future shape changes.
func extractStoreIDs(data json.RawMessage) []string {
	var envelope struct {
		Stores []map[string]any `json:"Stores"`
	}
	if err := json.Unmarshal(data, &envelope); err == nil && len(envelope.Stores) > 0 {
		return collectStoreIDs(envelope.Stores)
	}
	var bare []map[string]any
	if err := json.Unmarshal(data, &bare); err == nil {
		return collectStoreIDs(bare)
	}
	return nil
}

func collectStoreIDs(items []map[string]any) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		for _, key := range []string{"StoreID", "storeID", "store_id", "id", "ID"} {
			if v, ok := it[key]; ok {
				if s := stringify(v); s != "" {
					out = append(out, s)
					break
				}
			}
		}
	}
	return out
}

func stringify(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return fmt.Sprintf("%v", int64(t))
	case int:
		return fmt.Sprintf("%d", t)
	case json.Number:
		return t.String()
	}
	return ""
}

func fetchStoreWait(c *client.Client, storeID string) (storeWaitRow, error) {
	data, err := c.Get("/power/store/"+storeID+"/profile", nil)
	if err != nil {
		return storeWaitRow{}, err
	}
	var profile map[string]any
	if err := json.Unmarshal(data, &profile); err != nil {
		return storeWaitRow{}, fmt.Errorf("parsing store %s profile: %w", storeID, err)
	}
	row := storeWaitRow{StoreID: storeID}
	if v, ok := profile["EstimatedWaitMinutes"]; ok {
		row.WaitMin = numberOf(v)
	} else if v, ok := profile["MinDeliveryWaitMinutes"]; ok {
		row.WaitMin = numberOf(v)
	}
	if v, ok := profile["AddressDescription"].(string); ok {
		row.Address = v
	}
	if v, ok := profile["ServiceMethod"].(string); ok {
		row.ServiceMethod = v
	}
	return row, nil
}

func numberOf(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case json.Number:
		f, _ := t.Float64()
		return f
	case string:
		var f float64
		fmt.Sscanf(t, "%f", &f)
		return f
	}
	return 0
}
