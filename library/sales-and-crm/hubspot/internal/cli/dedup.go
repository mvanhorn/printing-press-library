// Copyright 2026 simplepathmedia. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// PATCH: Register a read-only local duplicate contact clustering command.
func newDedupCmd(flags *rootFlags) *cobra.Command {
	var flagKey string
	var flagThreshold string
	var flagLimit int
	cmd := &cobra.Command{
		Use:   "dedup",
		Short: "Find duplicate contact clusters in the local CRM mirror.",
		Long:  `Groups synced contacts by normalized email, phone, or domain and returns clusters with more than one contact.`,
		Example: `  hubspot-pp-cli dedup --agent
  hubspot-pp-cli dedup --key phone --limit 25 --agent`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			switch flagKey {
			case "email", "phone", "domain":
			default:
				return usageErr(fmt.Errorf("--key must be one of email, phone, domain"))
			}
			switch flagThreshold {
			case "strict", "loose":
			default:
				return usageErr(fmt.Errorf("--threshold must be one of strict, loose"))
			}
			db, err := openTranscendenceStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := computeDedup(cmd.Context(), db, flagKey, flagThreshold, flagLimit)
			if err != nil {
				return err
			}
			result := dedupResult{Rows: rows, Total: len(rows)}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return marshalAndPrint(cmd.OutOrStdout(), result, flags)
			}
			w := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(w, "key\tcount")
			for _, r := range rows {
				fmt.Fprintf(w, "%s\t%d\n", r.Key, r.Count)
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&flagKey, "key", "email", "Dedupe key: email, phone, or domain")
	cmd.Flags().StringVar(&flagThreshold, "threshold", "strict", "Match threshold: strict or loose")
	cmd.Flags().IntVar(&flagLimit, "limit", 100, "Max clusters to return")
	return cmd
}
