// Copyright 2026 waterpig and contributors. Licensed under Apache-2.0.
// Novel feature: "what did I miss" — finished events for a season.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// pp:data-source computed
func newNovelSinceCmd(flags *rootFlags) *cobra.Command {
	var withWinners bool
	var class string
	cmd := &cobra.Command{
		Use:   "since <year>",
		Short: "Show finished events for a season (catch up on what you missed).",
		Long: "Lists the finished rounds of a season in order, so you can quickly see how far the\n" +
			"calendar has progressed. Add --winners to also fetch each round's race winner.",
		Example: strings.Trim(`
  motogp-pp-cli since 2026 --agent
  motogp-pp-cli since 2026 --winners --class motogp`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 1 {
				return usageErr(fmt.Errorf("need <year>, e.g. since 2026"))
			}
			year, err := parseYearArg(args[0])
			if err != nil {
				return usageErr(err)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			season, err := resolveSeason(ctx, c, flags, year)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			events, err := seasonEvents(ctx, c, flags, season.ID, true)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			sortEventsByStart(events)

			var cat mgpCategory
			if withWinners {
				cat, err = resolveCategory(ctx, c, flags, season.ID, class)
				if err != nil {
					return classifyAPIError(err, flags)
				}
			}

			type outRow struct {
				Round   int    `json:"round"`
				Event   string `json:"event"`
				Circuit string `json:"circuit"`
				Country string `json:"country"`
				Date    string `json:"date"`
				Winner  string `json:"winner,omitempty"`
			}
			out := struct {
				Year     int      `json:"year"`
				Finished int      `json:"finished"`
				Events   []outRow `json:"events"`
			}{Year: year, Finished: len(events)}

			for i, ev := range events {
				row := outRow{
					Round:   i + 1,
					Event:   ev.label(),
					Circuit: ev.Circuit.Name,
					Country: ev.Country.Name,
					Date:    ev.DateStart,
				}
				if withWinners {
					// A fetch failure must not silently become a blank winner:
					// that is indistinguishable from a round that legitimately
					// has no recorded winner. Surface resolve/classification
					// errors; leave Winner blank only when the classification
					// was fetched successfully but has no P1 finisher.
					sess, err := resolveSession(ctx, c, flags, ev.ID, cat.ID, "race")
					if err != nil {
						return classifyAPIError(fmt.Errorf("fetching winner for %s: %w", ev.label(), err), flags)
					}
					rows, err := sessionClassification(ctx, c, flags, sess.ID)
					if err != nil {
						return classifyAPIError(fmt.Errorf("fetching winner for %s: %w", ev.label(), err), flags)
					}
					for _, r := range rows {
						if r.Position == 1 {
							row.Winner = r.Rider.fullName()
							break
						}
					}
				}
				out.Events = append(out.Events, row)
			}

			if flags.asJSON || flags.compact || flags.selectFields != "" || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, out)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d season: %d finished rounds\n", out.Year, out.Finished)
			headers := []string{"RND", "EVENT", "CIRCUIT", "DATE"}
			if withWinners {
				headers = append(headers, "WINNER")
			}
			tableRows := make([][]string, 0, len(out.Events))
			for _, r := range out.Events {
				row := []string{fmt.Sprintf("%d", r.Round), r.Event, r.Circuit, r.Date}
				if withWinners {
					row = append(row, r.Winner)
				}
				tableRows = append(tableRows, row)
			}
			return flags.printTable(cmd, headers, tableRows)
		},
	}
	cmd.Flags().BoolVar(&withWinners, "winners", false, "Also fetch each round's race winner (extra API calls)")
	cmd.Flags().StringVar(&class, "class", "motogp", "Class for --winners (motogp, moto2, moto3)")
	return cmd
}
