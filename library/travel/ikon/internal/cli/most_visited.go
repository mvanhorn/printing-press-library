// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// most-visited: rank the resorts you ski most across every Ikon season on your
// account. Enumerates your passes (/my-products), fetches each pass's redemption
// history (/my-products/{id}/pass-usage), and joins them by resort. Hand-authored.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

func newNovelMostVisitedCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "most-visited",
		Short: "Rank the resorts you ski most across every season you've held an Ikon Pass, with per-season day counts.",
		Long: "Rank the resorts you ski most across every Ikon season on your account.\n\n" +
			"Each season is a separate pass; this command enumerates every pass on your\n" +
			"account, pulls its redemption history, and joins them by resort so you get\n" +
			"lifetime day counts plus a per-season breakdown. Requires a logged-in session\n" +
			"('ikon auth login --chrome').",
		Example:     "  ikon-pp-cli most-visited --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would aggregate pass-usage across every pass on your account")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			raw, err := c.Get(ctx, "/api/v2/my-products", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			data, err := ikonData(raw)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var products []struct {
				ID int `json:"id"`
			}
			if err := json.Unmarshal(data, &products); err != nil {
				return fmt.Errorf("parsing your passes: %w", err)
			}

			perProduct := make([][]seasonUsage, 0, len(products))
			failures := make([]fetchFailure, 0)
			for _, p := range products {
				path := replacePathParam("/api/v2/my-products/{id}/pass-usage", "id", strconv.Itoa(p.ID))
				uraw, err := c.Get(ctx, path, nil)
				if err != nil {
					failures = append(failures, fetchFailure{ProductID: strconv.Itoa(p.ID), Error: err.Error()})
					continue
				}
				udata, err := ikonData(uraw)
				if err != nil {
					failures = append(failures, fetchFailure{ProductID: strconv.Itoa(p.ID), Error: err.Error()})
					continue
				}
				var seasons []seasonUsage
				if err := json.Unmarshal(udata, &seasons); err != nil {
					failures = append(failures, fetchFailure{ProductID: strconv.Itoa(p.ID), Error: err.Error()})
					continue
				}
				perProduct = append(perProduct, seasons)
			}

			view := aggregateMostVisited(perProduct, len(products), failures)
			if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"warning: %d of %d pass-usage fetches failed; totals computed over the rest\n",
					len(failures), len(products))
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	return cmd
}
