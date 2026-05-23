// Hand-written novel feature. Not generated.
package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// newRulesCmd is a novel parent ("rules dump"). The generated CRUD lives
// under "ticket-rules" / "workflow".
func newRulesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rules",
		Short: "Ticket-rule and workflow audit views (live API reads, formatted)",
		Long:  "Halo's UI has no export for ticket rules or workflows. 'rules dump' prints them as readable flat text for quarterly audits.",
	}
	cmd.AddCommand(newRulesDumpCmd(flags))
	return cmd
}

func newRulesDumpCmd(flags *rootFlags) *cobra.Command {
	var (
		workflow string
		limit    int
	)
	cmd := &cobra.Command{
		Use:   "dump",
		Short: "Print every ticket rule and workflow as readable flat text",
		Long:  "Fetches TicketRules and Workflow from the live API and renders them as one block per rule.",
		Example: strings.Trim(`
  # Dump every rule and workflow
  halopsa-pp-cli rules dump > rules-audit.txt

  # Filter workflow by name
  halopsa-pp-cli rules dump --workflow "New Ticket"

  # JSON shape for further processing
  halopsa-pp-cli rules dump --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			out := map[string]any{}
			// TicketRules
			rulesJSON, err := c.Get("/TicketRules", map[string]string{"page_size": fmt.Sprintf("%d", limit)})
			if err != nil {
				return fmt.Errorf("fetching ticket rules: %w", err)
			}
			rules := unwrapList(rulesJSON, "rules")
			out["ticket_rules"] = rules

			// Workflows
			params := map[string]string{"page_size": fmt.Sprintf("%d", limit)}
			if workflow != "" {
				params["search"] = workflow
			}
			wfJSON, err := c.Get("/Workflow", params)
			if err != nil {
				return fmt.Errorf("fetching workflows: %w", err)
			}
			workflows := unwrapList(wfJSON, "workflows")
			out["workflows"] = workflows

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, out)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "=== Ticket Rules (%d) ===\n\n", len(rules))
			for _, r := range rules {
				renderRule(cmd, r)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n=== Workflows (%d) ===\n\n", len(workflows))
			for _, w := range workflows {
				renderWorkflow(cmd, w)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&workflow, "workflow", "", "Filter workflows by name (search)")
	cmd.Flags().IntVar(&limit, "limit", 500, "Page size for rule/workflow fetch")
	_ = json.Compact
	return cmd
}

func unwrapList(raw json.RawMessage, listKey string) []map[string]any {
	var asObj map[string]any
	if err := json.Unmarshal(raw, &asObj); err == nil {
		if v, ok := asObj[listKey]; ok {
			if arr, ok := v.([]any); ok {
				return mapsFromAny(arr)
			}
		}
		for _, k := range []string{"items", "data", "results", "records"} {
			if v, ok := asObj[k]; ok {
				if arr, ok := v.([]any); ok {
					return mapsFromAny(arr)
				}
			}
		}
	}
	var asArr []map[string]any
	if err := json.Unmarshal(raw, &asArr); err == nil {
		return asArr
	}
	return nil
}

func mapsFromAny(arr []any) []map[string]any {
	out := make([]map[string]any, 0, len(arr))
	for _, x := range arr {
		if m, ok := x.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func renderRule(cmd *cobra.Command, r map[string]any) {
	fmt.Fprintf(cmd.OutOrStdout(), "Rule #%v: %v\n", r["id"], r["name"])
	if v, ok := r["description"]; ok && v != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "  Description: %v\n", v)
	}
	if v, ok := r["sequence"]; ok && v != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "  Sequence:    %v\n", v)
	}
	if v, ok := r["active"]; ok {
		fmt.Fprintf(cmd.OutOrStdout(), "  Active:      %v\n", v)
	}
	if cond, ok := r["criteria"]; ok && cond != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "  Criteria:    %v\n", trim(fmt.Sprintf("%v", cond), 200))
	}
	if act, ok := r["actions"]; ok && act != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "  Actions:     %v\n", trim(fmt.Sprintf("%v", act), 200))
	}
	fmt.Fprintln(cmd.OutOrStdout())
}

func renderWorkflow(cmd *cobra.Command, w map[string]any) {
	fmt.Fprintf(cmd.OutOrStdout(), "Workflow #%v: %v\n", w["id"], w["name"])
	if v, ok := w["description"]; ok && v != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "  Description: %v\n", v)
	}
	if steps, ok := w["steps"]; ok && steps != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "  Steps:       %v\n", trim(fmt.Sprintf("%v", steps), 200))
	}
	fmt.Fprintln(cmd.OutOrStdout())
}

func trim(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
