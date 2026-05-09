package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/silentnw/govprocure-pp-cli/cmd/compound"
	cmdgrants "github.com/silentnw/govprocure-pp-cli/cmd/grants"
	cmdsam "github.com/silentnw/govprocure-pp-cli/cmd/sam"
	cmdusa "github.com/silentnw/govprocure-pp-cli/cmd/usaspending"
	"github.com/silentnw/govprocure-pp-cli/internal/output"
	"github.com/spf13/cobra"
)

// GlobalOpts holds parsed global flag values, shared across subcommands.
var GlobalOpts = &output.Options{}

// rootCmd is the top-level Cobra command.
var rootCmd = &cobra.Command{
	Use:   "govprocure-pp-cli",
	Short: "U.S. government procurement intelligence CLI",
	Long: `govprocure-pp-cli — procurement intelligence across grants.gov, SAM.gov, and USASpending.gov.

Local SQLite mirror with FTS5 full-text search. Agent-native output (JSON when piped).

Quick start:
  govprocure-pp-cli doctor              # verify all APIs + database
  govprocure-pp-cli auth set-key --sam YOUR_SAM_KEY
  govprocure-pp-cli sync --all          # pull all sources into local DB
  govprocure-pp-cli grants search "AI consulting"
  govprocure-pp-cli compound pipeline "SDVOSB technology"`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command.
func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	return nil
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().BoolVar(&GlobalOpts.Compact, "compact", false, "minimal fields only (id, title, deadline, amount)")
	rootCmd.PersistentFlags().BoolVar(&GlobalOpts.AgentMode, "agent", false, "force JSON output (agent-native mode)")
	rootCmd.PersistentFlags().BoolVar(&GlobalOpts.DryRun, "dry-run", false, "show what would happen, no API calls")

	var selectStr string
	rootCmd.PersistentFlags().StringVar(&selectStr, "select", "", "comma-separated fields to include (e.g. id,title,deadline)")

	var outputFmt string
	rootCmd.PersistentFlags().StringVar(&outputFmt, "output", "auto", "output format: table|json|csv|auto")

	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if selectStr != "" {
			GlobalOpts.Select = strings.Split(selectStr, ",")
		}
		switch outputFmt {
		case "json":
			GlobalOpts.Format = output.FormatJSON
		case "table":
			GlobalOpts.Format = output.FormatTable
		case "csv":
			GlobalOpts.Format = output.FormatCSV
		case "compact":
			GlobalOpts.Format = output.FormatCompact
		default:
			GlobalOpts.Format = output.FormatAuto
		}
	}

	// Register subcommand groups
	rootCmd.AddCommand(cmdgrants.NewGrantsCmd(GlobalOpts))
	rootCmd.AddCommand(cmdsam.NewSAMCmd(GlobalOpts))
	rootCmd.AddCommand(cmdusa.NewUSASpendingCmd(GlobalOpts))
	rootCmd.AddCommand(compound.NewCompoundCmd(GlobalOpts))
	rootCmd.AddCommand(newAuthCmd())
	rootCmd.AddCommand(newSyncAllCmd())
	rootCmd.AddCommand(newDoctorCmd())

	rootCmd.Version = "1.0.0"
}
