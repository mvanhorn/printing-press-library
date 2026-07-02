// Copyright 2026 megumikuo and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: everything TPG published within a recent time window.
package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/thepointsguy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/thepointsguy/internal/tpg"
)

// pp:data-source auto
func newNovelSinceCmd(flags *rootFlags) *cobra.Command {
	var category string

	cmd := &cobra.Command{
		Use:   "since <duration>",
		Short: "List everything The Points Guy published within a recent time window",
		Long: strings.TrimSpace(`
Show articles published within a recent window across all categories. The
duration accepts hours/days/weeks shorthand, e.g. 24h, 3d, 1w. Filter to one
category with --category. For a fixed count of newest items, use 'latest'.`),
		Example: strings.Trim(`
  thepointsguy-pp-cli since 24h
  thepointsguy-pp-cli since 3d --category deals --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list recent articles")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a duration is required, e.g. 24h, 3d, 1w"))
			}
			window, err := cliutil.ParseDurationLoose(args[0])
			if err != nil || window <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid duration %q (try 24h, 3d, 1w)", args[0]))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			items, err := fetchFeed(cmd, flags, ctx)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			cutoff := time.Now().Add(-window)
			var out []tpg.FeedItem
			undated := 0
			for _, it := range items {
				if category != "" && !strings.EqualFold(it.Category, category) &&
					!strings.Contains(strings.ToLower(it.Link), "/"+strings.ToLower(category)+"/") {
					continue
				}
				if it.Published.IsZero() {
					undated++
					continue
				}
				if it.Published.After(cutoff) {
					out = append(out, it)
				}
			}
			if undated > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "note: %d item(s) had no parseable date and were excluded from the window\n", undated)
			}
			if flags.asJSON || flags.agent {
				return emitJSON(cmd, flags, struct {
					Window string         `json:"window"`
					Since  string         `json:"since"`
					Count  int            `json:"count"`
					Items  []tpg.FeedItem `json:"items"`
				}{args[0], cutoff.Format(time.RFC3339), len(out), out})
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "articles published in the last %s:\n", args[0])
			return emitFeed(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&category, "category", "", "Filter by category, e.g. news, deals, credit-cards")
	return cmd
}
