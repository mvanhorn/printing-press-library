// Copyright 2026 joltsconsulting and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/adminbyrequest/internal/store"
	"github.com/spf13/cobra"
)

// correlateRow is the joined audit-event view emitted by the correlate command.
type correlateRow struct {
	Audit  json.RawMessage   `json:"audit"`
	Events []json.RawMessage `json:"events"`
	Window string            `json:"window"`
}

func newCorrelateCmd(flags *rootFlags) *cobra.Command {
	var windowStr string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "correlate [auditlog-id]",
		Short: "Join an audit log entry to nearby events on the same computer (offline)",
		Long: `Given an audit log id from the local store, find events that happened on the same
computer within plus/minus <window> of the audit entry's start time. Useful for
reconstructing an elevation timeline from already-synced data.`,
		Example: "  adminbyrequest-pp-cli correlate 50461167 --window 5m --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id := args[0]
			window, err := time.ParseDuration(windowStr)
			if err != nil {
				return fmt.Errorf("invalid --window %q: %w", windowStr, err)
			}
			if dbPath == "" {
				dbPath = defaultDBPath("adminbyrequest-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store at %s: %w (run sync first)", dbPath, err)
			}
			defer db.Close()

			// Look up via the JSON-embedded id (stable across upstream/store id-typing
			// differences) and fall back to the store's own id column.
			row := db.DB().QueryRowContext(cmd.Context(),
				`SELECT data, json_extract(data, '$.computer.name'), json_extract(data, '$.startTime'), json_extract(data, '$.requestTime')
				 FROM auditlog
				 WHERE CAST(json_extract(data, '$.id') AS TEXT) = ?
				    OR id = ?
				 LIMIT 1`, id, id)
			var auditData []byte
			var computer, startTime, requestTime sql.NullString
			if err := row.Scan(&auditData, &computer, &startTime, &requestTime); err != nil {
				if err == sql.ErrNoRows {
					return fmt.Errorf("audit log id %s not found in local store (run sync first?)", id)
				}
				return fmt.Errorf("loading audit entry: %w", err)
			}
			if !computer.Valid || computer.String == "" {
				return fmt.Errorf("audit entry %s has no computer.name; cannot correlate", id)
			}
			anchor := pickAnchorTime(startTime, requestTime)
			if anchor.IsZero() {
				return fmt.Errorf("audit entry %s has no usable timestamp", id)
			}
			low := anchor.Add(-window).Format("2006-01-02T15:04:05")
			high := anchor.Add(window).Format("2006-01-02T15:04:05")

			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT data FROM events
				 WHERE computer_name = ? AND event_time >= ? AND event_time <= ?
				 ORDER BY event_time ASC`, computer.String, low, high)
			if err != nil {
				return fmt.Errorf("querying events: %w", err)
			}
			defer rows.Close()
			var evts []json.RawMessage
			for rows.Next() {
				var d []byte
				if err := rows.Scan(&d); err != nil {
					return err
				}
				evts = append(evts, json.RawMessage(d))
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating events: %w", err)
			}
			if evts == nil {
				evts = []json.RawMessage{}
			}

			view := correlateRow{
				Audit:  json.RawMessage(auditData),
				Events: evts,
				Window: window.String(),
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "audit id=%s computer=%s anchor=%s window=%s nearby_events=%d\n",
				id, computer.String, anchor.Format(time.RFC3339), window, len(evts))
			return nil
		},
	}
	cmd.Flags().StringVar(&windowStr, "window", "5m", "Time window around audit entry (e.g. 30s, 5m, 1h)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard CLI location)")
	return cmd
}

func pickAnchorTime(start, req sql.NullString) time.Time {
	for _, s := range []sql.NullString{start, req} {
		if !s.Valid || s.String == "" {
			continue
		}
		for _, layout := range []string{
			"2006-01-02T15:04:05.999999999",
			"2006-01-02T15:04:05.999",
			"2006-01-02T15:04:05",
			time.RFC3339Nano,
			time.RFC3339,
		} {
			if t, err := time.Parse(layout, strings.TrimSuffix(s.String, "Z")); err == nil {
				return t
			}
		}
	}
	return time.Time{}
}
