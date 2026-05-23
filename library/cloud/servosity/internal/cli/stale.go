// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/cloud/servosity/internal/store"
)

func newStaleBackupsCmd(flags *rootFlags) *cobra.Command {
	var days float64
	var resellerFilter, companyFilter, engineFilter, ownerFilter string
	var refresh bool
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "stale-backups",
		Short: "Stale-backup-set query (offline, against the local snapshot of /reports/stale-backup-sets/)",
		Long: `Slice the most-recent /reports/stale-backup-sets/ snapshot by reseller,
company, age window, backup engine, or owner. Pure local SQLite — no API
call per query.

The snapshot is populated by 'stale-backups --refresh' (which pulls the CSV
report once and stores it). Subsequent runs read from the store instantly.

The snapshot also feeds 'drift --metric stale' for day-over-day comparison.

NOTE: '--refresh' generates a fleet-wide CSV server-side and can take 60-120
seconds for large fleets. Use the global --timeout flag to override the
default 30s timeout, e.g. 'servosity-pp-cli --timeout 180s stale-backups
--refresh'.`,
		Example: `  # Refresh the snapshot from the live API, then show >7-day stale sets
  servosity-pp-cli stale-backups --refresh
  servosity-pp-cli stale-backups --days 7 --json

  # Filter to one reseller's restic backups
  servosity-pp-cli stale-backups --reseller 12 --engine restic --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				if flags.asJSON {
					_, _ = cmd.OutOrStdout().Write([]byte(`{"meta":{"source":"dry-run"},"results":[]}` + "\n"))
				}
				return nil
			}
			ctx := cmd.Context()
			if dbPath == "" {
				dbPath = defaultDBPath("servosity-pp-cli")
			}
			st, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return configErr(fmt.Errorf("open store: %w", err))
			}
			defer st.Close()
			if err := st.EnsureNovelTables(ctx); err != nil {
				return apiErr(err)
			}

			if refresh {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				rid, n, rerr := refreshStaleSnapshot(ctx, c, st)
				if rerr != nil {
					return classifyAPIError(rerr, flags)
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "stale: refreshed snapshot run_id=%s (%d rows)\n", rid, n)
			}

			runID, err := st.LatestStaleRunID(ctx)
			if err != nil {
				return apiErr(err)
			}
			if runID == "" {
				return notFoundErr(fmt.Errorf("no stale snapshot found — run 'stale --refresh' first to pull /reports/stale-backup-sets/"))
			}

			rows, err := st.StaleAt(ctx, runID, store.StaleFilter{
				Reseller: resellerFilter, Company: companyFilter, Engine: engineFilter,
				Owner: ownerFilter, MinDays: days, Limit: limit,
			})
			if err != nil {
				return apiErr(err)
			}

			out := buildStaleView(runID, rows)
			payload, _ := json.Marshal(out)
			return printOutputWithFlags(cmd.OutOrStdout(), payload, flags)
		},
	}
	cmd.Flags().Float64Var(&days, "days", 0, "Minimum days since last successful backup")
	cmd.Flags().StringVar(&resellerFilter, "reseller", "", "Reseller name filter (exact match)")
	cmd.Flags().StringVar(&companyFilter, "company", "", "Company name filter (exact match)")
	cmd.Flags().StringVar(&engineFilter, "engine", "", "Backup engine filter: classic | restic | dr")
	cmd.Flags().StringVar(&ownerFilter, "owner", "", "Owner / dedicated-support-staff filter")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Pull a fresh snapshot from /reports/stale-backup-sets/ before querying")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows (0 = no limit)")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite path (default: ~/.local/share/servosity-pp-cli/data.db)")
	return cmd
}

func buildStaleView(runID string, rows []store.StaleSnapshotRow) map[string]any {
	type rowOut struct {
		Reseller       string  `json:"reseller,omitempty"`
		Company        string  `json:"company,omitempty"`
		BackupAccount  string  `json:"backup_account,omitempty"`
		BackupSet      string  `json:"backup_set,omitempty"`
		LastCompleteAt string  `json:"last_complete_at,omitempty"`
		DaysStale      float64 `json:"days_stale"`
		Engine         string  `json:"engine,omitempty"`
		Owner          string  `json:"owner,omitempty"`
	}
	out := make([]rowOut, 0, len(rows))
	for _, r := range rows {
		out = append(out, rowOut{
			Reseller: r.Reseller, Company: r.Company,
			BackupAccount: r.BackupAccount, BackupSet: r.BackupSet,
			LastCompleteAt: r.LastCompleteAt, DaysStale: r.DaysStale,
			Engine: r.Engine, Owner: r.Owner,
		})
	}
	return map[string]any{
		"meta": map[string]any{
			"source": "store",
			"run_id": runID,
			"count":  len(out),
		},
		"results": out,
	}
}

// refreshStaleSnapshot pulls /reports/stale-backup-sets/ (CSV) and persists it.
// Returns runID and row count.
func refreshStaleSnapshot(ctx context.Context, c interface {
	GetWithHeaders(path string, params map[string]string, headers map[string]string) (json.RawMessage, error)
	Get(path string, params map[string]string) (json.RawMessage, error)
}, st *store.Store) (string, int, error) {
	// Ask for CSV explicitly — the report endpoints are CSV/JSON depending on Accept.
	headers := map[string]string{"Accept": "text/csv"}
	data, err := c.GetWithHeaders("/reports/stale-backup-sets/", nil, headers)
	if err != nil {
		// Fall back to default JSON
		data, err = c.Get("/reports/stale-backup-sets/", nil)
		if err != nil {
			return "", 0, fmt.Errorf("fetch /reports/stale-backup-sets/: %w", err)
		}
	}
	rows := parseStaleResponse(data)
	rid, werr := st.WriteStaleSnapshot(ctx, rows)
	if werr != nil {
		return "", 0, werr
	}
	return rid, len(rows), nil
}

// parseStaleResponse decodes either a CSV body or a JSON envelope. The Servosity
// /reports/stale-backup-sets/ endpoint returns CSV by default; some MSP
// integrations request JSON. Both shapes resolve to the same row shape.
func parseStaleResponse(data json.RawMessage) []store.StaleSnapshotRow {
	body := []byte(data)
	// Strip JSON-string wrapping if the report came back as a quoted string
	if len(body) > 1 && body[0] == '"' {
		var s string
		if err := json.Unmarshal(body, &s); err == nil {
			body = []byte(s)
		}
	}
	if isCSVBody(body) {
		return parseStaleCSV(body)
	}
	return parseStaleJSON(body)
}

func isCSVBody(b []byte) bool {
	// Simple heuristic: if we see a header-row-looking line ending in \n with commas,
	// and not starting with `[` or `{`, treat as CSV.
	for i, c := range b {
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			continue
		}
		return c != '[' && c != '{' && i < len(b)-1
	}
	return false
}

func parseStaleCSV(body []byte) []store.StaleSnapshotRow {
	r := csv.NewReader(strings.NewReader(string(body)))
	r.FieldsPerRecord = -1
	all, err := r.ReadAll()
	if err != nil || len(all) < 2 {
		return nil
	}
	header := all[0]
	idx := func(want ...string) int {
		for _, w := range want {
			for i, h := range header {
				if strings.EqualFold(strings.TrimSpace(h), w) {
					return i
				}
			}
		}
		return -1
	}
	iReseller := idx("Partner", "Reseller", "reseller", "partner", "reseller_name")
	iCompany := idx("Company", "company", "company_name")
	iAccount := idx("Backup", "Backup Account", "BackupAccount", "backup_account", "Account", "account")
	iSet := idx("Backup Set", "BackupSet", "backup_set", "Set", "set")
	iLast := idx("Last Backup Date", "Last Backup Complete", "LastBackupComplete", "last_complete_at", "Last Complete")
	iDays := idx("Days Stale", "DaysStale", "days_stale", "Days")
	iEngine := idx("Engine", "engine", "Backup Type", "BackupType", "Destination")
	iOwner := idx("Owner", "owner", "Dedicated Support Staff")
	now := time.Now().UTC()
	out := make([]store.StaleSnapshotRow, 0, len(all)-1)
	for _, row := range all[1:] {
		if len(row) == 0 {
			continue
		}
		get := func(i int) string {
			if i < 0 || i >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[i])
		}
		days := 0.0
		if d := get(iDays); d != "" {
			if v, err := strconv.ParseFloat(d, 64); err == nil {
				days = v
			}
		}
		// If the report has a Last-Backup-Date column but no Days-Stale column,
		// compute days_stale ourselves so the CLI's --days filter works.
		lastDate := get(iLast)
		if days == 0 && lastDate != "" {
			if t, err := parseLooseDate(lastDate); err == nil {
				delta := now.Sub(t).Hours() / 24.0
				if delta < 0 {
					delta = 0
				}
				days = delta
			}
		}
		// Echo the raw row as a JSON object for forensic recovery
		obj := map[string]string{}
		for i, h := range header {
			if i < len(row) {
				obj[strings.TrimSpace(h)] = strings.TrimSpace(row[i])
			}
		}
		raw, _ := json.Marshal(obj)
		out = append(out, store.StaleSnapshotRow{
			Reseller: get(iReseller), Company: get(iCompany),
			BackupAccount: get(iAccount), BackupSet: get(iSet),
			LastCompleteAt: lastDate, DaysStale: days,
			Engine: inferEngineFromContext(get(iSet), get(iAccount), get(iEngine)),
			Owner:  get(iOwner),
			Raw:    raw,
		})
	}
	return out
}

func parseStaleJSON(body []byte) []store.StaleSnapshotRow {
	// JSON might be either a paginated envelope or a flat list. Each item:
	// {reseller, company, backup_account, backup_set, last_complete_at, days_stale, engine, owner}
	var items []map[string]any
	if err := json.Unmarshal(body, &items); err != nil {
		var env struct {
			Results []map[string]any `json:"results"`
		}
		if err := json.Unmarshal(body, &env); err != nil {
			return nil
		}
		items = env.Results
	}
	out := make([]store.StaleSnapshotRow, 0, len(items))
	for _, m := range items {
		raw, _ := json.Marshal(m)
		days := 0.0
		switch v := m["days_stale"].(type) {
		case float64:
			days = v
		case string:
			if x, err := strconv.ParseFloat(v, 64); err == nil {
				days = x
			}
		}
		out = append(out, store.StaleSnapshotRow{
			Reseller:       anyToString(m["reseller"]),
			Company:        anyToString(m["company"]),
			BackupAccount:  anyToString(m["backup_account"]),
			BackupSet:      anyToString(m["backup_set"]),
			LastCompleteAt: anyToString(m["last_complete_at"]),
			DaysStale:      days,
			Engine:         normalizeEngine(anyToString(m["engine"])),
			Owner:          anyToString(m["owner"]),
			Raw:            raw,
		})
	}
	return out
}

func anyToString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case nil:
		return ""
	}
	b, _ := json.Marshal(v)
	return string(b)
}

// parseLooseDate accepts the Servosity stale-backup-sets timestamp formats
// (RFC3339-ish with optional fractional seconds, no zone, e.g.
// "2026-05-11T21:19:39.738000") plus a few common variants.
func parseLooseDate(s string) (time.Time, error) {
	layouts := []string{
		"2006-01-02T15:04:05.999999",
		"2006-01-02T15:04:05.999",
		"2006-01-02T15:04:05",
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unparsable date: %q", s)
}

// inferEngineFromContext picks an engine label from the Backup Set / Backup /
// Destination text. The Servosity stale-backup-sets CSV has no explicit Engine
// column so we infer: "DR Backup" / "DR Imaging" / "spx" → "dr"; restic-style
// names → "restic"; "ServosityUnlimited"/"default-backup-set" → "classic".
func inferEngineFromContext(backupSet, backupAccount, dest string) string {
	hay := strings.ToLower(backupSet + " " + backupAccount + " " + dest)
	switch {
	case strings.Contains(hay, "dr backup") || strings.Contains(hay, "dr-backup") ||
		strings.Contains(hay, "dr imaging") || strings.Contains(hay, "spx"):
		return "dr"
	case strings.Contains(hay, "restic"):
		return "restic"
	default:
		return "classic"
	}
}

func normalizeEngine(s string) string {
	t := strings.ToLower(strings.TrimSpace(s))
	switch t {
	case "restic", "restic-backup", "resticbackup":
		return "restic"
	case "dr", "drbackup", "dr-backup", "dr_imaging", "dr imaging":
		return "dr"
	case "classic", "backup", "spx":
		return "classic"
	}
	return t
}

// keep time import used
var _ = time.Now
