// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source local
//
// `persona usage` — join synced personas against synced clips to report how
// many clips reference each persona, and which personas are orphans (zero
// referencing clips). Local SQLite store only; no network, no auth. Read-only.

package cli

import (
	"database/sql"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/store"
	"github.com/spf13/cobra"
)

// personaUsageRow is one persona with its referencing-clip count.
type personaUsageRow struct {
	PersonaID  string `json:"persona_id"`
	Name       string `json:"name"`
	UsageCount int64  `json:"usage_count"`
	Orphan     bool   `json:"orphan"`
}

func newPersonaUsageCmd(flags *rootFlags) *cobra.Command {
	var (
		orphansOnly bool
		dbPath      string
	)
	cmd := &cobra.Command{
		Use:   "usage",
		Short: "Show how many clips use each persona (and which are orphans)",
		Long: "Join your synced personas against your synced clips and report, per " +
			"persona, how many clips reference it. Personas with zero referencing " +
			"clips are flagged as orphans.\n\nLocal-store only: run 'suno-pp-cli sync' " +
			"first to populate personas and clips.",
		Example:     "  suno-pp-cli persona usage\n  suno-pp-cli persona usage --orphans --json",
		Annotations: map[string]string{"pp:data-source": "local", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'suno-pp-cli sync' first.", err)
			}
			defer db.Close()

			hintIfUnsynced(cmd, db, "personas")
			hintIfStale(cmd, db, "personas", flags.maxAge)

			rows, err := personaUsageRows(db)
			if err != nil {
				return fmt.Errorf("joining personas and clips: %w", err)
			}
			if orphansOnly {
				filtered := rows[:0]
				for _, r := range rows {
					if r.Orphan {
						filtered = append(filtered, r)
					}
				}
				rows = filtered
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().BoolVar(&orphansOnly, "orphans", false, "Show only personas with zero referencing clips")
	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath("suno-pp-cli"), "Path to local SQLite store")
	return cmd
}

// personaUsageRows counts, per persona, how many synced clips reference it. A
// clip's persona reference is read from the stored clip JSON at either
// $.persona_id or $.metadata.persona_id (Suno carries it in both shapes across
// generation modes). Personas with no referencing clips are flagged orphans.
func personaUsageRows(db *store.Store) ([]personaUsageRow, error) {
	const query = `
		SELECT p.id,
		       COALESCE(p.name, ''),
		       COALESCE(c.cnt, 0)
		  FROM personas p
		  LEFT JOIN (
		      SELECT pid, COUNT(*) AS cnt
		        FROM (
		            SELECT COALESCE(
		                       json_extract(data, '$.persona_id'),
		                       json_extract(data, '$.metadata.persona_id')
		                   ) AS pid
		              FROM clips
		        )
		       WHERE pid IS NOT NULL
		       GROUP BY pid
		  ) c ON c.pid = p.id
		 ORDER BY COALESCE(c.cnt, 0) DESC, p.id`
	rows, err := db.DB().Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]personaUsageRow, 0)
	for rows.Next() {
		var r personaUsageRow
		var name sql.NullString
		if err := rows.Scan(&r.PersonaID, &name, &r.UsageCount); err != nil {
			return nil, err
		}
		r.Name = name.String
		r.Orphan = r.UsageCount == 0
		out = append(out, r)
	}
	return out, rows.Err()
}
