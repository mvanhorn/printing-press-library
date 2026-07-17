// Copyright 2026 waterpig and contributors. Licensed under Apache-2.0.
// Novel feature: race results resolved from human names instead of UUIDs.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelResultsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "results <year> <event> [class] [session]",
		Short: "Query races by year, class name, event name, and session type instead of chained UUIDs.",
		Long: "Resolve a session's classification from human inputs. Class defaults to MotoGP and\n" +
			"session defaults to the race (RAC). Session tokens: race, sprint, q, q1, q2, fp1, fp2, warmup.",
		Example: strings.Trim(`
  motogp-pp-cli results 2024 qatar motogp race --agent
  motogp-pp-cli results 2024 mugello moto2 sprint
  motogp-pp-cli results 2024 assen`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("need <year> and <event>, e.g. results 2024 qatar motogp race"))
			}
			year, err := parseYearArg(args[0])
			if err != nil {
				return usageErr(err)
			}
			eventQuery := args[1]
			class := "motogp"
			if len(args) >= 3 {
				class = args[2]
			}
			session := "race"
			if len(args) >= 4 {
				session = args[3]
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
			cat, err := resolveCategory(ctx, c, flags, season.ID, class)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			event, err := resolveEvent(ctx, c, flags, season.ID, eventQuery)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			sess, err := resolveSession(ctx, c, flags, event.ID, cat.ID, session)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			rows, err := sessionClassification(ctx, c, flags, sess.ID)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			type outRow struct {
				Position    int    `json:"position"`
				Rider       string `json:"rider"`
				Number      int    `json:"number"`
				Team        string `json:"team"`
				Constructor string `json:"constructor"`
				Points      int    `json:"points"`
				Status      string `json:"status"`
			}
			out := struct {
				Year      int      `json:"year"`
				Event     string   `json:"event"`
				Circuit   string   `json:"circuit"`
				Class     string   `json:"class"`
				Session   string   `json:"session"`
				Finishers []outRow `json:"finishers"`
			}{
				Year:    year,
				Event:   event.label(),
				Circuit: event.Circuit.Name,
				Class:   strings.ReplaceAll(cat.Name, "™", ""),
				Session: sessionLabel(sess),
			}
			for _, r := range rows {
				out.Finishers = append(out.Finishers, outRow{
					Position:    r.Position,
					Rider:       r.Rider.fullName(),
					Number:      r.Rider.Number,
					Team:        r.Team.Name,
					Constructor: r.Constructor.Name,
					Points:      r.Points,
					Status:      r.Status,
				})
			}

			if flags.asJSON || flags.compact || flags.selectFields != "" || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, out)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d %s — %s %s\n", out.Year, out.Event, out.Class, out.Session)
			rowsOut := make([][]string, 0, len(out.Finishers))
			for _, r := range out.Finishers {
				rowsOut = append(rowsOut, []string{
					fmt.Sprintf("%d", r.Position),
					r.Rider,
					r.Team,
					fmt.Sprintf("%d", r.Points),
					r.Status,
				})
			}
			return flags.printTable(cmd, []string{"POS", "RIDER", "TEAM", "PTS", "STATUS"}, rowsOut)
		},
	}
	return cmd
}

func sessionLabel(s mgpSession) string {
	if s.Number > 0 {
		return fmt.Sprintf("%s%d", s.Type, s.Number)
	}
	return s.Type
}
