// Copyright 2026 John Fiedler and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"time"

	"github.com/spf13/cobra"
)

var version = "2026.6.2"

// noColor is set by --no-color and implied by --agent.
var noColor bool

type rootFlags struct {
	asJSON       bool
	compact      bool
	quiet        bool
	dryRun       bool
	noInput      bool
	yes          bool
	agent        bool
	selectFields string
	timeout      time.Duration
}

func Execute() error {
	var flags rootFlags
	return newRootCmd(&flags).Execute()
}

func newRootCmd(flags *rootFlags) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "foxnews-pp-cli",
		Short: "Fox News headlines from Google Publisher RSS feeds",
		Long: `Read Fox News headlines from the public Google Publisher RSS feeds on moxie.foxnews.com.

No API key is required. Use --section to choose a topic feed (default: latest).

Output is JSON when stdout is piped (how agents invoke commands). In an interactive
terminal the default is a plain table; use --json or --agent for JSON there.

Machine JSON uses a meta + results envelope. Run 'foxnews-pp-cli agent-context' for
command and section discovery (this CLI does not ship drudgereport-style sync/splash/breaking).`,
		SilenceUsage: true,
		Version:      version,
	}
	rootCmd.SetVersionTemplate("foxnews-pp-cli {{ .Version }}\n")

	rootCmd.PersistentFlags().BoolVar(&flags.asJSON, "json", false, "Output as JSON")
	rootCmd.PersistentFlags().BoolVar(&flags.compact, "compact", false, "Keep only high-gravity headline fields (title, link, published, section)")
	rootCmd.PersistentFlags().BoolVar(&flags.quiet, "quiet", false, "Suppress output")
	rootCmd.PersistentFlags().BoolVar(&flags.dryRun, "dry-run", false, "Validate flags without fetching")
	rootCmd.PersistentFlags().BoolVar(&flags.noInput, "no-input", false, "Disable interactive prompts")
	rootCmd.PersistentFlags().BoolVar(&flags.yes, "yes", false, "Skip confirmation prompts")
	rootCmd.PersistentFlags().StringVar(&flags.selectFields, "select", "", "Comma-separated fields to include (JSON array output; wins over --compact)")
	rootCmd.PersistentFlags().DurationVar(&flags.timeout, "timeout", 30*time.Second, "HTTP timeout for RSS fetch")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	rootCmd.PersistentFlags().BoolVar(&flags.agent, "agent", false, "Agent defaults: --json --compact --no-input --no-color --yes")

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if flags.agent {
			if !cmd.Flags().Changed("json") {
				flags.asJSON = true
			}
			if !cmd.Flags().Changed("compact") {
				flags.compact = true
			}
			if !cmd.Flags().Changed("no-input") {
				flags.noInput = true
			}
			if !cmd.Flags().Changed("yes") {
				flags.yes = true
			}
			if !cmd.Flags().Changed("no-color") {
				noColor = true
			}
		}
		return nil
	}

	rootCmd.AddCommand(newHeadlinesCmd(flags))
	rootCmd.AddCommand(newSectionsCmd(flags))
	rootCmd.AddCommand(newDoctorCmd(flags))
	rootCmd.AddCommand(newAgentContextCmd(rootCmd))
	return rootCmd
}

func ExitCode(err error) int {
	var codeErr *cliError
	if As(err, &codeErr) {
		return codeErr.code
	}
	return 1
}
