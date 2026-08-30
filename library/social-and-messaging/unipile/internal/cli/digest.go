// Copyright 2026 fuushyn and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type digestResource struct {
	Resource     string `json:"resource"`
	Total        int    `json:"total"`
	New          int    `json:"new_in_window"`
	Dated        bool   `json:"dated"`
	LastSyncedAt string `json:"last_synced_at,omitempty"`
}

// recordTimeExpr maps a resource type to the SQL expression that yields the
// record's own creation time as an RFC3339 string. Resources absent from this
// map carry no usable timestamp in their payload, so "new in window" cannot be
// computed for them and is reported as unknown rather than as the sync count
// (which would report every row as new after any fresh sync).
var recordTimeExpr = map[string]string{
	"chats":                   "json_extract(data,'$.timestamp')",
	"messages":                "json_extract(data,'$.timestamp')",
	"chats_messages":          "json_extract(data,'$.timestamp')",
	"chat_attendees_messages": "json_extract(data,'$.timestamp')",
	"users-invite-sent":       "json_extract(data,'$.parsed_datetime')",
	"users-invite-received":   "json_extract(data,'$.parsed_datetime')",
	"users-relations":         "strftime('%Y-%m-%dT%H:%M:%SZ', json_extract(data,'$.created_at')/1000, 'unixepoch')",
	"accounts":                "json_extract(data,'$.created_at')",
	"emails":                  "json_extract(data,'$.date')",
}

type digestView struct {
	Window     string           `json:"window"`
	Since      string           `json:"since"`
	Resources  []digestResource `json:"resources"`
	NewChats   int              `json:"new_conversations"`
	NewInbound int              `json:"new_inbound_messages"`
	Undated    []string         `json:"undated_resources,omitempty"`
	Note       string           `json:"note,omitempty"`
}

func newNovelDigestCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		since  string
	)
	cmd := &cobra.Command{
		Use:   "digest",
		Short: "What changed across every connected provider since your last sync",
		Long: strings.Trim(`
Summarises the local mirror: rows per resource, how many arrived inside the
--since window, and when each resource was last synced.

The API keeps no per-resource sync bookkeeping for you, so this view only
exists once the mirror is local.

Use this command after 'sync' to catch up. Do NOT treat it as a live feed; it
reports what the mirror holds.`, "\n"),
		Example: strings.Trim(`
  unipile-pp-cli digest --agent
  unipile-pp-cli digest --since 24h
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "--since=7d",
			"pp:typed-exit-codes": "0",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "digest")
			}
			now := time.Now().UTC()
			cutoff, err := parseWindow(now, since)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--since %q is not a valid duration (try 24h, 7d, 2w)", since))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			view := digestView{Window: since, Since: cutoff.Format(time.RFC3339), Resources: make([]digestResource, 0)}
			db, ok, err := novelStore(cmd, flags, dbPath, view)
			if err != nil || !ok {
				return err
			}
			defer db.Close()

			rows, err := db.QueryContext(ctx, `SELECT resource_type, COUNT(*) FROM resources GROUP BY resource_type`)
			if err != nil {
				return fmt.Errorf("reading mirror summary: %w", err)
			}
			counts := make([]digestResource, 0)
			for rows.Next() {
				var rt sql.NullString
				var total sql.NullInt64
				if err := rows.Scan(&rt, &total); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scanning mirror summary: %w", err)
				}
				counts = append(counts, digestResource{Resource: rt.String, Total: int(total.Int64), New: -1})
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterating mirror summary: %w", err)
			}
			if err := rows.Close(); err != nil {
				return fmt.Errorf("closing mirror summary: %w", err)
			}

			// Count new records by the record's own timestamp, never by
			// synced_at: after a fresh sync every row looks new.
			cutoffRFC := cutoff.Format(time.RFC3339)
			undated := make([]string, 0)
			for i := range counts {
				expr, known := recordTimeExpr[counts[i].Resource]
				if !known {
					undated = append(undated, counts[i].Resource)
					continue
				}
				var fresh sql.NullInt64
				q := fmt.Sprintf(`SELECT COUNT(*) FROM resources WHERE resource_type = ? AND %s >= ?`, expr)
				if qerr := db.QueryRowContext(ctx, q, counts[i].Resource, cutoffRFC).Scan(&fresh); qerr != nil {
					undated = append(undated, counts[i].Resource)
					continue
				}
				counts[i].New = int(fresh.Int64)
				counts[i].Dated = true
			}

			syncedAt := make(map[string]string)
			sRows, err := db.QueryContext(ctx, `SELECT resource_type, COALESCE(last_synced_at,'') FROM sync_state`)
			if err == nil {
				for sRows.Next() {
					var rt, at sql.NullString
					if serr := sRows.Scan(&rt, &at); serr == nil {
						syncedAt[rt.String] = at.String
					}
				}
				_ = sRows.Err()
				_ = sRows.Close()
			}
			for i := range counts {
				counts[i].LastSyncedAt = syncedAt[counts[i].Resource]
			}
			sort.Slice(counts, func(i, j int) bool {
				if counts[i].New != counts[j].New {
					return counts[i].New > counts[j].New
				}
				return counts[i].Resource < counts[j].Resource
			})
			if len(undated) > 0 {
				view.Undated = undated
			}
			view.Resources = counts

			var newChats, newInbound sql.NullInt64
			_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM chats WHERE timestamp >= ?`, cutoff.Format(time.RFC3339)).Scan(&newChats)
			_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM messages WHERE timestamp >= ? AND is_sender != '1'`, cutoff.Format(time.RFC3339)).Scan(&newInbound)
			view.NewChats = int(newChats.Int64)
			view.NewInbound = int(newInbound.Int64)
			if len(view.Resources) == 0 {
				view.Note = "the local mirror is empty; run 'unipile-pp-cli sync' first"
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if view.Note != "" {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Since %s: %d conversations touched, %d inbound messages.\n\n", view.Since, view.NewChats, view.NewInbound)
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "RESOURCE\tNEW\tTOTAL\tLAST SYNCED")
			for _, r := range view.Resources {
				newCol := "?"
				if r.Dated {
					newCol = fmt.Sprintf("%d", r.New)
				}
				fmt.Fprintf(tw, "%s\t%s\t%d\t%s\n", r.Resource, newCol, r.Total, r.LastSyncedAt)
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			if len(view.Undated) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "\n? = these resources carry no record timestamp, so new-in-window cannot be computed: %s\n", strings.Join(view.Undated, ", "))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory)")
	cmd.Flags().StringVar(&since, "since", "24h", "window to report new records over (24h, 7d, 2w)")
	return cmd
}
