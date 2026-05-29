// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: unified engagement timeline for a contact/deal/company.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/client"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/store"
	"github.com/spf13/cobra"
)

func newNovelEngagementsOfCmd(flags *rootFlags) *cobra.Command {
	var since string
	var typesCSV string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "of [target]",
		Short:       "Timeline for target like contact:123, deal:456, or company:789",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     `  hubspot-pp-cli engagements of deal:456 --since 30d`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			targetType, targetID, err := parseTargetSpec(args[0])
			if err != nil {
				return err
			}
			cutoff := ""
			if since != "" {
				cutoff, err = parseDurationOrTimestamp(since)
				if err != nil {
					return err
				}
			}
			engTypes := []string{"calls", "emails", "meetings", "notes", "tasks"}
			if typesCSV != "" {
				engTypes = splitCSV(typesCSV)
			}

			if dbPath == "" {
				dbPath = defaultDBPath("hubspot-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			items, source, err := loadEngagementsFor(cmd, db, flags, targetType, targetID, engTypes, cutoff)
			if err != nil {
				return err
			}

			sort.Slice(items, func(i, j int) bool { return items[i].Timestamp > items[j].Timestamp })

			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"target":      args[0],
					"data_source": source,
					"results":     items,
				})
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "data_source: %s\n", source)
			headers := []string{"type", "id", "timestamp", "title_or_snippet", "outcome"}
			rows := make([][]string, 0, len(items))
			for _, it := range items {
				rows = append(rows, []string{it.Type, it.ID, it.Timestamp, snippet(it.Title, 80), it.Outcome})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Only include engagements newer than this (e.g. 30d, 4h, RFC3339)")
	cmd.Flags().StringVar(&typesCSV, "type", "", "Limit engagement types (calls,emails,meetings,notes,tasks)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func parseTargetSpec(arg string) (string, string, error) {
	parts := strings.SplitN(arg, ":", 2)
	if len(parts) != 2 || parts[1] == "" {
		return "", "", fmt.Errorf("target must be type:id (e.g. deal:123); got %q", arg)
	}
	t := strings.TrimSpace(parts[0])
	switch t {
	case "contact":
		return "contacts", parts[1], nil
	case "deal":
		return "deals", parts[1], nil
	case "company":
		return "companies", parts[1], nil
	default:
		return "", "", fmt.Errorf("unknown target type %q (expected contact, deal, company)", t)
	}
}

type engagementItem struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Title     string `json:"title_or_snippet"`
	Outcome   string `json:"outcome,omitempty"`
}

func loadEngagementsFor(cmd *cobra.Command, db *store.Store, flags *rootFlags, targetType, targetID string, engTypes []string, cutoff string) ([]engagementItem, string, error) {
	// First try local hubspot_associations join.
	var assocCount int
	hasAssoc := false
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM hubspot_associations`).Scan(&assocCount); err == nil && assocCount > 0 {
		hasAssoc = true
	}

	if hasAssoc {
		items, err := loadEngagementsLocal(cmd, db, targetType, targetID, engTypes, cutoff)
		if err == nil && len(items) > 0 {
			return items, "local", nil
		}
	}

	c, err := flags.newClient()
	if err != nil {
		return nil, "", err
	}
	items, err := loadEngagementsLive(cmd.Context(), c, targetType, targetID, engTypes, cutoff)
	if err != nil {
		return nil, "", err
	}
	return items, "live", nil
}

func loadEngagementsLocal(cmd *cobra.Command, db *store.Store, targetType, targetID string, engTypes []string, cutoff string) ([]engagementItem, error) {
	items := []engagementItem{}
	for _, et := range engTypes {
		info, ok := engagementTables[et]
		if !ok {
			continue
		}
		q := fmt.Sprintf(`
SELECT e.id,
  COALESCE(e.updated_at, e.created_at, '') AS ts,
  COALESCE(
    json_extract(e.data, '$.properties.hs_engagement_title'),
    json_extract(e.data, '$.properties.hs_task_subject'),
    json_extract(e.data, '$.properties.hs_meeting_title'),
    json_extract(e.data, '$.properties.hs_email_subject'),
    json_extract(e.data, '$.properties.hs_call_title'),
    json_extract(e.data, '$.properties.hs_note_body'),
    ''
  ) AS title,
  COALESCE(
    json_extract(e.data, '$.properties.hs_call_disposition'),
    json_extract(e.data, '$.properties.hs_meeting_outcome'),
    json_extract(e.data, '$.properties.hs_task_status'),
    ''
  ) AS outcome
FROM %s e
JOIN hubspot_associations a
  ON a.from_type = ? AND a.from_id = e.id
 AND a.to_type = ? AND a.to_id = ?
WHERE (? = '' OR ts > ?)
ORDER BY ts DESC`, info.Table)
		rows, err := db.DB().QueryContext(cmd.Context(), q, et, targetType, targetID, cutoff, cutoff)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id, ts, title, outcome sql.NullString
			if err := rows.Scan(&id, &ts, &title, &outcome); err != nil {
				rows.Close()
				return nil, err
			}
			items = append(items, engagementItem{
				Type: et, ID: nullStr(id), Timestamp: nullStr(ts),
				Title: stripHTML(nullStr(title)), Outcome: nullStr(outcome),
			})
		}
		rows.Close()
	}
	return items, nil
}

func loadEngagementsLive(ctx context.Context, c *client.Client, targetType, targetID string, engTypes []string, cutoff string) ([]engagementItem, error) {
	items := []engagementItem{}
	for _, et := range engTypes {
		info, ok := engagementTables[et]
		if !ok {
			continue
		}
		listPath := fmt.Sprintf("/crm/v4/objects/%s/%s/associations/%s", targetType, targetID, info.Path)
		body, err := c.Get(ctx, listPath, nil)
		if err != nil {
			continue
		}
		var resp struct {
			Results []struct {
				ToObjectID any `json:"toObjectId"`
			} `json:"results"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			continue
		}
		for _, r := range resp.Results {
			id := fmt.Sprintf("%v", r.ToObjectID)
			detail, err := c.Get(ctx, fmt.Sprintf("/crm/v3/objects/%s/%s", info.Path, id), nil)
			if err != nil {
				continue
			}
			var obj struct {
				ID         string            `json:"id"`
				UpdatedAt  string            `json:"updatedAt"`
				CreatedAt  string            `json:"createdAt"`
				Properties map[string]string `json:"properties"`
			}
			if err := json.Unmarshal(detail, &obj); err != nil {
				continue
			}
			ts := obj.UpdatedAt
			if ts == "" {
				ts = obj.CreatedAt
			}
			if cutoff != "" && ts != "" && ts < cutoff {
				continue
			}
			title := firstNonEmptyString(
				obj.Properties["hs_engagement_title"],
				obj.Properties["hs_task_subject"],
				obj.Properties["hs_meeting_title"],
				obj.Properties["hs_email_subject"],
				obj.Properties["hs_call_title"],
				obj.Properties["hs_note_body"],
			)
			outcome := firstNonEmptyString(
				obj.Properties["hs_call_disposition"],
				obj.Properties["hs_meeting_outcome"],
				obj.Properties["hs_task_status"],
			)
			items = append(items, engagementItem{
				Type:      et,
				ID:        obj.ID,
				Timestamp: ts,
				Title:     stripHTML(title),
				Outcome:   outcome,
			})
		}
	}
	return items, nil
}
