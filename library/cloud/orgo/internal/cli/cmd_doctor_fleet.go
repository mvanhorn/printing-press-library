// Hand-authored: novel feature `fleet` — cross-workspace health
// rollup that lists every workspace's computers and flags stuck or
// suspended ones. Created during Phase 3 build; survives regen because
// it lives in its own file.
package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/cloud/orgo/internal/client"

	"github.com/spf13/cobra"
)

func newFleetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fleet",
		Short: "Cross-workspace health rollup: surfaces suspended, errored, stuck-creating, and stuck-stopping computers, plus an API-key validity probe.",
		Long: `Walk every workspace via the live API, list its computers, and
flag those whose status is in {suspended, error, creating, stopping}
when the status has been held for more than 5 minutes (when timestamp
data is available).

Pairs with 'orgo-pp-cli doctor' for a full health snapshot:
  - doctor: CLI-side config, auth, cache health
  - fleet:  cross-workspace runtime state

Run together for incident response or as a daily cron.`,
		Example: strings.Trim(`
  # Default: human-readable rollup
  orgo-pp-cli fleet

  # JSON for piping into automation
  orgo-pp-cli fleet --agent

  # Filter to a single workspace
  orgo-pp-cli fleet --workspace 550e8400-e29b-41d4-a716-446655440000
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			workspaceFilter, _ := cmd.Flags().GetString("workspace")

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			report := map[string]any{}
			report["timestamp"] = time.Now().UTC().Format(time.RFC3339)

			// Auth probe — implicit via the first list call. A 401 surfaces
			// as auth_valid=false; everything else is success.
			wsData, err := c.Get("/workspaces", nil)
			if err != nil {
				report["auth_valid"] = false
				report["error"] = err.Error()
				return outputFleetReport(cmd, flags, report)
			}
			report["auth_valid"] = true

			workspaces := extractWorkspaces(wsData)
			report["workspace_count"] = len(workspaces)

			now := time.Now().UTC()
			stuckWindow := 5 * time.Minute
			stuckStatuses := map[string]bool{
				"suspended": true,
				"error":     true,
				"creating":  true,
				"stopping":  true,
			}

			type fleetIssue struct {
				WorkspaceID string `json:"workspace_id"`
				ComputerID  string `json:"computer_id"`
				Name        string `json:"name"`
				Status      string `json:"status"`
				StatusAge   string `json:"status_age,omitempty"`
				Kind        string `json:"kind"`
			}

			var issues []fleetIssue
			workspaceSummaries := make([]map[string]any, 0, len(workspaces))

			for _, ws := range workspaces {
				wsID, _ := ws["id"].(string)
				if workspaceFilter != "" && wsID != workspaceFilter {
					continue
				}

				// Per-workspace get to surface desktops[].
				path := replacePathParam("/workspaces/{id}", "id", wsID)
				wsDetail, derr := c.Get(path, nil)
				if derr != nil {
					issues = append(issues, fleetIssue{
						WorkspaceID: wsID,
						Kind:        "workspace_unreachable",
						Status:      "error",
					})
					continue
				}
				desktops := extractDesktops(wsDetail)
				workspaceSummaries = append(workspaceSummaries, map[string]any{
					"id":             wsID,
					"name":           ws["name"],
					"computer_count": len(desktops),
				})
				for _, d := range desktops {
					status, _ := d["status"].(string)
					if !stuckStatuses[status] {
						continue
					}
					issue := fleetIssue{
						WorkspaceID: wsID,
						ComputerID:  fmt.Sprintf("%v", d["id"]),
						Name:        fmt.Sprintf("%v", d["name"]),
						Status:      status,
						Kind:        classifyFleetIssue(status),
					}
					if createdStr, ok := d["created_at"].(string); ok && createdStr != "" {
						if created, perr := time.Parse(time.RFC3339, createdStr); perr == nil {
							age := now.Sub(created)
							issue.StatusAge = age.Round(time.Second).String()
							// Suppress short-lived transitional states.
							if (status == "creating" || status == "stopping") && age < stuckWindow {
								continue
							}
						}
					}
					issues = append(issues, issue)
				}
			}

			sort.SliceStable(issues, func(i, j int) bool {
				if issues[i].Status != issues[j].Status {
					return issues[i].Status < issues[j].Status
				}
				return issues[i].WorkspaceID < issues[j].WorkspaceID
			})

			report["workspaces"] = workspaceSummaries
			report["issues"] = issues
			report["issue_count"] = len(issues)

			return outputFleetReport(cmd, flags, report)
		},
	}

	cmd.Flags().String("workspace", "", "Restrict the rollup to a single workspace ID")

	return cmd
}

func classifyFleetIssue(status string) string {
	switch status {
	case "suspended":
		return "suspended"
	case "error":
		return "errored"
	case "creating":
		return "stuck_creating"
	case "stopping":
		return "stuck_stopping"
	default:
		return "other"
	}
}

func outputFleetReport(cmd *cobra.Command, flags *rootFlags, report map[string]any) error {
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		return printJSONFiltered(cmd.OutOrStdout(), report, flags)
	}
	w := cmd.OutOrStdout()
	if v, ok := report["auth_valid"].(bool); ok && !v {
		fmt.Fprintf(w, "%s API auth: invalid (%v)\n", red("FAIL"), report["error"])
		return nil
	}
	fmt.Fprintf(w, "%s API auth: valid\n", green("OK"))
	if wsCount, ok := report["workspace_count"]; ok {
		fmt.Fprintf(w, "  workspaces: %v\n", wsCount)
	}
	issues, _ := report["issues"].([]struct {
		WorkspaceID string
		ComputerID  string
		Name        string
		Status      string
		StatusAge   string
		Kind        string
	})
	_ = issues // we render via the typed slice via JSON below

	// Re-render issues by re-marshalling — easier than reasserting types
	// on the heterogeneous map[string]any value.
	if rawIssues, ok := report["issues"]; ok {
		// Force-marshal into a known shape for printing.
		raw, _ := json.Marshal(rawIssues)
		var concrete []map[string]any
		_ = json.Unmarshal(raw, &concrete)
		if len(concrete) == 0 {
			fmt.Fprintf(w, "%s no issues across the fleet\n", green("OK"))
			return nil
		}
		fmt.Fprintf(w, "%s %d issue(s) across the fleet:\n", yellow("WARN"), len(concrete))
		tw := newTabWriter(w)
		fmt.Fprintln(tw, bold("WORKSPACE")+"\t"+bold("COMPUTER")+"\t"+bold("NAME")+"\t"+bold("STATUS")+"\t"+bold("KIND")+"\t"+bold("AGE"))
		for _, i := range concrete {
			fmt.Fprintf(tw, "%v\t%v\t%v\t%v\t%v\t%v\n",
				truncate(asStr(i["workspace_id"]), 8),
				truncate(asStr(i["computer_id"]), 8),
				asStr(i["name"]),
				asStr(i["status"]),
				asStr(i["kind"]),
				asStr(i["status_age"]))
		}
		return tw.Flush()
	}
	return nil
}

func asStr(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// PATCH(fleet-projects-envelope): accept {"projects":[...]} so fleet sees the live Orgo response.
// extractWorkspaces unwraps the /workspaces response into []map[string]any.
// Handles bare-array, {"workspaces":[...]}, and {"projects":[...]} shapes —
// the last one because the live API exposes workspaces as projects (see
// rewriteSpecPath in internal/client/client.go).
func extractWorkspaces(data json.RawMessage) []map[string]any {
	var envelope struct {
		Workspaces []map[string]any `json:"workspaces"`
		Projects   []map[string]any `json:"projects"`
	}
	if err := json.Unmarshal(data, &envelope); err == nil {
		if envelope.Workspaces != nil {
			return envelope.Workspaces
		}
		if envelope.Projects != nil {
			return envelope.Projects
		}
	}
	var direct []map[string]any
	if err := json.Unmarshal(data, &direct); err == nil {
		return direct
	}
	return nil
}

// extractDesktops pulls `desktops` out of a /workspaces/{id} response.
func extractDesktops(data json.RawMessage) []map[string]any {
	var ws map[string]any
	if err := json.Unmarshal(data, &ws); err != nil {
		return nil
	}
	desktops, ok := ws["desktops"].([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(desktops))
	for _, d := range desktops {
		if dm, ok := d.(map[string]any); ok {
			out = append(out, dm)
		}
	}
	return out
}

// Compile guard — keep client import hot in case future hooks land here.
var _ = client.New
