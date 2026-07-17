// Copyright 2026 waterpig and contributors. Licensed under Apache-2.0.
// Novel feature: winners at a circuit/event across seasons.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/motogp/internal/cliutil"

	"github.com/spf13/cobra"
)

// pp:data-source computed
func newNovelCircuitHistoryCmd(flags *rootFlags) *cobra.Command {
	var seasonsBack int
	cmd := &cobra.Command{
		Use:   "circuit-history <circuit-or-event> [class]",
		Short: "List race winners at a circuit across recent seasons.",
		Long: "Walks recent seasons, finds the event matching the given circuit/country/name, and\n" +
			"reports each year's race winner. Class defaults to MotoGP.",
		Example: strings.Trim(`
  motogp-pp-cli circuit-history mugello motogp --agent
  motogp-pp-cli circuit-history qatar moto2 --seasons 5`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 1 {
				return usageErr(fmt.Errorf("need a circuit or event name, e.g. circuit-history mugello motogp"))
			}
			query := args[0]
			class := "motogp"
			if len(args) >= 2 {
				class = args[1]
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			// Discover the available seasons (newest first).
			rawSeasons, err := novelFetch(ctx, c, flags, "auto", "seasons", true, "/results/seasons", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var seasons []mgpSeason
			if err := jsonDecode(rawSeasons, &seasons); err != nil {
				return classifyAPIError(err, flags)
			}
			sort.SliceStable(seasons, func(i, j int) bool { return seasons[i].Year > seasons[j].Year })

			back := seasonsBack
			if back <= 0 {
				back = 10
			}
			if cliutil.IsDogfoodEnv() && back > 2 {
				back = 2
			}
			if back < len(seasons) {
				seasons = seasons[:back]
			}

			type outRow struct {
				Year   int    `json:"year"`
				Event  string `json:"event"`
				Winner string `json:"winner"`
				Team   string `json:"team"`
			}
			var history []outRow
			matchedAny := false
			for _, s := range seasons {
				ev, err := resolveEvent(ctx, c, flags, s.ID, query)
				if err != nil {
					continue // circuit not on that season's calendar
				}
				matchedAny = true
				cat, err := resolveCategory(ctx, c, flags, s.ID, class)
				if err != nil {
					continue
				}
				sess, err := resolveSession(ctx, c, flags, ev.ID, cat.ID, "race")
				if err != nil {
					continue
				}
				rows, err := sessionClassification(ctx, c, flags, sess.ID)
				if err != nil {
					continue
				}
				for _, r := range rows {
					// Skip unfinished/future rounds: a winner has position 1 AND a name.
					if r.Position == 1 && r.Rider.fullName() != "" {
						history = append(history, outRow{Year: s.Year, Event: ev.label(), Winner: r.Rider.fullName(), Team: r.Team.Name})
						break
					}
				}
			}

			out := struct {
				Query   string   `json:"query"`
				Class   string   `json:"class"`
				Winners []outRow `json:"winners"`
			}{Query: query, Class: normClass(class), Winners: history}

			if !matchedAny {
				return notFoundErr(fmt.Errorf("no circuit or event matched %q in the last %d seasons", query, back))
			}
			if len(history) == 0 {
				// The circuit exists on the calendar but has no finished races yet.
				if flags.asJSON || flags.compact || flags.selectFields != "" || !isTerminal(cmd.OutOrStdout()) {
					return flags.printJSON(cmd, out)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s has no finished races in the last %d seasons yet\n", query, back)
				return nil
			}

			if flags.asJSON || flags.compact || flags.selectFields != "" || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, out)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s winners (%s)\n", history[0].Event, out.Class)
			tableRows := make([][]string, 0, len(history))
			for _, r := range history {
				tableRows = append(tableRows, []string{fmt.Sprintf("%d", r.Year), r.Winner, r.Team})
			}
			return flags.printTable(cmd, []string{"YEAR", "WINNER", "TEAM"}, tableRows)
		},
	}
	cmd.Flags().IntVar(&seasonsBack, "seasons", 10, "How many recent seasons to walk")
	return cmd
}
