// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/qsys/internal/qsys"
)

type bomVerdict struct {
	Model        string `json:"model"`
	Family       string `json:"family,omitempty"`
	CompatStatus string `json:"compat_status"`
	AddedIn      string `json:"added_in,omitempty"`
	Discontinued bool   `json:"discontinued,omitempty"`
	SpecSheet    bool   `json:"spec_sheet_available"`
	SpecPDFURL   string `json:"spec_pdf_url,omitempty"`
	Detail       string `json:"detail,omitempty"`
}

type bomVerifyResult struct {
	QDSVersion        string       `json:"qds_version"`
	Models            []bomVerdict `json:"models"`
	Supported         int          `json:"supported"`
	RequiresNewer     int          `json:"requires_newer"`
	Unknown           int          `json:"unknown"`
	DiscontinuedCount int          `json:"discontinued_count"`
	WithoutSpecSheet  int          `json:"without_spec_sheet"`
	MatrixRows        int          `json:"matrix_rows_scanned"`
	Note              string       `json:"note,omitempty"`
}

func newNovelBomVerifyCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		qds    string
	)
	cmd := &cobra.Command{
		Use:   "verify [models...]",
		Short: "One report per model in an equipment list: version support, EOL status, and spec-sheet availability in a single pass.",
		Long: strings.Trim(`
Bom verify is the complete pre-quote check: for every model in an equipment
list it reports whether the model runs on the given Q-SYS Designer version,
whether it is deprecated or discontinued, and whether a spec sheet was resolved.

Where compat check answers one question ("is this supported on QDS X") across
the whole list, bom verify answers three questions per model. Neither QSC
website can do this in one pass - the support matrix, the deprecation notices,
and the discontinued-products family live on different pages.

Accepts models as arguments or as newline-separated input on stdin, so a BOM
export can be piped straight in.

Compat statuses are deliberately conservative:

  supported       the model appears in hardware added at or before --qds
  requires-newer  the model appears, but only in a release after --qds
  unknown         the model does not appear in the matrix at all

"unknown" is NOT "unsupported" - matrix entries name series, so verify those by
hand. Spec-sheet availability is a local extraction fact: false means the
product page did not resolve a PDF, not that the vendor has no spec sheet.
`, "\n"),
		Example: strings.Trim(`
  qsys-pp-cli bom verify CX-Q TSC-70-G3 NL-C4 --qds 9.4 --agent
  cat bom.txt | qsys-pp-cli bom verify --qds 9.4
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "bom verify")
			}
			if strings.TrimSpace(qds) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--qds is required, e.g. --qds 9.4"))
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
					return printJSONFiltered(cmd.OutOrStdout(), bomVerifyResult{QDSVersion: qds, Models: make([]bomVerdict, 0)}, flags)
				}
				return nil
			}
			st, err := openCorpus(ctx, dbPath)
			if err != nil {
				return err
			}
			defer st.Close()
			db := st.DB()

			rows, err := db.QueryContext(ctx,
				`SELECT qds_version, release_date, added_hardware, removed_hardware FROM qsys_compat`)
			if err != nil {
				return fmt.Errorf("reading compatibility matrix: %w", err)
			}
			matrix := make([]qsys.CompatRow, 0, 64)
			for rows.Next() {
				var r qsys.CompatRow
				var rel, added, removed sql.NullString
				if err := rows.Scan(&r.QDSVersion, &rel, &added, &removed); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scanning compatibility row: %w", err)
				}
				r.ReleaseDate, r.AddedHardware, r.RemovedHardware = rel.String, added.String, removed.String
				matrix = append(matrix, r)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterating compatibility matrix: %w", err)
			}
			if err := rows.Close(); err != nil {
				return err
			}

			sort.Slice(matrix, func(i, j int) bool { return versionLess(matrix[i].QDSVersion, matrix[j].QDSVersion) })

			res := bomVerifyResult{QDSVersion: qds, Models: make([]bomVerdict, 0, len(models)), MatrixRows: len(matrix)}
			if len(matrix) == 0 {
				res.Note = "compatibility matrix is empty; run `qsys-pp-cli harvest --only compat`"
			}
			for _, m := range models {
				v := bomVerdict{Model: m}
				if len(matrix) > 0 {
					cv := classify(m, qds, matrix)
					v.CompatStatus, v.AddedIn, v.Detail = cv.Status, cv.AddedIn, cv.Detail
				} else {
					v.CompatStatus = "unknown"
					v.Detail = "matrix not harvested"
				}
				p, found, err := findProduct(ctx, db, m)
				if err != nil {
					return fmt.Errorf("looking up %q: %w", m, err)
				}
				if found {
					v.Family = p.Family
					v.Discontinued = p.Discontinued
					v.SpecSheet = p.SpecPDFURL != ""
					v.SpecPDFURL = p.SpecPDFURL
				}
				switch v.CompatStatus {
				case "supported":
					res.Supported++
				case "requires-newer":
					res.RequiresNewer++
				default:
					res.Unknown++
				}
				if v.Discontinued {
					res.DiscontinuedCount++
				}
				if !v.SpecSheet {
					res.WithoutSpecSheet++
				}
				res.Models = append(res.Models, v)
			}
			if res.Unknown > 0 && res.Note == "" {
				res.Note = "models reported unknown were not found in the matrix; that is not the same as unsupported - verify those by hand"
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-15s %-14s %-5s %-5s %s\n", "MODEL", "STATUS", "FAMILY", "SPEC", "EOL", "DETAIL")
			for _, v := range res.Models {
				spec, eol := "yes", "no"
				if !v.SpecSheet {
					spec = "no"
				}
				if v.Discontinued {
					eol = "yes"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-15s %-14s %-5s %-5s %s\n",
					trimTo(v.Model, 20), v.CompatStatus, trimTo(v.Family, 14), spec, eol, v.Detail)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d supported, %d require newer, %d unknown; %d discontinued, %d without spec sheet (against QDS %s)\n",
				res.Supported, res.RequiresNewer, res.Unknown, res.DiscontinuedCount, res.WithoutSpecSheet, qds)
			if res.Note != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "note: %s\n", res.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Corpus database path")
	cmd.Flags().StringVar(&qds, "qds", "", "Q-SYS Designer version to check against, e.g. 9.4")
	return cmd
}
