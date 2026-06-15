// Copyright 2026 Dev Basu and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored transcendence command (Phase 3). Safe to edit.
// pp:data-source local

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/squire/internal/store"

	"github.com/spf13/cobra"
)

type watchSnapshot struct {
	CapturedAt  string         `json:"captured_at"`
	ShopID      string         `json:"shop_id"`
	Name        string         `json:"name"`
	BarberCount int            `json:"barber_count"`
	Rating      float64        `json:"rating"`
	NumRatings  int            `json:"num_ratings"`
	Services    map[string]int `json:"services"`
}

type watchResult struct {
	Shop             string        `json:"shop"`
	FirstSnapshot    bool          `json:"first_snapshot"`
	Changed          bool          `json:"changed"`
	Since            string        `json:"since,omitempty"`
	PriceChanges     []PriceChange `json:"price_changes"`
	ServicesAdded    []string      `json:"services_added"`
	ServicesRemoved  []string      `json:"services_removed"`
	BarberCountDelta int           `json:"barber_count_delta"`
	RatingDelta      float64       `json:"rating_delta"`
	Snapshot         watchSnapshot `json:"snapshot"`
}

func newNovelWatchCmd(flags *rootFlags) *cobra.Command {
	var flagDB string

	cmd := &cobra.Command{
		Use:   "watch <shop>",
		Short: "Snapshot a shop's prices, staff, and rating; on re-run, diff against the last snapshot and show what changed.",
		Example: strings.Trim(`
  squire-pp-cli watch barber-theory-toronto
  squire-pp-cli watch barber-theory-toronto --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would snapshot the shop and diff against the last run")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a shop slug or UUID is required"))
			}
			shop := args[0]
			dbPath := flagDB
			if dbPath == "" {
				dbPath = defaultDBPath("squire-pp-cli")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			uuid, name, _, barberCount, _, _, err := resolveShop(ctx, c, shop)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			cur := watchSnapshot{
				CapturedAt:  time.Now().UTC().Format(time.RFC3339),
				ShopID:      uuid,
				Name:        name,
				BarberCount: barberCount,
				Services:    map[string]int{},
			}
			if svcs, err := fetchServices(ctx, c, uuid); err == nil {
				for _, s := range svcs {
					cur.Services[strings.TrimSpace(sqString(s, "name"))] = sqInt(s, "cost")
				}
			}
			if avg, num, _, err := fetchReviewMeta(ctx, c, uuid); err == nil {
				cur.Rating = avg
				cur.NumRatings = num
			}

			s, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return configErr(fmt.Errorf("open store %s: %w", dbPath, err))
			}
			defer s.Close()

			res := watchResult{Shop: shop, Snapshot: cur}
			if prevRaw, err := s.Get("watch_snapshot", uuid); err == nil && len(prevRaw) > 0 {
				var prev watchSnapshot
				if json.Unmarshal(prevRaw, &prev) == nil {
					res.Since = prev.CapturedAt
					res.PriceChanges, res.ServicesAdded, res.ServicesRemoved = diffServicePrices(prev.Services, cur.Services)
					res.BarberCountDelta = cur.BarberCount - prev.BarberCount
					res.RatingDelta = cur.Rating - prev.Rating
					res.Changed = len(res.PriceChanges) > 0 || len(res.ServicesAdded) > 0 ||
						len(res.ServicesRemoved) > 0 || res.BarberCountDelta != 0 || res.RatingDelta != 0
				}
			} else {
				res.FirstSnapshot = true
				res.PriceChanges = make([]PriceChange, 0)
				res.ServicesAdded = make([]string, 0)
				res.ServicesRemoved = make([]string, 0)
			}

			snapJSON, _ := json.Marshal(cur)
			if err := s.Upsert("watch_snapshot", uuid, snapJSON); err != nil {
				return fmt.Errorf("store snapshot: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), res, flags)
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "Local SQLite database path (default: per-user squire-pp-cli store)")
	return cmd
}
