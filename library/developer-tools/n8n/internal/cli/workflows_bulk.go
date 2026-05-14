// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type bulkResult struct {
	WorkflowID string `json:"workflow_id"`
	Name       string `json:"name"`
	Action     string `json:"action"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
}

func newWorkflowsBulkCmd(flags *rootFlags) *cobra.Command {
	var action string
	var tagFilter string
	var nameFilter string
	var activeOnly bool
	var inactiveOnly bool

	cmd := &cobra.Command{
		Use:   "bulk",
		Short: "Bulk activate, deactivate, or archive workflows by tag or name filter",
		Long: `Apply an action to multiple workflows at once, filtered by tag or name.
Supports activate, deactivate, and archive actions. Use --dry-run to preview
which workflows would be affected before committing.`,
		Example: strings.Trim(`
  # Deactivate all workflows tagged "staging" (preview first)
  n8n-pp-cli workflows bulk --action deactivate --tag staging --dry-run

  # Archive all inactive workflows matching "test-"
  n8n-pp-cli workflows bulk --action archive --name test- --inactive

  # Activate all workflows tagged "production"
  n8n-pp-cli workflows bulk --action activate --tag production --json`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if action == "" {
				return usageErr(fmt.Errorf("--action is required (activate, deactivate, or archive)"))
			}
			switch action {
			case "activate", "deactivate", "archive":
			default:
				return usageErr(fmt.Errorf("--action must be one of: activate, deactivate, archive"))
			}
			if tagFilter == "" && nameFilter == "" {
				return usageErr(fmt.Errorf("at least one of --tag or --name is required to scope the operation"))
			}

			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), `{"dry_run":true,"action":%q,"tag":%q,"name":%q}`+"\n",
					action, tagFilter, nameFilter)
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// List all workflows
			params := map[string]string{"limit": "250"}
			if activeOnly {
				params["active"] = "true"
			}
			if tagFilter != "" {
				params["tags"] = tagFilter
			}
			data, err := c.Get("/workflows", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			type wfItem struct {
				ID     string           `json:"id"`
				Name   string           `json:"name"`
				Active bool             `json:"active"`
				Tags   []map[string]any `json:"tags"`
			}
			var wfs []wfItem
			// Try envelope
			var envelope map[string]json.RawMessage
			if json.Unmarshal(data, &envelope) == nil {
				if arr, ok := envelope["data"]; ok {
					_ = json.Unmarshal(arr, &wfs)
				}
			}
			if wfs == nil {
				_ = json.Unmarshal(data, &wfs)
			}

			var results []bulkResult
			for _, wf := range wfs {
				if nameFilter != "" && !strings.Contains(wf.Name, nameFilter) {
					continue
				}
				if inactiveOnly && wf.Active {
					continue
				}
				if activeOnly && !wf.Active {
					continue
				}

				r := bulkResult{WorkflowID: wf.ID, Name: wf.Name, Action: action}

				if flags.dryRun {
					r.Status = "would_" + action
					results = append(results, r)
					continue
				}

				var opErr error
				switch action {
				case "activate":
					_, _, opErr = c.Post("/workflows/"+wf.ID+"/activate", nil)
				case "deactivate":
					_, _, opErr = c.Post("/workflows/"+wf.ID+"/deactivate", nil)
				case "archive":
					_, _, opErr = c.Post("/workflows/"+wf.ID+"/archive", nil)
				}
				if opErr != nil {
					r.Status = "error"
					r.Error = opErr.Error()
				} else {
					r.Status = "ok"
				}
				results = append(results, r)
			}

			if len(results) == 0 {
				if flags.asJSON {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
					return nil
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No workflows matched the filter criteria.")
				return nil
			}

			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringVar(&action, "action", "", "Action to perform: activate, deactivate, or archive")
	cmd.Flags().StringVar(&tagFilter, "tag", "", "Filter workflows by tag name")
	cmd.Flags().StringVar(&nameFilter, "name", "", "Filter workflows by name substring")
	cmd.Flags().BoolVar(&activeOnly, "active", false, "Only affect currently active workflows")
	cmd.Flags().BoolVar(&inactiveOnly, "inactive", false, "Only affect currently inactive workflows")
	return cmd
}
