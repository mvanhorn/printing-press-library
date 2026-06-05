// Copyright 2026 ardihanan and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// pp:data-source local

// filterSandboxRules returns the rules matching the optional scenario and
// method filters. An empty filter matches all.
func filterSandboxRules(scenario, method string) []sandboxRule {
	scenario = strings.TrimSpace(strings.ToLower(scenario))
	method = strings.TrimSpace(strings.ToLower(method))
	out := make([]sandboxRule, 0, len(sandboxRules))
	for _, r := range sandboxRules {
		if scenario != "" && r.Scenario != scenario {
			continue
		}
		if method != "" && r.Method != method {
			continue
		}
		out = append(out, r)
	}
	return out
}

// sandboxScenarioNames returns the sorted distinct scenario values.
func sandboxScenarioNames() []string {
	seen := map[string]bool{}
	for _, r := range sandboxRules {
		seen[r.Scenario] = true
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// sandboxMethodNames returns the sorted distinct method values.
func sandboxMethodNames() []string {
	seen := map[string]bool{}
	for _, r := range sandboxRules {
		seen[r.Method] = true
	}
	out := make([]string, 0, len(seen))
	for m := range seen {
		out = append(out, m)
	}
	sort.Strings(out)
	return out
}

func newNovelSandboxSimulateCmd(flags *rootFlags) *cobra.Command {
	var flagScenario string
	var flagMethod string

	cmd := &cobra.Command{
		Use:         "simulate",
		Short:       "Print the exact sandbox magic values (even/odd account numbers, Simulator steps) that force a success, pending, or failed outcome.",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--scenario=invalid-account;--method=bank-transfer"},
		Long: `Print Durianpay sandbox simulation rules: which magic input values (e.g.
even/odd beneficiaryAccountNo) and which Simulator steps force a given
outcome. Static and offline — never calls the API.

With no flags, lists every scenario. Filter with --scenario and/or --method.`,
		Example: strings.TrimLeft(`
  durianpay-pp-cli sandbox simulate
  durianpay-pp-cli sandbox simulate --scenario=invalid-account --method=bank-transfer
  durianpay-pp-cli sandbox simulate --method=payment --json`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags != nil && flags.dataSource == "live" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("sandbox simulate is an offline static lookup with no live equivalent; drop --data-source live"))
			}
			if dryRunOK(flags) {
				return nil
			}
			rules := filterSandboxRules(flagScenario, flagMethod)
			if len(rules) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("no sandbox scenario matches --scenario=%q --method=%q\n  scenarios: %s\n  methods:   %s",
					flagScenario, flagMethod,
					strings.Join(sandboxScenarioNames(), ", "),
					strings.Join(sandboxMethodNames(), ", ")))
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), rules, flags)
			}
			out := cmd.OutOrStdout()
			for i, r := range rules {
				if i > 0 {
					fmt.Fprintln(out)
				}
				fmt.Fprintf(out, "%s  [scenario=%s method=%s]\n", r.Title, r.Scenario, r.Method)
				fmt.Fprintf(out, "  magic:  %s\n", r.Magic)
				fmt.Fprintf(out, "  how-to: %s\n", r.HowTo)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagScenario, "scenario", "", fmt.Sprintf("Filter by outcome: %s", strings.Join(sandboxScenarioNames(), "|")))
	cmd.Flags().StringVar(&flagMethod, "method", "", fmt.Sprintf("Filter by method: %s", strings.Join(sandboxMethodNames(), "|")))
	return cmd
}
