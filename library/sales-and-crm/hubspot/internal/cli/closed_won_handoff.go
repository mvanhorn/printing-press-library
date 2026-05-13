// Copyright 2026 simplepathmedia. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// PATCH: Register a read-only local closed-won handoff export command.
func newClosedWonHandoffCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var flagFormat string
	var flagLimit int
	cmd := &cobra.Command{
		Use:   "closed-won-handoff",
		Short: "Export contacts attached to recently closed-won deals.",
		Long:  `Reads local closed-won deals and emits full contact property bundles, optionally shaped for ClickUp import.`,
		Example: `  hubspot-pp-cli closed-won-handoff --agent
  hubspot-pp-cli closed-won-handoff --since 168h --format json --agent`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			dur, err := parseLookbackDuration(flagSince)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --since: %w", err))
			}
			if flagFormat != "json" && flagFormat != "clickup" {
				return usageErr(fmt.Errorf("--format must be json or clickup"))
			}
			db, err := openTranscendenceStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := computeClosedWonHandoff(cmd.Context(), db, time.Now().Add(-dur), flagFormat, flagLimit)
			if err != nil {
				return err
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return marshalAndPrint(cmd.OutOrStdout(), map[string]any{"rows": rows, "total": len(rows)}, flags)
			}
			w := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(w, "name\temail\tdeal")
			for _, row := range rows {
				m, ok := row.(map[string]any)
				if !ok {
					continue
				}
				fmt.Fprintf(w, "%s\t%s\t%v\n", asString(m["name"]), asString(m["email"]), m["_deal"])
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "7d", "Lookback duration such as 7d, 168h, or 30m")
	cmd.Flags().StringVar(&flagFormat, "format", "clickup", "Output format: json or clickup")
	cmd.Flags().IntVar(&flagLimit, "limit", 200, "Max rows to return")
	return cmd
}
