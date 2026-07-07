// Copyright 2026 Dhilip Subramanian and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var version = "1.0.0"

type rootFlags struct {
	json    bool
	agent   bool
	compact bool
	timeout time.Duration
}

// Execute runs the CLI.
func Execute() error {
	flags := rootFlags{timeout: 20 * time.Second}
	return newRootCmd(&flags).Execute()
}

func newRootCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "air-quality-pp-cli",
		Short:        "Air quality source checks and comparison recipes for agents",
		SilenceUsage: true,
		Version:      version,
	}
	cmd.SetVersionTemplate("air-quality-pp-cli {{ .Version }}\n")
	cmd.PersistentFlags().BoolVar(&flags.json, "json", false, "Print JSON output")
	cmd.PersistentFlags().BoolVar(&flags.agent, "agent", false, "Print compact agent-ready JSON")
	cmd.PersistentFlags().BoolVar(&flags.compact, "compact", false, "Print compact JSON")
	cmd.PersistentFlags().DurationVar(&flags.timeout, "timeout", 20*time.Second, "HTTP timeout")
	cmd.AddCommand(
		newCurrentCmd(flags),
		newNearestCmd(flags),
		newLocationCmd(flags),
		newHistoryCmd(flags),
		newCompareCmd(flags),
		newAirNowCmd(flags),
		newSourcesCmd(flags),
		newDoctorCmd(flags),
		newVersionCmd(),
	)
	return cmd
}

// ExitCode extracts exit code from an error (always 1 for now).
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

func env(name string) string {
	return os.Getenv(name)
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
	case GuidanceResult:
		fmt.Fprintf(w, "%s\n", v.Title)
		for _, item := range v.Setup {
			fmt.Fprintf(w, "- %s\n", item)
		}
		for _, item := range v.Caveats {
			fmt.Fprintf(w, "- %s\n", item)
		}
	case CurrentResult:
		fmt.Fprintf(w, "%s: %s\n", v.Source, v.Location.Name)
		for _, m := range v.Measurements {
			fmt.Fprintf(w, "- %s: %s %s (%s)\n", m.Parameter, m.Value, m.Unit, m.Timestamp)
		}
		for _, item := range v.Caveats {
			fmt.Fprintf(w, "- %s\n", item)
		}
	case NearestResult:
		fmt.Fprintf(w, "%s: %d locations\n", v.Source, len(v.Locations))
		for _, loc := range v.Locations {
			fmt.Fprintf(w, "- %s (%s)\n", loc.Name, loc.ID)
		}
	case HistoryResult:
		fmt.Fprintf(w, "%s: %d measurements\n", v.Source, len(v.Measurements))
		for _, m := range v.Measurements {
			fmt.Fprintf(w, "- %s: %s %s (%s)\n", m.Parameter, m.Value, m.Unit, m.Timestamp)
		}
	case CompareResult:
		fmt.Fprintf(w, "Compare %s vs %s\n", v.Left.Location.Name, v.Right.Location.Name)
		for _, item := range v.Caveats {
			fmt.Fprintf(w, "- %s\n", item)
		}
	case SourcesResult:
		fmt.Fprintf(w, "air-quality sources\n")
		for _, source := range v.Sources {
			fmt.Fprintf(w, "- %s: configured=%t\n", source.Name, source.Configured)
		}
	case DoctorResult:
		fmt.Fprintf(w, "air-quality-pp-cli doctor\n")
		fmt.Fprintf(w, "  openaq: %s\n", configuredText(v.OpenAQConfigured))
		fmt.Fprintf(w, "  airnow: %s\n", configuredText(v.AirNowConfigured))
	default:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(value)
	}
	return nil
}

func configuredText(ok bool) string {
	if ok {
		return "configured"
	}
	return "missing key"
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "air-quality-pp-cli %s\n", version)
		},
	}
}
