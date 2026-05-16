// Copyright 2026 joltsconsulting. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/devices/adminbyrequest/internal/store"
	"github.com/spf13/cobra"
)

type riskScored struct {
	Computer         string  `json:"computer"`
	AbrClientVersion string  `json:"abrClientVersion"`
	Elevations       int     `json:"elevations"`
	LocalAdmins      int     `json:"localAdmins"`
	Score            float64 `json:"score"`
}

func newInventoryRiskScoreCmd(flags *rootFlags) *cobra.Command {
	var topN int
	var dbPath string

	cmd := &cobra.Command{
		Use:     "risk-score",
		Short:   "Composite risk score per endpoint (offline)",
		Long:    "Score each device by elevation count (audit log entries on this computer), number of local admin accounts (from inventory JSON), and AbR client version recency. Higher score = higher risk.",
		Example: "  adminbyrequest-pp-cli inventory risk-score --top 10 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("adminbyrequest-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store at %s: %w (run sync first)", dbPath, err)
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(cmd.Context(), `
				WITH elevations_per_computer AS (
				  SELECT json_extract(data, '$.computer.name') AS computer, COUNT(*) AS n
				  FROM auditlog
				  WHERE json_extract(data, '$.computer.name') IS NOT NULL
				  GROUP BY computer
				)
				SELECT
				  i.name,
				  COALESCE(i.abr_client_version, '') AS ver,
				  COALESCE(e.n, 0) AS elevations,
				  COALESCE(json_array_length(json_extract(i.data, '$.computer.localAdmins')), 0) AS local_admins
				FROM inventory i
				LEFT JOIN elevations_per_computer e ON e.computer = i.name
				WHERE i.inventory_available = 1
			`)
			if err != nil {
				return fmt.Errorf("querying risk metrics: %w", err)
			}
			defer rows.Close()

			// Find the highest version in the dataset as the "current" baseline.
			latestVer := []int{0, 0, 0}
			var all []riskScored
			for rows.Next() {
				var name string
				var ver sql.NullString
				var elev, admins int
				if err := rows.Scan(&name, &ver, &elev, &admins); err != nil {
					return err
				}
				v := parseSemverLike(ver.String)
				if compareSemverLike(v, latestVer) > 0 {
					latestVer = v
				}
				all = append(all, riskScored{
					Computer:         name,
					AbrClientVersion: ver.String,
					Elevations:       elev,
					LocalAdmins:      admins,
				})
			}
			// rows.Err() check is critical here: latestVer is derived from the
			// scan, so a truncated read would compute behindHint against a
			// baseline lower than the real fleet maximum.
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating risk metrics: %w", err)
			}
			for i := range all {
				cur := parseSemverLike(all[i].AbrClientVersion)
				behindHint := 0.0
				if compareSemverLike(cur, latestVer) < 0 {
					behindHint = 2.0
				}
				// Simple weighted sum, intentionally crude — interpret it as a hint, not a CVSS.
				all[i].Score = float64(all[i].Elevations)*1.0 +
					float64(all[i].LocalAdmins)*0.5 +
					behindHint
			}
			// Sort descending by score.
			for i := 1; i < len(all); i++ {
				for j := i; j > 0 && all[j].Score > all[j-1].Score; j-- {
					all[j], all[j-1] = all[j-1], all[j]
				}
			}
			if topN > 0 && len(all) > topN {
				all = all[:topN]
			}
			if all == nil {
				all = []riskScored{}
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), all, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Top %d endpoints by composite risk (latest seen client %d.%d.%d):\n", len(all), latestVer[0], idxOr(latestVer, 1), idxOr(latestVer, 2))
			for _, r := range all {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-30s score=%.1f  elev=%d  admins=%d  ver=%s\n",
					r.Computer, r.Score, r.Elevations, r.LocalAdmins, r.AbrClientVersion)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&topN, "top", 10, "Return this many top endpoints")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard CLI location)")
	return cmd
}

func idxOr(s []int, i int) int {
	if i < len(s) {
		return s[i]
	}
	return 0
}
