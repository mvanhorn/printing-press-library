// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: series grouping — every document for a product family across
// all alphabetical letters.

package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/devices/extron/internal/extron"
	"github.com/spf13/cobra"
)

// pp:data-source auto

func newNovelLiteratureFamilyCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:     "family [series]",
		Short:   "Browse every document for a product family (DTP, MAV, IPL, DVS, ...) across all alphabetical letters",
		Long:    "Browse every document for one product family (DTP, MAV, IPL, DVS, SMD, PCS, Matrix, Annotator, ...) across all alphabetical letters of the catalog. Use this command for whole-family browsing. Do NOT use it for a single model's official docs; use 'literature get' instead.",
		Example: "  extron-pp-cli literature family dtp --json\n  extron-pp-cli literature family MAV --limit 20",
		Annotations: map[string]string{
			"mcp:read-only":          "true",
			"pp:happy-args":          "<series>=dtp",
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "list family literature")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a family/series name is required (e.g. dtp, mav)"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			q := strings.ToLower(strings.TrimSpace(args[0]))

			docs, err := catalogOrLive(ctx, cmd, flags, dbPath, q)
			if err != nil {
				return err
			}
			rows := make([]extron.Doc, 0, len(docs))
			for _, d := range docs {
				title := strings.ToLower(d.Title)
				if strings.HasPrefix(title, q) || hasFamilyPrefix(title, q) {
					rows = append(rows, d)
				}
			}
			orderDocsByTitle(rows)
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No documents match that family. Run 'extron-pp-cli catalog sync' first if the catalog is empty.")
				return nil
			}
			return printAutoTable(cmd.OutOrStdout(), docsToTable(rows))
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum rows to return (0 = no limit)")
	return cmd
}

// catalogOrLive returns catalog docs when the local mirror exists and is
// non-empty, otherwise live-fetches the letter buckets relevant to the query
// so the command works before the first sync.
func catalogOrLive(ctx context.Context, cmd *cobra.Command, flags *rootFlags, dbPath, query string) ([]extron.Doc, error) {
	dbPath = resolveCatalogDB(flags, dbPath)
	if db, ok := openCatalogReadOnly(cmd, dbPath); ok {
		docs, err := loadCatalogDocs(ctx, db)
		db.Close()
		if err != nil {
			return nil, err
		}
		if len(docs) > 0 {
			hintIfCatalogIncomplete(cmd, db)
			return docs, nil
		}
	}
	client := extron.New()
	letter := "All"
	for _, r := range query {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			letter = strings.ToUpper(string(r))
			break
		}
	}
	docs, refs, err := client.FetchIndex(ctx, letter)
	if err != nil {
		return nil, fmt.Errorf("catalog empty and live fetch failed: %w", err)
	}
	// The combined letter page shows only the first 20 rows per category;
	// follow each category's page-2 (best-effort) so titles like DTP (which
	// sort past the cap) are still found before the first sync.
	for _, ref := range refs {
		paged, _, perr := client.FetchCategoryPage(ctx, letter, ref)
		if perr != nil {
			continue
		}
		docs = append(docs, paged...)
	}
	return docs, nil
}

// hasFamilyPrefix reports whether title starts with a known family token
// matching the query (e.g. query "ma" matches a title starting with "MAV").
func hasFamilyPrefix(title, q string) bool {
	if q == "" {
		return false
	}
	for _, p := range familyPrefixes {
		lp := strings.ToLower(p)
		if strings.HasPrefix(lp, q) && strings.HasPrefix(title, lp) {
			return true
		}
		if strings.HasPrefix(title, lp) && strings.Contains(lp, q) {
			return true
		}
	}
	return false
}
