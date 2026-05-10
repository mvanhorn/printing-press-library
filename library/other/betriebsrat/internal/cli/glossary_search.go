// Copyright 2026 dawid-piaskowski. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newGlossarySearchCmd(flags *rootFlags) *cobra.Command {
	var flagTerm string

	cmd := &cobra.Command{
		Use:         "search",
		Short:       "Search for a specific legal term",
		Example:     "  betriebsrat glossary search --term Mitbestimmung",
		Annotations: map[string]string{"pp:endpoint": "glossary.search", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("term") && !flags.dryRun {
				return fmt.Errorf("required flag \"%s\" not set", "term")
			}
			if dryRunOK(flags) {
				return nil
			}

			needle := strings.ToLower(strings.TrimSpace(flagTerm))
			var matches []glossaryTerm
			for _, t := range embeddedGlossary {
				if strings.Contains(strings.ToLower(t.Term), needle) ||
					strings.Contains(strings.ToLower(t.Definition), needle) {
					matches = append(matches, t)
				}
			}

			if len(matches) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Kein Begriff gefunden für: %s\n", flagTerm)
				return nil
			}

			if flags.asJSON || flags.agent {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(matches)
			}

			for _, t := range matches {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n  %s\n\n", t.Term, t.Definition)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagTerm, "term", "", "Legal term to look up (e.g. Mitbestimmung, Betriebsvereinbarung, Sozialplan)")

	return cmd
}
