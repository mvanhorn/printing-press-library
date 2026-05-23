// Copyright 2026 servosity. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/cloud/servosity-msp/internal/store"
)

// staleBackupSnapshotKey is the single-row primary key in pp_stale_snapshot.
// Pinned to the report name so future report-type expansions can co-exist in
// the same table without a schema migration.
const staleBackupSnapshotKey = "stale-backup-sets"

// staleBackupEntry is the per-row shape we expose externally. The upstream
// /reports/stale-backup-sets/ endpoint shape is unspecified, so we probe
// both `{"results": [...]}` and `[...]` and tolerate either snake_case or
// camelCase keys on individual entries.
type staleBackupEntry struct {
	CompanyID    int    `json:"company_id"`
	CompanyName  string `json:"company_name,omitempty"`
	Engine       string `json:"engine"`
	BackupID     string `json:"backup_id"`
	Hostname     string `json:"hostname"`
	LastBackupAt string `json:"last_backup_at"`
	DaysStale    int    `json:"days_stale"`
}

func newStaleBackupsCmd(flags *rootFlags) *cobra.Command {
	var days int
	var engine string
	var company int
	var refresh bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "stale-backups",
		Short: "Friday sweep: companies with stale backups, sliced by age/engine",
		Long: `Slice the stale-backup-sets report by company, age window, and backup engine.

Reads from a local cache by default (pp_stale_snapshot). Pass --refresh to
re-pull the live report and replace the cache before slicing. Joins with
the local companies table to render company names.`,
		Example: `  # Use cached snapshot (run --refresh first if empty)
  servosity-msp-cli stale-backups --days 3

  # Pull live and slice in one go
  servosity-msp-cli stale-backups --refresh --engine restic

  # Single client, restic engine, 7+ days stale
  servosity-msp-cli stale-backups --company 4421 --engine restic --days 7`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			switch engine {
			case "all", "classic", "restic", "dr":
			default:
				return usageErr(fmt.Errorf("invalid --engine %q: must be one of classic, restic, dr, all", engine))
			}

			if dbPath == "" {
				dbPath = defaultDBPath("servosity-msp-cli")
			}

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			if _, err := db.DB().ExecContext(cmd.Context(), `CREATE TABLE IF NOT EXISTS pp_stale_snapshot (
  report_name TEXT PRIMARY KEY,
  taken_at TEXT NOT NULL,
  data BLOB NOT NULL
);`); err != nil {
				return fmt.Errorf("creating pp_stale_snapshot table: %w", err)
			}

			var rawData []byte
			var takenAt string

			if refresh {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				data, err := c.Get("/reports/stale-backup-sets/", nil)
				if err != nil {
					// /reports/stale-backup-sets/ is admin-only. Partner
					// tokens get 403. Give the user a clean directive
					// instead of a stack trace.
					return fmt.Errorf("stale-backup-sets report is admin-only on Servosity's API. " +
						"For a partner-visible alternative, run: " +
						"`servosity-msp-cli backup-facts --since 7d` " +
						"(filters synced backup-facts to ones with last_successful_at older than 7 days). " +
						"This command will be reshaped to derive from local backup tables in a v0.2 fix.")
				}
				takenAt = time.Now().UTC().Format(time.RFC3339)
				if _, err := db.DB().ExecContext(cmd.Context(),
					`INSERT INTO pp_stale_snapshot (report_name, taken_at, data) VALUES (?, ?, ?)
ON CONFLICT(report_name) DO UPDATE SET taken_at = excluded.taken_at, data = excluded.data`,
					staleBackupSnapshotKey, takenAt, []byte(data),
				); err != nil {
					return fmt.Errorf("upserting stale-backup snapshot: %w", err)
				}
				rawData = []byte(data)
			} else {
				row := db.DB().QueryRowContext(cmd.Context(),
					`SELECT taken_at, data FROM pp_stale_snapshot WHERE report_name = ?`,
					staleBackupSnapshotKey,
				)
				if err := row.Scan(&takenAt, &rawData); err != nil {
					return fmt.Errorf("no cached stale-backups snapshot. Run with --refresh first")
				}
			}

			entries, err := parseStaleBackupResponse(rawData)
			if err != nil {
				return fmt.Errorf("parsing stale-backup-sets response: %w", err)
			}

			now := time.Now()
			companyNames := loadCompanyNames(db)

			filtered := make([]staleBackupEntry, 0, len(entries))
			for _, e := range entries {
				if e.DaysStale == 0 && e.LastBackupAt != "" {
					if t, perr := parseFlexTime(e.LastBackupAt); perr == nil {
						e.DaysStale = int(now.Sub(t).Hours() / 24)
					}
				}
				if e.DaysStale < days {
					continue
				}
				if engine != "all" && !engineMatches(e.Engine, engine) {
					continue
				}
				if company != 0 && e.CompanyID != company {
					continue
				}
				if name, ok := companyNames[e.CompanyID]; ok {
					e.CompanyName = name
				}
				filtered = append(filtered, e)
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(filtered)
			}

			if len(filtered) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No stale backups match the filter.")
				return nil
			}

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "COMPANY\tENGINE\tBACKUP_ID\tHOSTNAME\tLAST_OK\tDAYS_STALE")
			for _, e := range filtered {
				companyCell := fmt.Sprintf("%s (%d)", coalesceCompany(e.CompanyName), e.CompanyID)
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%d\n",
					companyCell, e.Engine, e.BackupID, e.Hostname, e.LastBackupAt, e.DaysStale)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().IntVar(&days, "days", 3, "Only include backups stale for at least N days")
	cmd.Flags().StringVar(&engine, "engine", "all", "Backup engine filter: classic, restic, dr, all")
	cmd.Flags().IntVar(&company, "company", 0, "Only include this company ID (0 = all)")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Re-pull the live /reports/stale-backup-sets/ and replace the local cache")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/servosity-msp-cli/data.db)")

	return cmd
}

// parseStaleBackupResponse accepts either `[...]` or `{"results":[...]}`
// (also tolerates `data`/`items`) and normalizes each entry into
// staleBackupEntry with snake_case + camelCase field probing.
func parseStaleBackupResponse(raw []byte) ([]staleBackupEntry, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var direct []map[string]any
	if err := json.Unmarshal(raw, &direct); err == nil {
		return normalizeStaleEntries(direct), nil
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, err
	}
	for _, key := range []string{"results", "data", "items"} {
		if rawList, ok := envelope[key]; ok {
			var list []map[string]any
			if err := json.Unmarshal(rawList, &list); err == nil {
				return normalizeStaleEntries(list), nil
			}
		}
	}
	// Last-resort: any single-array value in envelope.
	for _, rawVal := range envelope {
		var list []map[string]any
		if err := json.Unmarshal(rawVal, &list); err == nil && len(list) > 0 {
			return normalizeStaleEntries(list), nil
		}
	}
	return nil, nil
}

func normalizeStaleEntries(rows []map[string]any) []staleBackupEntry {
	out := make([]staleBackupEntry, 0, len(rows))
	for _, r := range rows {
		out = append(out, staleBackupEntry{
			CompanyID:    pickInt(r, "company_id", "companyId", "company"),
			Engine:       normalizeEngineLabel(pickString(r, "backup_engine", "backupEngine", "engine", "type")),
			BackupID:     pickString(r, "backup_id", "backupId", "id", "set_id", "setId"),
			Hostname:     pickString(r, "hostname", "host_name", "host", "device_name", "deviceName"),
			LastBackupAt: pickString(r, "last_backup_at", "lastBackupAt", "last_ok", "last_success", "lastSuccess"),
			DaysStale:    pickInt(r, "days_since_last_backup", "daysSinceLastBackup", "days_stale", "daysStale"),
		})
	}
	return out
}

func pickString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			switch s := v.(type) {
			case string:
				if s != "" {
					return s
				}
			default:
				return fmt.Sprintf("%v", v)
			}
		}
	}
	return ""
}

func pickInt(m map[string]any, keys ...string) int {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			switch n := v.(type) {
			case float64:
				return int(n)
			case int:
				return n
			case int64:
				return int(n)
			case string:
				var i int
				if _, err := fmt.Sscanf(n, "%d", &i); err == nil {
					return i
				}
			}
		}
	}
	return 0
}

// normalizeEngineLabel maps any of the upstream engine spellings down to
// one of our flag values: classic | restic | dr.
func normalizeEngineLabel(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "restic", "restic-backup":
		return "restic"
	case "dr", "dr-backup", "disaster_recovery", "disasterrecovery":
		return "dr"
	case "classic", "spx", "backup":
		return "classic"
	}
	return strings.ToLower(s)
}

func engineMatches(rowEngine, filter string) bool {
	return normalizeEngineLabel(rowEngine) == filter
}

// parseFlexTime tries RFC3339 first, then a few common report-time spellings.
func parseFlexTime(s string) (time.Time, error) {
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	var lastErr error
	for _, l := range layouts {
		t, err := time.Parse(l, s)
		if err == nil {
			return t, nil
		}
		lastErr = err
	}
	return time.Time{}, lastErr
}

// loadCompanyNames pulls id -> name from the companies table (best-effort).
// Returns an empty map on any failure so the display still works with
// just numeric IDs.
func loadCompanyNames(db *store.Store) map[int]string {
	out := map[int]string{}
	rows, err := db.Query(`SELECT id, name FROM companies WHERE name IS NOT NULL AND name != ''`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var idStr, name string
		if err := rows.Scan(&idStr, &name); err != nil {
			continue
		}
		var id int
		if _, err := fmt.Sscanf(idStr, "%d", &id); err == nil {
			out[id] = name
		}
	}
	return out
}

func coalesceCompany(name string) string {
	if name == "" {
		return "(unknown)"
	}
	return name
}
