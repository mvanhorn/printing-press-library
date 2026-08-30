// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/qsys/internal/qsys"
)

type compatVerdict struct {
	Model        string `json:"model"`
	Status       string `json:"status"`
	AddedIn      string `json:"added_in,omitempty"`
	ReleaseDate  string `json:"release_date,omitempty"`
	Discontinued bool   `json:"discontinued,omitempty"`
	Detail       string `json:"detail,omitempty"`
}

type compatCheckResult struct {
	QDSVersion string          `json:"qds_version"`
	Models     []compatVerdict `json:"models"`
	Supported  int             `json:"supported"`
	Unknown    int             `json:"unknown"`
	TooNew     int             `json:"requires_newer"`
	MatrixRows int             `json:"matrix_rows_scanned"`
	Note       string          `json:"note,omitempty"`
}

func newNovelCompatCheckCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		qds    string
	)
	cmd := &cobra.Command{
		Use:   "check [models...]",
		Short: "Check a whole equipment list against a Q-SYS Designer version and get back what is supported and what is not.",
		Long: strings.Trim(`
Check tells you whether an equipment list runs on a given Q-SYS Designer
version, using the hardware-support matrix from the Q-SYS Help compatibility
page.

The vendor publishes this as a 59-row table of "hardware added" per release, so
answering the question by hand means reading the table once per model. This
command inverts it: models in, verdicts out. Accepts models as arguments or as
newline-separated input on stdin, so a BOM export can be piped straight in.

Statuses are deliberately conservative:

  supported       the model appears in hardware added at or before --qds
  requires-newer  the model appears, but only in a release after --qds
  unknown         the model does not appear in the matrix at all

"unknown" is NOT "unsupported". Matrix entries name series ("MPA-Q Series"),
accessories are often omitted entirely, and anything shipping before the matrix
begins will never appear. Treat unknown as "verify by hand".
`, "\n"),
		Example: strings.Trim(`
  qsys-pp-cli compat check CX-Q TSC-70-G3 NL-C4 --qds 9.4 --agent
  cat bom.txt | qsys-pp-cli compat check --qds 9.4
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "compat check")
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
					return printJSONFiltered(cmd.OutOrStdout(), compatCheckResult{QDSVersion: qds, Models: make([]compatVerdict, 0)}, flags)
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

			res := compatCheckResult{QDSVersion: qds, Models: make([]compatVerdict, 0, len(models)), MatrixRows: len(matrix)}
			if len(matrix) == 0 {
				res.Note = "compatibility matrix is empty; run `qsys-pp-cli harvest --only compat`"
				for _, m := range models {
					res.Models = append(res.Models, compatVerdict{Model: m, Status: "unknown", Detail: "matrix not harvested"})
					res.Unknown++
				}
			} else {
				sort.Slice(matrix, func(i, j int) bool { return versionLess(matrix[i].QDSVersion, matrix[j].QDSVersion) })
				for _, m := range models {
					v := classify(m, qds, matrix)
					// Discontinued status is independent of version support and
					// worth surfacing on the same line: a part can be supported
					// by the software and still be unorderable.
					p, found, err := findProduct(ctx, db, m)
					if err != nil {
						return fmt.Errorf("looking up %q: %w", m, err)
					}
					if found {
						v.Discontinued = p.Discontinued
					}
					switch v.Status {
					case "supported":
						res.Supported++
					case "requires-newer":
						res.TooNew++
					default:
						res.Unknown++
					}
					res.Models = append(res.Models, v)
				}
			}
			if res.Unknown > 0 && res.Note == "" {
				res.Note = "models reported unknown were not found in the matrix; that is not the same as unsupported - verify those by hand"
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-22s %-15s %-10s %s\n", "MODEL", "STATUS", "ADDED IN", "NOTE")
			for _, v := range res.Models {
				note := v.Detail
				if v.Discontinued {
					note = strings.TrimSpace(note + " [discontinued]")
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-22s %-15s %-10s %s\n", trimTo(v.Model, 22), v.Status, v.AddedIn, note)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d supported, %d require newer, %d unknown (of %d, against QDS %s)\n",
				res.Supported, res.TooNew, res.Unknown, len(res.Models), qds)
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

func classify(model, target string, matrix []qsys.CompatRow) compatVerdict {
	for _, row := range matrix {
		if !qsys.SupportsModel(row, model) {
			continue
		}
		v := compatVerdict{Model: model, AddedIn: row.QDSVersion, ReleaseDate: row.ReleaseDate}
		if versionLess(target, row.QDSVersion) {
			v.Status = "requires-newer"
			v.Detail = "added in " + row.QDSVersion + ", after " + target
			return v
		}
		v.Status = "supported"
		return v
	}
	return compatVerdict{Model: model, Status: "unknown", Detail: "not listed in matrix"}
}

// versionLess compares dotted version strings numerically, so 9.10 sorts after
// 9.4 rather than before it as a string comparison would.
func versionLess(a, b string) bool {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) || i < len(bs); i++ {
		av, bv := 0, 0
		if i < len(as) {
			av, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			bv, _ = strconv.Atoi(bs[i])
		}
		if av != bv {
			return av < bv
		}
	}
	return false
}

// Self-registration: same reasoning as product get - the generated `compat`
// parent is regenerated from the spec, so the novel leaf attaches itself from
// this preserved file rather than relying on a line inside generated code.
func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		compatCmd, _, err := root.Find([]string{"compat"})
		if err == nil && compatCmd != nil && compatCmd != root {
			addNovelCommandIfAbsent(compatCmd, newNovelCompatCheckCmd(flags))
		}
	})
}
