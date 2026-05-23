// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/cloud/servosity/internal/store"
)

// newBackupFactsCmd queries the cross-engine view (synthesized at runtime from
// classic backups + restic_backups + dr_backups) for "give me one row per
// backup regardless of engine, with last_successful_at and current_status."
//
// The view is synthesized at runtime from whatever the synced tables hold,
// so it works without a schema migration step. Run `sync` to populate.
func newBackupFactsCmd(flags *rootFlags) *cobra.Command {
	var companyFilter, engineFilter, statusFilter string
	var lastSuccessBefore string
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "backup-facts",
		Short: "Cross-engine backup view: classic + restic + DR unified by company / last successful job / status",
		Long: `Query the unified backup_facts view (engine + id + company_id +
last_successful_at + last_status + size_bytes) over all three backup engines
synced into the local store. The Servosity API has no cross-engine surface;
this view exists only here.

Run 'sync' first to populate. Then this command runs entirely against
the local SQLite store.`,
		Example: `  # Every backup that has not had a successful run since 2026-05-04
  servosity-pp-cli backup-facts --last-success-before 2026-05-04 --json

  # Restic backups for one company
  servosity-pp-cli backup-facts --company "ACME Corp" --engine restic --json`,
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

			cutoff := time.Time{}
			if lastSuccessBefore != "" {
				t, err := parseHumanTime(lastSuccessBefore, time.Now())
				if err != nil {
					return usageErr(err)
				}
				cutoff = t
			}

			facts, err := readBackupFacts(ctx, st, companyFilter, engineFilter, statusFilter, cutoff, limit)
			if err != nil {
				return apiErr(err)
			}

			out := map[string]any{
				"meta": map[string]any{
					"source":  "store",
					"db":      dbPath,
					"count":   len(facts),
					"engines": []string{"classic", "restic", "dr"},
				},
				"results": facts,
			}
			payload, _ := json.Marshal(out)
			return printOutputWithFlags(cmd.OutOrStdout(), payload, flags)
		},
	}
	cmd.Flags().StringVar(&companyFilter, "company", "", "Company name (substring match) or numeric ID")
	cmd.Flags().StringVar(&engineFilter, "engine", "", "Backup engine: classic | restic | dr")
	cmd.Flags().StringVar(&statusFilter, "status", "", "Last status filter (substring match)")
	cmd.Flags().StringVar(&lastSuccessBefore, "last-success-before", "", "Show only backups with last successful job before this human time (e.g. 'yesterday', '7d', '2026-05-04')")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows (0 = no limit)")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite path (default: ~/.local/share/servosity-pp-cli/data.db)")
	return cmd
}

// BackupFact is one normalized cross-engine row.
type BackupFact struct {
	Engine           string `json:"engine"`
	ID               string `json:"id"`
	CompanyID        string `json:"company_id,omitempty"`
	CompanyName      string `json:"company_name,omitempty"`
	DeviceName       string `json:"device_name,omitempty"`
	LastSuccessfulAt string `json:"last_successful_at,omitempty"`
	LastStatus       string `json:"last_status,omitempty"`
	SizeBytes        int64  `json:"size_bytes,omitempty"`
}

// readBackupFacts walks each backup-engine table the generator emitted (classic
// `backups`, `restic_backups`, `dr_backups`) and unifies the rows. We don't
// hardcode a schema — we read each row's data JSON and pluck candidate fields,
// because the generator's tables typically have an `id` column plus a `data`
// JSON column with the full server response.
func readBackupFacts(ctx context.Context, st *store.Store, company, engine, status string, cutoff time.Time, limit int) ([]BackupFact, error) {
	tables := []struct {
		engine string
		table  string
	}{
		{"classic", "backups"},
		{"restic", "restic_backups"},
		{"dr", "dr_backups"},
	}
	var out []BackupFact
	for _, t := range tables {
		if engine != "" && t.engine != engine {
			continue
		}
		rows, err := readBackupTable(ctx, st, t.engine, t.table)
		if err != nil {
			// Table may not exist yet (sync hasn't covered this engine). Skip silently.
			continue
		}
		for _, row := range rows {
			if company != "" && !companyMatch(row, company) {
				continue
			}
			if status != "" && !strings.Contains(strings.ToLower(row.LastStatus), strings.ToLower(status)) {
				continue
			}
			if !cutoff.IsZero() && row.LastSuccessfulAt != "" {
				if t, err := time.Parse(time.RFC3339, row.LastSuccessfulAt); err == nil {
					if !t.Before(cutoff) {
						continue
					}
				}
			}
			out = append(out, row)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		// Oldest last-success first (most stale)
		return out[i].LastSuccessfulAt < out[j].LastSuccessfulAt
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func companyMatch(r BackupFact, want string) bool {
	w := strings.ToLower(want)
	return strings.Contains(strings.ToLower(r.CompanyName), w) ||
		strings.Contains(strings.ToLower(r.CompanyID), w)
}

func readBackupTable(ctx context.Context, st *store.Store, engine, table string) ([]BackupFact, error) {
	if !validIdentifier(table) {
		return nil, fmt.Errorf("invalid table identifier: %q", table)
	}
	q := fmt.Sprintf("SELECT id, data FROM %s LIMIT 100000", table)
	rows, err := st.DB().QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BackupFact
	for rows.Next() {
		var id string
		var dataStr string
		if err := rows.Scan(&id, &dataStr); err != nil {
			continue
		}
		obj := map[string]any{}
		_ = json.Unmarshal([]byte(dataStr), &obj)
		fact := BackupFact{Engine: engine, ID: id}
		fact.CompanyID = anyToString(firstAny(obj, "company_id", "company"))
		fact.CompanyName = anyToString(firstAny(obj, "company_name", "company__name", "name"))
		fact.DeviceName = anyToString(firstAny(obj, "device_name", "hostname"))
		fact.LastSuccessfulAt = anyToString(firstAny(obj, "last_successful_at", "last_complete_at", "last_backup_complete", "last_backup_at"))
		fact.LastStatus = anyToString(firstAny(obj, "last_status", "status", "current_status", "state"))
		switch v := firstAny(obj, "size_bytes", "size").(type) {
		case float64:
			fact.SizeBytes = int64(v)
		}
		out = append(out, fact)
	}
	return out, rows.Err()
}

func firstAny(obj map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := obj[k]; ok && v != nil {
			return v
		}
	}
	return nil
}

func validIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if !(r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (i > 0 && r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}
