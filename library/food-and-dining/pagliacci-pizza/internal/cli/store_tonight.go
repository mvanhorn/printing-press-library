// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/pagliacci-pizza/internal/client"
)

// TonightRow is one candidate store with delivery ETA and today's window.
type TonightRow struct {
	StoreID    int    `json:"store_id"`
	StoreName  string `json:"store_name"`
	Address    string `json:"address,omitempty"`
	OpenHour   string `json:"open"`
	CloseHour  string `json:"close"`
	EstMinutes int    `json:"est_minutes"`
	Available  bool   `json:"available_today"`
}

// parseClockTime parses Pagliacci's "11:00am" / "10:00pm" hour strings on the
// supplied date in the supplied location, returning a time.Time.
func parseClockTime(s string, day time.Time, loc *time.Location) (time.Time, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return time.Time{}, fmt.Errorf("empty time")
	}
	for _, layout := range []string{"3:04pm", "3pm", "15:04"} {
		if t, err := time.ParseInLocation(layout, s, loc); err == nil {
			return time.Date(day.Year(), day.Month(), day.Day(), t.Hour(), t.Minute(), 0, 0, loc), nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized time %q", s)
}

// hoursForToday returns the open/close strings for today from a /Store
// record. Pagliacci's Hours is a 7-element array indexed by Day (0=Sunday).
func hoursForToday(storeRec map[string]any, now time.Time) (open, closeStr string, ok bool) {
	hours, hasHours := storeRec["Hours"].([]any)
	if !hasHours {
		return "", "", false
	}
	dayIdx := int(now.Weekday()) // Sunday=0
	for _, h := range hours {
		hm, isMap := h.(map[string]any)
		if !isMap {
			continue
		}
		idx := extractInt(hm, "Day", "day")
		if idx == dayIdx {
			open, _ = hm["Open"].(string)
			closeStr, _ = hm["Close"].(string)
			return open, closeStr, true
		}
	}
	return "", "", false
}

// isOpenNow returns true when `now` falls inside the [open, close] window
// for the supplied day record. Times like "11:00pm" rolling past midnight
// are treated as same-day windows; Pagliacci's published hours never cross
// midnight in practice.
func isOpenNow(open, closeStr string, now time.Time, loc *time.Location) bool {
	openT, err := parseClockTime(open, now, loc)
	if err != nil {
		return false
	}
	closeT, err := parseClockTime(closeStr, now, loc)
	if err != nil {
		return false
	}
	return !now.Before(openT) && now.Before(closeT)
}

// extractDeliveryMinutes pulls a delivery wait-minute integer out of a
// /QuoteStore response. Field can be string ("30") or number.
func extractDeliveryMinutes(raw json.RawMessage) int {
	var o map[string]any
	if json.Unmarshal(raw, &o) != nil {
		return 0
	}
	return extractInt(o, "Delivery", "DeliveryMinutes", "DeliveryWait")
}

// findStoreInTimeWindowDays scans a /TimeWindowDays/{store}/{service}
// response and returns true when a record for `day` (YYYYMMDD) is marked
// Available.
func findStoreInTimeWindowDays(raw json.RawMessage, day time.Time) bool {
	var arr []map[string]any
	if json.Unmarshal(raw, &arr) != nil {
		return false
	}
	want := day.Format("20060102")
	for _, d := range arr {
		id := fmt.Sprintf("%d", extractInt(d, "ID", "id"))
		if id == want {
			if avail, ok := d["Available"].(bool); ok {
				return avail
			}
		}
	}
	return false
}

// candidateStores returns the set of stores to evaluate for `tonight`. When
// an explicit store ID list is provided (e.g., post-/AddressInfo deliverable
// stores), only those are checked; otherwise every store is considered.
func candidateStores(storesRaw json.RawMessage, ids map[int]bool) []map[string]any {
	var arr []map[string]any
	if json.Unmarshal(storesRaw, &arr) != nil {
		return nil
	}
	if len(ids) == 0 {
		return arr
	}
	out := make([]map[string]any, 0)
	for _, s := range arr {
		if ids[extractInt(s, "ID", "id")] {
			out = append(out, s)
		}
	}
	return out
}

func newStoreTonightCmd(flags *rootFlags) *cobra.Command {
	var addressLabel string
	var address string
	var serviceType string
	var limit int

	cmd := &cobra.Command{
		Use:   "tonight",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "List stores still open and able to deliver right now to your saved address",
		Example: `  pagliacci-pizza-pp-cli store tonight --address-label home
  pagliacci-pizza-pp-cli store tonight --address "350 5th Ave, Seattle, WA"
  pagliacci-pizza-pp-cli store tonight --service-type PICK`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if serviceType != "DEL" && serviceType != "PICK" {
				return usageErr(fmt.Errorf("--service-type must be DEL or PICK, got %q", serviceType))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			deliverIDs, err := resolveDeliverableStores(c, addressLabel, address)
			if err != nil {
				return err
			}

			// Stores: prefer cached, fall back to live.
			stores, err := loadStoresJSON(c)
			if err != nil {
				return classifyAPIError(err)
			}

			loc, _ := time.LoadLocation("America/Los_Angeles")
			now := time.Now().In(loc)

			candidates := candidateStores(stores, deliverIDs)

			rows := []TonightRow{}
			for _, s := range candidates {
				storeID := extractInt(s, "ID", "id")
				if storeID == 0 {
					continue
				}
				name, _ := s["Name"].(string)
				addrStr, _ := s["Address"].(string)

				open, closeStr, hasH := hoursForToday(s, now)
				if !hasH || !isOpenNow(open, closeStr, now, loc) {
					continue
				}

				// Today must be in TimeWindowDays/{store}/{service}.
				wdRaw, wdErr := c.Get(fmt.Sprintf("/TimeWindowDays/%d/%s", storeID, serviceType), nil)
				if wdErr != nil || !findStoreInTimeWindowDays(wdRaw, now) {
					continue
				}

				// Wait minutes from QuoteStore.
				qsRaw, qsErr := c.Get(fmt.Sprintf("/QuoteStore/%d", storeID), nil)
				wait := 0
				if qsErr == nil {
					wait = extractDeliveryMinutes(qsRaw)
				}

				rows = append(rows, TonightRow{
					StoreID:    storeID,
					StoreName:  name,
					Address:    addrStr,
					OpenHour:   open,
					CloseHour:  closeStr,
					EstMinutes: wait,
					Available:  true,
				})
			}

			sort.Slice(rows, func(i, j int) bool {
				return rows[i].EstMinutes < rows[j].EstMinutes
			})
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}

			out, err := json.Marshal(rows)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&addressLabel, "address-label", "", "Saved address label (e.g. home)")
	cmd.Flags().StringVar(&address, "address", "", "Free-form delivery address (validated via /AddressInfo)")
	cmd.Flags().StringVar(&serviceType, "service-type", "DEL", "DEL (delivery) or PICK (pickup)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows to return (0 = no limit)")
	return cmd
}

// resolveDeliverableStores returns the set of store IDs that can serve the
// supplied address. Returns an empty set + nil error when neither flag is
// supplied (caller treats that as "every open store qualifies"). Returns a
// usage error when both flags are absent and no `home` saved address can
// be located.
func resolveDeliverableStores(c *client.Client, label, addressFree string) (map[int]bool, error) {
	if label == "" && addressFree == "" {
		// Try the conventional "home" label as a default.
		label = "home"
	}

	if label != "" {
		addr, err := resolveAddressByLabel(c, label)
		if err != nil {
			return nil, classifyAPIError(err)
		}
		if addr == nil {
			if addressFree == "" {
				return nil, usageErr(fmt.Errorf("no saved address with label %q. Pass --address \"<full address>\" or save one via `address create`", label))
			}
			// fall through to free-form address path
		} else {
			if id := extractInt(addr, "StoreID", "DeliveryStoreID"); id != 0 {
				return map[int]bool{id: true}, nil
			}
			// Otherwise validate via /AddressInfo
			body := map[string]any{}
			for _, k := range []string{"Address", "City", "State", "Zip"} {
				if v, ok := addr[k].(string); ok {
					body[k] = v
				}
			}
			return validateAndExtractDeliverable(c, body)
		}
	}

	// Free-form: pass the raw string. /AddressInfo accepts a single
	// "Address" field for short addresses. Also accept user supplying
	// the comma-separated form which we split into Address/City/State.
	parts := strings.Split(addressFree, ",")
	body := map[string]any{}
	if len(parts) >= 1 {
		body["Address"] = strings.TrimSpace(parts[0])
	}
	if len(parts) >= 2 {
		body["City"] = strings.TrimSpace(parts[1])
	}
	if len(parts) >= 3 {
		stateZip := strings.Fields(strings.TrimSpace(parts[2]))
		if len(stateZip) >= 1 {
			body["State"] = stateZip[0]
		}
		if len(stateZip) >= 2 {
			body["Zip"] = stateZip[1]
		}
	}
	return validateAndExtractDeliverable(c, body)
}

func validateAndExtractDeliverable(c *client.Client, body map[string]any) (map[int]bool, error) {
	resp, _, err := c.Post("/AddressInfo", body)
	if err != nil {
		return nil, classifyAPIError(err)
	}
	out := map[int]bool{}
	// /AddressInfo returns an array of candidate buildings
	var arr []map[string]any
	if json.Unmarshal(resp, &arr) == nil {
		for _, info := range arr {
			if id := extractInt(info, "Store", "StoreID", "DeliveryStoreID"); id > 0 {
				out[id] = true
			}
		}
	} else {
		// Fallback for object-shaped responses
		var info map[string]any
		if json.Unmarshal(resp, &info) == nil {
			if id := extractInt(info, "Store", "StoreID", "DeliveryStoreID"); id > 0 {
				out[id] = true
			}
		}
	}
	if len(out) == 0 {
		return nil, notFoundErr(fmt.Errorf("address is outside Pagliacci's delivery zone"))
	}
	return out, nil
}

// loadStoresJSON returns the stores array from local cache or live /Store.
func loadStoresJSON(c *client.Client) (json.RawMessage, error) {
	if db, err := openStoreForRead("pagliacci-pizza-pp-cli"); err == nil && db != nil {
		items, _ := db.List("store", 0)
		db.Close()
		if len(items) > 0 {
			if marshaled, mErr := json.Marshal(items); mErr == nil {
				return marshaled, nil
			}
		}
	}
	return c.Get("/Store", nil)
}
