// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: show the local mutation audit log (every sync, bulk-update,
// import this CLI ran), read from the hubspot_local_mutations table.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/store"
	"github.com/spf13/cobra"
)

// mutationStartKey types the cmd.Context value used to pass the per-command
// start timestamp from PersistentPreRunE to PersistentPostRunE. Private type
// so unrelated packages cannot accidentally collide on it.
type mutationStartKey struct{}

func withMutationStart(ctx context.Context, start time.Time) context.Context {
	return context.WithValue(ctx, mutationStartKey{}, start)
}

func mutationStartFrom(ctx context.Context) (time.Time, bool) {
	if ctx == nil {
		return time.Time{}, false
	}
	v, ok := ctx.Value(mutationStartKey{}).(time.Time)
	return v, ok
}

// mutatingCommands is the allowlist of fully-qualified command paths whose
// runs should be appended to the local audit log. Paths are relative to the
// CLI name (the leaf set Cobra returns from cmd.CommandPath(), minus the
// "hubspot-pp-cli " prefix). Keep this tight — the win is "I can see what I
// just did," not perfect coverage of every endpoint mirror.
var mutatingCommands = map[string]bool{
	"sync":                 true,
	"contacts bulk-update": true,
	"import":               true,
}

// logMutationIfApplicable writes one audit-log row for the just-finished
// command iff it is in the mutating allowlist. Best-effort: every failure
// path is a silent no-op so audit-logging never breaks a user command.
func logMutationIfApplicable(cmd *cobra.Command, args []string, exitCode int) {
	if cmd == nil {
		return
	}
	root := cmd.Root()
	if root == nil {
		return
	}
	full := cmd.CommandPath()
	rel := strings.TrimPrefix(full, root.Name()+" ")
	if rel == full || !mutatingCommands[rel] {
		return
	}
	start, ok := mutationStartFrom(cmd.Context())
	var duration time.Duration
	if ok {
		duration = time.Since(start)
	}
	// Prefer the just-finished command's --db flag (sync/import/contacts
	// bulk-update all expose one) so the audit row lands in the same SQLite
	// file the user just wrote to. Fall back to the canonical default.
	dbPath := defaultDBPath("hubspot-pp-cli")
	if dbFlag := cmd.Flag("db"); dbFlag != nil {
		if v := dbFlag.Value.String(); v != "" {
			dbPath = v
		}
	}
	// Use a detached background context — the foreground cmd.Context() may
	// already be cancelled by the time PersistentPostRunE runs (e.g. on
	// SIGINT). Bound the audit write so a hung database file does not stall
	// the process indefinitely.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return
	}
	defer db.Close()
	db.LogMutation(ctx, rel, sanitizeArgs(args), exitCode, -1, "cli", duration)
}

// sanitizeArgs returns a copy of args with anything that looks like a
// credential token or env-injected secret elided. Best-effort — the audit
// log is local-only, but we still avoid persisting raw tokens.
func sanitizeArgs(args []string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		// Strip values after well-known credential flags (e.g. --token=xyz,
		// --token xyz). The next arg gets elided too when the previous was
		// a known credential flag.
		out[i] = a
	}
	for i := 0; i < len(out); i++ {
		lower := strings.ToLower(out[i])
		if strings.HasPrefix(lower, "--token=") || strings.HasPrefix(lower, "--api-key=") || strings.HasPrefix(lower, "--secret=") {
			out[i] = strings.SplitN(out[i], "=", 2)[0] + "=[REDACTED]"
			continue
		}
		if i+1 < len(out) && (lower == "--token" || lower == "--api-key" || lower == "--secret") {
			out[i+1] = "[REDACTED]"
		}
	}
	return out
}

func newHistoryCmd(flags *rootFlags) *cobra.Command {
	var sinceArg string
	var kinds []string
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:         "history",
		Short:       "Show local mutation audit log for this CLI",
		Long:        `Read the local hubspot_local_mutations table and emit every mutation event (sync, contacts bulk-update, import) the CLI has run, most recent first. Filter by --since and --kind. The audit log is best-effort and local to this machine; failures to write it never block the foreground command.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  # Last 24h of all mutations
  hubspot-pp-cli history

  # Sync events in the past hour
  hubspot-pp-cli history --since 1h --kind sync

  # Multiple kinds (OR)
  hubspot-pp-cli history --kind sync --kind 'contacts bulk-update'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			cutoff, err := parseSinceArg(sinceArg)
			if err != nil {
				return fmt.Errorf("--since: %w", err)
			}
			if dbPath == "" {
				dbPath = defaultDBPath("hubspot-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			rows, err := queryHistory(cmd, db, cutoff, kinds, limit)
			if err != nil {
				return err
			}

			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"since":   cutoff.UTC().Format(time.RFC3339),
					"kinds":   kinds,
					"limit":   limit,
					"results": rows,
				})
			}
			headers := []string{"timestamp", "command", "exit_code", "affected", "duration_ms", "args"}
			out := make([][]string, 0, len(rows))
			for _, r := range rows {
				out = append(out, []string{
					r.Timestamp,
					r.Command,
					fmt.Sprintf("%d", r.ExitCode),
					fmt.Sprintf("%d", r.AffectedCount),
					fmt.Sprintf("%d", r.DurationMs),
					truncate(r.ArgsSummary, 60),
				})
			}
			return flags.printTable(cmd, headers, out)
		},
	}
	cmd.Flags().StringVar(&sinceArg, "since", "24h", "Only events newer than this duration (e.g. 1h, 24h, 7d) or RFC3339 timestamp")
	cmd.Flags().StringSliceVar(&kinds, "kind", nil, "Filter by command (e.g. 'sync', 'contacts bulk-update'); repeatable for OR")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum number of rows to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

type historyRow struct {
	ID            int64  `json:"id"`
	Timestamp     string `json:"timestamp"`
	Command       string `json:"command"`
	ArgsJSON      string `json:"args_json"`
	ArgsSummary   string `json:"args_summary"`
	ExitCode      int    `json:"exit_code"`
	AffectedCount int    `json:"affected_count"`
	Source        string `json:"source"`
	DurationMs    int64  `json:"duration_ms"`
}

func queryHistory(cmd *cobra.Command, db *store.Store, cutoff time.Time, kinds []string, limit int) ([]historyRow, error) {
	q := `SELECT id, timestamp, command, args_json, exit_code, affected_count, source, duration_ms
FROM hubspot_local_mutations
WHERE timestamp >= ?`
	args := []any{cutoff.UTC().Format("2006-01-02 15:04:05")}
	if len(kinds) > 0 {
		placeholders := make([]string, len(kinds))
		for i, k := range kinds {
			placeholders[i] = "?"
			args = append(args, k)
		}
		q += " AND command IN (" + strings.Join(placeholders, ", ") + ")"
	}
	q += " ORDER BY timestamp DESC, id DESC LIMIT ?"
	args = append(args, limit)

	rows, err := db.DB().QueryContext(cmd.Context(), q, args...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	out := []historyRow{}
	for rows.Next() {
		var r historyRow
		var argsCell, sourceCell sql.NullString
		if err := rows.Scan(&r.ID, &r.Timestamp, &r.Command, &argsCell, &r.ExitCode, &r.AffectedCount, &sourceCell, &r.DurationMs); err != nil {
			return nil, err
		}
		r.ArgsJSON = nullStr(argsCell)
		r.ArgsSummary = r.ArgsJSON
		r.Source = nullStr(sourceCell)
		out = append(out, r)
	}
	return out, nil
}

// parseSinceArg accepts either a relative duration ("1h", "24h", "7d", "30m",
// "1w") or an RFC3339 absolute timestamp. Returns the absolute cutoff time
// the query should use as a lower bound.
func parseSinceArg(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Now().Add(-24 * time.Hour), nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}
	// parseSinceDuration (sync.go) already handles d/h/w/m forms and returns
	// an absolute time anchored to time.Now(). Reuse it so the two flags
	// share one grammar.
	return parseSinceDuration(s)
}
