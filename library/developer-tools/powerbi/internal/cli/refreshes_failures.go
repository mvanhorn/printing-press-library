// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type refreshRecord struct {
	ID                   string `json:"id"`
	RefreshType          string `json:"refreshType"`
	Status               string `json:"status"`
	StartTime            string `json:"startTime"`
	EndTime              string `json:"endTime"`
	ServiceExceptionJSON string `json:"serviceExceptionJson"`
}

type refreshListEnvelope struct {
	OData string          `json:"@odata.context"`
	Value []refreshRecord `json:"value"`
}

type groupSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type datasetSummary struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	ConfiguredBy  string `json:"configuredBy"`
	IsRefreshable bool   `json:"isRefreshable"`
}

type failureEntry struct {
	GroupID      string `json:"group_id"`
	GroupName    string `json:"group_name"`
	DatasetID    string `json:"dataset_id"`
	DatasetName  string `json:"dataset_name"`
	RefreshType  string `json:"refresh_type"`
	Status       string `json:"status"`
	StartTime    string `json:"start_time"`
	EndTime      string `json:"end_time"`
	Age          string `json:"age"`
	ErrorCode    string `json:"error_code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

func newRefreshesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "refreshes",
		Short: "Surface dataset refresh health across all accessible workspaces",
	}
	cmd.AddCommand(newRefreshesFailuresCmd(flags))
	return cmd
}

func newRefreshesFailuresCmd(flags *rootFlags) *cobra.Command {
	var days int
	var topN int
	var workspaceFilter string
	cmd := &cobra.Command{
		Use:   "failures",
		Short: "List datasets whose most recent refresh failed in the last N days",
		Long: `Iterates every workspace the identity can see, every refreshable dataset
in those workspaces, and pulls the most recent refresh history record. Emits
only datasets whose latest run is in the Failed state.

The error message (when present) comes from Power BI's serviceExceptionJson
field on the refresh record. Use --json to get structured output an agent can
re-pipe into other tools.`,
		Example: `  powerbi-pp-cli refreshes failures --days 7
  powerbi-pp-cli refreshes failures --json --select group_name,dataset_name,error_message`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// Step 1: list workspaces.
			grpRaw, err := c.Get("/groups", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var grpEnv struct {
				Value []groupSummary `json:"value"`
			}
			if err := json.Unmarshal(grpRaw, &grpEnv); err != nil {
				return apiErr(fmt.Errorf("decoding /groups: %w", err))
			}
			cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)

			var failures []failureEntry
			for _, g := range grpEnv.Value {
				if workspaceFilter != "" && !strings.EqualFold(g.Name, workspaceFilter) && g.ID != workspaceFilter {
					continue
				}
				// Step 2: list datasets in this workspace.
				dsRaw, err := c.Get(fmt.Sprintf("/groups/%s/datasets", g.ID), nil)
				if err != nil {
					// Skip workspaces we can't read instead of failing the whole command.
					continue
				}
				var dsEnv struct {
					Value []datasetSummary `json:"value"`
				}
				if err := json.Unmarshal(dsRaw, &dsEnv); err != nil {
					continue
				}
				for _, d := range dsEnv.Value {
					if !d.IsRefreshable {
						continue
					}
					// Step 3: fetch the most recent refresh.
					params := map[string]string{"$top": "1"}
					rRaw, err := c.Get(fmt.Sprintf("/groups/%s/datasets/%s/refreshes", g.ID, d.ID), params)
					if err != nil {
						continue
					}
					var rEnv refreshListEnvelope
					if err := json.Unmarshal(rRaw, &rEnv); err != nil {
						continue
					}
					if len(rEnv.Value) == 0 {
						continue
					}
					r := rEnv.Value[0]
					// Parse start time. Refresh history times are ISO8601 in UTC.
					started, _ := time.Parse(time.RFC3339, r.StartTime)
					if !started.IsZero() && started.Before(cutoff) {
						continue
					}
					status := strings.ToLower(r.Status)
					if status != "failed" {
						continue
					}
					fe := failureEntry{
						GroupID:     g.ID,
						GroupName:   g.Name,
						DatasetID:   d.ID,
						DatasetName: d.Name,
						RefreshType: r.RefreshType,
						Status:      r.Status,
						StartTime:   r.StartTime,
						EndTime:     r.EndTime,
					}
					if !started.IsZero() {
						fe.Age = time.Since(started).Round(time.Minute).String()
					}
					// serviceExceptionJson is itself JSON wrapped in a string; unwrap it.
					if r.ServiceExceptionJSON != "" {
						var ex struct {
							ErrorCode string `json:"errorCode"`
							ErrorDesc string `json:"errorDescription"`
						}
						if err := json.Unmarshal([]byte(r.ServiceExceptionJSON), &ex); err == nil {
							fe.ErrorCode = ex.ErrorCode
							fe.ErrorMessage = ex.ErrorDesc
						} else {
							fe.ErrorMessage = r.ServiceExceptionJSON
						}
					}
					failures = append(failures, fe)
				}
			}

			// Order most-recently-failed first.
			sort.Slice(failures, func(i, j int) bool {
				return failures[i].StartTime > failures[j].StartTime
			})
			if topN > 0 && len(failures) > topN {
				failures = failures[:topN]
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), failures, flags)
			}
			if len(failures) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "%s No failed refreshes in the last %d day(s).\n", green("[ok]"), days)
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "WORKSPACE\tDATASET\tWHEN\tERROR")
			for _, f := range failures {
				err := f.ErrorMessage
				if f.ErrorCode != "" {
					err = f.ErrorCode + ": " + err
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", truncate(f.GroupName, 30), truncate(f.DatasetName, 40), f.StartTime, truncate(err, 60))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().IntVar(&days, "days", 7, "Look back this many days")
	cmd.Flags().IntVar(&topN, "top", 0, "Limit to N most recent failures (0 = no limit)")
	cmd.Flags().StringVar(&workspaceFilter, "workspace", "", "Only look at this workspace (ID or name)")
	return cmd
}
