// Hand-authored novel feature: TRQL recipe library.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type queryRecipe struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Params      map[string]string `json:"params,omitempty"`
	Query       string            `json:"query"`
}

// pp:novel-static-reference — curated TRQL recipes are static catalog data.
// Recipes are NOT synthesizing API responses; execution still POSTs to the
// real /api/v1/query endpoint via Trigger.dev.
var queryRecipes = []queryRecipe{
	{
		Name:        "failed-last-hour",
		Description: "Failed runs in the last hour, newest first.",
		Query:       "SELECT id, taskIdentifier, status, error.message FROM runs WHERE status IN ('FAILED','CRASHED','SYSTEM_FAILURE') AND createdAt >= now() - interval '1 hour' ORDER BY createdAt DESC LIMIT 100",
	},
	{
		Name:        "cost-by-model-7d",
		Description: "LLM cost grouped by model over the last 7 days.",
		Query:       "SELECT spans.model AS model, SUM(spans.costInCents) AS cost_cents, COUNT(*) AS span_count FROM spans WHERE spans.kind = 'LLM' AND spans.createdAt >= now() - interval '7 day' GROUP BY spans.model ORDER BY cost_cents DESC",
	},
	{
		Name:        "oncall-deploy-window",
		Description: "Failures in the 60 minutes after a deployment.",
		Params: map[string]string{
			"deploymentId": "ID of the deployment to anchor the window",
		},
		Query: "SELECT id, taskIdentifier, status, error.message FROM runs WHERE deploymentId = '${deploymentId}' AND status IN ('FAILED','CRASHED','SYSTEM_FAILURE') ORDER BY createdAt LIMIT 100",
	},
	{
		Name:        "task-success-rate-7d",
		Description: "Per-task run count and success rate over 7 days.",
		Query:       "SELECT taskIdentifier, COUNT(*) AS total, SUM(IF(status = 'COMPLETED', 1, 0)) AS succeeded, ROUND(SUM(IF(status = 'COMPLETED', 1, 0)) / COUNT(*), 3) AS success_rate FROM runs WHERE createdAt >= now() - interval '7 day' GROUP BY taskIdentifier ORDER BY total DESC LIMIT 50",
	},
	{
		Name:        "p95-duration-by-task",
		Description: "p50 and p95 run duration by task over the last 30 days.",
		Query:       "SELECT taskIdentifier, PERCENTILE(durationMs, 0.5) AS p50_ms, PERCENTILE(durationMs, 0.95) AS p95_ms, COUNT(*) AS runs FROM runs WHERE status = 'COMPLETED' AND createdAt >= now() - interval '30 day' GROUP BY taskIdentifier ORDER BY p95_ms DESC LIMIT 50",
	},
	{
		Name:        "queue-saturation",
		Description: "Queues whose recent runs were waiting longest before execution.",
		Query:       "SELECT queueName, AVG(EXTRACT(EPOCH FROM startedAt - createdAt)) AS avg_wait_sec, COUNT(*) AS runs FROM runs WHERE startedAt IS NOT NULL AND createdAt >= now() - interval '1 day' GROUP BY queueName ORDER BY avg_wait_sec DESC LIMIT 20",
	},
}

func newQueryRecipesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "recipes",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "List curated TRQL recipes (no API call)",
		Long: `Lists the curated TRQL one-liners agents can run via 'query run <name>'.
Recipes are static reference data; running them executes a real /api/v1/query
POST.`,
		Example: `  trigger-dev-pp-cli query recipes
  trigger-dev-pp-cli query recipes --json --select 'name,description'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			rows := append([]queryRecipe(nil), queryRecipes...)
			sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d TRQL recipes available — use 'query run <name>' to execute.\n\n", len(rows))
			fmt.Fprintf(cmd.OutOrStdout(), "%-25s %-10s %s\n", "name", "params", "description")
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", strings.Repeat("-", 90))
			for _, r := range rows {
				params := "—"
				if len(r.Params) > 0 {
					var keys []string
					for k := range r.Params {
						keys = append(keys, k)
					}
					sort.Strings(keys)
					params = strings.Join(keys, ",")
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-25s %-10s %s\n", r.Name, params, r.Description)
			}
			return nil
		},
	}
	return cmd
}

func newQueryRunCmd(flags *rootFlags) *cobra.Command {
	var paramFlags []string

	cmd := &cobra.Command{
		Use:         "run <recipe>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Run a curated TRQL recipe (POST /api/v1/query)",
		Long: `Substitutes --param key=value into the recipe's query template, then POSTs
to /api/v1/query. The recipe catalog is curated static reference; this command
calls the real Trigger.dev query endpoint.`,
		Example: `  trigger-dev-pp-cli query run failed-last-hour
  trigger-dev-pp-cli query run oncall-deploy-window --param deploymentId=dep_123 --json`,
		// pp:client-call — recipe execution is a real /api/v1/query POST.
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			name := args[0]
			recipe := lookupRecipe(name)
			if recipe == nil {
				return notFoundErr(fmt.Errorf("recipe %q not found — see 'query recipes' for the catalog", name))
			}
			params := map[string]string{}
			for _, p := range paramFlags {
				k, v, ok := strings.Cut(p, "=")
				if !ok {
					return usageErr(fmt.Errorf("--param expects key=value, got %q", p))
				}
				params[k] = v
			}
			query := recipe.Query
			for k, v := range params {
				query = strings.ReplaceAll(query, "${"+k+"}", v)
			}
			if strings.Contains(query, "${") {
				return usageErr(fmt.Errorf("recipe %q has unfilled params; pass them with --param key=value", name))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would POST /api/v1/query: %s\n", query)
				return nil
			}
			data, _, err := c.Post("/api/v1/query", map[string]any{"query": query})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringSliceVar(&paramFlags, "param", nil, "Recipe parameter (key=value), repeatable")
	return cmd
}

func lookupRecipe(name string) *queryRecipe {
	for i := range queryRecipes {
		if queryRecipes[i].Name == name {
			return &queryRecipes[i]
		}
	}
	return nil
}
