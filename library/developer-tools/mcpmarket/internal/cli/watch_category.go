// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source computed
// This command computes results from locally stored history (resource_snapshots)
// built up as the user browses; it does not read a single upstream resource type.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newNovelWatchCategoryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "category <name>",
		Short:       "Flag newly-appeared listings in a category since your last sync.",
		Example:     "  mcpmarket-pp-cli watch category 'Developer Tools' --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "name=developer-tools"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "watch category")
			}
			if len(args) == 0 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("category slug is required"))
			}
			categorySlug := slugifyCategory(args[0])
			pseudoType := "category-watch:" + categorySlug

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			items, err := fetchItemList(ctx, c, "/categories/"+categorySlug, nil)
			if err != nil {
				return apiErr(err)
			}

			dbPath := defaultDBPath("mcpmarket-pp-cli")
			db, err := storeOpenForNovel(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			// Find the most recent snapshot date recorded for THIS category's
			// pseudo-resource-type BEFORE recording today's membership, so
			// "new since last watch" compares against the prior run, not the
			// one we're about to make. Scoped by resource_type directly since
			// the generic SnapshotDates helper spans every resource type.
			var priorDateNS sql.NullString
			if scanErr := db.DB().QueryRowContext(ctx,
				`SELECT MAX(snapshot_date) FROM resource_snapshots WHERE resource_type = ?`, pseudoType,
			).Scan(&priorDateNS); scanErr != nil {
				return scanErr
			}
			priorDate := priorDateNS.String

			currentSlugs := make(map[string]map[string]any, len(items))
			for _, item := range items {
				url, _ := item["url"].(string)
				slug := slugFromMCPMarketURL(url)
				if slug == "" {
					continue
				}
				currentSlugs[slug] = item
				data, merr := json.Marshal(item)
				if merr == nil {
					_ = db.Upsert(pseudoType, slug, data)
				}
			}
			today, _, err := db.CaptureSnapshot(ctx, pseudoType)
			if err != nil {
				return fmt.Errorf("capturing category watch snapshot: %w", err)
			}

			priorRows, err := db.SnapshotRows(ctx, priorDate, pseudoType)
			if err != nil {
				return err
			}
			if priorDate == "" || priorDate == today || len(priorRows) == 0 {
				note := fmt.Sprintf("first time watching category %q — %d current members recorded as the baseline. Run this again on a later day to see new entrants.", categorySlug, len(currentSlugs))
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"category": categorySlug, "new_entrants": []map[string]any{}, "note": note}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), note)
				return nil
			}

			priorSlugs := make(map[string]bool, len(priorRows))
			for _, r := range priorRows {
				priorSlugs[r.ResourceID] = true
			}
			newEntrants := make([]map[string]any, 0)
			for slug, item := range currentSlugs {
				if !priorSlugs[slug] {
					newEntrants = append(newEntrants, item)
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"category": categorySlug, "since": priorDate, "new_entrants": newEntrants,
			}, flags)
		},
	}
	return cmd
}
