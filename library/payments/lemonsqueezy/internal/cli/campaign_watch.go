// Copyright 2026 Joseph Alvin Castillo and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source local

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/lemonsqueezy/internal/store"
	"github.com/spf13/cobra"
)

type campaignRow struct {
	DiscountID         string  `json:"discount_id"`
	Code               string  `json:"code"`
	Status             string  `json:"status"`
	Used               int     `json:"used"`
	Cap                int     `json:"cap"`
	PercentFull        float64 `json:"percent_full"`
	Redemptions24h     int     `json:"redemptions_last_24h"`
	RedemptionsPerHour float64 `json:"redemptions_per_hour"`
	SelloutETA         string  `json:"projected_sellout_eta,omitempty"`
	Note               string  `json:"note,omitempty"`
}

type campaignWatchView struct {
	Codes       []campaignRow `json:"codes"`
	Queried     []string      `json:"queried,omitempty"`
	Count       int           `json:"count"`
	GeneratedAt string        `json:"generated_at"`
	Note        string        `json:"note,omitempty"`
}

func newNovelCampaignWatchCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "campaign-watch [code...]",
		Short: "Per discount code: used vs cap, 24h velocity, projected sellout time",
		Long: `Live capacity + pace tracking for capped discount campaigns.

For each code (positional args; default: every discount in the local mirror):
  - used vs cap (with percent_full)
  - redemptions in the last 24h
  - redemptions per hour
  - projected sellout time at current pace (linear extrapolation)

Use this for live capacity + pace tracking during a sale. For broad discount
inventory regardless of activity, use the generated 'list-discounts' instead.

Data source: local. Run 'sync --resources discounts,discount-redemptions' first.`,
		Example: "  lemonsqueezy-pp-cli sync --resources discounts,discount-redemptions\n  lemonsqueezy-pp-cli campaign-watch FOUNDING-LIFETIME FOUNDING-2YR FOUNDING-1YR --json",
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// campaign-watch is a read-only local query; --dry-run is a
			// no-op (there are no mutations to suppress). We still run
			// the query so --dry-run --json returns a real view.
			if dbPath == "" {
				dbPath = defaultDBPath("lemonsqueezy-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if !hintIfUnsynced(cmd, db, "discounts") {
				hintIfStale(cmd, db, "discounts", flags.maxAge)
			}

			filter := map[string]bool{}
			queried := make([]string, 0, len(args))
			for _, a := range args {
				filter[strings.ToUpper(a)] = true
				queried = append(queried, a)
			}
			view, err := buildCampaignWatch(db, filter)
			if err != nil {
				return err
			}
			// Echo the queried codes so the caller (and agents reading
			// --json output) can see what was searched for even when the
			// local mirror is empty or no codes matched.
			if len(queried) > 0 {
				view.Queried = queried
			}
			return flags.printJSON(cmd, view)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local SQLite database path")
	return cmd
}

func buildCampaignWatch(db *store.Store, filter map[string]bool) (campaignWatchView, error) {
	now := time.Now().UTC()
	view := campaignWatchView{Codes: []campaignRow{}, GeneratedAt: now.Format(time.RFC3339)}

	velocity := loadRedemptionVelocityByDiscount(db, now)

	rows, err := db.Query(`SELECT data FROM resources WHERE resource_type = 'discounts' LIMIT 10000`)
	if err != nil {
		return view, fmt.Errorf("querying discounts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var data sql.NullString
		if err := rows.Scan(&data); err != nil {
			continue
		}
		if !data.Valid {
			continue
		}
		var env struct {
			ID         string `json:"id"`
			Attributes struct {
				Code           string `json:"code"`
				Status         string `json:"status"`
				IsLimited      any    `json:"is_limited_redemptions"`
				MaxRedemptions any    `json:"max_redemptions"`
				DurationCount  any    `json:"duration_in_months"`
				StartsAt       string `json:"starts_at"`
				ExpiresAt      string `json:"expires_at"`
				// LS exposes a server-maintained usage counter on the discount itself;
				// prefer it over our local discount-redemptions count which may be
				// behind if that resource is unsynced.
				UsedCount any `json:"used"`
			} `json:"attributes"`
		}
		if err := json.Unmarshal([]byte(data.String), &env); err != nil {
			continue
		}
		code := strings.ToUpper(env.Attributes.Code)
		if len(filter) > 0 && !filter[code] {
			continue
		}
		capacity := int(toFloatLS(env.Attributes.MaxRedemptions))
		vel, tableUsed := velocity[env.ID][0], velocity[env.ID][1]
		// Prefer the discount's own server-counted 'used' when present; fall
		// back to our locally-counted redemption rows when LS doesn't expose
		// the attribute.
		used := int(toFloatLS(env.Attributes.UsedCount))
		if used == 0 && tableUsed > 0 {
			used = int(tableUsed)
		}

		row := campaignRow{
			DiscountID:         env.ID,
			Code:               env.Attributes.Code,
			Status:             env.Attributes.Status,
			Used:               used,
			Cap:                capacity,
			Redemptions24h:     int(vel),
			RedemptionsPerHour: vel / 24.0,
		}
		if capacity > 0 {
			row.PercentFull = 100.0 * float64(row.Used) / float64(capacity)
		}
		if vel > 0 && capacity > 0 {
			remaining := capacity - row.Used
			if remaining > 0 {
				hoursToSellout := float64(remaining) / row.RedemptionsPerHour
				// Cap projection horizon to 10 years to avoid Duration overflow
				// when velocity is vanishingly small.
				if hoursToSellout > 24*365*10 {
					row.SelloutETA = "more than 10 years at current pace"
				} else {
					eta := now.Add(time.Duration(hoursToSellout * float64(time.Hour)))
					row.SelloutETA = eta.Format(time.RFC3339)
				}
			} else {
				row.SelloutETA = "sold out"
			}
		} else if capacity > 0 {
			row.Note = "no redemptions in the last 24 hours; sellout projection needs recent activity"
		}
		view.Codes = append(view.Codes, row)
	}
	sort.Slice(view.Codes, func(i, j int) bool {
		return view.Codes[i].PercentFull > view.Codes[j].PercentFull
	})
	view.Count = len(view.Codes)
	if view.Count == 0 {
		if len(filter) > 0 {
			view.Note = "no matching discount codes in local mirror"
		} else {
			view.Note = "no discounts in local mirror; run 'sync --resources discounts,discount-redemptions' first"
		}
	}
	return view, nil
}

// loadRedemptionVelocityByDiscount returns map[discountID]{redemptions_24h, total_used}.
func loadRedemptionVelocityByDiscount(db *store.Store, now time.Time) map[string][2]float64 {
	out := map[string][2]float64{}
	cutoff := now.Add(-24 * time.Hour)

	const loadRedemptionVelocityCap = 1000000
	rows, err := db.Query(
		`SELECT data FROM resources WHERE resource_type = 'discount-redemptions' LIMIT ?`,
		loadRedemptionVelocityCap,
	)
	if err != nil {
		return out
	}
	defer rows.Close()
	loaded := 0
	defer func() {
		if loaded >= loadRedemptionVelocityCap {
			fmt.Fprintf(os.Stderr, "warning: loadRedemptionVelocityByDiscount hit %d-row cap; sellout-pace projection may underreport historical redemption velocity\n", loadRedemptionVelocityCap)
		}
	}()
	for rows.Next() {
		loaded++
		var data sql.NullString
		if rows.Scan(&data) != nil || !data.Valid {
			continue
		}
		var env struct {
			Attributes struct {
				DiscountID any    `json:"discount_id"`
				CreatedAt  string `json:"created_at"`
			} `json:"attributes"`
		}
		if json.Unmarshal([]byte(data.String), &env) != nil {
			continue
		}
		dID := toStringLS(env.Attributes.DiscountID)
		if dID == "" {
			continue
		}
		cur := out[dID]
		cur[1]++
		when := parseLSTime(env.Attributes.CreatedAt)
		if !when.IsZero() && when.After(cutoff) {
			cur[0]++
		}
		out[dID] = cur
	}
	return out
}
