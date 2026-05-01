package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/gql"
	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/store"
)

// newHistoryCmd is the parent for all local-purchase-history commands.
// Subcommands: sync, list, search, stats. The data they read lives in
// the SQLite store populated by `history sync` (or incrementally by
// successful `add` calls writing back their picks).
func newHistoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Inspect, search, and sync your local Instacart purchase history",
		Long: `Your Instacart order history mirrored to a local SQLite DB so the CLI
can weight what you have actually bought before over live-catalog
first-match results. Data lives at ~/.config/instacart/instacart.db.

'history import <path>' reads a JSONL dump produced by the companion
browser-side scripts (see library/commerce/instacart/docs/dumper.js).
This is the working path -- 'history sync' exists but cannot be made
to work because Instacart has no clean GraphQL op for order history.

See docs/patterns/authenticated-session-scraping.md for the full
walkthrough.

Once data is loaded, 'add' checks the local history first and, when
confidence is high, skips the live search entirely.`,
	}
	cmd.AddCommand(
		newHistorySyncCmd(),
		newHistoryImportCmd(),
		newHistoryListCmd(),
		newHistorySearchCmd(),
		newHistoryStatsCmd(),
	)
	return cmd
}

func newHistorySyncCmd() *cobra.Command {
	var maxOrders int
	var since string
	var retailerOverride string
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Download your Instacart order history into the local store",
		Long: `Fetches recent orders and frequently-bought items from Instacart and
upserts them into the local SQLite history tables.

Default: last 50 orders or 12 months, whichever is smaller (caps the
first-run cost). Pass --max-orders 0 for unlimited; --since for an
explicit cutoff. Subsequent runs can be incremental -- pass nothing
and the code fetches orders placed after the most recent known order.`,
		Example: `  instacart history sync
  instacart history sync --max-orders 100
  instacart history sync --since 2026-01-01
  instacart history sync --store qfc`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(cmd)
			if err != nil {
				return err
			}
			defer app.Store.Close()
			if err := app.RequireSession(); err != nil {
				return err
			}

			var sinceTime time.Time
			if since != "" {
				sinceTime, err = time.Parse("2006-01-02", since)
				if err != nil {
					return coded(ExitUsage, "--since must be YYYY-MM-DD: %v", err)
				}
			} else {
				// Incremental default: start just past the newest known order.
				latest, _ := app.Store.MostRecentOrderAt(retailerOverride)
				if !latest.IsZero() {
					sinceTime = latest.Add(-time.Minute) // small safety margin
				} else {
					// First run: 12 months ago.
					sinceTime = time.Now().AddDate(-1, 0, 0)
				}
			}
			if maxOrders < 0 {
				maxOrders = 0
			}
			if !cmd.Flags().Changed("max-orders") && maxOrders == 0 {
				maxOrders = 50 // first-run cap
			}

			client := gql.NewClient(app.Session, app.Cfg, app.Store)
			result, err := gql.SyncHistory(app.Ctx, client, maxOrders, sinceTime)
			if err != nil {
				// Translate ErrHashMissing into a helpful pointer.
				if errors.Is(err, gql.ErrHashMissing) {
					msg := fmt.Sprintf("%v\n\n`history sync` is not viable for Instacart -- the BuyItAgainPage / CustomerOrderHistory ops referenced here do not actually exist in their API.\nUse `instacart history import <path>` instead. See docs/patterns/authenticated-session-scraping.md for the browser-side dump flow.", err)
					_ = app.Store.UpsertHistorySyncMeta(store.HistorySyncMeta{
						RetailerSlug:   "*",
						LastSyncAt:     time.Now(),
						LastSyncStatus: "error: hash missing",
						LastSyncError:  err.Error(),
					})
					return coded(ExitAuth, "%s", msg)
				}
				_ = app.Store.UpsertHistorySyncMeta(store.HistorySyncMeta{
					RetailerSlug:   "*",
					LastSyncAt:     time.Now(),
					LastSyncStatus: "error",
					LastSyncError:  err.Error(),
				})
				return coded(ExitConflict, "sync failed: %v", err)
			}

			// Record per-retailer sync state.
			for retailerSlug, orderCount := range result.PerRetailer {
				_ = app.Store.UpsertHistorySyncMeta(store.HistorySyncMeta{
					RetailerSlug:       retailerSlug,
					LastSyncAt:         time.Now(),
					LastSyncStatus:     "ok",
					LastSyncOrderCount: orderCount,
					LastSyncItemCount:  result.PurchasedItemsWritten,
				})
			}

			if app.JSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
					"orders_written":          result.OrdersWritten,
					"purchased_items_written": result.PurchasedItemsWritten,
					"per_retailer":            result.PerRetailer,
					"since":                   sinceTime.Format(time.RFC3339),
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "synced %d orders, %d purchased-item rows\n",
				result.OrdersWritten, result.PurchasedItemsWritten)
			for retailer, n := range result.PerRetailer {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s: %d orders\n", retailer, n)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&maxOrders, "max-orders", 50, "Maximum orders to fetch (0 = unlimited)")
	cmd.Flags().StringVar(&since, "since", "", "Only fetch orders placed on or after YYYY-MM-DD")
	cmd.Flags().StringVar(&retailerOverride, "store", "", "Only sync one retailer slug (default: all retailers with orders)")
	return cmd
}

func newHistoryListCmd() *cobra.Command {
	var limit int
	var retailerOverride string
	cmd := &cobra.Command{
		Use:   "list",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Show your most-purchased items from the local history",
		Long: `Lists purchased_items sorted by purchase_count DESC then last_purchased_at DESC.
Use --store to filter to one retailer. Default limit is 25.`,
		Example: `  instacart history list
  instacart history list --limit 10
  instacart history list --store qfc --limit 20`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(cmd)
			if err != nil {
				return err
			}
			defer app.Store.Close()

			rows, err := app.Store.ListPurchasedItems(retailerOverride, limit)
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStderr(), backfillHint())
				if app.JSON {
					return json.NewEncoder(cmd.OutOrStdout()).Encode([]any{})
				}
				return nil
			}
			if app.JSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(rows)
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "COUNT\tLAST\tRETAILER\tNAME\tITEM_ID")
			for _, r := range rows {
				fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\n",
					r.PurchaseCount,
					r.LastPurchasedAt.Format("2006-01-02"),
					r.RetailerSlug,
					truncate(r.Name, 60),
					r.ItemID,
				)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum rows to return")
	cmd.Flags().StringVar(&retailerOverride, "store", "", "Filter to one retailer slug")
	return cmd
}

func newHistorySearchCmd() *cobra.Command {
	var limit int
	var retailerOverride string
	cmd := &cobra.Command{
		Use:   "search <query>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Full-text search your local purchase history",
		Long: `Searches purchased_items_fts and returns the top N results ranked by
FTS relevance then by recency. Filter to one retailer with --store.

Useful for diagnosing why 'add' did or did not resolve via history.`,
		Example: `  instacart history search "sorbet"
  instacart history search "limoncello" --store pcc-community-markets`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(cmd)
			if err != nil {
				return err
			}
			defer app.Store.Close()

			query := strings.Join(args, " ")
			if retailerOverride == "" {
				fmt.Fprintln(cmd.OutOrStderr(), "note: --store is recommended; searching across retailers is slower and noisier")
			}
			// When --store is empty, search every retailer we know about by
			// iterating retailers with sync history.
			var rows []store.PurchasedItem
			if retailerOverride != "" {
				rows, err = app.Store.SearchPurchasedItems(query, retailerOverride, limit)
			} else {
				retailers, _ := app.Store.ListHistorySyncMeta()
				for _, r := range retailers {
					if r.RetailerSlug == "*" {
						continue
					}
					batch, berr := app.Store.SearchPurchasedItems(query, r.RetailerSlug, limit)
					if berr == nil {
						rows = append(rows, batch...)
					}
				}
			}
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStderr(), "no matches in local history")
				if app.JSON {
					return json.NewEncoder(cmd.OutOrStdout()).Encode([]any{})
				}
				return nil
			}
			if app.JSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(rows)
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "COUNT\tLAST\tRETAILER\tNAME\tITEM_ID")
			for _, r := range rows {
				fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\n",
					r.PurchaseCount,
					r.LastPurchasedAt.Format("2006-01-02"),
					r.RetailerSlug,
					truncate(r.Name, 60),
					r.ItemID,
				)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum rows to return")
	cmd.Flags().StringVar(&retailerOverride, "store", "", "Filter to one retailer slug")
	return cmd
}

func newHistoryStatsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Show history sync state and row counts",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(cmd)
			if err != nil {
				return err
			}
			defer app.Store.Close()

			orderCount, _ := app.Store.CountOrders()
			itemCount, lastPurchased, _ := app.Store.CountPurchasedItems()
			metas, _ := app.Store.ListHistorySyncMeta()

			if app.JSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
					"orders":              orderCount,
					"purchased_items":     itemCount,
					"last_purchased_at":   lastPurchased,
					"per_retailer_status": metas,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "orders:          %d\n", orderCount)
			fmt.Fprintf(cmd.OutOrStdout(), "purchased items: %d\n", itemCount)
			if !lastPurchased.IsZero() {
				fmt.Fprintf(cmd.OutOrStdout(), "last purchase:   %s\n", lastPurchased.Format(time.RFC3339))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "last purchase:   (none yet)\n")
			}
			if len(metas) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "\nper-retailer sync:")
				for _, m := range metas {
					when := "never"
					if !m.LastSyncAt.IsZero() {
						when = m.LastSyncAt.Format("2006-01-02 15:04:05")
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  %-30s status=%s orders=%d items=%d last=%s%s\n",
						m.RetailerSlug, m.LastSyncStatus, m.LastSyncOrderCount, m.LastSyncItemCount, when,
						optedOutSuffix(m.OptedOut),
					)
				}
			}
			return nil
		},
	}
}

func optedOutSuffix(optedOut bool) string {
	if optedOut {
		return " (opted out)"
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
