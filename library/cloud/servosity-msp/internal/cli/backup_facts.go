// Copyright 2026 servosity. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/cloud/servosity-msp/internal/store"
)

// backupFact is the unified shape rendered across all three engines.
// Fields are nullable because no single engine guarantees every key
// (e.g. restic's size field name differs from classic; missing values
// become "-" in table output and null in JSON).
type backupFact struct {
	CompanyID        *int64 `json:"company_id"`
	CompanyName      string `json:"company_name"`
	Engine           string `json:"engine"`
	ID               string `json:"id"`
	Hostname         string `json:"hostname"`
	LastSuccessfulAt string `json:"last_successful_at"`
	Status           string `json:"status"`
	SizeBytes        *int64 `json:"size_bytes"`
}

func newBackupFactsCmd(flags *rootFlags) *cobra.Command {
	var companyID int
	var engine string
	var status string
	var since string

	cmd := &cobra.Command{
		Use:         "backup-facts",
		Short:       "Unified backup view across classic, restic, and DR engines",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Produce a unified row-per-backup view across all three Servosity backup engines
(classic /backups/, restic /restic-backups/, and DR /dr-backups/) from the local
synced store. Optional filters scope by company, engine, status, or freshness.

Requires that 'sync' has been run for the relevant resources at least once.`,
		Example: `  # All backups across all engines, all companies
  servosity-msp-cli backup-facts

  # One company, all engines, last 7 days only
  servosity-msp-cli backup-facts --company 4421 --since 7d

  # Only failed restic backups, JSON for piping
  servosity-msp-cli backup-facts --engine restic --status fail --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			// Validate enums up-front so typos exit cleanly rather than
			// silently producing an empty result via a WHERE clause that
			// never matches.
			engine = strings.ToLower(strings.TrimSpace(engine))
			switch engine {
			case "", "all", "classic", "restic", "dr":
			default:
				return usageErr(fmt.Errorf("invalid --engine %q (want classic|restic|dr|all)", engine))
			}
			status = strings.ToLower(strings.TrimSpace(status))
			switch status {
			case "", "all", "ok", "fail", "stale":
			default:
				return usageErr(fmt.Errorf("invalid --status %q (want ok|fail|stale|all)", status))
			}

			var sinceTS string
			if since != "" {
				t, err := parseSinceDuration(since)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --since %q: %w", since, err))
				}
				sinceTS = t.UTC().Format(time.RFC3339)
			}

			dbPath := defaultDBPath("servosity-msp-cli")
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'servosity-msp-cli sync' first.", err)
			}
			defer db.Close()

			// One SELECT per engine UNIONed together. Each leg uses
			// json_extract against the data BLOB so column-shape drift
			// across engines doesn't require touching SQL — only the
			// expressions inside each leg. company_id is cast to INTEGER
			// so the LEFT JOIN to companies.id (TEXT) compares cleanly.
			//
			// hostname / size field names vary; we try the most common
			// names with COALESCE so at least one returns non-null in
			// real data. Add more aliases here if dogfood shows misses.
			classicLeg := `
SELECT
  CAST(json_extract(b.data, '$.company_id') AS INTEGER) AS company_id,
  COALESCE(c.name, '') AS company_name,
  'classic' AS engine,
  b.id AS id,
  COALESCE(json_extract(b.data, '$.hostname'),
           json_extract(b.data, '$.display_name'),
           json_extract(b.data, '$.login'), '') AS hostname,
  COALESCE(json_extract(b.data, '$.last_successful_at'),
           json_extract(b.data, '$.last_backup_at'),
           json_extract(b.data, '$.last_success_at'), '') AS last_successful_at,
  COALESCE(json_extract(b.data, '$.last_status'),
           json_extract(b.data, '$.status'),
           json_extract(b.data, '$.state'), '') AS status,
  CAST(COALESCE(json_extract(b.data, '$.size_bytes'),
                json_extract(b.data, '$.size')) AS INTEGER) AS size_bytes
FROM backups b
LEFT JOIN companies c
  ON CAST(json_extract(b.data, '$.company_id') AS TEXT) = c.id
`
			resticLeg := `
SELECT
  CAST(json_extract(b.data, '$.company_id') AS INTEGER) AS company_id,
  COALESCE(c.name, '') AS company_name,
  'restic' AS engine,
  b.id AS id,
  COALESCE(json_extract(b.data, '$.hostname'),
           json_extract(b.data, '$.display_name'),
           json_extract(b.data, '$.login'), '') AS hostname,
  COALESCE(json_extract(b.data, '$.last_successful_at'),
           json_extract(b.data, '$.last_backup_at'),
           json_extract(b.data, '$.last_success_at'), '') AS last_successful_at,
  COALESCE(json_extract(b.data, '$.last_status'),
           json_extract(b.data, '$.status'),
           json_extract(b.data, '$.state'), '') AS status,
  CAST(COALESCE(json_extract(b.data, '$.size_bytes'),
                json_extract(b.data, '$.size')) AS INTEGER) AS size_bytes
FROM restic_backups b
LEFT JOIN companies c
  ON CAST(json_extract(b.data, '$.company_id') AS TEXT) = c.id
`
			drLeg := `
SELECT
  CAST(json_extract(b.data, '$.company_id') AS INTEGER) AS company_id,
  COALESCE(c.name, '') AS company_name,
  'dr' AS engine,
  b.id AS id,
  COALESCE(json_extract(b.data, '$.hostname'),
           json_extract(b.data, '$.display_name'),
           json_extract(b.data, '$.login'), '') AS hostname,
  COALESCE(json_extract(b.data, '$.last_successful_at'),
           json_extract(b.data, '$.last_backup_at'),
           json_extract(b.data, '$.last_success_at'), '') AS last_successful_at,
  COALESCE(json_extract(b.data, '$.last_status'),
           json_extract(b.data, '$.status'),
           json_extract(b.data, '$.state'), '') AS status,
  CAST(COALESCE(json_extract(b.data, '$.size_bytes'),
                json_extract(b.data, '$.size')) AS INTEGER) AS size_bytes
FROM dr_backups b
LEFT JOIN companies c
  ON CAST(json_extract(b.data, '$.company_id') AS TEXT) = c.id
`

			// Engine selection: build UNION ALL of just the legs we need.
			var legs []string
			switch engine {
			case "classic":
				legs = []string{classicLeg}
			case "restic":
				legs = []string{resticLeg}
			case "dr":
				legs = []string{drLeg}
			default: // "" / "all"
				legs = []string{classicLeg, resticLeg, drLeg}
			}

			unioned := "(" + strings.Join(legs, "\nUNION ALL\n") + ")"

			// Outer wrapper applies post-UNION filters so each engine's
			// json_extract logic stays self-contained and the filters
			// hit normalized column aliases.
			var (
				where []string
				bind  []any
			)
			if companyID > 0 {
				where = append(where, "u.company_id = ?")
				bind = append(bind, int64(companyID))
			}
			if status != "" && status != "all" {
				where = append(where, "lower(u.status) = ?")
				bind = append(bind, status)
			}
			if sinceTS != "" {
				where = append(where, "u.last_successful_at >= ?")
				bind = append(bind, sinceTS)
			}

			query := "SELECT u.company_id, u.company_name, u.engine, u.id, u.hostname, u.last_successful_at, u.status, u.size_bytes FROM " + unioned + " AS u"
			if len(where) > 0 {
				query += " WHERE " + strings.Join(where, " AND ")
			}
			query += " ORDER BY u.company_name, u.engine, u.last_successful_at DESC"

			rows, err := db.DB().QueryContext(cmd.Context(), query, bind...)
			if err != nil {
				return fmt.Errorf("querying backup facts: %w", err)
			}
			defer rows.Close()

			var facts []backupFact
			for rows.Next() {
				var (
					cid      sql.NullInt64
					cname    sql.NullString
					eng      sql.NullString
					id       sql.NullString
					hostname sql.NullString
					lastOK   sql.NullString
					st       sql.NullString
					size     sql.NullInt64
				)
				if err := rows.Scan(&cid, &cname, &eng, &id, &hostname, &lastOK, &st, &size); err != nil {
					return fmt.Errorf("scanning backup-facts row: %w", err)
				}
				f := backupFact{
					CompanyName:      cname.String,
					Engine:           eng.String,
					ID:               id.String,
					Hostname:         hostname.String,
					LastSuccessfulAt: lastOK.String,
					Status:           st.String,
				}
				if cid.Valid {
					v := cid.Int64
					f.CompanyID = &v
				}
				if size.Valid {
					v := size.Int64
					f.SizeBytes = &v
				}
				facts = append(facts, f)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating backup-facts rows: %w", err)
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if facts == nil {
					facts = []backupFact{}
				}
				return enc.Encode(facts)
			}

			if len(facts) == 0 {
				fmt.Fprintln(os.Stderr, "no backup facts matched (try 'sync' first, or relax filters)")
				return nil
			}

			headers := []string{"COMPANY", "ENGINE", "ID", "HOSTNAME", "LAST_OK", "STATUS", "SIZE"}
			rowsOut := make([][]string, 0, len(facts))
			for _, f := range facts {
				company := f.CompanyName
				if f.CompanyID != nil {
					if company == "" {
						company = fmt.Sprintf("(%d)", *f.CompanyID)
					} else {
						company = fmt.Sprintf("%s (%d)", company, *f.CompanyID)
					}
				}
				size := "-"
				if f.SizeBytes != nil {
					size = humanBytes(*f.SizeBytes)
				}
				lastOK := f.LastSuccessfulAt
				if lastOK == "" {
					lastOK = "-"
				}
				st := f.Status
				if st == "" {
					st = "-"
				}
				host := f.Hostname
				if host == "" {
					host = "-"
				}
				rowsOut = append(rowsOut, []string{company, f.Engine, f.ID, host, lastOK, st, size})
			}
			return flags.printTable(cmd, headers, rowsOut)
		},
	}

	cmd.Flags().IntVar(&companyID, "company", 0, "Filter to one company ID (0 = all)")
	cmd.Flags().StringVar(&engine, "engine", "all", "Engine to include: classic|restic|dr|all")
	cmd.Flags().StringVar(&status, "status", "all", "Status filter: ok|fail|stale|all")
	cmd.Flags().StringVar(&since, "since", "", "Only backups whose last_successful_at is newer than (e.g. 7d, 24h)")

	return cmd
}

// humanBytes renders an int64 byte count as a short human-readable string
// (e.g. 128.4 GB). Kept local to backup-facts so its formatting stays
// stable independent of any shared helper that might shift conventions.
func humanBytes(n int64) string {
	const unit = 1024.0
	if n < int64(unit) {
		return fmt.Sprintf("%d B", n)
	}
	f := float64(n)
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	for _, u := range units {
		f /= unit
		if f < unit {
			return fmt.Sprintf("%.1f %s", f, u)
		}
	}
	return fmt.Sprintf("%.1f EB", f/unit)
}
