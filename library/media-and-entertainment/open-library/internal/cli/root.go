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

var version = "2026.8.2"

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
	rootCmd := &cobra.Command{
		Use:          "open-library-pp-cli",
		Short:        "Open Library book, author, edition, and subject metadata recipes",
		SilenceUsage: true,
		Version:      version,
	}
	rootCmd.SetVersionTemplate("open-library-pp-cli {{ .Version }}\n")
	rootCmd.PersistentFlags().BoolVar(&flags.json, "json", false, "Print JSON output")
	rootCmd.PersistentFlags().BoolVar(&flags.agent, "agent", false, "Print compact agent-ready JSON")
	rootCmd.PersistentFlags().BoolVar(&flags.compact, "compact", false, "Print compact JSON")
	rootCmd.PersistentFlags().DurationVar(&flags.timeout, "timeout", 20*time.Second, "HTTP timeout")
	rootCmd.AddCommand(
		newBookCmd(flags),
		newISBNCmd(flags),
		newAuthorCmd(flags),
		newWorkCmd(flags),
		newEditionsCmd(flags),
		newSubjectsCmd(flags),
		newSourcesCmd(flags),
		newDoctorCmd(flags),
		newVersionCmd(),
	)

	return rootCmd
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

func commandContext(cmd *cobra.Command, flags *rootFlags) (context.Context, context.CancelFunc) {
	timeout := flags.timeout
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	return context.WithTimeout(cmd.Context(), timeout)
}

func env(name string) string {
	return os.Getenv(name)
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
	case BookSearchResult:
		fmt.Fprintf(w, "%s: %d of %d results\n", v.Source, len(v.Results), v.Total)
		for _, item := range v.Results {
			fmt.Fprintf(w, "- %s", item.Title)
			if len(item.Authors) > 0 {
				fmt.Fprintf(w, " by %s", item.Authors[0])
			}
			if item.FirstPublishYear != 0 {
				fmt.Fprintf(w, " (%d)", item.FirstPublishYear)
			}
			fmt.Fprintln(w)
		}
	case EditionResult:
		fmt.Fprintf(w, "%s: %s\n", v.Source, v.Edition.Title)
		fmt.Fprintf(w, "- key: %s\n", v.Edition.Key)
		fmt.Fprintf(w, "- source: %s\n", v.SourceURL)
	case AuthorResult:
		fmt.Fprintf(w, "%s: %s\n", v.Source, v.Author.Name)
		for _, work := range v.Works {
			fmt.Fprintf(w, "- %s\n", work.Title)
		}
	case WorkResult:
		fmt.Fprintf(w, "%s: %s\n", v.Source, v.Work.Title)
		fmt.Fprintf(w, "- key: %s\n", v.Work.Key)
	case EditionsResult:
		fmt.Fprintf(w, "%s: %d editions\n", v.Source, len(v.Editions))
		for _, edition := range v.Editions {
			fmt.Fprintf(w, "- %s\n", edition.Title)
		}
	case SubjectResult:
		fmt.Fprintf(w, "%s: %s (%d works)\n", v.Source, v.Subject.Name, v.Subject.WorkCount)
		for _, work := range v.Works {
			fmt.Fprintf(w, "- %s\n", work.Title)
		}
	case SourcesResult:
		fmt.Fprintf(w, "Open Library sources\n")
		for _, endpoint := range v.Endpoints {
			fmt.Fprintf(w, "- %s: %s\n", endpoint.Name, endpoint.URL)
		}
	case DoctorResult:
		fmt.Fprintf(w, "open-library-pp-cli doctor\n")
		fmt.Fprintf(w, "  auth: none\n")
		fmt.Fprintf(w, "  identified requests: %t\n", v.Identified)
	default:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(value)
	}
	return nil
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "open-library-pp-cli %s\n", version)
		},
	}
}
