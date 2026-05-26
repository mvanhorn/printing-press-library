package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"notebooklm-pp-cli/internal/nlm"
)

var version = "1.0.0"

var runner *nlm.Runner

type rootFlags struct {
	json     bool
	compact  bool
	noInput  bool
	noColor  bool
	agent    bool
	yes      bool
	profile  string
}

var flags rootFlags

func Execute() error {
	rootCmd := &cobra.Command{
		Use:          "notebooklm-pp-cli",
		Short:        "Query and manage Google NotebookLM notebooks from the terminal",
		SilenceUsage: true,
		Version:      version,
	}
	rootCmd.SetVersionTemplate("notebooklm-pp-cli {{ .Version }}\n")

	rootCmd.PersistentFlags().BoolVar(&flags.json, "json", false, "JSON output")
	rootCmd.PersistentFlags().BoolVar(&flags.compact, "compact", false, "Compact output")
	rootCmd.PersistentFlags().BoolVar(&flags.noInput, "no-input", false, "Non-interactive mode")
	rootCmd.PersistentFlags().BoolVar(&flags.noColor, "no-color", false, "Disable color output")
	rootCmd.PersistentFlags().BoolVar(&flags.agent, "agent", false, "Agent-friendly defaults (--json --compact --no-input --no-color --yes)")
	rootCmd.PersistentFlags().BoolVarP(&flags.yes, "yes", "y", false, "Skip confirmation prompts")

	rootCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if flags.agent {
			flags.json = true
			flags.compact = true
			flags.noInput = true
			flags.noColor = true
			flags.yes = true
		}
		return nil
	}

	// Initialize runner for commands that need nlm.
	// doctor and version can run without it.
	var runnerErr error
	runner, runnerErr = nlm.NewRunner()

	rootCmd.AddCommand(newQueryCmd())
	rootCmd.AddCommand(newCrossQueryCmd())
	rootCmd.AddCommand(newSyncCmd())
	rootCmd.AddCommand(newSearchCmd())
	rootCmd.AddCommand(newDoctorCmd(runnerErr))
	rootCmd.AddCommand(newAgentContextCmd())
	rootCmd.AddCommand(newExportCmd())
	rootCmd.AddCommand(newImportCmd())
	rootCmd.AddCommand(newAnalyticsCmd())
	rootCmd.AddCommand(newFeedbackCmd())
	rootCmd.AddCommand(newProfileCmd())
	rootCmd.AddCommand(newTailCmd())
	rootCmd.AddCommand(newWhichCmd())
	rootCmd.AddCommand(newWorkflowCmd())
	rootCmd.AddCommand(newAuthCmd())
	rootCmd.AddCommand(newNotebooksCmd())
	rootCmd.AddCommand(newNotesCmd())
	rootCmd.AddCommand(newResearchCmd())
	rootCmd.AddCommand(newSourcesCmd())
	rootCmd.AddCommand(newStudioCmd())

	return rootCmd.Execute()
}

func ExitCode(err error) int {
	return 1
}

func requireRunner() (*nlm.Runner, error) {
	if runner == nil {
		return nil, fmt.Errorf("nlm not available: install with 'pipx install notebooklm-mcp-cli'")
	}
	return runner, nil
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
