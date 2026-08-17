// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source auto
package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/isitagentready/internal/store"
)

func newCheckCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check <url>",
		Short: "Scan a site for AI-agent readiness and save the result",
		Long: "Scan a URL for AI-agent readiness, store the result in local history, and print the\n" +
			"readiness level plus a per-category pass/fail summary.\n\n" +
			"This is the primary command: run it first, then use 'advice', 'report', 'history',\n" +
			"or 'diff' to work with the stored scan. With --data-source local it prints the most\n" +
			"recent stored scan instead of running a new one.",
		Example: "  isitagentready-pp-cli check https://example.com\n" +
			"  isitagentready-pp-cli check https://example.com --json --select level,levelName",
		// The scan API returns HTTP 200 + a siteError block for any unreachable
		// URL, so a "bad" URL is a valid result (exit 0), not a usage error;
		// skip the dogfood error-path probe rather than invent error semantics.
		// Not mcp:read-only: a successful scan appends a record to the local
		// scan store (persistScan), which changes what history/diff/open-advice
		// return on a later call — an observable side effect agents should see.
		Annotations: map[string]string{"pp:no-error-path-probe": "true"},
		Args:        cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would scan a URL for agent readiness")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a URL argument is required, e.g. check https://example.com"))
			}
			url := args[0]

			var raw json.RawMessage
			if flags.dataSource == "local" {
				recs, err := loadStore()
				if err != nil {
					return err
				}
				rec, ok := store.Latest(recs, url)
				if !ok {
					return notFoundErr(fmt.Errorf("no stored scan for %s; run a live scan first (omit --data-source local)", url))
				}
				raw = rec.Raw
			} else {
				ctx, cancel := boundCtx(cmd.Context(), flags)
				defer cancel()
				data, err := performScan(ctx, flags, url)
				if err != nil {
					return err
				}
				raw = data
				persistScan(raw)
			}
			return renderScan(cmd, flags, raw)
		},
	}
	return cmd
}
