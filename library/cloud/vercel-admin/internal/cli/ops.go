// Copyright 2026 hiten-shah. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

type opsDeploymentSummary struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	State     string `json:"state,omitempty"`
	Target    string `json:"target,omitempty"`
	URL       string `json:"url,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

type opsFailureBrief struct {
	Deployment opsDeploymentSummary `json:"deployment"`
	Events     []map[string]any     `json:"events,omitempty"`
	Checks     []map[string]any     `json:"checks,omitempty"`
	Logs       []map[string]any     `json:"logs,omitempty"`
}

func newOpsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "ops",
		Short:       "Operator briefs that join Vercel admin signals",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newOpsRecentDeploymentsCmd(flags))
	cmd.AddCommand(newOpsFailureBriefCmd(flags))
	return cmd
}

func newOpsRecentDeploymentsCmd(flags *rootFlags) *cobra.Command {
	var projectID, target, state string
	var limit int
	cmd := &cobra.Command{
		Use:   "recent-deployments",
		Short: "Summarize recent deployments by project, target, or state",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if limit <= 0 || limit > 100 {
				limit = 20
			}
			params := map[string]string{"limit": strconv.Itoa(limit)}
			if projectID != "" {
				params["projectId"] = projectID
			}
			if target != "" {
				params["target"] = target
			}
			if state != "" {
				params["state"] = state
			}
			raw, err := c.Get(context.Background(), "/v7/deployments", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			items := objectListFromEnvelope(raw, "deployments")
			out := make([]opsDeploymentSummary, 0, len(items))
			for _, item := range items {
				out = append(out, deploymentSummaryFromMap(item))
			}
			return printOpsJSON(cmd, out, flags)
		},
	}
	cmd.Flags().StringVar(&projectID, "project-id", "", "Filter by Vercel project ID")
	cmd.Flags().StringVar(&target, "target", "", "Filter by deployment target, e.g. production or preview")
	cmd.Flags().StringVar(&state, "state", "", "Filter by deployment state")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum deployments to fetch")
	return cmd
}

func newOpsFailureBriefCmd(flags *rootFlags) *cobra.Command {
	var projectID string
	cmd := &cobra.Command{
		Use:   "failure-brief <deployment-id-or-url>",
		Short: "Join deployment metadata, events, checks, and runtime logs for incident triage",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if projectID == "" {
				return usageErr(fmt.Errorf("--project-id is required for runtime logs"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			deploymentID := args[0]
			deploymentRaw, err := c.Get(context.Background(), "/v13/deployments/"+deploymentID, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			eventsRaw, err := c.Get(context.Background(), "/v3/deployments/"+deploymentID+"/events", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			checksRaw, err := c.Get(context.Background(), "/v2/deployments/"+deploymentID+"/check-runs", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			logsRaw, err := c.Get(context.Background(), "/v1/projects/"+projectID+"/deployments/"+deploymentID+"/runtime-logs", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var deployment map[string]any
			if err := json.Unmarshal(deploymentRaw, &deployment); err != nil {
				return fmt.Errorf("decode deployment: %w", err)
			}
			out := opsFailureBrief{
				Deployment: deploymentSummaryFromMap(deployment),
				Events:     objectListFromEnvelope(eventsRaw, "events"),
				Checks:     objectListFromEnvelope(checksRaw, "checks"),
				Logs:       objectListFromEnvelope(logsRaw, "logs"),
			}
			return printOpsJSON(cmd, out, flags)
		},
	}
	cmd.Flags().StringVar(&projectID, "project-id", "", "Project ID used for runtime logs")
	return cmd
}

func deploymentSummaryFromMap(item map[string]any) opsDeploymentSummary {
	return opsDeploymentSummary{
		ID:        firstString(item, "uid", "id"),
		Name:      firstString(item, "name", "projectName"),
		State:     firstString(item, "state", "readyState"),
		Target:    firstString(item, "target", "environment"),
		URL:       firstString(item, "url", "alias"),
		CreatedAt: unixMillisString(item, "createdAt", "created"),
	}
}

func objectListFromEnvelope(raw json.RawMessage, keys ...string) []map[string]any {
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil
	}
	for _, key := range keys {
		if v, ok := envelope[key]; ok {
			if err := json.Unmarshal(v, &arr); err == nil {
				return arr
			}
		}
	}
	for _, v := range envelope {
		if err := json.Unmarshal(v, &arr); err == nil {
			return arr
		}
	}
	return nil
}

func firstString(item map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := item[key]; ok {
			switch typed := v.(type) {
			case string:
				return typed
			case fmt.Stringer:
				return typed.String()
			}
		}
	}
	return ""
}

func unixMillisString(item map[string]any, keys ...string) string {
	for _, key := range keys {
		switch v := item[key].(type) {
		case float64:
			if v > 0 {
				return time.UnixMilli(int64(v)).UTC().Format(time.RFC3339)
			}
		case string:
			return v
		}
	}
	return ""
}

func printOpsJSON(cmd *cobra.Command, v any, flags *rootFlags) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if flags.selectFields != "" {
		data = filterFields(data, flags.selectFields)
	} else if flags.compact {
		data = compactFields(data)
	}
	return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
}
