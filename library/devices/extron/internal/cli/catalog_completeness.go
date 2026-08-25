// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: doc-set completeness — per-model gap report across the six
// literature categories, for a --bom list or a single --model.

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/devices/extron/internal/extron"
	"github.com/spf13/cobra"
)

// pp:data-source local

type completenessRow struct {
	Model             string   `json:"model"`
	DocCount          int      `json:"doc_count"`
	CategoriesPresent []string `json:"categories_present"`
	CategoriesMissing []string `json:"categories_missing"`
}

func newNovelCatalogCompletenessCmd(flags *rootFlags) *cobra.Command {
	var dbPath, flagBom, flagModel string

	cmd := &cobra.Command{
		Use:     "completeness",
		Short:   "Per-model gap report across Brochure, Declaration of Conformity, Design Guide, Product Guide, Manual, Revit BIM",
		Long:    "Report, per model (from --bom or --model), which of the six literature categories are present in the catalog and which are missing. Use this command to find which doc types are missing per model for bids/commissioning. Do NOT use it for a single model's doc list; use 'literature get' instead.",
		Example: "  extron-pp-cli catalog completeness --bom ./rack.csv --json\n  extron-pp-cli catalog completeness --model \"MAV Plus Series\"",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "--bom=/dev/null",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "catalog completeness")
			}
			if flagBom == "" && flagModel == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--bom or --model is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			dbPath = resolveCatalogDB(flags, dbPath)
			db, ok := openCatalogReadOnly(cmd, dbPath)
			if !ok {
				return printJSONFiltered(cmd.OutOrStdout(), make([]completenessRow, 0), flags)
			}
			defer db.Close()
			hintIfCatalogIncomplete(cmd, db)

			var models []string
			if flagBom != "" {
				m, err := readBOM(flagBom)
				if err != nil {
					return err
				}
				models = m
			} else {
				models = []string{flagModel}
			}
			docs, err := loadCatalogDocs(ctx, db)
			if err != nil {
				return err
			}
			rows := make([]completenessRow, 0, len(models))
			for _, m := range models {
				matched := resolveDocs(docs, m, 1000)
				presentSet := map[string]bool{}
				for _, d := range matched {
					if d.Category != "" {
						presentSet[d.Category] = true
					}
				}
				present := make([]string, 0, len(presentSet))
				for _, c := range extron.Categories {
					if presentSet[c] {
						present = append(present, c)
					}
				}
				missing := make([]string, 0, len(extron.Categories))
				for _, c := range extron.Categories {
					if !presentSet[c] {
						missing = append(missing, c)
					}
				}
				rows = append(rows, completenessRow{
					Model:             m,
					DocCount:          len(matched),
					CategoriesPresent: present,
					CategoriesMissing: missing,
				})
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			table := make([]map[string]any, 0, len(rows))
			for _, r := range rows {
				table = append(table, map[string]any{
					"model":              r.Model,
					"doc_count":          r.DocCount,
					"categories_present": strings.Join(r.CategoriesPresent, ", "),
					"categories_missing": strings.Join(r.CategoriesMissing, ", "),
				})
			}
			return printAutoTable(cmd.OutOrStdout(), table)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	cmd.Flags().StringVar(&flagBom, "bom", "", "path to a BOM file (one model per line or CSV first column)")
	cmd.Flags().StringVar(&flagModel, "model", "", "a single model name to check")
	return cmd
}
