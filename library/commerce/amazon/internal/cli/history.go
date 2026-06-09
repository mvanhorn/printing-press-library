package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon/internal/history"
)

func newHistoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "history",
		Short:       "Inspect, import, and search local Amazon order history",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Amazon has no real history API. The browser-side dumper at docs/dumper.js
produces a JSONL file; ` + "`history import <path>` " + `loads it into the per-profile
SQLite DB. The denormalized purchased_items table powers the history-first
add resolver and the FTS search.`,
	}
	cmd.AddCommand(newHistoryImportCmd(), newHistoryListCmd(), newHistorySearchCmd(), newHistoryStatsCmd(), newHistorySyncCmd())
	return cmd
}

func newHistorySyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Walk amazon.com/gp/legacy/order-history and import any new orders into local history",
		Long: `Drives a headless Chromium against amazon.com/gp/legacy/order-history,
extracts orders + line items into JSONL, and imports them into the per-profile
SQLite store. Use this whenever the local history is out of date — e.g. after
placing a fresh order in Safari that Andy then can't find via repurchase
resolution.

Exits 9 (manual) with a deeplink if Amazon shows a CAPTCHA / sign-in challenge.`,
		Example: "  amazon-pp-cli --profile personal history sync\n  amazon-pp-cli --profile personal history sync --json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(cmd)
			if err != nil {
				return err
			}
			defer app.Store.Close()
			if err := app.RequireSession(); err != nil {
				return err
			}
			if app.DryRun {
				fmt.Fprintln(cmd.OutOrStdout(), "would walk /gp/legacy/order-history in headless browser and import any new orders")
				return nil
			}
			ctx, cancel := contextWithTimeout(app.Ctx, 240*time.Second)
			defer cancel()
			res, herr := runBrowserHelperRaw(ctx, "history-sync", app.Cfg.CookiesPath(app.Profile.Name), nil)
			if herr != nil {
				if app.JSON && res != nil {
					_ = json.NewEncoder(cmd.OutOrStdout()).Encode(res)
				}
				return herr
			}
			if res == nil || res.JSONL == "" {
				if app.JSON {
					return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{"imported_orders": 0, "imported_items": 0})
				}
				fmt.Fprintln(cmd.OutOrStdout(), "no orders found on history page")
				return nil
			}
			// Stage the JSONL to a temp file so we can reuse history.ImportFile.
			tmp, err := os.CreateTemp("", "amazon-history-sync-*.jsonl")
			if err != nil {
				return coded(ExitTransient, "create temp: %v", err)
			}
			defer os.Remove(tmp.Name())
			if _, err := tmp.WriteString(res.JSONL); err != nil {
				tmp.Close()
				return coded(ExitTransient, "write temp: %v", err)
			}
			tmp.Close()
			ir, err := history.ImportFile(app.Ctx, app.Store, tmp.Name())
			if err != nil {
				return coded(ExitConflict, "import: %v", err)
			}
			if app.JSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(ir)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "synced: %d orders, %d items (%d skipped)\n",
				ir.OrdersWritten, ir.ItemsWritten, ir.Skipped)
			return nil
		},
	}
}

func newHistoryImportCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "import <path>",
		Short:   "Import a JSONL order dump into the local store",
		Example: "  amazon-pp-cli --profile personal history import ~/Downloads/amazon-orders.jsonl",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(cmd)
			if err != nil {
				return err
			}
			defer app.Store.Close()
			if app.DryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "would import %s into %s\n", args[0], app.Cfg.DBPath(app.Profile.Name))
				return nil
			}
			res, err := history.ImportFile(app.Ctx, app.Store, args[0])
			if err != nil {
				return coded(ExitConflict, "import: %v", err)
			}
			if app.JSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "imported %d orders, %d items (%d skipped)\n", res.OrdersWritten, res.ItemsWritten, res.Skipped)
			return nil
		},
	}
}

func newHistoryListCmd() *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "Show the most-purchased items in local history",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(cmd)
			if err != nil {
				return err
			}
			defer app.Store.Close()
			rows, err := app.Store.ListPurchasedItems(limit)
			if err != nil {
				return err
			}
			if app.JSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(rows)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStderr(), "no purchased items in history. Run `amazon-pp-cli history import <path>` first.")
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "COUNT\tLAST\tASIN\tTITLE")
			for _, r := range rows {
				fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n",
					r.PurchaseCount,
					r.LastPurchasedAt.Format("2006-01-02"),
					r.ASIN,
					truncate(r.Title, 60),
				)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum rows to return")
	return cmd
}

func newHistorySearchCmd() *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "search <query>",
		Short:       "Full-text search local purchase history",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     "  amazon-pp-cli history search 'bath tissue'\n  amazon-pp-cli history search 'coffee' --limit 5 --json",
		Args:        cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(cmd)
			if err != nil {
				return err
			}
			defer app.Store.Close()
			query := joinArgs(args)
			rows, _, err := app.Store.SearchPurchasedItems(query, limit)
			if err != nil {
				return err
			}
			if app.JSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(rows)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStderr(), "no matches in local history")
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "COUNT\tLAST\tASIN\tTITLE")
			for _, r := range rows {
				fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n",
					r.PurchaseCount,
					r.LastPurchasedAt.Format("2006-01-02"),
					r.ASIN,
					truncate(r.Title, 60),
				)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum rows to return")
	return cmd
}

func newHistoryStatsCmd() *cobra.Command {
	return &cobra.Command{
		Use:         "stats",
		Short:       "Show history row counts and last sync timestamps",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(cmd)
			if err != nil {
				return err
			}
			defer app.Store.Close()
			orders, _ := app.Store.CountOrders()
			items, lastPurchased, _ := app.Store.CountPurchasedItems()
			out := map[string]any{
				"orders":          orders,
				"purchased_items": items,
			}
			if !lastPurchased.IsZero() {
				out["last_purchased_at"] = lastPurchased.Format(time.RFC3339)
			}
			if app.JSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "orders:          %d\n", orders)
			fmt.Fprintf(cmd.OutOrStdout(), "purchased items: %d\n", items)
			if !lastPurchased.IsZero() {
				fmt.Fprintf(cmd.OutOrStdout(), "last purchase:   %s\n", lastPurchased.Format(time.RFC3339))
			}
			return nil
		},
	}
}

func joinArgs(args []string) string {
	if len(args) == 0 {
		return ""
	}
	out := args[0]
	for _, a := range args[1:] {
		out += " " + a
	}
	return out
}
