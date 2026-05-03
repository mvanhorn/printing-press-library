// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/pagliacci/internal/client"
)

// BestTimeResult is the structured output of `address best-time`.
type BestTimeResult struct {
	Label     string   `json:"label"`
	Address   string   `json:"address"`
	StoreID   int      `json:"store_id"`
	StoreName string   `json:"store_name,omitempty"`
	NextSlot  string   `json:"next_slot"`
	AltSlots  []string `json:"alt_slots"`
}

// resolveAddressByLabel returns the saved address matching the user-supplied
// label. Looks at local cache first, then falls back to the live
// /AddressName endpoint. Pass `primary` to pick the address flagged
// Primary: true. Match is case-insensitive on common label fields
// (Label, Name, Tag, Description) with a fuzzy substring fallback.
//
// Returns (nil, nil, nil) when no match is found, with the second slice
// holding the user's actual saved label names (so callers can build a
// helpful error). Returns (nil, nil, err) only on transport errors.
func resolveAddressByLabel(c *client.Client, label string) (map[string]any, []string, error) {
	target := strings.ToLower(strings.TrimSpace(label))
	if target == "" {
		return nil, nil, fmt.Errorf("--label is required")
	}

	// Local first
	if db, err := openStoreForRead(context.Background(), "pagliacci-pp-cli"); err == nil && db != nil {
		items, _ := db.List("address", 0)
		db.Close()
		if hit := findLabel(items, target); hit != nil {
			return hit, nil, nil
		}
	}

	// Live
	raw, err := c.Get("/AddressName", nil)
	if err != nil {
		return nil, nil, err
	}
	var arr []json.RawMessage
	if json.Unmarshal(raw, &arr) != nil {
		return nil, nil, fmt.Errorf("unexpected /AddressName response shape")
	}
	if hit := findLabel(arr, target); hit != nil {
		return hit, nil, nil
	}
	return nil, availableLabels(arr), nil
}

func findLabel(items []json.RawMessage, target string) map[string]any {
	want := strings.ToLower(strings.TrimSpace(target))
	// "primary" is a special pseudo-label that picks the saved address
	// flagged Primary: true. Pagliacci's UI distinguishes Primary vs
	// Alternative addresses; address Name fields are user-custom and
	// don't follow the conventional "home"/"work" pattern, so the
	// conventional default rarely matches.
	if want == "primary" {
		for _, raw := range items {
			var o map[string]any
			if json.Unmarshal(raw, &o) != nil {
				continue
			}
			if p, ok := o["Primary"].(bool); ok && p {
				return o
			}
		}
		return nil
	}
	// Exact match (case-insensitive) against common label fields.
	for _, raw := range items {
		var o map[string]any
		if json.Unmarshal(raw, &o) != nil {
			continue
		}
		for _, key := range []string{"Label", "Name", "Tag", "Description", "label", "name"} {
			if v, ok := o[key].(string); ok {
				if strings.ToLower(strings.TrimSpace(v)) == want {
					return o
				}
			}
		}
	}
	// Fuzzy substring fallback on Name/Label only. "recent" matches
	// "Most Recent"; "work" matches "Work Office"; etc.
	for _, raw := range items {
		var o map[string]any
		if json.Unmarshal(raw, &o) != nil {
			continue
		}
		for _, key := range []string{"Name", "Label"} {
			if v, ok := o[key].(string); ok && v != "" {
				if strings.Contains(strings.ToLower(v), want) {
					return o
				}
			}
		}
	}
	return nil
}

// availableLabels returns the user-visible names of saved addresses so
// error messages can show the user what they have to work with.
func availableLabels(items []json.RawMessage) []string {
	seen := map[string]bool{}
	var labels []string
	for _, raw := range items {
		var o map[string]any
		if json.Unmarshal(raw, &o) != nil {
			continue
		}
		for _, key := range []string{"Label", "Name", "Tag"} {
			if v, ok := o[key].(string); ok && v != "" && !seen[v] {
				labels = append(labels, v)
				seen[v] = true
				break
			}
		}
	}
	return labels
}

// addressLine builds a single-line "Street, City, State Zip" representation
// from a saved address record. Falls back to whichever fields are populated.
func addressLine(o map[string]any) string {
	get := func(k string) string {
		if v, ok := o[k].(string); ok {
			return v
		}
		return ""
	}
	parts := []string{}
	if s := get("Address"); s != "" {
		parts = append(parts, s)
	}
	cityState := strings.TrimSpace(strings.Join([]string{get("City"), get("State")}, ", "))
	cityState = strings.TrimPrefix(cityState, ", ")
	cityState = strings.TrimSuffix(cityState, ",")
	if zip := get("Zip"); zip != "" {
		cityState = strings.TrimSpace(cityState + " " + zip)
	}
	if cityState != "" {
		parts = append(parts, cityState)
	}
	if len(parts) == 0 {
		// fallback to whole-line string fields
		for _, k := range []string{"Display", "FullAddress", "Line"} {
			if s := get(k); s != "" {
				return s
			}
		}
	}
	return strings.Join(parts, ", ")
}

// extractSlotTimes pulls a flat list of "HH:MM" or RFC3339 slot strings out
// of a /TimeWindows response. The endpoint returns several shapes across
// stores; this is forgiving about field names.
func extractSlotTimes(raw json.RawMessage) []string {
	var out []string
	// Top-level array of slot objects
	var arr []map[string]any
	if json.Unmarshal(raw, &arr) == nil && len(arr) > 0 {
		for _, slot := range arr {
			if s := slotTime(slot); s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	// Object with a Windows / Slots field
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) == nil {
		for _, k := range []string{"Windows", "Slots", "TimeWindows", "windows", "slots"} {
			if sub, ok := obj[k]; ok {
				var arr2 []map[string]any
				if json.Unmarshal(sub, &arr2) == nil {
					for _, slot := range arr2 {
						if s := slotTime(slot); s != "" {
							out = append(out, s)
						}
					}
				}
			}
		}
	}
	return out
}

func slotTime(slot map[string]any) string {
	for _, k := range []string{"Time", "StartTime", "Start", "time", "start", "Window"} {
		if v, ok := slot[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func newAddressBestTimeCmd(flags *rootFlags) *cobra.Command {
	var label string
	var serviceType string
	var limit int

	cmd := &cobra.Command{
		Use:   "best-time",
		Short: "Resolve a saved address label to the next available delivery (or pickup) slot",
		Example: `  pagliacci-pp-cli address best-time --label home
  pagliacci-pp-cli address best-time --label work --limit 3 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if label == "" {
				return usageErr(fmt.Errorf("--label is required (e.g., --label home)"))
			}
			if serviceType != "DEL" && serviceType != "PICK" {
				return usageErr(fmt.Errorf("--service-type must be DEL or PICK, got %q", serviceType))
			}
			if limit < 1 {
				limit = 1
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			addr, available, err := resolveAddressByLabel(c, label)
			if err != nil {
				return classifyAPIError(err)
			}
			if addr == nil {
				if len(available) > 0 {
					return notFoundErr(fmt.Errorf("no saved address matching %q. Available labels: %s. Pass --label primary to pick the address marked Primary, or use one of the names listed", label, strings.Join(available, ", ")))
				}
				return notFoundErr(fmt.Errorf("no saved addresses on this account. Save one in pagliacci.com first, then re-run"))
			}

			line := addressLine(addr)

			// Validate the address against /AddressInfo to get the
			// delivery store ID. Some saved addresses already carry it.
			storeID := extractInt(addr, "StoreID", "StoreId", "DeliveryStoreID", "Store")
			if storeID == 0 {
				body := map[string]any{
					"Address": line,
				}
				if v, ok := addr["Address"].(string); ok {
					body["Address"] = v
				}
				if v, ok := addr["City"].(string); ok {
					body["City"] = v
				}
				if v, ok := addr["State"].(string); ok {
					body["State"] = v
				}
				if v, ok := addr["Zip"].(string); ok {
					body["Zip"] = v
				}
				resp, _, perr := c.Post("/AddressInfo", body)
				if perr != nil {
					return classifyAPIError(perr)
				}
				var info map[string]any
				if json.Unmarshal(resp, &info) == nil {
					storeID = extractInt(info, "StoreID", "StoreId", "DeliveryStoreID")
				}
			}

			if storeID == 0 {
				return notFoundErr(fmt.Errorf("address %q is outside Pagliacci's delivery zone", line))
			}

			// Pacific time "today" — Pagliacci is Seattle.
			loc, _ := time.LoadLocation("America/Los_Angeles")
			today := time.Now().In(loc).Format("20060102")

			path := fmt.Sprintf("/TimeWindows/%d/%s/%s", storeID, serviceType, today)
			slotResp, err := c.Get(path, nil)
			if err != nil {
				return classifyAPIError(err)
			}
			slots := extractSlotTimes(slotResp)

			storeName := ""
			if db, derr := openStoreForRead(context.Background(), "pagliacci-pp-cli"); derr == nil && db != nil {
				if raw, gerr := db.Get("store", fmt.Sprintf("%d", storeID)); gerr == nil && raw != nil {
					var s map[string]any
					if json.Unmarshal(raw, &s) == nil {
						if n, ok := s["Name"].(string); ok {
							storeName = n
						}
					}
				}
				db.Close()
			}

			result := BestTimeResult{
				Label:     label,
				Address:   line,
				StoreID:   storeID,
				StoreName: storeName,
				AltSlots:  []string{},
			}
			if len(slots) > 0 {
				result.NextSlot = slots[0]
				if limit > 1 && len(slots) > 1 {
					end := limit
					if end > len(slots) {
						end = len(slots)
					}
					result.AltSlots = slots[1:end]
				}
			}

			out, err := json.Marshal(result)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&label, "label", "", "Saved address label (e.g. home, work)")
	cmd.Flags().StringVar(&serviceType, "service-type", "DEL", "DEL (delivery) or PICK (pickup)")
	cmd.Flags().IntVar(&limit, "limit", 1, "Number of upcoming slots to return (1 = next slot only)")
	return cmd
}
