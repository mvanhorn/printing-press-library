// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/pagliacci/internal/client"
)

// OrderPlan is the structured output of `orders plan`.
//
// Each section is optional: an unauthenticated user gets store, time, and
// suggestion sections but no rewards stack. The summary line at the top
// names the headline numbers so an agent can route on a single field.
type OrderPlan struct {
	People         int                 `json:"people"`
	StoreID        int                 `json:"store_id"`
	StoreName      string              `json:"store_name,omitempty"`
	StoreAddress   string              `json:"store_address,omitempty"`
	ServiceType    string              `json:"service_type"`
	NextSlot       string              `json:"next_slot,omitempty"`
	NextSlotDate   string              `json:"next_slot_date,omitempty"`
	Suggestion     OrderSuggestion     `json:"suggestion"`
	EstimatedTotal float64             `json:"estimated_total"`
	RewardsBest    map[string]any      `json:"rewards_best,omitempty"`
	Notes          []string            `json:"notes,omitempty"`
	Components     OrderPlanProvenance `json:"components"`
}

// OrderSuggestion is a sized cart suggestion based on the people count.
// The slice/pizza math uses the standard 2.5 slices/person heuristic and
// 8 slices per large pie.
type OrderSuggestion struct {
	Servings       int      `json:"servings"`
	LargePies      int      `json:"large_pizzas"`
	SuggestedItems []string `json:"suggested_items,omitempty"`
}

// OrderPlanProvenance lists which API endpoints fed each section, so an
// agent (or curious user) can verify the plan was actually composed and
// which calls succeeded.
type OrderPlanProvenance struct {
	Store        string `json:"store"`
	NextSlot     string `json:"next_slot,omitempty"`
	MenuTop      string `json:"menu_top,omitempty"`
	RewardsStack string `json:"rewards_stack,omitempty"`
}

func newOrdersPlanCmd(flags *rootFlags) *cobra.Command {
	var people int
	var addressLabel string
	var address string
	var serviceType string
	var unitPrice float64

	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Plan a small-party order: store, delivery slot, sized cart contents, and the best discount stack",
		Long: `Compose a complete order plan for N people from a saved address. The plan
combines the closest deliverable store, the next available delivery slot,
a sized cart suggestion (using a 2.5 slices/person heuristic), and — if
authenticated — the optimal rewards/coupon/credit stack against the
estimated total.

Each component (store, slot, suggestion, rewards) is sourced from a
separate API call and recorded under "components" so the agent can verify
which legs succeeded and which fell through.`,
		Example: `  pagliacci-pp-cli orders plan --people 6 --address-label home
  pagliacci-pp-cli orders plan --people 4 --address "350 5th Ave, Seattle, WA" --json
  pagliacci-pp-cli orders plan --people 8 --address-label home --service-type PICK`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if people < 1 {
				return usageErr(fmt.Errorf("--people must be >= 1"))
			}
			if serviceType != "DEL" && serviceType != "PICK" {
				return usageErr(fmt.Errorf("--service-type must be DEL or PICK, got %q", serviceType))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			plan := OrderPlan{
				People:      people,
				ServiceType: serviceType,
			}

			// 1. Resolve a store from the saved address (or free-form).
			deliverIDs, err := resolveDeliverableStores(c, addressLabel, address)
			if err != nil {
				return err
			}
			storeID, store, sErr := pickClosestStore(c, deliverIDs)
			if sErr != nil {
				return sErr
			}
			plan.StoreID = storeID
			if store != nil {
				if n, ok := store["Name"].(string); ok {
					plan.StoreName = n
				}
				if a, ok := store["Address"].(string); ok {
					plan.StoreAddress = a
				}
			}
			plan.Components.Store = fmt.Sprintf("/Store + /AddressInfo (resolved %d candidate stores)", len(deliverIDs))

			// 2. Next available delivery slot.
			loc, _ := time.LoadLocation("America/Los_Angeles")
			today := time.Now().In(loc).Format("20060102")
			slotURL := fmt.Sprintf("/TimeWindows/%d/%s/%s", storeID, serviceType, today)
			if slotRaw, slotErr := c.Get(slotURL, nil); slotErr == nil {
				if slot := firstAllowedSlot(slotRaw); slot != "" {
					plan.NextSlot = slot
					plan.NextSlotDate = today
					plan.Components.NextSlot = slotURL
				}
			} else {
				plan.Notes = append(plan.Notes, fmt.Sprintf("could not fetch slots from %s: %v", slotURL, slotErr))
			}
			if plan.NextSlot == "" {
				// Fall back to the next available day from TimeWindowDays.
				wdURL := fmt.Sprintf("/TimeWindowDays/%d/%s", storeID, serviceType)
				if wdRaw, wdErr := c.Get(wdURL, nil); wdErr == nil {
					if nd := nextAvailableDay(wdRaw); nd != "" {
						slotURL := fmt.Sprintf("/TimeWindows/%d/%s/%s", storeID, serviceType, nd)
						if slotRaw, slotErr := c.Get(slotURL, nil); slotErr == nil {
							if slot := firstAllowedSlot(slotRaw); slot != "" {
								plan.NextSlot = slot
								plan.NextSlotDate = nd
								plan.Components.NextSlot = slotURL
							}
						}
					}
				}
			}

			// 3. Sized cart suggestion. 2.5 slices/person, 8 slices/large.
			servings := people * 5 / 2 // slices needed (2.5 * people)
			largePies := int(math.Ceil(float64(servings) / 8.0))
			if largePies < 1 {
				largePies = 1
			}
			plan.Suggestion = OrderSuggestion{
				Servings:  servings,
				LargePies: largePies,
			}

			// Best-effort menu-top to surface featured items.
			menuTopURL := fmt.Sprintf("/MenuTop/%d", storeID)
			if mtRaw, mtErr := c.Get(menuTopURL, nil); mtErr == nil {
				plan.Suggestion.SuggestedItems = topItemNames(mtRaw, 5)
				plan.Components.MenuTop = menuTopURL
			}

			// 4. Estimated total (rough). Defaults to a large-pizza unit price
			// of $20 unless --unit-price overrides.
			unit := unitPrice
			if unit <= 0 {
				unit = 20.0
			}
			plan.EstimatedTotal = float64(largePies) * unit

			// 5. Rewards stack — only if authenticated. Calls the same logic
			// surfaced by `rewards stack` against the estimated total.
			if best, rerr := bestRewardsStackQuiet(c, plan.EstimatedTotal); rerr == nil && best != nil {
				plan.RewardsBest = best
				plan.Components.RewardsStack = "/RewardCard + /StoredCoupons + /StoredCredit"
			} else if rerr != nil {
				plan.Notes = append(plan.Notes, fmt.Sprintf("rewards stack skipped: %v (try `auth login --chrome`)", rerr))
			}

			out, err := json.Marshal(plan)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), out, flags)
		},
	}

	cmd.Flags().IntVar(&people, "people", 0, "Number of people to feed (required)")
	cmd.Flags().StringVar(&addressLabel, "address-label", "", "Saved address label (e.g. home). Defaults to 'home'.")
	cmd.Flags().StringVar(&address, "address", "", "Free-form delivery address (validated via /AddressInfo)")
	cmd.Flags().StringVar(&serviceType, "service-type", "DEL", "DEL (delivery) or PICK (pickup)")
	cmd.Flags().Float64Var(&unitPrice, "unit-price", 0, "Override the per-pizza unit price used to estimate the total (default $20)")
	return cmd
}

// pickClosestStore picks one store ID from the deliverable set. When the
// set has exactly one entry, that's the store. When it has many, we use
// the first by iteration; the /AddressInfo response order already reflects
// proximity to the address.
func pickClosestStore(c *client.Client, ids map[int]bool) (int, map[string]any, error) {
	storesRaw, err := loadStoresJSON(c)
	if err != nil {
		return 0, nil, classifyAPIError(err)
	}
	var arr []map[string]any
	if err := json.Unmarshal(storesRaw, &arr); err != nil {
		return 0, nil, apiErr(fmt.Errorf("parsing /Store: %w", err))
	}
	if len(ids) > 0 {
		// Walk the store list in order, return the first match.
		for _, s := range arr {
			id := extractInt(s, "ID", "id")
			if ids[id] {
				return id, s, nil
			}
		}
	}
	if len(arr) > 0 {
		id := extractInt(arr[0], "ID", "id")
		return id, arr[0], nil
	}
	return 0, nil, apiErr(fmt.Errorf("no stores returned"))
}

// firstAllowedSlot extracts the first time string from a /TimeWindows
// response (which has shape {"Allowed": [...]}). Returns "" when there
// are no slots.
func firstAllowedSlot(raw json.RawMessage) string {
	var o struct {
		Allowed []string `json:"Allowed"`
	}
	if err := json.Unmarshal(raw, &o); err != nil {
		return ""
	}
	if len(o.Allowed) == 0 {
		return ""
	}
	return o.Allowed[0]
}

// nextAvailableDay scans /TimeWindowDays and returns the first ID
// (YYYYMMDD) marked Available. Returns "" when none are available.
func nextAvailableDay(raw json.RawMessage) string {
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err != nil {
		return ""
	}
	type dayEntry struct {
		id        int
		available bool
	}
	var days []dayEntry
	for _, d := range arr {
		id := extractInt(d, "ID", "id")
		avail, _ := d["Available"].(bool)
		days = append(days, dayEntry{id: id, available: avail})
	}
	sort.Slice(days, func(i, j int) bool { return days[i].id < days[j].id })
	for _, d := range days {
		if d.available && d.id > 0 {
			return fmt.Sprintf("%d", d.id)
		}
	}
	return ""
}

// topItemNames extracts up to n names from /MenuTop's category array.
func topItemNames(raw json.RawMessage, n int) []string {
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil
	}
	names := make([]string, 0, n)
	for _, c := range arr {
		if name, ok := c["Name"].(string); ok && name != "" {
			names = append(names, name)
			if len(names) >= n {
				break
			}
		}
	}
	return names
}

// bestRewardsStackQuiet computes the best rewards application against the
// supplied total. Returns nil + nil error when the user is not
// authenticated (rewards endpoints return 401), so the plan can degrade
// gracefully.
func bestRewardsStackQuiet(c *client.Client, total float64) (map[string]any, error) {
	if total <= 0 {
		return nil, nil
	}
	cardRaw, cErr := c.Get("/RewardCard", nil)
	if cErr != nil {
		// Treat any error as "rewards unavailable" — usually 401 on
		// an unauthenticated session.
		return nil, fmt.Errorf("rewards unavailable: %w", cErr)
	}
	var card map[string]any
	_ = json.Unmarshal(cardRaw, &card)
	rewardBalance := 0.0
	if v, ok := card["Balance"].(float64); ok {
		rewardBalance = v
	}

	creditRaw, _ := c.Get("/StoredCredit", nil)
	creditTotal := 0.0
	if creditRaw != nil {
		var arr []map[string]any
		_ = json.Unmarshal(creditRaw, &arr)
		for _, e := range arr {
			if v, ok := e["Balance"].(float64); ok {
				creditTotal += v
			} else if v, ok := e["Amount"].(float64); ok {
				creditTotal += v
			}
		}
	}

	couponsRaw, _ := c.Get("/StoredCoupons", nil)
	bestCoupon := 0.0
	bestCouponID := ""
	if couponsRaw != nil {
		var arr []map[string]any
		_ = json.Unmarshal(couponsRaw, &arr)
		for _, e := range arr {
			val := 0.0
			if v, ok := e["Value"].(float64); ok {
				val = v
			} else if v, ok := e["DiscountValue"].(float64); ok {
				val = v
			}
			if val > bestCoupon {
				bestCoupon = val
				if id, ok := e["Serial"].(string); ok {
					bestCouponID = id
				} else if id, ok := e["ID"].(string); ok {
					bestCouponID = id
				}
			}
		}
	}

	applied := bestCoupon + creditTotal + rewardBalance
	if applied > total {
		applied = total
	}
	out := map[string]any{
		"reward_balance":  rewardBalance,
		"credit_balance":  creditTotal,
		"best_coupon":     bestCoupon,
		"best_coupon_id":  bestCouponID,
		"applied_total":   applied,
		"projected_total": total - applied,
		"order_total":     total,
		"strategy":        "best-coupon + credit + rewards",
	}
	return out, nil
}
