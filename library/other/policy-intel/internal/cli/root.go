// Copyright 2026 Dhilip Subramanian and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var version = "2026.6.1"

type rootFlags struct {
	json    bool
	agent   bool
	compact bool
	timeout time.Duration
}

func Execute() error {
	flags := rootFlags{timeout: 20 * time.Second}
	cmd := newRootCmd(&flags)
	return cmd.Execute()
}

func newRootCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "policy-intel-pp-cli",
		Short:   "Federal rulemaking and policy intelligence for agents",
		Version: version,
	}
	cmd.PersistentFlags().BoolVar(&flags.json, "json", false, "Print JSON output")
	cmd.PersistentFlags().BoolVar(&flags.agent, "agent", false, "Print compact agent-ready JSON")
	cmd.PersistentFlags().BoolVar(&flags.compact, "compact", false, "Print compact JSON")
	cmd.PersistentFlags().DurationVar(&flags.timeout, "timeout", 20*time.Second, "HTTP timeout")
	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)
	cmd.AddCommand(
		newFederalRegisterCmd(flags),
		newRulesCmd(flags),
		newDocketCmd(flags),
		newCommentsCmd(flags),
		newDeadlinesCmd(flags),
		newSourcesCmd(flags),
		newDoctorCmd(flags),
	)
	return cmd
}

func commandContext(cmd *cobra.Command, flags *rootFlags) (context.Context, context.CancelFunc) {
	timeout := flags.timeout
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	return context.WithTimeout(cmd.Context(), timeout)
}

func printResult(cmd *cobra.Command, flags *rootFlags, value any) error {
	jsonOutput := flags.json || flags.agent
	compact := flags.compact || flags.agent
	if jsonOutput || compact {
		enc := json.NewEncoder(cmd.OutOrStdout())
		if !compact {
			enc.SetIndent("", "  ")
		}
		return enc.Encode(value)
	}
	return printText(cmd.OutOrStdout(), value)
}

func printText(w io.Writer, value any) error {
	switch v := value.(type) {
	case FederalRegisterSearchResult:
		fmt.Fprintf(w, "%s: %d matches\n", v.Source, v.Count)
		for _, item := range v.Results {
			fmt.Fprintf(w, "- %s (%s, %s)\n", item.Title, item.Type, item.PublicationDate)
			if item.HTMLURL != "" {
				fmt.Fprintf(w, "  %s\n", item.HTMLURL)
			}
		}
	case RegulationsListResult:
		fmt.Fprintf(w, "%s: %d matches\n", v.Source, v.Total)
		for _, item := range v.Results {
			fmt.Fprintf(w, "- %s (%s, %s)\n", item.Title, item.AgencyID, item.ID)
		}
	case DocketResult:
		fmt.Fprintf(w, "%s (%s)\n", v.Title, v.ID)
		fmt.Fprintf(w, "Agency: %s\n", v.AgencyID)
		if v.Abstract != "" {
			fmt.Fprintf(w, "%s\n", v.Abstract)
		}
	case GuidanceResult:
		fmt.Fprintf(w, "%s\n", v.Title)
		for _, item := range v.Messages {
			fmt.Fprintf(w, "- %s\n", item)
		}
	default:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(value)
	}
	return nil
}
