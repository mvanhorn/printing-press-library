// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// Novel command: menu and price drift between two local snapshots.
//
// The API is stateless and exposes no price history, so drift is only knowable
// from snapshots this CLI stores itself.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/foodpanda/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/foodpanda/internal/store"
)

type fpPriceChange struct {
	Item     string  `json:"item"`
	Category string  `json:"category,omitempty"`
	Was      float64 `json:"was"`
	Now      float64 `json:"now"`
	Delta    float64 `json:"delta"`
	PctDelta float64 `json:"delta_percentage"`
}

type fpMenuDiffView struct {
	VendorCode   string          `json:"vendor_code"`
	VendorName   string          `json:"vendor_name"`
	BaselineAt   string          `json:"baseline_captured_at,omitempty"`
	CurrentAt    string          `json:"current_captured_at"`
	Added        []string        `json:"added_items"`
	Removed      []string        `json:"removed_items"`
	Repriced     []fpPriceChange `json:"repriced_items"`
	BaselineSize int             `json:"baseline_item_count"`
	CurrentSize  int             `json:"current_item_count"`
	Note         string          `json:"note,omitempty"`
}

func newNovelMenuDiffCmd(flags *rootFlags) *cobra.Command {
	var (
		flagVendorCode string
		since          string
		country        string
		dbPath         string
		save           bool
	)

	cmd := &cobra.Command{
		Use:   "menu-diff",
		Short: "Show what changed in a restaurant's menu and prices between two syncs.",
		Long: "Diff a restaurant's current menu against a stored snapshot.\n\n" +
			"foodpanda exposes no price history, so this compares against snapshots this CLI\n" +
			"recorded earlier (via 'menu-diff --save' or 'dish --snapshot').\n" +
			"The first run on a vendor records a baseline and reports no drift — that is expected.",
		Example: "  foodpanda-pp-cli menu-diff --vendor-code pk2v --since 7d --agent",
		// read-only by default: --save is opt-in, so a plain diff never
		// writes and stays idempotent. With --save the write stays inside the
		// CLI's own SQLite store, recording the baseline the next call diffs
		// against; it never leaves the machine.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true", "pp:happy-args": "--vendor-code=pk2v"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "menu-diff")
			}
			code := strings.TrimSpace(flagVendorCode)
			if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
				code = strings.TrimSpace(args[0])
			}
			if code == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a vendor code is required (positional or --vendor-code), e.g. pk2v"))
			}

			var window time.Duration
			if strings.TrimSpace(since) != "" {
				d, err := cliutil.ParseDurationLoose(since)
				if err != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("invalid --since %q: %w", since, err))
				}
				window = d
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, err := fpFetchMenu(ctx, c, country, code)
			if err != nil {
				return err
			}
			vendorName, current, err := fpParseMenu(raw)
			if err != nil {
				return err
			}

			if dbPath == "" {
				dbPath = defaultDBPath("foodpanda-pp-cli")
			}
			s, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer func() { _ = s.Close() }()

			snaps, err := s.ListMenuSnapshots(ctx, code, 50)
			if err != nil {
				return err
			}

			view := fpMenuDiffView{
				VendorCode: code, VendorName: vendorName,
				CurrentAt:   time.Now().UTC().Format(time.RFC3339),
				CurrentSize: len(current),
				Added:       make([]string, 0), Removed: make([]string, 0),
				Repriced: make([]fpPriceChange, 0),
			}

			var baseline []fpProduct
			for _, sn := range snaps {
				if window > 0 && time.Since(sn.CapturedAt) < window {
					continue // too recent to be a useful baseline
				}
				_, prods, perr := fpParseMenu(sn.Payload)
				if perr != nil {
					continue
				}
				baseline = prods
				view.BaselineAt = sn.CapturedAt.UTC().Format(time.RFC3339)
				view.BaselineSize = len(prods)
				break
			}

			if baseline == nil {
				view.Note = fmt.Sprintf("no baseline snapshot older than %s for %s; recorded one now, re-run later to see drift",
					orDefault(since, "any age"), code)
			} else {
				prev := indexProducts(baseline)
				curr := indexProducts(current)
				for k, cp := range curr {
					pp, ok := prev[k]
					if !ok {
						view.Added = append(view.Added, cp.label())
						continue
					}
					if pp.Price != cp.Price {
						delta := fpRound2(cp.Price - pp.Price)
						pct := 0.0
						if pp.Price != 0 {
							pct = fpRound2(delta / pp.Price * 100)
						}
						view.Repriced = append(view.Repriced, fpPriceChange{
							Item: cp.label(), Category: cp.Category,
							Was: pp.Price, Now: cp.Price, Delta: delta, PctDelta: pct,
						})
					}
				}
				for k, pp := range prev {
					if _, ok := curr[k]; !ok {
						view.Removed = append(view.Removed, pp.label())
					}
				}
				sort.Strings(view.Added)
				sort.Strings(view.Removed)
				sort.SliceStable(view.Repriced, func(i, j int) bool {
					return view.Repriced[i].Delta > view.Repriced[j].Delta
				})
				if len(view.Added) == 0 && len(view.Removed) == 0 && len(view.Repriced) == 0 {
					view.Note = "no menu or price changes since the baseline snapshot"
				}
			}

			if save {
				if err := s.SaveMenuSnapshot(ctx, code, vendorName, country, raw); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not record snapshot: %v\n", err)
				}
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s (%s) — %d items now", vendorName, code, view.CurrentSize)
			if view.BaselineAt != "" {
				fmt.Fprintf(cmd.OutOrStdout(), ", %d at baseline %s", view.BaselineSize, view.BaselineAt)
			}
			fmt.Fprintln(cmd.OutOrStdout())
			if view.Note != "" {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
			}
			if len(view.Repriced) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "\nRepriced:")
				for _, r := range view.Repriced {
					fmt.Fprintf(cmd.OutOrStdout(), "  %-42s %8.0f -> %-8.0f (%+.1f%%)\n",
						truncate(r.Item, 42), r.Was, r.Now, r.PctDelta)
				}
			}
			for label, list := range map[string][]string{"Added": view.Added, "Removed": view.Removed} {
				if len(list) == 0 {
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "\n%s:\n", label)
				for _, it := range list {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", truncate(it, 60))
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&since, "since", "", "Only use a baseline at least this old (e.g. 7d, 24h, 2w)")
	cmd.Flags().StringVar(&country, "country", "pk", "Market code: pk, bd, sg, my, hk, th")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local database path")
	cmd.Flags().BoolVar(&save, "save", false, "Also record this fetch as a new baseline snapshot (off by default so repeated diffs stay idempotent)")
	cmd.Flags().StringVar(&flagVendorCode, "vendor-code", "", "Vendor code (alternative to the positional argument)")
	return cmd
}

func (p fpProduct) label() string {
	if p.Variation != "" && !strings.EqualFold(p.Variation, p.Name) {
		return p.Name + " (" + p.Variation + ")"
	}
	return p.Name
}

func indexProducts(ps []fpProduct) map[string]fpProduct {
	m := make(map[string]fpProduct, len(ps))
	for _, p := range ps {
		m[strings.ToLower(p.Category+"|"+p.label())] = p
	}
	return m
}

func orDefault(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}
