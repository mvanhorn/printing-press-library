// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-built novel feature: site registry inventory. ht-ml.app has no accounts
// and no list endpoint, so this local registry is the only inventory of what
// you have published that can exist. Survives generate --force.
// pp:data-source local

package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

func newNovelListCmd(flags *rootFlags) *cobra.Command {
	var orphaned bool
	var sortBy string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "See every site you've ever published, with key and asset health flags.",
		Long: trimNL(`
List every site recorded in your local store. ht-ml.app is accountless and has
no list endpoint, so this registry is the only inventory of your published
sites. Each row shows whether the update_key is still recoverable (orphaned
sites have lost it), how many assets the HTML references, and the version count.`),
		Example:     "  ht-ml-pp-cli list --agent --select url,title,status",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := htmlxOpenStore(ctx, flags)
			if err != nil {
				return err
			}
			defer db.Close()

			sites, err := db.ListSites()
			if err != nil {
				return err
			}

			type siteRow struct {
				SiteID    string `json:"site_id"`
				URL       string `json:"url"`
				Title     string `json:"title,omitempty"`
				Status    string `json:"status,omitempty"`
				Alias     string `json:"alias,omitempty"`
				KeyStored bool   `json:"key_stored"`
				Orphaned  bool   `json:"orphaned"`
				Assets    int    `json:"assets"`
				Versions  int    `json:"versions"`
				CreatedAt string `json:"created_at,omitempty"`
			}

			rows := make([]siteRow, 0, len(sites))
			for _, s := range sites {
				assets, _ := db.AssetCount(s.SiteID)
				versions, _ := db.ListVersions(s.SiteID)
				r := siteRow{
					SiteID:    s.SiteID,
					URL:       s.URL,
					Title:     s.Title,
					Status:    s.Status,
					Alias:     s.Alias,
					KeyStored: s.HasKey,
					Orphaned:  !s.HasKey,
					Assets:    assets,
					Versions:  len(versions),
					CreatedAt: s.CreatedAt,
				}
				if orphaned && !r.Orphaned {
					continue
				}
				rows = append(rows, r)
			}

			switch sortBy {
			case "versions":
				sort.SliceStable(rows, func(i, j int) bool { return rows[i].Versions > rows[j].Versions })
			case "title":
				sort.SliceStable(rows, func(i, j int) bool { return rows[i].Title < rows[j].Title })
			default: // age: newest first
				sort.SliceStable(rows, func(i, j int) bool { return rows[i].CreatedAt > rows[j].CreatedAt })
			}

			if len(rows) == 0 {
				// The store is populated by publish/update, not by `sync` (the
				// API has no list endpoint), so the generic unsynced hint would
				// be misleading here. Point at publish instead.
				fmt.Fprintln(cmd.ErrOrStderr(), "no sites in the local store yet; publish one with 'ht-ml-pp-cli publish <file>'")
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				tw := newTabWriter(cmd.OutOrStdout())
				fmt.Fprintln(tw, bold("SITE_ID")+"\t"+bold("KEY")+"\t"+bold("ASSETS")+"\t"+bold("VER")+"\t"+bold("TITLE")+"\t"+bold("URL"))
				for _, r := range rows {
					key := green("✓")
					if !r.KeyStored {
						key = red("orphan")
					}
					fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%s\t%s\n", r.SiteID, key, r.Assets, r.Versions, truncate(r.Title, 32), r.URL)
				}
				return tw.Flush()
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().BoolVar(&orphaned, "orphaned", false, "Only show sites whose update_key is no longer stored (cannot be updated)")
	cmd.Flags().StringVar(&sortBy, "sort", "age", "Sort order: age | title | versions")
	return cmd
}
