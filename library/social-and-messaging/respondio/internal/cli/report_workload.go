// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command report workload: per-agent handling volume.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/respondio/internal/store"
	"github.com/spf13/cobra"
)

type workloadRow struct {
	Agent string `json:"agent"`
	Count int    `json:"count"`
	Open  int    `json:"open"`
}

type workloadView struct {
	Agents        []workloadRow `json:"agents"`
	TotalContacts int           `json:"total_contacts"`
	AssignedInbox int           `json:"assigned_inbox"`
}

// pp:data-source local

func newNovelReportWorkloadCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:         "workload",
		Short:       "Per-agent message and conversation handling volume from the synced workspace.",
		Long:        "Counts assigned contacts per agent from the local contact mirror, with open-conversation breakdown.",
		Example:     "  respondio-pp-cli report workload --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "report workload")
			}
			ctx := cmd.Context()
			if dbPath == "" {
				dbPath = defaultDBPath("respondio-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: respondio-pp-cli sync --resources contact --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), emptyWorkload(), flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No synced contacts yet.")
				return nil
			}
			db, err := store.OpenReadOnlyContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = 'contact'`)
			if err != nil {
				return fmt.Errorf("querying contacts: %w", err)
			}
			var datas [][]byte
			for rows.Next() {
				var data []byte
				if err := rows.Scan(&data); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scan contact: %w", err)
				}
				datas = append(datas, data)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterate contacts: %w", err)
			}
			_ = rows.Close()

			view := emptyWorkload()
			idx := map[string]int{}
			for _, raw := range datas {
				var c map[string]any
				if err := json.Unmarshal(raw, &c); err != nil {
					continue
				}
				view.TotalContacts++
				var assignee map[string]any
				if a, ok := c["assignee"].(map[string]any); ok {
					assignee = a
				}
				if assignee == nil {
					continue
				}
				agent := agentName(assignee)
				if i, ok := idx[agent]; ok {
					view.Agents[i].Count++
					if str(c["status"]) == "open" {
						view.Agents[i].Open++
					}
				} else {
					idx[agent] = len(view.Agents)
					row := workloadRow{Agent: agent, Count: 1}
					if str(c["status"]) == "open" {
						row.Open = 1
					}
					view.Agents = append(view.Agents, row)
				}
			}
			sort.Slice(view.Agents, func(i, j int) bool { return view.Agents[i].Count > view.Agents[j].Count })
			view.AssignedInbox = 0
			for _, a := range view.Agents {
				view.AssignedInbox += a.Count
			}
			if limit > 0 && len(view.Agents) > limit {
				view.Agents = view.Agents[:limit]
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			for _, a := range view.Agents {
				fmt.Fprintf(cmd.OutOrStdout(), "%-24s count=%d open=%d\n", a.Agent, a.Count, a.Open)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "maximum agents to list")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func emptyWorkload() workloadView {
	return workloadView{Agents: make([]workloadRow, 0)}
}
