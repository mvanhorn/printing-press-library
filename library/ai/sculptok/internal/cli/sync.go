// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored. Backfills the local SQLite mirror from the free history
// endpoints (/point/page -> credit_events, /image/page -> drawings) so the
// offline search/analytics/reconcile commands have data on a fresh machine.
//
// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/sculptok/internal/sculptok"
	"github.com/mvanhorn/printing-press-library/library/ai/sculptok/internal/store"
)

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var resources, db string
	var maxPages, pageSize int
	var full bool

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Mirror credit history and drawing history into the local store",
		Long: strings.Trim(`
Page the free history endpoints into the local SQLite mirror so 'search',
'analytics', and 'reconcile' work offline. Reads only — no credits are spent.

--resources selects what to sync: credits (credit-change history) and/or
drawings (your generated-image history). Unknown resource names are ignored.`, "\n"),
		Example: strings.Trim(`
  sculptok-pp-cli sync --resources credits,drawings
  sculptok-pp-cli sync --resources credits --max-pages 10
  sculptok-pp-cli sync --full`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would sync local mirror")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := newSculptokClient(flags)
			if err != nil {
				return err
			}
			if !c.HasKey() {
				// A mirror refresh with no credentials is an empty no-op, not a
				// hard error: report nothing-synced and how to authenticate.
				fmt.Fprintln(cmd.ErrOrStderr(), "no API key configured; nothing to sync. Set SCULPTOK_API_KEY or run 'sculptok-pp-cli auth set-token <key>'")
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"synced": map[string]int{}}, flags)
			}
			st, err := store.Open(ctx, resolveDBPath(db))
			if err != nil {
				return err
			}
			defer st.Close()

			// --full fetches every page; otherwise honor --max-pages.
			pages := maxPages
			if full {
				pages = 10000
			}

			want := map[string]bool{}
			for _, r := range strings.Split(resources, ",") {
				r = strings.TrimSpace(r)
				if r != "" {
					want[r] = true
				}
			}
			if len(want) == 0 {
				want["credits"] = true
				want["drawings"] = true
			}

			result := map[string]int{}
			if want["credits"] || want["credit_events"] {
				n, err := syncCredits(ctx, c, st, pages, pageSize)
				if err != nil {
					return fmt.Errorf("syncing credits: %w", err)
				}
				result["credit_events"] = n
			}
			if want["drawings"] {
				n, err := syncDrawings(ctx, c, st, pages, pageSize)
				if err != nil {
					return fmt.Errorf("syncing drawings: %w", err)
				}
				result["drawings"] = n
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"synced": result}, flags)
		},
	}
	cmd.Flags().StringVar(&resources, "resources", "credits,drawings", "Comma-separated resources to sync: credits,drawings")
	cmd.Flags().IntVar(&maxPages, "max-pages", 20, "Maximum pages to fetch per resource")
	cmd.Flags().IntVar(&pageSize, "limit", 50, "Items per page")
	cmd.Flags().BoolVar(&full, "full", false, "Fetch all available pages (ignore --max-pages)")
	cmd.Flags().StringVar(&db, "db", "", "Local store path (default ~/.config/sculptok-pp-cli/sculptok.db)")
	return cmd
}

func syncCredits(ctx context.Context, c *sculptok.Client, st *store.Store, maxPages, pageSize int) (int, error) {
	count := 0
	for page := 1; page <= maxPages; page++ {
		items, total, err := c.ListPage(ctx, "/point/page", page, pageSize)
		if err != nil {
			return count, err
		}
		if len(items) == 0 {
			break
		}
		for _, raw := range items {
			var e store.CreditEvent
			if err := json.Unmarshal(raw, &e); err != nil {
				continue
			}
			if e.ID == "" {
				continue
			}
			if err := st.UpsertCreditEvent(ctx, e); err == nil {
				count++
			}
		}
		if total > 0 && page*pageSize >= total {
			break
		}
	}
	return count, nil
}

func syncDrawings(ctx context.Context, c *sculptok.Client, st *store.Store, maxPages, pageSize int) (int, error) {
	count := 0
	for page := 1; page <= maxPages; page++ {
		items, total, err := c.ListPage(ctx, "/image/page", page, pageSize)
		if err != nil {
			return count, err
		}
		if len(items) == 0 {
			break
		}
		for _, raw := range items {
			var d store.Drawing
			if err := json.Unmarshal(raw, &d); err != nil {
				continue
			}
			if d.ID == "" {
				continue
			}
			if err := st.UpsertDrawing(ctx, d); err == nil {
				count++
			}
		}
		if total > 0 && page*pageSize >= total {
			break
		}
	}
	return count, nil
}
