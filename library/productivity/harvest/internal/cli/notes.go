// Copyright 2026 dan-bronson. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/harvest/internal/store"
)

func newNotesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "notes",
		Short: "Search and inspect time-entry notes",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
	}
	cmd.AddCommand(newNotesSearchCmd(flags))
	return cmd
}

type notesHit struct {
	ID         string  `json:"id"`
	SpentDate  string  `json:"spent_date"`
	Hours      float64 `json:"hours"`
	UserID     int64   `json:"user_id"`
	UserName   string  `json:"user_name,omitempty"`
	ProjectID  int64   `json:"project_id,omitempty"`
	ProjectKey string  `json:"project,omitempty"`
	TaskKey    string  `json:"task,omitempty"`
	Notes      string  `json:"notes"`
}

func newNotesSearchCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath    string
		mine      bool
		projectID string
		from      string
		to        string
		limit     int
	)

	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Full-text search across your time-entry notes",
		Long: `Searches your locally synced time-entry notes using SQL LIKE matching.
The query is case-insensitive and matches any substring. Use --mine to limit
results to your own entries, --project to filter by project ID, and --from/--to
to bound the date range.

This is a LOCAL command — it queries the SQLite store populated by 'sync'. Run
'harvest-pp-cli sync --resource time-entries' first to refresh.`,
		Example: `  # Find every entry mentioning "auth refactor"
  harvest-pp-cli notes search "auth refactor" --mine --json --select id,spent_date,hours,notes

  # Recent client work
  harvest-pp-cli notes search "client demo" --from 2026-04-01 --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return flags.printJSON(cmd, []notesHit{})
			}
			query := strings.Join(args, " ")
			if dbPath == "" {
				dbPath = defaultDBPath("harvest-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'harvest-pp-cli sync --resources time-entries' first.", err)
			}
			defer db.Close()

			meID := ""
			if mine {
				meID, err = resolveCurrentUserID(cmd, flags, db)
				if err != nil {
					return err
				}
			}

			rows, err := searchTimeEntryNotes(cmd, db, query, meID, projectID, from, to, limit)
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, rows)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().BoolVar(&mine, "mine", false, "Limit to the current user's entries")
	cmd.Flags().StringVar(&projectID, "project", "", "Filter by project ID")
	cmd.Flags().StringVar(&from, "from", "", "Earliest spent_date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&to, "to", "", "Latest spent_date (YYYY-MM-DD)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum results")
	return cmd
}

func searchTimeEntryNotes(cmd *cobra.Command, db *store.Store, query, userID, projectID, from, to string, limit int) ([]notesHit, error) {
	sql := `SELECT data FROM time_entries WHERE LOWER(IFNULL(json_extract(data, '$.notes'), '')) LIKE LOWER(?)`
	q := "%" + query + "%"
	params := []any{q}

	if userID != "" {
		sql += ` AND IFNULL(CAST(json_extract(data, '$.user.id') AS TEXT), '') = ?`
		params = append(params, userID)
	}
	if projectID != "" {
		sql += ` AND IFNULL(CAST(json_extract(data, '$.project.id') AS TEXT), '') = ?`
		params = append(params, projectID)
	}
	if from != "" {
		sql += ` AND IFNULL(json_extract(data, '$.spent_date'), '') >= ?`
		params = append(params, from)
	}
	if to != "" {
		sql += ` AND IFNULL(json_extract(data, '$.spent_date'), '') <= ?`
		params = append(params, to)
	}
	sql += ` ORDER BY json_extract(data, '$.spent_date') DESC LIMIT ?`
	params = append(params, limit)

	rows, err := db.DB().QueryContext(cmd.Context(), sql, params...)
	if err != nil {
		return nil, fmt.Errorf("query notes: %w", err)
	}
	defer rows.Close()

	var hits []notesHit
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err != nil {
			continue
		}
		hits = append(hits, hitFromTimeEntry(obj))
	}
	return hits, rows.Err()
}

func hitFromTimeEntry(obj map[string]any) notesHit {
	h := notesHit{}
	if v, ok := obj["id"].(float64); ok {
		h.ID = fmt.Sprintf("%d", int64(v))
	}
	if v, ok := obj["spent_date"].(string); ok {
		h.SpentDate = v
	}
	if v, ok := obj["hours"].(float64); ok {
		h.Hours = v
	}
	if v, ok := obj["notes"].(string); ok {
		h.Notes = v
	}
	if user, ok := obj["user"].(map[string]any); ok {
		if id, ok := user["id"].(float64); ok {
			h.UserID = int64(id)
		}
		if name, ok := user["name"].(string); ok {
			h.UserName = name
		}
	}
	if proj, ok := obj["project"].(map[string]any); ok {
		if id, ok := proj["id"].(float64); ok {
			h.ProjectID = int64(id)
		}
		if name, ok := proj["name"].(string); ok {
			h.ProjectKey = name
		}
	}
	if task, ok := obj["task"].(map[string]any); ok {
		if name, ok := task["name"].(string); ok {
			h.TaskKey = name
		}
	}
	return h
}

// resolveCurrentUserID returns the authenticated user's ID, preferring a local
// users-me cache, falling back to a live API call.
func resolveCurrentUserID(cmd *cobra.Command, flags *rootFlags, db *store.Store) (string, error) {
	rows, err := db.DB().QueryContext(cmd.Context(), `SELECT data FROM users WHERE json_extract(data, '$.is_active') IN (1, 'true') LIMIT 1000`)
	if err == nil {
		defer rows.Close()
	}
	c, cerr := flags.newClient()
	if cerr == nil {
		raw, gerr := c.Get("/users/me", nil)
		if gerr == nil {
			var obj map[string]any
			if uerr := json.Unmarshal(raw, &obj); uerr == nil {
				if id, ok := obj["id"].(float64); ok {
					return fmt.Sprintf("%d", int64(id)), nil
				}
			}
		}
	}
	return "", fmt.Errorf("could not resolve current user — provide HARVEST_ACCESS_TOKEN or sync users first")
}
