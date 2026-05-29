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
			// Read-only local query — no mutation to suppress under --dry-run;
			// we still run so --dry-run --json emits a real view.
			if dbPath == "" {
				dbPath = defaultDBPath("lemonsqueezy-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			hintIfMultiUnsynced(cmd, db, "discounts",
				[]string{"discount-redemptions"}, flags.maxAge)

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
			return emitCampaignWatch(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local SQLite database path")
	return cmd
}

func emitCampaignWatch(cmd *cobra.Command, flags *rootFlags, view campaignWatchView) error {
	if len(view.Codes) > 0 && wantsHumanTable(cmd.OutOrStdout(), flags) {
		items := make([]map[string]any, 0, len(view.Codes))
		for _, c := range view.Codes {
			items = append(items, map[string]any{
				"code":         c.Code,
				"status":       c.Status,
				"used":         c.Used,
				"cap":          c.Cap,
				"percent_full": fmt.Sprintf("%.1f%%", c.PercentFull),
				"24h":          c.Redemptions24h,
				"per_hour":     fmt.Sprintf("%.2f", c.RedemptionsPerHour),
				"sellout_eta":  c.SelloutETA,
			})
		}
		if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "\n%d code(s) tracked  (snapshot %s)\n",
			view.Count, view.GeneratedAt)
		if view.Note != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Note: %s\n", view.Note)
		}
		return nil
	}
	return flags.printJSON(cmd, view)
}

// campaignWatchDiscountScanCap caps the discounts scan. Saturation surfaces
// a stderr warning to distinguish "no discounts" from "scan truncated".
const campaignWatchDiscountScanCap = 100000

func buildCampaignWatch(db *store.Store, filter map[string]bool) (campaignWatchView, error) {
	now := time.Now().UTC()
	view := campaignWatchView{Codes: []campaignRow{}, GeneratedAt: now.Format(time.RFC3339)}

	velocity := loadRedemptionVelocityByDiscount(db, now)

	rows, err := db.Query(
		`SELECT data FROM resources WHERE resource_type = 'discounts' LIMIT ?`,
		campaignWatchDiscountScanCap,
	)
	if err != nil {
		return view, fmt.Errorf("querying discounts: %w", err)
	}
	defer rows.Close()
	scannedDiscounts := 0

	for rows.Next() {
		scannedDiscounts++
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
		stat := velocity[env.ID]
		// Prefer the discount's own server-counted 'used' when present; fall
		// back to our locally-counted redemption rows when LS doesn't expose
		// the attribute.
		used := int(toFloatLS(env.Attributes.UsedCount))
		if used == 0 && stat.Total > 0 {
			used = int(stat.Total)
		}

		row := campaignRow{
			DiscountID:         env.ID,
			Code:               env.Attributes.Code,
			Status:             env.Attributes.Status,
			Used:               used,
			Cap:                capacity,
			Redemptions24h:     int(stat.Velocity24h),
			RedemptionsPerHour: stat.Velocity24h / 24.0,
		}
		if capacity > 0 {
			row.PercentFull = 100.0 * float64(row.Used) / float64(capacity)
		}
		if stat.Velocity24h > 0 && capacity > 0 {
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
	if scannedDiscounts >= campaignWatchDiscountScanCap {
		fmt.Fprintf(os.Stderr, "warning: campaign-watch hit the %d-discount scan cap; some codes may be missing from this view\n", campaignWatchDiscountScanCap)
		view.Note = fmt.Sprintf("hit the %d-discount scan cap; some codes may be missing.", campaignWatchDiscountScanCap)
	}
	sort.Slice(view.Codes, func(i, j int) bool {
		if view.Codes[i].PercentFull != view.Codes[j].PercentFull {
			return view.Codes[i].PercentFull > view.Codes[j].PercentFull
		}
		return view.Codes[i].Code < view.Codes[j].Code
	})
	view.Count = len(view.Codes)
	if view.Count == 0 && view.Note == "" {
		if len(filter) > 0 {
			view.Note = "no matching discount codes in local mirror"
		} else {
			view.Note = "no discounts in local mirror; run 'sync --resources discounts,discount-redemptions' first"
		}
	}
	return view, nil
}
