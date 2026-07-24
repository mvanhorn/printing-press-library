// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newNovelTodayCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "today",
		Short:       "Turn today’s dailies, due to-dos, and character state into one actionable quest queue.",
		Example:     "  habitica-pp-cli today --agent --select tasks.text,tasks.type,stats.gp",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return flags.printJSON(cmd, map[string]any{"action": "would fetch user state and actionable tasks", "tasks": []any{}})
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
			userRaw, err := c.GetWithHeaders(ctx, "/user", nil, headers)
			if err != nil {
				return fmt.Errorf("fetching user state: %w", err)
			}
			tasksRaw, err := c.GetWithHeaders(ctx, "/tasks/user", map[string]string{"type": "todos,dailys"}, headers)
			if err != nil {
				return fmt.Errorf("fetching today’s tasks: %w", err)
			}
			user, err := habiticaData(userRaw)
			if err != nil {
				return err
			}
			tasks, err := habiticaData(tasksRaw)
			if err != nil {
				return err
			}
			var userValue map[string]any
			if err := json.Unmarshal(user, &userValue); err != nil {
				return fmt.Errorf("decoding user state: %w", err)
			}
			var taskValues []map[string]any
			if err := json.Unmarshal(tasks, &taskValues); err != nil {
				return fmt.Errorf("decoding tasks: %w", err)
			}
			queue := make([]map[string]any, 0, len(taskValues))
			for _, task := range taskValues {
				if completed, _ := task["completed"].(bool); completed {
					continue
				}
				queue = append(queue, task)
			}
			return flags.printJSON(cmd, map[string]any{"tasks": queue, "stats": userValue["stats"], "preferences": userValue["preferences"]})
		},
	}
	return cmd
}
