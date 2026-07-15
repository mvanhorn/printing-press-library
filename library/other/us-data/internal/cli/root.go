// Copyright 2026 Dhilip Subramanian. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var version = "2026.7.1"

type rootFlags struct {
	json    bool
	agent   bool
	compact bool
	timeout time.Duration
}

func Execute() error {
	flags := rootFlags{}
	rootCmd := &cobra.Command{
		Use:          "us-data-pp-cli",
		Short:        "Official US public data recipes for agents",
		SilenceUsage: true,
		Version:      version,
	}
	rootCmd.SetVersionTemplate("us-data-pp-cli {{ .Version }}\n")
	rootCmd.PersistentFlags().BoolVar(&flags.json, "json", false, "Print JSON output")
	rootCmd.PersistentFlags().BoolVar(&flags.agent, "agent", false, "Print compact agent-ready JSON")
	rootCmd.PersistentFlags().BoolVar(&flags.compact, "compact", false, "Print compact JSON")
	rootCmd.PersistentFlags().DurationVar(&flags.timeout, "timeout", 20*time.Second, "HTTP timeout")

	rootCmd.AddCommand(newCPICmd(&flags))
	rootCmd.AddCommand(newUnemploymentCmd(&flags))
	rootCmd.AddCommand(newPopulationCmd(&flags))
	rootCmd.AddCommand(newWagesCmd(&flags))
	rootCmd.AddCommand(newIndustryCmd(&flags))
	rootCmd.AddCommand(newCompareRegionsCmd(&flags))
	rootCmd.AddCommand(newSourcesCmd(&flags))
	rootCmd.AddCommand(newDoctorCmd(&flags))
	rootCmd.AddCommand(newVersionCmd())
	return rootCmd.Execute()
}

func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var usage usageError
	if errors.As(err, &usage) {
		return 2
	}
	return 1
}

type usageError struct {
	err error
}

func (e usageError) Error() string {
	return e.err.Error()
}

func usageErr(format string, args ...any) error {
	return usageError{err: fmt.Errorf(format, args...)}
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
	case SeriesResult:
		fmt.Fprintf(w, "%s (%s)\n", v.Title, v.SeriesID)
		fmt.Fprintf(w, "Latest: %s %s = %s\n", v.Latest.PeriodName, v.Latest.Year, v.Latest.Value)
		if v.Prior != nil && v.Prior.Value != "" {
			fmt.Fprintf(w, "Prior: %s %s = %s\n", v.Prior.PeriodName, v.Prior.Year, v.Prior.Value)
		}
		if v.PercentChange != nil {
			fmt.Fprintf(w, "Change: %.2f%%\n", *v.PercentChange)
		}
		fmt.Fprintf(w, "Source: %s\n", v.Source)
	case GuidanceResult:
		fmt.Fprintf(w, "%s\n", v.Title)
		for _, item := range v.Messages {
			fmt.Fprintf(w, "- %s\n", item)
		}
	case PopulationResult:
		fmt.Fprintf(w, "%s population: %s (%s)\n", v.Place, v.Population, v.Dataset)
		fmt.Fprintf(w, "Source: %s\n", v.Source)
	case CompareResult:
		fmt.Fprintf(w, "Compare regions: %s vs %s\n", v.Left.Region, v.Right.Region)
		for _, notice := range v.Notices {
			fmt.Fprintf(w, "- %s\n", notice)
		}
	default:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(value)
	}
	return nil
}

func env(name string) string {
	return strings.TrimSpace(os.Getenv(name))
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "us-data-pp-cli %s\n", version)
			return err
		},
	}
}
