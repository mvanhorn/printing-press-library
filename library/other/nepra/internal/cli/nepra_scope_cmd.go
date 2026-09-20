// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// nepraPrintScope emits the declared scope limits.
func nepraPrintScope(cmd *cobra.Command, flags *rootFlags) error {
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		for _, l := range nepraScopeLimits {
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n  verdict: %s\n  %s\n", l.Subject, l.Verdict, l.Reason)
			if l.Instead != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  instead: %s\n", l.Instead)
			}
			fmt.Fprintln(cmd.OutOrStdout())
		}
		return nil
	}
	payload, err := json.Marshal(nepraScopeLimits)
	if err != nil {
		return err
	}
	wrapped, err := wrapPlatformStructuredOutput(payload, flags, "results", true)
	if err != nil {
		return err
	}
	return printOutput(cmd.OutOrStdout(), wrapped, true)
}
