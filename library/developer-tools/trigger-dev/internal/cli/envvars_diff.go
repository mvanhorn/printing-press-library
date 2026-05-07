// Hand-authored novel feature: env-var diff between two environments.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newEnvvarsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "envvars",
		Short: "Cross-environment env-var operations (diff)",
		Long:  "Project-scoped env-var operations that the per-endpoint CLI doesn't cover. See 'projects envvars' for raw CRUD.",
	}
	cmd.AddCommand(newEnvvarsDiffCmd(flags))
	return cmd
}

func newEnvvarsDiffCmd(flags *rootFlags) *cobra.Command {
	var projectRef, fromEnv, toEnv string

	cmd := &cobra.Command{
		Use:         "diff [env1] [env2]",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Side-by-side diff of env vars between two environments (values masked)",
		Long: `Fetches env vars for two environments via /api/v1/projects/{ref}/envvars/{env}
and computes the set diff. Pass two environments either positionally
(env1 env2) or via --from/--to. Values are masked by default.`,
		Example: `  trigger-dev-pp-cli envvars diff prod staging --project proj_abc
  trigger-dev-pp-cli envvars diff --from prod --to staging --project proj_abc --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			env1, env2 := fromEnv, toEnv
			if env1 == "" || env2 == "" {
				if len(args) < 2 {
					return cmd.Help()
				}
				env1, env2 = args[0], args[1]
			}
			if projectRef == "" {
				return usageErr(fmt.Errorf("--project <projectRef> is required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would diff %s vs %s for project %s\n", env1, env2, projectRef)
				return nil
			}

			fetchEnv := func(env string) (map[string]string, error) {
				path := fmt.Sprintf("/api/v1/projects/%s/envvars/%s", projectRef, env)
				resp, err := c.Get(path, nil)
				if err != nil {
					return nil, classifyAPIError(err, flags)
				}
				out := map[string]string{}
				type pair struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				}
				var direct []pair
				if err := json.Unmarshal(resp, &direct); err == nil && direct != nil {
					for _, p := range direct {
						out[p.Name] = p.Value
					}
					return out, nil
				}
				var envelope struct {
					Data []pair `json:"data"`
				}
				if err := json.Unmarshal(resp, &envelope); err == nil {
					for _, p := range envelope.Data {
						out[p.Name] = p.Value
					}
				}
				return out, nil
			}

			vars1, err := fetchEnv(env1)
			if err != nil {
				return fmt.Errorf("fetching %s: %w", env1, err)
			}
			vars2, err := fetchEnv(env2)
			if err != nil {
				return fmt.Errorf("fetching %s: %w", env2, err)
			}

			keys := map[string]bool{}
			for k := range vars1 {
				keys[k] = true
			}
			for k := range vars2 {
				keys[k] = true
			}
			ordered := make([]string, 0, len(keys))
			for k := range keys {
				ordered = append(ordered, k)
			}
			sort.Strings(ordered)

			type entry struct {
				Name   string `json:"name"`
				Status string `json:"status"`
				Env1   string `json:"env1_value,omitempty"`
				Env2   string `json:"env2_value,omitempty"`
			}
			var (
				entries                             []entry
				same, different, onlyEnv1, onlyEnv2 int
			)
			for _, k := range ordered {
				v1, in1 := vars1[k]
				v2, in2 := vars2[k]
				e := entry{Name: k}
				switch {
				case in1 && !in2:
					e.Status = "only_" + env1
					e.Env1 = maskEnvValue(v1)
					onlyEnv1++
				case !in1 && in2:
					e.Status = "only_" + env2
					e.Env2 = maskEnvValue(v2)
					onlyEnv2++
				case v1 == v2:
					e.Status = "same"
					same++
				default:
					e.Status = "different"
					e.Env1 = maskEnvValue(v1)
					e.Env2 = maskEnvValue(v2)
					different++
				}
				entries = append(entries, e)
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"project":      projectRef,
					"env1":         env1,
					"env2":         env2,
					"same":         same,
					"different":    different,
					"only_" + env1: onlyEnv1,
					"only_" + env2: onlyEnv2,
					"entries":      entries,
				}, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Env-var diff: %s vs %s (project %s)\n", env1, env2, projectRef)
			fmt.Fprintf(cmd.OutOrStdout(), "%d same, %d different, %d only in %s, %d only in %s\n\n",
				same, different, onlyEnv1, env1, onlyEnv2, env2)
			if different+onlyEnv1+onlyEnv2 == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Environments are identical.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-12s %-22s %-22s\n", "variable", "status", env1, env2)
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", strings.Repeat("-", 92))
			for _, e := range entries {
				if e.Status == "same" {
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-12s %-22s %-22s\n",
					truncate(e.Name, 30), e.Status, truncate(e.Env1, 22), truncate(e.Env2, 22))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&projectRef, "project", "", "Project reference (proj_abc...) — required")
	cmd.Flags().StringVar(&fromEnv, "from", "", "Source environment (alternative to first positional arg)")
	cmd.Flags().StringVar(&toEnv, "to", "", "Target environment (alternative to second positional arg)")
	return cmd
}

func maskEnvValue(v string) string {
	if len(v) <= 4 {
		return strings.Repeat("*", len(v))
	}
	return v[:2] + strings.Repeat("*", len(v)-4) + v[len(v)-2:]
}
