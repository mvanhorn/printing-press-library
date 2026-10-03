// Copyright 2026 Jet Sng and contributors. Licensed under Apache-2.0.
// pp:data-source auto
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/wheelog/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/wheelog/internal/store"
	"github.com/mvanhorn/printing-press-library/library/travel/wheelog/internal/wheelog"
	"github.com/spf13/cobra"
	"strconv"
	"strings"
)

func newNovelSpotsCompareCmd(flags *rootFlags) *cobra.Command {
	var ids string
	var options wheelogReadOptions
	cmd := &cobra.Command{Use: "compare [spot-id...]", Short: "Compare exact source question evidence across a bounded candidate set.",
		Long:        "Compare chosen public spot IDs against source question IDs. Inapplicable categories, unanswered questions, conflicting counts and unavailable details remain distinct. For keyword discovery use spots search; for saved rechecks use shortlist list --audit.",
		Example:     "  wheelog-pp-cli spots compare 166345 166344 --require-question 102 --agent\n  wheelog-pp-cli spots compare --ids 166345,166344 --require-question 102,103 --data-source local --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "first-id=166345;second-id=166344;--require-question=102;--data-source=live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "compare recorded WheeLog question evidence")
			}
			if len(args) > 0 && cmd.Flags().Changed("ids") {
				return usageErr(fmt.Errorf("provide positional IDs or --ids, not both"))
			}
			if ids != "" {
				args = strings.Split(ids, ",")
			}
			if len(args) < 1 || len(args) > 5 {
				return usageErr(fmt.Errorf("compare requires 1..5 spot IDs"))
			}
			parsed := make([]int64, 0, len(args))
			seen := map[int64]bool{}
			for _, value := range args {
				id, err := wheelogID(strings.TrimSpace(value))
				if err != nil {
					return err
				}
				if seen[id] {
					return usageErr(fmt.Errorf("duplicate spot ID %d", id))
				}
				seen[id] = true
				parsed = append(parsed, id)
			}
			maxAge, err := validateWheelogOptions(options)
			if err != nil {
				return err
			}
			mode, err := wheelogMode(flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			var cached []store.WheelogObservation
			var c *client.Client
			if mode == "local" {
				cached, err = savedWheelog(ctx, options.DB)
				if err != nil {
					return err
				}
				if mode == "local" {
					wheelogLocalHint(cmd, cached, maxAge)
				}
			}
			if mode != "local" {
				c, err = newWheelogClient(flags)
				if err != nil {
					return err
				}
			}
			rows := make([]wheelogSpotView, 0, len(parsed))
			failures := make([]wheelogFailure, 0)
			live, local, available := 0, 0, 0
			for _, id := range parsed {
				spot, err := resolveWheelogSpot(ctx, flags, options, id, c, cached)
				if err != nil {
					if isWheelogThrottle(err) {
						return wheelogError(err)
					}
					failures = append(failures, wheelogFailure{id, err.Error()})
					spot = wheelog.Spot{ID: id, SourceURL: "https://app.wheelog.com/?spotId=" + strconv.FormatInt(id, 10) + "&la=ja", DetailStatus: "unavailable", Questions: []wheelog.Question{}, Gaps: []string{}}
				} else {
					available++
					if spot.Source == "live" {
						live++
					} else {
						local++
					}
				}
				rows = append(rows, viewWheelog(spot, options.Questions, maxAge))
			}
			if available == 0 {
				return apiErr(fmt.Errorf("none of the %d requested spots has an available observation: %s", len(parsed), failures[0].Error))
			}
			if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d detail fetches failed; comparison uses %d available observations\n", len(failures), len(parsed), available)
			}
			source := "auto"
			if local == 0 {
				source = "live"
			} else if live == 0 {
				source = "local"
			}
			return emitWheelog(cmd, flags, map[string]any{"results": rows, "fetch_failures": failures, "coverage": map[string]any{"requested": len(parsed), "available": available, "unavailable": len(failures), "live": live, "local": local}, "note": wheelogEvidenceNote}, source)
		}}
	cmd.Flags().StringVar(&ids, "ids", "", "Comma-separated source spot IDs, as an alternative to positional IDs.")
	addWheelogReadFlags(cmd, &options)
	return cmd
}
