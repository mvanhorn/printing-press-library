// Copyright 2026 error. Licensed under Apache-2.0.
// Transcendence command: aggregate approvals across all configured agents
// into one stream. With --watch, polls for new requests live. Replaces
// alternating between approvals_list_ori and approvals_list_adam in the bridge.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/ori/internal/client"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/ori/internal/cliutil"
)

func newApprovalsRootCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "approvals",
		Short: "Cross-agent approval queue commands",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newApprovalsPendingCmd(flags))
	return cmd
}

type pendingApproval struct {
	Agent       string         `json:"agent"`
	ApprovalID  string         `json:"approval_id"`
	TaskID      string         `json:"task_id,omitempty"`
	Summary     string         `json:"summary"`
	RequestedAt string         `json:"requested_at,omitempty"`
	Context     map[string]any `json:"context,omitempty"`
}

func newApprovalsPendingCmd(flags *rootFlags) *cobra.Command {
	var agentFilter string
	var watch bool
	var interval time.Duration
	cmd := &cobra.Command{
		Use:   "pending",
		Short: "List pending approvals across all configured agents",
		Long: `Fans out to /a2a/{agent}/approvals for every configured agent (or just
--agent if set), merges the results, and prints them in one table. With
--watch, polls every --interval and prints new arrivals as they appear.`,
		Example: `  ori-pp-cli approvals pending
  ori-pp-cli approvals pending --agent ori --json
  ori-pp-cli approvals pending --watch --interval 10s`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			var agents []string
			if agentFilter != "" {
				agents = []string{agentFilter}
			} else {
				agents, err = discoverAgentNames(c)
				if err != nil {
					return apiErr(err)
				}
			}

			if cliutil.IsVerifyEnv() || !watch {
				items, errs := fetchApprovals(c, agents)
				return renderApprovals(cmd, flags, items, errs, false)
			}

			// Watch mode: poll forever or until ctx done; print only new ids.
			seen := map[string]bool{}
			ctx := cmd.Context()
			// Initial sweep (no new-only filter).
			items, errs := fetchApprovals(c, agents)
			for _, it := range items {
				seen[it.Agent+"|"+it.ApprovalID] = true
			}
			if err := renderApprovals(cmd, flags, items, errs, false); err != nil {
				return err
			}
			tick := time.NewTicker(interval)
			defer tick.Stop()
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-tick.C:
					items, errs := fetchApprovals(c, agents)
					fresh := []pendingApproval{}
					for _, it := range items {
						k := it.Agent + "|" + it.ApprovalID
						if !seen[k] {
							seen[k] = true
							fresh = append(fresh, it)
						}
					}
					if len(fresh) > 0 || len(errs) > 0 {
						_ = renderApprovals(cmd, flags, fresh, errs, true)
					}
				}
			}
		},
	}
	cmd.Flags().StringVar(&agentFilter, "agent", "", "Limit to one agent (default: all configured)")
	cmd.Flags().BoolVar(&watch, "watch", false, "Poll for new approvals continuously")
	cmd.Flags().DurationVar(&interval, "interval", 10*time.Second, "Poll interval when --watch is set")
	return cmd
}

func fetchApprovals(c *client.Client, agents []string) ([]pendingApproval, map[string]string) {
	out := []pendingApproval{}
	errs := map[string]string{}
	for _, name := range agents {
		body, err := c.Get("/a2a/"+name+"/approvals", nil)
		if err != nil {
			errs[name] = err.Error()
			continue
		}
		var resp struct {
			Approvals []struct {
				ApprovalID  string         `json:"approval_id"`
				TaskID      string         `json:"task_id"`
				Summary     string         `json:"summary"`
				RequestedAt string         `json:"requested_at"`
				Context     map[string]any `json:"context"`
			} `json:"approvals"`
		}
		if jerr := json.Unmarshal(body, &resp); jerr != nil {
			errs[name] = "parse: " + jerr.Error()
			continue
		}
		for _, a := range resp.Approvals {
			out = append(out, pendingApproval{
				Agent:       name,
				ApprovalID:  a.ApprovalID,
				TaskID:      a.TaskID,
				Summary:     a.Summary,
				RequestedAt: a.RequestedAt,
				Context:     a.Context,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].RequestedAt != out[j].RequestedAt {
			return out[i].RequestedAt < out[j].RequestedAt
		}
		if out[i].Agent != out[j].Agent {
			return out[i].Agent < out[j].Agent
		}
		return out[i].ApprovalID < out[j].ApprovalID
	})
	return out, errs
}

func renderApprovals(cmd *cobra.Command, flags *rootFlags, items []pendingApproval, errs map[string]string, watchUpdate bool) error {
	w := cmd.OutOrStdout()
	if flags.asJSON || (!isTerminal(w) && !flags.csv && !flags.quiet && !flags.plain) {
		payload := map[string]any{"approvals": items, "errors": errs, "count": len(items)}
		return printJSONFiltered(w, payload, flags)
	}
	if watchUpdate && len(items) > 0 {
		fmt.Fprintf(w, "\n[%s] %d new pending approvals:\n", time.Now().Format("15:04:05"), len(items))
	}
	if len(items) == 0 && !watchUpdate {
		if len(errs) == 0 {
			fmt.Fprintln(w, "No pending approvals.")
		}
	}
	for _, it := range items {
		short := it.ApprovalID
		if len(short) > 12 {
			short = short[:12]
		}
		fmt.Fprintf(w, "  %-4s  %s  %s\n", it.Agent, short, it.Summary)
		if it.RequestedAt != "" {
			fmt.Fprintf(w, "        requested: %s\n", it.RequestedAt)
		}
	}
	for agent, err := range errs {
		fmt.Fprintf(w, "  %s %s: %s\n", red("FAIL"), agent, err)
	}
	return nil
}
