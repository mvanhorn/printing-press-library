// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/woocommerce/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/woocommerce/internal/source/storeapi"
	"github.com/mvanhorn/printing-press-library/library/commerce/woocommerce/internal/store"
)

type catalogWatchView struct {
	Store        string `json:"store"`
	SnapshotAt   string `json:"snapshot_at"`
	Products     int    `json:"products"`
	PagesFetched int    `json:"pages_fetched"`
	Currency     string `json:"currency,omitempty"`
	Truncated    bool   `json:"truncated"`
	Note         string `json:"note,omitempty"`
}

func newNovelCatalogWatchCmd(flags *rootFlags) *cobra.Command {
	var flagStore string
	var flagMaxPages int
	var flagPerPage int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Record a price and stock snapshot of any public WooCommerce store",
		Long: "Reads a store's public Store API (/wp-json/wc/store/v1) and records a timestamped snapshot of\n" +
			"every product's price, sale state, and stock. The Store API needs no credentials, so this works\n" +
			"against stores you do not own. Prices arrive in minor units and are normalized on write.\n\n" +
			"Use this command to record a snapshot of any public WooCommerce store's catalog.\n" +
			"Do NOT use this command to see what changed between snapshots; it only records — use\n" +
			"'catalog diff' for the comparison.\n" +
			"Do NOT use it to check your own store for missing images or unpriced variations; use\n" +
			"'catalog audit' instead.",
		Example: "  woocommerce-pp-cli catalog watch --store https://woocommerce.com --agent",
		Annotations: map[string]string{
			"pp:happy-args":  "--store=https://woocommerce.com;--max-pages=1",
			"pp:data-source": "live",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				target := flagStore
				if target == "" {
					target = "(no --store given)"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "would snapshot the public catalog of %s\n", target)
				return nil
			}
			if flags != nil && flags.dataSource == "local" {
				return usageErr(fmt.Errorf("no local data source for \"catalog watch\": it records a fresh snapshot from the live Store API"))
			}
			if flagStore == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--store is required (for example --store https://woocommerce.com)"))
			}
			if flagMaxPages <= 0 {
				return usageErr(fmt.Errorf("--max-pages must be greater than zero"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			client, err := storeapi.New(flagStore, flags.timeout)
			if err != nil {
				return usageErr(err)
			}

			// Verify runs must not hit the network.
			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "would snapshot the public catalog of %s\n", client.Origin())
				return nil
			}
			maxPages := flagMaxPages
			// Live dogfood enforces a flat per-command timeout; one page proves the path.
			if cliutil.IsDogfoodEnv() && maxPages > 1 {
				maxPages = 1
			}

			dbPath = resolveDBPath(dbPath)
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			snapshotAt := time.Now().UTC().Format(time.RFC3339)
			rows := make([]store.CatalogSnapshotRow, 0)
			currency := ""
			pages := 0
			truncated := false

			for page := 1; page <= maxPages; page++ {
				products, totalPages, err := client.Products(ctx, page, flagPerPage)
				if err != nil {
					return fmt.Errorf("fetching catalog page %d from %s: %w", page, client.Origin(), err)
				}
				pages++
				// A full catalog is tens of pages of slow WordPress responses
				// (~1700 products took ~54s in testing). Report progress on
				// stderr so stdout stays clean JSON and the user is not left
				// staring at a blank terminal.
				if !flags.quiet {
					fmt.Fprintf(cmd.ErrOrStderr(), "{\"event\":\"snapshot_progress\",\"page\":%d,\"products\":%d}\n",
						page, len(rows)+len(products))
				}
				for _, p := range products {
					price, regular, sale := p.MajorPrices()
					if currency == "" {
						currency = p.Prices.CurrencyCode
					}
					rows = append(rows, store.CatalogSnapshotRow{
						StoreURL:     client.Origin(),
						ProductID:    fmt.Sprintf("%d", p.ID),
						SnapshotAt:   snapshotAt,
						Name:         p.CleanName(),
						SKU:          p.SKU,
						Permalink:    p.Permalink,
						Currency:     p.Prices.CurrencyCode,
						Price:        price,
						RegularPrice: regular,
						SalePrice:    sale,
						OnSale:       p.OnSale,
						InStock:      p.InStock,
					})
				}
				if len(products) == 0 {
					break
				}
				if totalPages > 0 && page >= totalPages {
					break
				}
				if page == maxPages && (totalPages == 0 || totalPages > maxPages) {
					truncated = true
				}
			}

			// Stamp the flag on every row so `catalog diff` can tell a partial
			// snapshot from a complete one and refuse to invent membership deltas.
			for i := range rows {
				rows[i].Truncated = truncated
			}
			if err := db.InsertCatalogSnapshot(ctx, rows); err != nil {
				return err
			}

			if len(rows) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s returned no products; no snapshot was recorded\n", client.Origin())
			}
			view := catalogWatchView{
				Store:        client.Origin(),
				SnapshotAt:   snapshotAt,
				Products:     len(rows),
				PagesFetched: pages,
				Currency:     currency,
				Truncated:    truncated,
			}
			if truncated {
				view.Note = fmt.Sprintf("reached --max-pages cap of %d; re-run with a higher cap to capture the full catalog", maxPages)
			}
			// This command always fetches over the network; the default
			// envelope would label it "local" and mislead agents.
			raw, err := json.Marshal(view)
			if err != nil {
				return err
			}
			return printOutputWithFlagsMeta(cmd.OutOrStdout(), json.RawMessage(raw), flags, map[string]any{"source": "live"})
		},
	}
	cmd.Flags().StringVar(&flagStore, "store", "", "Public WooCommerce store URL to snapshot (no credentials needed)")
	cmd.Flags().IntVar(&flagMaxPages, "max-pages", 20, "Maximum catalog pages to fetch")
	cmd.Flags().IntVar(&flagPerPage, "per-page", 100, "Products per page (Store API maximum is 100)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
