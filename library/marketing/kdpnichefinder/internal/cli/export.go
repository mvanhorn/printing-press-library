// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// pp:data-source local

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/kdpnichefinder/internal/kdpsource"
	"github.com/mvanhorn/printing-press-library/library/marketing/kdpnichefinder/internal/store"
)

func newNovelExportCmd(flags *rootFlags) *cobra.Command {
	var flagType string
	var flagDB string

	cmd := &cobra.Command{
		Use:         "export",
		Short:       "Export title, ASIN, price, estimated sales, and revenue as CSV for KDP backend keyword and cover work.",
		Example:     "  kdpnichefinder-pp-cli export --type evergreen --csv",
		Long:        "Use to export synced niches as CSV (title, asin, bucket, price, sales, revenue, amazon_url). Optionally scope to one bucket with --type.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flagType != "" {
				valid := false
				for _, b := range kdpsource.Buckets {
					if b == flagType {
						valid = true
						break
					}
				}
				if !valid {
					return usageErr(fmt.Errorf("unknown --type %q (valid: %v)", flagType, kdpsource.Buckets))
				}
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			// Resolve the mirror directly so the missing-mirror path stays
			// clean for both CSV (header-only) and JSON ([]) output.
			dbPath := resolveKDPDBPath(flagDB)
			var niches []nicheRow
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: kdpnichefinder-pp-cli refresh\n", dbPath)
			} else {
				db, err := store.OpenWithContext(ctx, dbPath)
				if err != nil {
					return err
				}
				if err := db.EnsureKDPSchema(ctx); err != nil {
					_ = db.Close()
					return err
				}
				defer db.Close()
				niches, err = loadNiches(ctx, db, flagType)
				if err != nil {
					return err
				}
			}

			// --json emits a JSON array (agent-friendly); the default and
			// --csv emit CSV. The header is always written so CSV output is
			// never empty, even before the first refresh.
			if flags.asJSON {
				type exportRow struct {
					Title     string  `json:"title"`
					ASIN      string  `json:"asin"`
					Bucket    string  `json:"bucket"`
					Price     string  `json:"price"`
					Sales     int     `json:"estimated_monthly_sales"`
					Revenue   float64 `json:"estimated_monthly_revenue"`
					AmazonURL string  `json:"amazon_url"`
				}
				rows := make([]exportRow, 0, len(niches))
				for _, n := range niches {
					rows = append(rows, exportRow{n.Title, n.ASIN, n.Bucket, n.PriceStr, n.Sales, n.Revenue, n.AmazonURL})
				}
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}

			w := csv.NewWriter(cmd.OutOrStdout())
			if err := w.Write([]string{"title", "asin", "bucket", "price", "estimated_monthly_sales", "estimated_monthly_revenue", "amazon_url"}); err != nil {
				return err
			}
			for _, n := range niches {
				rec := []string{
					n.Title,
					n.ASIN,
					n.Bucket,
					n.PriceStr,
					strconv.Itoa(n.Sales),
					strconv.FormatFloat(n.Revenue, 'f', -1, 64),
					n.AmazonURL,
				}
				if err := w.Write(rec); err != nil {
					return err
				}
			}
			w.Flush()
			return w.Error()
		},
	}
	cmd.Flags().StringVar(&flagType, "type", "", "Limit to a single bucket (evergreen, fresh_money, hidden_gems, high_ticket)")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local mirror database (defaults to the standard location)")
	return cmd
}
