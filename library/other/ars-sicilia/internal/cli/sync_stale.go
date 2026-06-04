// pp:data-source local
// Novel feature — stato della sync per ognuno dei 12 archivi ARS.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	icaro "github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/internal/icaroclient"
	"github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/internal/store"
	"github.com/spf13/cobra"
)

func newNovelSyncStaleCmd(flags *rootFlags) *cobra.Command {
	var (
		flagDB     string
		flagMaxAge string
	)
	cmd := &cobra.Command{
		Use:     "stale",
		Short:   "Mostra timestamp ultima sync, n. record locali e staleness per ognuno dei 12 archivi.",
		Example: "  ars-sicilia-pp-cli sync stale --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			return runSyncStale(cmd, flags, flagDB, flagMaxAge)
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "Percorso del database SQLite (default: ~/.local/share/ars-sicilia-pp-cli/store.db).")
	cmd.Flags().StringVar(&flagMaxAge, "max-age", "7d", "Soglia di staleness (es. 24h, 7d).")
	return cmd
}

type staleEntry struct {
	Archivio   string `json:"archivio"`
	ArchiveID  string `json:"archive_id"`
	LastSync   string `json:"last_sync,omitempty"`
	AgeSeconds int64  `json:"age_seconds,omitempty"`
	AgeHuman   string `json:"age_human,omitempty"`
	Stale      bool   `json:"stale"`
	Records    int64  `json:"records"`
	Hint       string `json:"hint,omitempty"`
}

func runSyncStale(cmd *cobra.Command, flags *rootFlags, dbPath, maxAge string) error {
	if dbPath == "" {
		dbPath = defaultDBPath("ars-sicilia-pp-cli")
	}
	threshold := parseDurationLoose(maxAge, 7*24*time.Hour)
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	db, err := store.OpenReadOnly(dbPath)
	if err != nil {
		// Print a graceful empty report — first-use case before any sync.
		report := []staleEntry{}
		for _, arc := range icaro.All {
			report = append(report, staleEntry{
				Archivio:  arc.Slug,
				ArchiveID: arc.ID,
				Stale:     true,
				Hint:      "Nessun database locale trovato. Esegui `ars-sicilia-pp-cli sync` per crearlo.",
			})
		}
		return emitJSONOrTable(cmd, flags, report)
	}
	defer db.Close()

	entries := make([]staleEntry, 0, len(icaro.All))
	now := time.Now()
	for _, arc := range icaro.All {
		e := staleEntry{Archivio: arc.Slug, ArchiveID: arc.ID}
		// Conteggio righe nella tabella resources con resource_type = slug.
		// Tutte le tabelle generic sono mappate su `resources`.
		var n int64
		row := db.DB().QueryRowContext(ctx,
			`SELECT COUNT(*) FROM resources WHERE resource_type = ?`, arc.Slug)
		_ = row.Scan(&n)
		e.Records = n

		// Last sync timestamp (sync_state table emitted by the framework).
		var last sql.NullString
		row = db.DB().QueryRowContext(ctx,
			`SELECT last_synced_at FROM sync_state WHERE resource_type = ?`, arc.Slug)
		_ = row.Scan(&last)
		if last.Valid && last.String != "" {
			e.LastSync = last.String
			if t, err := time.Parse(time.RFC3339, last.String); err == nil {
				age := now.Sub(t)
				e.AgeSeconds = int64(age.Seconds())
				e.AgeHuman = humanizeDuration(age)
				if age > threshold {
					e.Stale = true
					e.Hint = fmt.Sprintf("Sync vecchia di %s. Esegui `ars-sicilia-pp-cli sync --resources %s` per rinfrescare.", e.AgeHuman, arc.Slug)
				}
			}
		} else if n == 0 {
			e.Stale = true
			e.Hint = "Mai sincronizzato. Esegui `ars-sicilia-pp-cli sync --resources " + arc.Slug + "`."
		}
		entries = append(entries, e)
	}
	// Stable order: stale first, then alpha.
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Stale != entries[j].Stale {
			return entries[i].Stale
		}
		return entries[i].Archivio < entries[j].Archivio
	})
	return emitJSONOrTable(cmd, flags, entries)
}

func emitJSONOrTable(cmd *cobra.Command, flags *rootFlags, entries []staleEntry) error {
	out := cmd.OutOrStdout()
	if flags.asJSON || !isTerminal(out) {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	}
	fmt.Fprintf(out, "%-15s %-10s %-25s %-10s %-8s  %s\n", "ARCHIVIO", "ID", "ULTIMA SYNC", "ETÀ", "RECORDS", "NOTE")
	for _, e := range entries {
		marker := " "
		if e.Stale {
			marker = "!"
		}
		fmt.Fprintf(out, "%s%-14s %-10s %-25s %-10s %-8d  %s\n",
			marker, e.Archivio, e.ArchiveID, valueOr(e.LastSync, "—"),
			valueOr(e.AgeHuman, "—"), e.Records, e.Hint)
	}
	return nil
}

func valueOr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func humanizeDuration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func parseDurationLoose(s string, fallback time.Duration) time.Duration {
	if s == "" {
		return fallback
	}
	if d, err := time.ParseDuration(s); err == nil {
		return d
	}
	// Try day suffix.
	if len(s) > 1 && s[len(s)-1] == 'd' {
		var n int
		_, err := fmt.Sscanf(s, "%dd", &n)
		if err == nil && n > 0 {
			return time.Duration(n) * 24 * time.Hour
		}
	}
	return fallback
}
