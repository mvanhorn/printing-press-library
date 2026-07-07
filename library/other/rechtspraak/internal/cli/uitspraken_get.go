// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/rechtspraak/internal/rechtspraak"
)

func newUitsprakenGetCmd(flags *rootFlags) *cobra.Command {
	var flagId string
	var flagMeta bool
	var flagSummaryOnly bool

	cmd := &cobra.Command{
		Use:   "get",
		Short: "Fetch a single decision's full RDF metadata, summary, and body",
		Long: `Fetch /uitspraken/content for the given ECLI and parse the RDF into typed
metadata, the inhoudsindicatie (summary), and the uitspraak/conclusie body
as plain text. With --meta-only the body is skipped (saves bandwidth on
large decisions); with --summary-only only the inhoudsindicatie is printed
in human mode.`,
		Example: `  rechtspraak-pp-cli uitspraken get --id ECLI:NL:HR:2024:1
  rechtspraak-pp-cli uitspraken get --id ECLI:NL:HR:2024:1 --summary-only
  rechtspraak-pp-cli uitspraken get --id ECLI:NL:HR:2024:1 --meta-only --json`,
		Annotations: map[string]string{"pp:endpoint": "uitspraken.get", "pp:method": "GET", "pp:path": "/uitspraken/content", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagId == "" && len(args) > 0 {
				flagId = args[0]
			}
			if flagId == "" && !flags.dryRun {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if _, err := rechtspraak.ParseECLI(flagId); err != nil {
				return err
			}
			d, err := mustHTTP().Get(cmd.Context(), flagId, flagMeta)
			if err != nil {
				return err
			}
			if shouldEmitJSON(cmd.OutOrStdout(), flags) {
				return writeJSONOut(cmd.OutOrStdout(), d)
			}
			if flagSummaryOnly {
				fmt.Fprintln(cmd.OutOrStdout(), d.Summary)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "ECLI:         %s\n", d.ECLI)
			fmt.Fprintf(cmd.OutOrStdout(), "Court:        %s\n", d.Court)
			fmt.Fprintf(cmd.OutOrStdout(), "Decision date: %s\n", d.DecisionDate)
			fmt.Fprintf(cmd.OutOrStdout(), "Publication:   %s\n", d.PublicationDate)
			fmt.Fprintf(cmd.OutOrStdout(), "Type:          %s\n", d.Type)
			fmt.Fprintf(cmd.OutOrStdout(), "Procedure:     %s\n", d.Procedure)
			fmt.Fprintf(cmd.OutOrStdout(), "Subject:       %s\n", d.Subject)
			if len(d.Zaaknummer) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Zaaknummer:    %v\n", d.Zaaknummer)
			}
			if len(d.Contributors) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Judges:        %v\n", d.Contributors)
			}
			if d.Alternative != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Landmark:      %s\n", d.Alternative)
			}
			if d.Summary != "" {
				fmt.Fprintln(cmd.OutOrStdout(), "\nInhoudsindicatie:")
				fmt.Fprintln(cmd.OutOrStdout(), d.Summary)
			}
			if d.Body != "" && !flagMeta {
				fmt.Fprintln(cmd.OutOrStdout(), "\nUitspraak:")
				fmt.Fprintln(cmd.OutOrStdout(), d.Body)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagId, "id", "", "The ECLI to fetch, e.g. ECLI:NL:HR:2024:1")
	cmd.Flags().BoolVar(&flagMeta, "meta-only", false, "Skip the document body (return=META) to save bandwidth on large decisions")
	cmd.Flags().BoolVar(&flagSummaryOnly, "summary-only", false, "Print only the inhoudsindicatie summary (human output)")
	return cmd
}
