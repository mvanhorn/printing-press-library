// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/rechtspraak/internal/rechtspraak"
)

func newNovelEcliParseCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "parse <ecli>",
		Short: "Destructure an ECLI into country, court, year, and sequence",
		Long: `Parse an ECLI string into its component parts entirely offline. Accepts
NL ECLIs (ECLI:NL:HR:2024:1), EU ECLIs (ECLI:EU:C:2002:118), and ECHR
ECLIs (ECLI:CE:ECHR:2003:0204JUD005090199). Also recognises a bare LJN
code (e.g. AA1005 or LJN:AA1005) and tags it as legacy-ljn for routing
to the LJN resolver.`,
		Example: `  rechtspraak-pp-cli ecli parse ECLI:NL:HR:2024:1
  rechtspraak-pp-cli ecli parse ECLI:EU:C:2002:118 --json
  rechtspraak-pp-cli ecli parse AA1005`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			parsed, err := rechtspraak.ParseECLI(args[0])
			if err != nil {
				return err
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(parsed)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "ECLI:      %s\n", parsed.Raw)
			fmt.Fprintf(cmd.OutOrStdout(), "Country:   %s\n", orDash(parsed.Country))
			fmt.Fprintf(cmd.OutOrStdout(), "Court:     %s\n", orDash(parsed.Court))
			fmt.Fprintf(cmd.OutOrStdout(), "Year:      %s\n", orDash(parsed.Year))
			fmt.Fprintf(cmd.OutOrStdout(), "Sequence:  %s\n", orDash(parsed.Sequence))
			fmt.Fprintf(cmd.OutOrStdout(), "Variant:   %s\n", orDash(parsed.Variant))
			if parsed.URL != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "URL:       %s\n", parsed.URL)
			}
			return nil
		},
	}
	return cmd
}

func newNovelEcliURLCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "url <ecli>",
		Short: "Emit the canonical deeplink for an ECLI",
		Long: `Print the canonical deeplink URL for an ECLI. For NL ECLIs this is the
uitspraken.rechtspraak.nl details page; for EU ECLIs it's the CJEU
juris page; for CE/ECHR ECLIs it's the HUDOC search page.`,
		Example:     `  rechtspraak-pp-cli ecli url ECLI:NL:HR:2024:1`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			parsed, err := rechtspraak.ParseECLI(args[0])
			if err != nil {
				return err
			}
			if parsed.URL == "" {
				return fmt.Errorf("no canonical deeplink for variant %q", parsed.Variant)
			}
			fmt.Fprintln(cmd.OutOrStdout(), strings.TrimSpace(parsed.URL))
			return nil
		},
	}
	return cmd
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
