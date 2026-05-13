// Copyright 2026 simplepathmedia. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// PATCH: Register a read-only local contact trace command.
func newLeadTraceCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "lead-trace <contact-id>",
		Short:       "Trace one contact through deals, companies, and engagements.",
		Long:        `Reads one local contact plus associated deals, companies, engagement timeline, and source attribution into a composite object.`,
		Example:     `  hubspot-pp-cli lead-trace 12345 --agent`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) != 1 {
				return usageErr(fmt.Errorf("contact-id is required"))
			}
			db, err := openTranscendenceStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			result, err := computeLeadTrace(cmd.Context(), db, args[0])
			if err != nil {
				return err
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return marshalAndPrint(cmd.OutOrStdout(), result, flags)
			}
			w := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(w, "contactId\temail\tdeals\tcompanies\tengagements")
			fmt.Fprintf(w, "%s\t%v\t%d\t%d\t%d\n", result.ContactID, result.Contact["email"], len(result.Deals), len(result.Companies), len(result.Engagements))
			return w.Flush()
		},
	}
	return cmd
}
