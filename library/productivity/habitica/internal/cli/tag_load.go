// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newNovelTagLoadCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "load",
		Short:       "Compare active, due, overdue, and checklist-blocked work across Habitica tags.",
		Example:     "  habitica-pp-cli tag load --agent --select tags.name,tags.overdue",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return flags.printJSON(cmd, map[string]any{"tags": []any{}, "action": "would group live tasks by Habitica tag"})
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			headers, err := habiticaHeaders()
			if err != nil {
				return err
			}
			tagsRaw, err := c.GetWithHeaders(ctx, "/tags", nil, headers)
			if err != nil {
				return fmt.Errorf("fetching tags: %w", err)
			}
			tasksRaw, err := c.GetWithHeaders(ctx, "/tasks/user", nil, headers)
			if err != nil {
				return fmt.Errorf("fetching tasks: %w", err)
			}
			tagsData, err := habiticaData(tagsRaw)
			if err != nil {
				return err
			}
			tasksData, err := habiticaData(tasksRaw)
			if err != nil {
				return err
			}
			var tags []map[string]any
			var tasks []map[string]any
			if err := json.Unmarshal(tagsData, &tags); err != nil {
				return fmt.Errorf("decoding tags: %w", err)
			}
			if err := json.Unmarshal(tasksData, &tasks); err != nil {
				return fmt.Errorf("decoding tasks: %w", err)
			}
			loads := make(map[string]map[string]any, len(tags))
			for _, tag := range tags {
				id, _ := tag["_id"].(string)
				name, _ := tag["name"].(string)
				loads[id] = map[string]any{"id": id, "name": name, "active": 0, "due": 0, "overdue": 0, "checklist_blocked": 0}
			}
			now := time.Now()
			for _, task := range tasks {
				if completed, _ := task["completed"].(bool); completed {
					continue
				}
				tagIDs, _ := task["tags"].([]any)
				for _, rawID := range tagIDs {
					id, _ := rawID.(string)
					load, exists := loads[id]
					if !exists {
						load = map[string]any{"id": id, "name": "(unknown tag)", "active": 0, "due": 0, "overdue": 0, "checklist_blocked": 0}
						loads[id] = load
					}
					load["active"] = load["active"].(int) + 1
					if date, ok := task["date"].(string); ok && date != "" {
						if due, parseErr := time.Parse(time.RFC3339, date); parseErr == nil {
							if due.Before(now) {
								load["overdue"] = load["overdue"].(int) + 1
							} else {
								load["due"] = load["due"].(int) + 1
							}
						}
					}
					if checklist, ok := task["checklist"].([]any); ok && len(checklist) > 0 {
						open := false
						for _, item := range checklist {
							if m, ok := item.(map[string]any); ok {
								if done, _ := m["completed"].(bool); !done {
									open = true
								}
							}
						}
						if open {
							load["checklist_blocked"] = load["checklist_blocked"].(int) + 1
						}
					}
				}
			}
			result := make([]map[string]any, 0, len(loads))
			for _, load := range loads {
				result = append(result, load)
			}
			return flags.printJSON(cmd, map[string]any{"tags": result})
		},
	}
	return cmd
}
