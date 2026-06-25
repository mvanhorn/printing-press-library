// uk-train-goat hand-authored: programmatic agent eval grader. Transcendence feature.
//
// Validates that fixture-defined NL prompts map to real CLI commands.
// v0.1 shipping scope: structural eval (each fixture's expected.tool
// must resolve to a registered command in the CLI tree, args_subset keys
// must match the command's flag set). The full model-in-the-loop run
// (where an LLM picks a tool and we compare to expected) lands in v0.2
// behind EVAL_AGENT_MODEL — see internal/evals/README.md.
//
// The 80% pass-rate threshold (see Phase 1.5 absorb manifest) applies
// here: structural pass rate must be >= 80%, otherwise CI gate fails.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/travel/uk-train-goat/internal/evals"

	"github.com/spf13/cobra"
)

func newEvalCmd(flags *rootFlags) *cobra.Command {
	var (
		datasetPath string
		threshold   float64
	)
	cmd := &cobra.Command{
		Use:   "eval",
		Short: "Programmatic eval grader for the agent surface (transcendence)",
		Long: `Validate the eval fixture suite against the live CLI command tree. v0.1
shipping scope is structural: every fixture's expected.tool must resolve to
a real registered command. Pass rate is reported as a percentage and the
process exits non-zero if it falls below --threshold (default 80%).

The full agent-in-the-loop eval (where a configured LLM picks a tool, and
the grader compares the chosen tool to expected) lands in v0.2 behind the
EVAL_AGENT_MODEL env var.`,
		Example: "  uk-train-goat-pp-cli eval --json\n  uk-train-goat-pp-cli eval --threshold 90 --json",
		Annotations: map[string]string{
			"mcp:hidden":            "true",
			"pp:typed-exit-codes":   "0,5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if datasetPath == "" {
				datasetPath = filepath.Join("internal", "evals", "dataset", "v0.1.yaml")
			}
			fixtures, err := evals.LoadFixtures(datasetPath)
			if err != nil {
				return fmt.Errorf("loading fixtures from %s: %w", datasetPath, err)
			}

			// Build the registered-command set for structural validation.
			rootCmd := cmd.Root()
			tools := registeredCommandSet(rootCmd)

			results := evals.GradeStructural(fixtures, tools)

			if threshold == 0 {
				threshold = 80.0
			}
			passRate := results.PassRate()
			payload := map[string]any{
				"fixtures":    len(fixtures),
				"passed":      results.Passed,
				"failed":      results.Failed,
				"pass_rate":   passRate,
				"threshold":   threshold,
				"per_fixture": results.PerFixture,
				"agent_model": os.Getenv("EVAL_AGENT_MODEL"),
				"mode":        "structural", // v0.2 will add "agent-in-the-loop"
			}
			data, _ := json.Marshal(payload)
			if err := printOutputWithFlags(cmd.OutOrStdout(), data, flags); err != nil {
				return err
			}
			if passRate < threshold {
				return apiErr(fmt.Errorf("eval pass rate %.1f%% below threshold %.1f%%", passRate, threshold))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&datasetPath, "dataset", "", "Path to fixture YAML (default internal/evals/dataset/v0.1.yaml)")
	cmd.Flags().Float64Var(&threshold, "threshold", 0, "Minimum pass rate percentage required (default 80)")
	return cmd
}

// registeredCommandSet collects the tool names exposed at the CLI root.
// Used to validate that every fixture's expected.tool refers to a real
// command. Does not flatten subcommands (saved status etc.) in v0.1; the
// fixture suite scopes to top-level tool selection which matches what an
// agent would do.
func registeredCommandSet(root *cobra.Command) map[string]bool {
	set := map[string]bool{}
	for _, c := range root.Commands() {
		if c.Hidden {
			continue
		}
		set[c.Name()] = true
	}
	// Stable ordering for deterministic output (debug paths only).
	names := make([]string, 0, len(set))
	for n := range set {
		names = append(names, n)
	}
	sort.Strings(names)
	return set
}
