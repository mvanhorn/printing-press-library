// Copyright 2026 nikica-jokic. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

func newWorklogSummaryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "summary <issueKey>",
		Short: "Show total time logged on an issue, grouped by author",
		Example: `  jira-cloud-platform-pp-cli issue worklog summary CC-132
  jira-cloud-platform-pp-cli issue worklog summary CC-132 --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			issueKey := args[0]
			data, err := c.Get("/rest/api/3/issue/"+issueKey+"/worklog", map[string]string{
				"maxResults": "1000",
			})
			if err != nil {
				return fmt.Errorf("fetching worklogs for %s: %w", issueKey, err)
			}

			var envelope struct {
				Worklogs []struct {
					Author struct {
						DisplayName string `json:"displayName"`
					} `json:"author"`
					TimeSpentSeconds int    `json:"timeSpentSeconds"`
					TimeSpent        string `json:"timeSpent"`
					Started          string `json:"started"`
				} `json:"worklogs"`
				Total int `json:"total"`
			}
			if err := json.Unmarshal(data, &envelope); err != nil {
				return fmt.Errorf("parsing worklogs: %w", err)
			}

			type authorSummary struct {
				Name    string `json:"author"`
				Seconds int    `json:"total_seconds"`
				Entries int    `json:"entries"`
			}

			byAuthor := map[string]*authorSummary{}
			totalSeconds := 0
			for _, w := range envelope.Worklogs {
				name := w.Author.DisplayName
				if name == "" {
					name = "(unknown)"
				}
				if byAuthor[name] == nil {
					byAuthor[name] = &authorSummary{Name: name}
				}
				byAuthor[name].Seconds += w.TimeSpentSeconds
				byAuthor[name].Entries++
				totalSeconds += w.TimeSpentSeconds
			}

			rows := make([]*authorSummary, 0, len(byAuthor))
			for _, v := range byAuthor {
				rows = append(rows, v)
			}
			sort.Slice(rows, func(i, j int) bool {
				return rows[i].Seconds > rows[j].Seconds
			})

			if flags.asJSON {
				result := map[string]any{
					"issue":         issueKey,
					"total_seconds": totalSeconds,
					"total_human":   formatSeconds(totalSeconds),
					"entries":       envelope.Total,
					"by_author":     rows,
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%s  Total logged: %s (%d entries)\n\n", issueKey, formatSeconds(totalSeconds), envelope.Total)
			for _, r := range rows {
				fmt.Fprintf(out, "  %-30s  %s  (%d entries)\n", r.Name, formatSeconds(r.Seconds), r.Entries)
			}
			return nil
		},
	}
	return cmd
}

func formatSeconds(s int) string {
	h := s / 3600
	m := (s % 3600) / 60
	if h == 0 {
		return fmt.Sprintf("%dm", m)
	}
	if m == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh %dm", h, m)
}
