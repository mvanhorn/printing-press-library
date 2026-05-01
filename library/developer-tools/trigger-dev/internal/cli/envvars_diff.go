package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newEnvvarsDiffCmd(flags *rootFlags) *cobra.Command {
	var projectRef string

	cmd := &cobra.Command{
		Use:   "diff [env1] [env2]",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Compare environment variables between two environments",
		Long: `Side-by-side comparison of environment variables between two environments.
Highlights missing, different, and matching values. Helps catch drift between
dev, staging, and prod.`,
		Example: `  # Compare dev and prod
  trigger-dev-pp-cli envvars diff dev prod

  # Compare staging and prod with specific project
  trigger-dev-pp-cli envvars diff staging prod --project proj_abc123

  # JSON output
  trigger-dev-pp-cli envvars diff dev prod --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			env1, env2 := args[0], args[1]

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			if projectRef == "" {
				projectRef = "default"
			}

			if flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "Would diff envvars between %s and %s for project %s\n", env1, env2, projectRef)
				return nil
			}

			// Fetch both environments
			fetchEnv := func(env string) (map[string]string, error) {
				path := fmt.Sprintf("/api/v1/projects/%s/envvars/%s", projectRef, env)
				resp, err := c.Get(path, nil)
				if err != nil {
					return nil, err
				}

				var vars []struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				}
				if err := json.Unmarshal(resp, &vars); err != nil {
					// Try envelope
					var env2 struct {
						Data []struct {
							Name  string `json:"name"`
							Value string `json:"value"`
						} `json:"data"`
					}
					if err2 := json.Unmarshal(resp, &env2); err2 == nil {
						for _, v := range env2.Data {
							vars = append(vars, v)
						}
					}
				}

				result := make(map[string]string)
				for _, v := range vars {
					result[v.Name] = v.Value
				}
				return result, nil
			}

			vars1, err := fetchEnv(env1)
			if err != nil {
				return fmt.Errorf("fetching %s vars: %w", env1, err)
			}

			vars2, err := fetchEnv(env2)
			if err != nil {
				return fmt.Errorf("fetching %s vars: %w", env2, err)
			}

			// Collect all keys
			allKeys := make(map[string]bool)
			for k := range vars1 {
				allKeys[k] = true
			}
			for k := range vars2 {
				allKeys[k] = true
			}

			var keys []string
			for k := range allKeys {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			type diffEntry struct {
				Name   string `json:"name"`
				Status string `json:"status"` // same, different, only_env1, only_env2
				Env1   string `json:"env1_value,omitempty"`
				Env2   string `json:"env2_value,omitempty"`
			}

			var diffs []diffEntry
			var same, different, onlyEnv1, onlyEnv2 int

			for _, k := range keys {
				v1, in1 := vars1[k]
				v2, in2 := vars2[k]

				entry := diffEntry{Name: k}
				switch {
				case in1 && !in2:
					entry.Status = "only_" + env1
					entry.Env1 = maskValue(v1)
					onlyEnv1++
				case !in1 && in2:
					entry.Status = "only_" + env2
					entry.Env2 = maskValue(v2)
					onlyEnv2++
				case v1 == v2:
					entry.Status = "same"
					same++
				default:
					entry.Status = "different"
					entry.Env1 = maskValue(v1)
					entry.Env2 = maskValue(v2)
					different++
				}
				diffs = append(diffs, entry)
			}

			if flags.asJSON {
				result := map[string]any{
					"env1":         env1,
					"env2":         env2,
					"same":         same,
					"different":    different,
					"only_" + env1: onlyEnv1,
					"only_" + env2: onlyEnv2,
					"entries":      diffs,
				}
				return flags.printJSON(cmd, result)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Environment Diff: %s vs %s\n", env1, env2)
			fmt.Fprintf(cmd.OutOrStdout(), "%d same, %d different, %d only in %s, %d only in %s\n\n",
				same, different, onlyEnv1, env1, onlyEnv2, env2)

			if different+onlyEnv1+onlyEnv2 == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Environments are identical.\n")
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-10s %-25s %-25s\n",
				"variable", "status", env1, env2)
			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-10s %-25s %-25s\n",
				strings.Repeat("-", 30), strings.Repeat("-", 10), strings.Repeat("-", 25), strings.Repeat("-", 25))

			for _, d := range diffs {
				if d.Status == "same" {
					continue // Only show differences
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-10s %-25s %-25s\n",
					truncate(d.Name, 30), d.Status,
					truncate(d.Env1, 25), truncate(d.Env2, 25))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&projectRef, "project", "", "Project reference (e.g., proj_abc123)")

	return cmd
}

func maskValue(v string) string {
	if len(v) <= 4 {
		return "****"
	}
	return v[:2] + strings.Repeat("*", len(v)-4) + v[len(v)-2:]
}
