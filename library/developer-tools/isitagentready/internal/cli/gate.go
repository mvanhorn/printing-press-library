// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source auto
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/isitagentready/internal/store"
)

func newNovelGateCmd(flags *rootFlags) *cobra.Command {
	var minLevel int
	var noRegress bool
	var strict bool

	cmd := &cobra.Command{
		Use:   "gate <url>",
		Short: "Fail a build when a site drops below a target readiness level or a check regresses",
		Long: "Scan a site and exit non-zero when its readiness level is below --min-level or (with\n" +
			"--no-regress) when any check that passed in the previous scan now fails. A target\n" +
			"siteError (the scanner could not fetch the site) is reported but does NOT fail the gate\n" +
			"unless --strict, so a transient target outage does not flap CI. Pair with --agent for a\n" +
			"machine-readable result on stdout; the exit code is the gate signal.\n\n" +
			"Exit codes: 0 = gate passed, 1 = gate failed.",
		Example: "  isitagentready-pp-cli gate https://example.com --min-level 3\n" +
			"  isitagentready-pp-cli gate https://example.com --min-level 3 --no-regress --agent",
		Annotations: map[string]string{
			"pp:typed-exit-codes": "0,1",
			// An unreachable URL scans to a 200 siteError (gate passes without
			// --strict), so there is no bad-input error path to probe.
			"pp:no-error-path-probe": "true",
			// Not mcp:read-only: gate scans the site and appends the result to
			// the local scan store (persistScan), an observable side effect
			// (see check.go).
		},
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would scan a URL and gate on its readiness level")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a URL argument is required, e.g. gate https://example.com --min-level 3"))
			}
			url := args[0]

			recs, err := loadStore()
			if err != nil {
				return err
			}

			var current, prev *store.Report
			if flags.dataSource == "local" {
				hist := store.HistoryFor(recs, url)
				if len(hist) == 0 {
					return notFoundErr(fmt.Errorf("no stored scan for %s; run 'isitagentready-pp-cli check %s' or omit --data-source local", url, url))
				}
				if current, err = store.ParseReport(hist[len(hist)-1].Raw); err != nil {
					return err
				}
				if len(hist) >= 2 {
					prev, _ = store.ParseReport(hist[len(hist)-2].Raw)
				}
			} else {
				if rec, ok := store.Latest(recs, url); ok {
					prev, _ = store.ParseReport(rec.Raw)
				}
				ctx, cancel := boundCtx(cmd.Context(), flags)
				defer cancel()
				raw, err := performScan(ctx, flags, url)
				if err != nil {
					return err
				}
				persistScan(raw)
				if current, err = store.ParseReport(raw); err != nil {
					return err
				}
			}

			res := store.EvaluateGate(current, prev, minLevel, noRegress, strict)

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				renderGateHuman(cmd, res)
			} else if err := printJSONFiltered(cmd.OutOrStdout(), res, flags); err != nil {
				return err
			}

			if !res.Pass {
				// Exit 1 for CI; the structured reason is already on stdout.
				return &cliError{code: 1, err: fmt.Errorf("gate failed: %s", strings.Join(res.Reasons, "; "))}
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&minLevel, "min-level", 0, "Fail if the readiness level is below this (0-5; 0 disables the level check)")
	cmd.Flags().BoolVar(&noRegress, "no-regress", false, "Fail if any check that passed in the previous scan now fails")
	cmd.Flags().BoolVar(&strict, "strict", false, "Also fail when the target site itself is unreachable (siteError)")
	return cmd
}

func renderGateHuman(cmd *cobra.Command, res store.GateResult) {
	out := cmd.OutOrStdout()
	verdict := green("PASS")
	if !res.Pass {
		verdict = red("FAIL")
	}
	fmt.Fprintf(out, "%s  %s  (level %d — %s", verdict, res.URL, res.Level, res.LevelName)
	if res.MinLevel > 0 {
		fmt.Fprintf(out, ", min %d", res.MinLevel)
	}
	fmt.Fprintln(out, ")")
	for _, r := range res.Reasons {
		fmt.Fprintf(out, "  - %s\n", r)
	}
}
