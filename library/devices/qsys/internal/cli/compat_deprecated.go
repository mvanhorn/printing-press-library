// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type deprecationVerdict struct {
	Model        string `json:"model"`
	Status       string `json:"status"`
	Family       string `json:"family,omitempty"`
	RemovedIn    string `json:"removed_in,omitempty"`
	Detail       string `json:"detail,omitempty"`
}

type deprecationResult struct {
	Models  []deprecationVerdict `json:"models"`
	Flagged int                  `json:"flagged"`
	Clear   int                  `json:"clear"`
	Unknown int                  `json:"unknown"`
	Note    string               `json:"note,omitempty"`
}

func newNovelCompatDeprecatedCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "deprecated [models...]",
		Short: "Flag which models in a list are deprecated or discontinued before they reach a quote.",
		Long: strings.Trim(`
Deprecated cross-references an equipment list against two independent signals:

  1. membership in the vendor's discontinued-products family, and
  2. appearance in the "hardware removed" column of the Designer version matrix.

Neither vendor site answers this for an arbitrary list of models, which is why
end-of-life parts tend to surface at order time rather than design time.

Accepts models as arguments or newline-separated on stdin. As with 'check',
"unknown" means the model was not found locally - not that it is current.
`, "\n"),
		Example: strings.Trim(`
  qsys-pp-cli compat deprecated CX-Q CXD-Q --agent
  cat bom.txt | qsys-pp-cli compat deprecated
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "compat deprecated")
			}
			models := readModels(cmd, args)
			if len(models) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("at least one model is required, as arguments or on stdin"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			dbPath = corpusDBPath(dbPath)
			if corpusMissing(cmd, flags, dbPath) {
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), deprecationResult{Models: make([]deprecationVerdict, 0)}, flags)
				}
				return nil
			}
			st, err := openCorpus(ctx, dbPath)
			if err != nil {
				return err
			}
			defer st.Close()
			db := st.DB()

			res := deprecationResult{Models: make([]deprecationVerdict, 0, len(models))}
			for _, m := range models {
				v := deprecationVerdict{Model: m, Status: "unknown", Detail: "not found in local corpus"}
				if p, found, err := findProduct(ctx, db, m); err != nil {
					return err
				} else if found {
					v.Family = p.Family
					if p.Discontinued {
						v.Status = "discontinued"
						v.Detail = "listed under the discontinued-products family"
					} else {
						v.Status = "current"
						v.Detail = ""
					}
				}
				if rel, err := removedIn(ctx, db, m); err != nil {
					return err
				} else if rel != "" {
					v.Status = "removed-support"
					v.RemovedIn = rel
					v.Detail = "hardware support removed in Designer " + rel
				}
				switch v.Status {
				case "discontinued", "removed-support":
					res.Flagged++
				case "current":
					res.Clear++
				default:
					res.Unknown++
				}
				res.Models = append(res.Models, v)
			}
			if res.Unknown > 0 {
				res.Note = "unknown models were not found locally; harvest products first or verify by hand"
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-22s %-16s %s\n", "MODEL", "STATUS", "DETAIL")
			for _, v := range res.Models {
				fmt.Fprintf(cmd.OutOrStdout(), "%-22s %-16s %s\n", trimTo(v.Model, 22), v.Status, v.Detail)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d flagged, %d current, %d unknown (of %d)\n",
				res.Flagged, res.Clear, res.Unknown, len(res.Models))
			if res.Note != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "note: %s\n", res.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Corpus database path")
	return cmd
}

// removedIn returns the earliest Designer version whose "hardware removed"
// column names this model, or "" when none does.
//
// Matching mirrors qsys.SupportsModel: case-insensitive substring, because the
// matrix names series while a BOM names SKUs. Rows are ordered numerically by
// version so the earliest removal wins rather than whichever row SQLite
// happened to return first.
func removedIn(ctx context.Context, db *sql.DB, model string) (string, error) {
	needle := strings.ToLower(strings.TrimSpace(model))
	if needle == "" {
		return "", nil
	}
	rows, err := db.QueryContext(ctx,
		`SELECT qds_version, removed_hardware FROM qsys_compat WHERE removed_hardware != ''`)
	if err != nil {
		return "", fmt.Errorf("reading removed-hardware rows: %w", err)
	}
	defer rows.Close()

	earliest := ""
	for rows.Next() {
		var version string
		var removed sql.NullString
		if err := rows.Scan(&version, &removed); err != nil {
			return "", fmt.Errorf("scanning removed-hardware row: %w", err)
		}
		if !strings.Contains(strings.ToLower(removed.String), needle) {
			continue
		}
		if earliest == "" || versionLess(version, earliest) {
			earliest = version
		}
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("iterating removed-hardware rows: %w", err)
	}
	return earliest, nil
}
