// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

// psx_snapshot.go is the local-history substrate. The PSX portal renders only
// a current view and retains nothing per user, so every longitudinal command
// (diff, drift, unusual, rotation) reads accumulated snapshots written here.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/psx/internal/psx"
	"github.com/mvanhorn/printing-press-library/library/payments/psx/internal/store"
)

const (
	snapshotKindMarket   = "market"
	snapshotKindScreener = "screener"
	snapshotKindSector   = "sector"
)

// openLocalStore resolves the CLI database and opens it. Callers that only read
// should check mirrorMissing first so an unsynced CLI returns an empty result
// instead of a SQLite error.
func openLocalStore(ctx context.Context, dbPath string) (*store.Store, string, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("psx-pp-cli")
	}
	s, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, dbPath, fmt.Errorf("opening database %s: %w", dbPath, err)
	}
	return s, dbPath, nil
}

// mirrorMissing reports whether the local database file exists yet.
func mirrorMissing(dbPath string) bool {
	if dbPath == "" {
		dbPath = defaultDBPath("psx-pp-cli")
	}
	_, err := os.Stat(dbPath)
	return os.IsNotExist(err)
}

// writeMirrorHint prints the sync command that would populate an empty mirror,
// and emits an empty machine result so agents get valid JSON rather than an error.
func writeMirrorHint(cmd *cobra.Command, flags *rootFlags, dbPath, what string) error {
	fmt.Fprintf(cmd.ErrOrStderr(), "no local snapshot history at %s\nrun: psx-pp-cli snapshot take --db %s\n", dbPath, dbPath)
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printJSONFiltered(cmd.OutOrStdout(), make([]map[string]any, 0), flags)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "No %s history yet.\n", what)
	return nil
}

// snapshotRow is one persisted row of a captured table.
type snapshotRow struct {
	TakenAt string
	Symbol  string
	Data    map[string]string
}

// takeSnapshot captures one portal table into psx_snapshots. Snapshots are
// append-only: the whole point is that PSX forgets and we do not.
func takeSnapshot(ctx context.Context, s *store.Store, c *psx.Client, kind string) (int, string, error) {
	var path, keyCol string
	var mustHave []string
	switch kind {
	case snapshotKindMarket:
		path, keyCol, mustHave = "/market-watch", "symbol", []string{"symbol", "current"}
	case snapshotKindScreener:
		path, keyCol, mustHave = "/screener", "symbol", []string{"symbol"}
	case snapshotKindSector:
		path, keyCol, mustHave = "/sector-summary/sectorwise", "sector_name", nil
	default:
		return 0, "", fmt.Errorf("unknown snapshot kind %q", kind)
	}

	t, err := fetchTable(ctx, c, path, "", mustHave...)
	if err != nil {
		return 0, "", err
	}
	takenAt := nowUTC().Format(snapshotTimeFormat)

	tx, err := s.DB().BeginTx(ctx, nil)
	if err != nil {
		return 0, "", fmt.Errorf("begin snapshot tx: %w", err)
	}
	stmt, err := tx.PrepareContext(ctx,
		`INSERT OR REPLACE INTO psx_snapshots (taken_at, kind, symbol, data) VALUES (?, ?, ?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		return 0, "", fmt.Errorf("prepare snapshot insert: %w", err)
	}
	defer stmt.Close()

	n := 0
	for _, row := range t.Rows {
		key := strings.TrimSpace(row[keyCol])
		if key == "" {
			continue
		}
		blob, err := json.Marshal(row)
		if err != nil {
			continue
		}
		if _, err := stmt.ExecContext(ctx, takenAt, kind, key, string(blob)); err != nil {
			_ = tx.Rollback()
			return 0, "", fmt.Errorf("insert snapshot row %s: %w", key, err)
		}
		n++
	}
	if err := tx.Commit(); err != nil {
		return 0, "", fmt.Errorf("commit snapshot: %w", err)
	}
	return n, takenAt, nil
}

// listSnapshotTimes returns distinct capture timestamps for a kind, newest first.
func listSnapshotTimes(ctx context.Context, s *store.Store, kind string) ([]string, error) {
	rows, err := s.DB().QueryContext(ctx,
		`SELECT DISTINCT taken_at FROM psx_snapshots WHERE kind = ? ORDER BY taken_at DESC`, kind)
	if err != nil {
		return nil, fmt.Errorf("listing snapshots: %w", err)
	}
	out := make([]string, 0)
	for rows.Next() {
		var t sql.NullString
		if err := rows.Scan(&t); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scanning snapshot time: %w", err)
		}
		if t.String != "" {
			out = append(out, t.String)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}

// loadSnapshot reads every row captured at one timestamp. The result set is
// drained and closed before any caller issues follow-up queries, because SQLite
// keeps a single connection and nested queries deadlock or fail.
func loadSnapshot(ctx context.Context, s *store.Store, kind, takenAt string) (map[string]map[string]string, error) {
	rows, err := s.DB().QueryContext(ctx,
		`SELECT symbol, data FROM psx_snapshots WHERE kind = ? AND taken_at = ?`, kind, takenAt)
	if err != nil {
		return nil, fmt.Errorf("loading snapshot: %w", err)
	}
	out := make(map[string]map[string]string)
	for rows.Next() {
		var sym, data sql.NullString
		if err := rows.Scan(&sym, &data); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scanning snapshot: %w", err)
		}
		var m map[string]string
		if err := json.Unmarshal([]byte(data.String), &m); err != nil {
			continue
		}
		out[sym.String] = m
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}

// loadSeries returns one symbol's snapshot history for a kind, oldest first.
func loadSeries(ctx context.Context, s *store.Store, kind, symbol, since string) ([]snapshotRow, error) {
	q := `SELECT taken_at, data FROM psx_snapshots WHERE kind = ? AND symbol = ?`
	args := []any{kind, strings.ToUpper(strings.TrimSpace(symbol))}
	if since != "" {
		q += ` AND taken_at >= ?`
		args = append(args, since)
	}
	q += ` ORDER BY taken_at ASC`
	rows, err := s.DB().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("loading series: %w", err)
	}
	out := make([]snapshotRow, 0)
	for rows.Next() {
		var at, data sql.NullString
		if err := rows.Scan(&at, &data); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scanning series: %w", err)
		}
		var m map[string]string
		if err := json.Unmarshal([]byte(data.String), &m); err != nil {
			continue
		}
		out = append(out, snapshotRow{TakenAt: at.String, Symbol: strings.ToUpper(symbol), Data: m})
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}

// parseNum strips the portal's thousands separators, percent signs and unit
// suffixes (12.3M, 4.5B) and returns a float. Unparseable input reports false
// rather than silently becoming zero.
func parseNum(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return 0, false
	}
	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimSuffix(s, "%")
	mult := 1.0
	if n := len(s); n > 0 {
		switch s[n-1] {
		case 'K', 'k':
			mult, s = 1e3, s[:n-1]
		case 'M', 'm':
			mult, s = 1e6, s[:n-1]
		case 'B', 'b':
			mult, s = 1e9, s[:n-1]
		case 'T':
			mult, s = 1e12, s[:n-1]
		}
	}
	// Movers render change as "1.00 (103.09%)"; take the leading number.
	if i := strings.IndexAny(s, " ("); i > 0 {
		s = s[:i]
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, false
	}
	return v * mult, true
}

// newSnapshotCmd captures the current market/screener/sector tables so later
// runs have something to compare against.
func newSnapshotCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "snapshot",
		Short:       "Capture and inspect local point-in-time market history",
		Example:     "  psx-pp-cli snapshot take --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}

	var dbPath string
	var kinds string
	take := &cobra.Command{
		Use:   "take",
		Short: "Capture the current market, screener and sector tables into local history",
		Long: "Use this command to record a point-in-time snapshot the portal will not keep.\n" +
			"Run it on a schedule; 'diff', 'drift', 'unusual' and 'rotation' all read what it stores.",
		Example:     "  psx-pp-cli snapshot take --json",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "snapshot take")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			s, path, err := openLocalStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer s.Close()

			want := []string{snapshotKindMarket, snapshotKindScreener, snapshotKindSector}
			if strings.TrimSpace(kinds) != "" {
				want = nil
				for _, k := range strings.Split(kinds, ",") {
					want = append(want, strings.ToLower(strings.TrimSpace(k)))
				}
			}
			type result struct {
				Kind    string `json:"kind"`
				Rows    int    `json:"rows"`
				TakenAt string `json:"taken_at,omitempty"`
				Error   string `json:"error,omitempty"`
			}
			c := psxClient(flags)
			out := make([]result, 0, len(want))
			failures := 0
			for _, k := range want {
				n, at, err := takeSnapshot(ctx, s, c, k)
				if err != nil {
					failures++
					out = append(out, result{Kind: k, Error: err.Error()})
					continue
				}
				out = append(out, result{Kind: k, Rows: n, TakenAt: at})
			}
			if failures > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d snapshot kinds failed; the rest were stored\n", failures, len(want))
			}
			// Total failure must be non-zero, or a scheduled capture reports
			// success while storing nothing and the longitudinal commands go
			// quietly empty for weeks.
			if failures == len(want) && len(want) > 0 {
				return apiErr(fmt.Errorf("all %d snapshot kind(s) failed; nothing was stored", failures))
			}
			view := struct {
				DB      string   `json:"db"`
				Results []result `json:"results"`
			}{DB: path, Results: out}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			for _, r := range out {
				if r.Error != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "%-9s FAILED  %s\n", r.Kind, r.Error)
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-9s %5d rows at %s\n", r.Kind, r.Rows, r.TakenAt)
			}
			return nil
		},
	}
	take.Flags().StringVar(&dbPath, "db", "", "database path")
	take.Flags().StringVar(&kinds, "kinds", "", "comma-separated subset: market, screener, sector")
	cmd.AddCommand(take)

	var listDB string
	list := &cobra.Command{
		Use:         "list",
		Short:       "List captured snapshot timestamps",
		Example:     "  psx-pp-cli snapshot list --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "snapshot list")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if mirrorMissing(listDB) {
				return writeMirrorHint(cmd, flags, orDefaultDB(listDB), "snapshot")
			}
			s, _, err := openLocalStore(ctx, listDB)
			if err != nil {
				return err
			}
			defer s.Close()
			type entry struct {
				Kind    string `json:"kind"`
				TakenAt string `json:"taken_at"`
			}
			out := make([]entry, 0)
			for _, k := range []string{snapshotKindMarket, snapshotKindScreener, snapshotKindSector} {
				times, err := listSnapshotTimes(ctx, s, k)
				if err != nil {
					return err
				}
				for _, t := range times {
					out = append(out, entry{Kind: k, TakenAt: t})
				}
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No snapshots captured yet. Run: psx-pp-cli snapshot take")
				return nil
			}
			for _, e := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "%-9s %s\n", e.Kind, e.TakenAt)
			}
			return nil
		},
	}
	list.Flags().StringVar(&listDB, "db", "", "database path")
	cmd.AddCommand(list)
	return cmd
}

func orDefaultDB(p string) string {
	if p == "" {
		return defaultDBPath("psx-pp-cli")
	}
	return p
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newSnapshotCmd(flags))
	})
}
