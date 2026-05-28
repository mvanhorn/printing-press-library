// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0.

package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/store"

	"github.com/spf13/cobra"
)

func newGapsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "gaps",
		Short: "Surface what's in the corpus that you haven't yet sat with",
		Long: `Gaps takes the anti-repeat instinct to its logical extreme. Where
'today' avoids the pieces you just sat with and 'coverage' summarises
breadth, gaps points at specific overlooked regions and mediums in the
local corpus so you can steer your next sit there.

The result is two ranked lists:

  • culture regions with works in the corpus you have never sat with
  • mediums with works in the corpus you have never sat with

Each entry shows the number of unsat works available in that region or
medium — a larger count means a deeper unexplored pocket. Run after a
'sync' to keep the corpus side of the comparison fresh.`,
		Example: `  art-goat-pp-cli gaps
  art-goat-pp-cli gaps --limit 5
  art-goat-pp-cli gaps --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() && !flags.asJSON {
				return emitGapsVerifyEnvelope(cmd)
			}

			if dbPath == "" {
				dbPath = defaultDBPath("art-goat-pp-cli")
			}
			if limit <= 0 {
				limit = 10
			}

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			if err := db.EnsureArtGoatTables(cmd.Context()); err != nil {
				return err
			}

			regions, err := gapsForDimension(cmd.Context(), db, "culture_region", limit)
			if err != nil {
				return err
			}
			mediums, err := gapsForDimension(cmd.Context(), db, "medium", limit)
			if err != nil {
				return err
			}

			envelope := map[string]any{
				"regions": regions,
				"mediums": mediums,
				"limit":   limit,
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
			}
			renderGaps(cmd, flags, regions, mediums)
			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum entries per gap list")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/art-goat-pp-cli/data.db)")
	return cmd
}

// gapEntry is one row of a gaps list: a region or medium with works in
// the local corpus that you have never sat with, and how many such works
// exist.
type gapEntry struct {
	Name           string `json:"name"`
	AvailableCount int    `json:"available_count"`
}

// gapsQueries maps a dimension slug to the exact SQL that ranks unsat
// values for that dimension. SQLite does not bind identifiers as
// parameters, and string-interpolation-after-whitelist creates a lint
// flag every time. A constant map is easier to audit: every column the
// CLI can ever query is listed here, full text, and gapsForDimension
// looks up rather than constructs.
//
// Uses LEFT JOIN ... WHERE s.work_id IS NULL rather than NOT IN
// (SELECT ...) so the plan scales with sit count instead of degrading
// to a quadratic check.
var gapsQueries = map[string]string{
	"culture_region": `
SELECT w.culture_region, COUNT(*) AS n
FROM works w
LEFT JOIN sits s ON s.work_id = w.id AND s.work_id != ''
WHERE w.culture_region != ''
  AND s.work_id IS NULL
GROUP BY w.culture_region
ORDER BY n DESC, w.culture_region ASC
LIMIT ?`,
	"medium": `
SELECT w.medium, COUNT(*) AS n
FROM works w
LEFT JOIN sits s ON s.work_id = w.id AND s.work_id != ''
WHERE w.medium != ''
  AND s.work_id IS NULL
GROUP BY w.medium
ORDER BY n DESC, w.medium ASC
LIMIT ?`,
}

// gapsForDimension returns the top-N (region or medium) values whose
// works in the corpus have never been touched by a sit.
func gapsForDimension(ctx context.Context, db *store.Store, dim string, limit int) ([]gapEntry, error) {
	q, ok := gapsQueries[dim]
	if !ok {
		return nil, fmt.Errorf("gaps: unsupported dimension %q", dim)
	}
	rows, err := db.DB().QueryContext(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("gaps %s: %w", dim, err)
	}
	defer rows.Close()
	var out []gapEntry
	for rows.Next() {
		var name string
		var n int
		if err := rows.Scan(&name, &n); err != nil {
			return nil, err
		}
		out = append(out, gapEntry{Name: name, AvailableCount: n})
	}
	return out, rows.Err()
}

func renderGaps(cmd *cobra.Command, flags *rootFlags, regions, mediums []gapEntry) {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Regions you have not sat with (top by unsat works available):")
	if len(regions) == 0 {
		fmt.Fprintln(out, "  (none — every region in the corpus has at least one sit)")
	} else {
		_ = flags.printTable(cmd, []string{"Region", "Available"}, gapsToRows(regions))
	}

	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Mediums you have not sat with (top by unsat works available):")
	if len(mediums) == 0 {
		fmt.Fprintln(out, "  (none — every medium in the corpus has at least one sit)")
	} else {
		_ = flags.printTable(cmd, []string{"Medium", "Available"}, gapsToRows(mediums))
	}
	fmt.Fprintln(out, "")
}

func gapsToRows(entries []gapEntry) [][]string {
	rows := make([][]string, 0, len(entries))
	for _, e := range entries {
		rows = append(rows, []string{e.Name, fmt.Sprintf("%d", e.AvailableCount)})
	}
	return rows
}

func emitGapsVerifyEnvelope(cmd *cobra.Command) error {
	envelope := map[string]any{
		"command":                 "gaps",
		"verify_noop":             true,
		"success":                 true,
		"__pp_verify_synthetic__": true,
		"reason":                  "verify_short_circuit",
		"note":                    "gaps renders ranked tables by default; PRINTING_PRESS_VERIFY=1 short-circuits the rendering. Pass --json to get the data envelope.",
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(envelope)
}
