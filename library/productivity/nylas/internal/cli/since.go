// Copyright 2026 Nathan Kettles and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/nylas/internal/store"
	"github.com/spf13/cobra"
)

// sinceTables maps the --resource flag value to the typed table in the
// local mirror. The local mirror always uses the typed tables; the
// generic resources table is a fallback only for resource types
// without a dedicated schema.
var sinceTables = map[string]string{
	"messages":   "grants_messages",
	"threads":    "threads",
	"events":     "events",
	"contacts":   "contacts",
	"drafts":     "drafts",
	"folders":    "folders",
	"notetakers": "grants_notetakers",
	"webhooks":   "webhooks",
}

func newSinceCmd(flags *rootFlags) *cobra.Command {
	var resourceList string
	var grantID string
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "since <duration>",
		Short: "Show resources synced within the last duration (e.g. 2h, 24h, 7d)",
		Long: `Query the local mirror for resources whose synced_at falls within the
given duration window. Spans every connected grant by default; pass
--grant <id> to scope to one.

Resources supported: messages, threads, events, contacts, drafts,
folders, notetakers, webhooks. Pass a comma-separated list with
--resource.`,
		Example: strings.Trim(`
  # Everything synced in the last two hours, across all grants
  nylas-pp-cli since 2h --agent

  # New messages and events only, last 24 hours
  nylas-pp-cli since 24h --resource messages,events --agent --select id,subject,grants_id

  # One specific grant only
  nylas-pp-cli since 7d --grant grant_abc --resource messages
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			autoRefreshIfStale(cmd.Context(), dbPath, cmd.ErrOrStderr())
			ts, err := parseSinceDuration(args[0])
			if err != nil {
				return fmt.Errorf("invalid duration %q: %w (try 30m, 2h, 24h, 7d)", args[0], err)
			}
			cutoff := ts.UTC().Format("2006-01-02 15:04:05")

			if dbPath == "" {
				dbPath = defaultDBPath("nylas-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'nylas-pp-cli sync' first.", err)
			}
			defer db.Close()

			selected := []string{"messages", "threads", "events", "contacts"}
			if resourceList != "" {
				selected = nil
				for _, r := range strings.Split(resourceList, ",") {
					r = strings.TrimSpace(r)
					if r != "" {
						selected = append(selected, r)
					}
				}
			}

			type item struct {
				Resource string          `json:"resource"`
				ID       string          `json:"id"`
				GrantsID string          `json:"grants_id,omitempty"`
				SyncedAt string          `json:"synced_at"`
				Data     json.RawMessage `json:"data,omitempty"`
			}
			out := make([]item, 0, 64)
			for _, r := range selected {
				table, ok := sinceTables[r]
				if !ok {
					return fmt.Errorf("unknown resource %q (try: messages, threads, events, contacts, drafts, folders, notetakers, webhooks)", r)
				}
				q := fmt.Sprintf(`SELECT id, COALESCE(grants_id, '') AS grants_id, synced_at, data FROM %q WHERE synced_at >= ?`, table)
				params := []any{cutoff}
				if grantID != "" {
					q += " AND grants_id = ?"
					params = append(params, grantID)
				}
				q += " ORDER BY synced_at DESC"
				if limit > 0 {
					q += fmt.Sprintf(" LIMIT %d", limit)
				}
				rows, err := db.DB().QueryContext(cmd.Context(), q, params...)
				if err != nil {
					// Webhooks has no grants_id column; gracefully skip.
					if strings.Contains(err.Error(), "no such column: grants_id") {
						continue
					}
					return fmt.Errorf("querying %s: %w", table, err)
				}
				for rows.Next() {
					var it item
					it.Resource = r
					if err := rows.Scan(&it.ID, &it.GrantsID, &it.SyncedAt, &it.Data); err != nil {
						rows.Close()
						return err
					}
					out = append(out, it)
				}
				if err := rows.Err(); err != nil {
					rows.Close()
					return fmt.Errorf("iterating %s: %w", table, err)
				}
				rows.Close()
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&resourceList, "resource", "", "Comma-separated resource types (default: messages,threads,events,contacts)")
	cmd.Flags().StringVar(&grantID, "grant", "", "Scope to one grant ID (default: all grants)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local SQLite database")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows per resource (0 = no cap)")
	return cmd
}
