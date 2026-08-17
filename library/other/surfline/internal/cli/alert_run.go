// Copyright 2026 Shoffner and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel command: evaluate stored alert rules against a fresh
// forecast. Designed for cron — with --fail-on-match it exits 8 when any rule
// fires so `surfline alert run --fail-on-match || notify` works.
//
// pp:data-source live

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/surfline/internal/cliutil"

	"github.com/spf13/cobra"
)

type alertMatch struct {
	Name     string  `json:"name"`
	SpotID   string  `json:"spot_id"`
	SpotName string  `json:"spot_name"`
	Matched  bool    `json:"matched"`
	At       string  `json:"at,omitempty"`
	SurfMax  float64 `json:"surf_max,omitempty"`
	Period   float64 `json:"swell_period_s,omitempty"`
	WindKts  float64 `json:"wind_kts,omitempty"`
	WindType string  `json:"wind_type,omitempty"`
	Rating   string  `json:"rating,omitempty"`
	Reason   string  `json:"reason,omitempty"`
}

type alertRunView struct {
	Matched       []alertMatch `json:"matched"`
	Evaluated     int          `json:"evaluated"`
	FetchFailures []string     `json:"fetch_failures"`
}

// evaluateRule fetches the spot's forecast and returns the first point that
// satisfies every set threshold, or a non-matching result.
func evaluateRule(cmd *cobra.Command, flags *rootFlags, r alertRule, days int) (alertMatch, error) {
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	c, err := flags.newClient()
	if err != nil {
		return alertMatch{}, err
	}
	waves, err := fetchWave(ctx, c, r.SpotID, days, 1)
	if err != nil {
		return alertMatch{}, fmt.Errorf("%s: %w", r.Name, err)
	}
	winds, _ := fetchWind(ctx, c, r.SpotID, days, 1)
	ratings, _ := fetchRating(ctx, c, r.SpotID, days)
	name := fetchSpotName(ctx, c, r.SpotID)
	windIdx := windByTimestamp(winds)
	ratingIdx := ratingByTimestamp(ratings)

	m := alertMatch{Name: r.Name, SpotID: r.SpotID, SpotName: name}
	for _, wv := range waves {
		if r.MinSurf > 0 && wv.Surf.Max < r.MinSurf {
			continue
		}
		sw, hasSwell := wv.topSwell()
		if r.MinPeriod > 0 && (!hasSwell || sw.Period < r.MinPeriod) {
			continue
		}
		wd, hasWind := windIdx[wv.Timestamp]
		// Fail closed: if a max-wind threshold is set but wind data is missing,
		// don't fire — we can't confirm the wind is calm. (Mirrors OffshoreOnly.)
		if r.MaxWind > 0 && (!hasWind || wd.Speed > r.MaxWind) {
			continue
		}
		if r.OffshoreOnly && (!hasWind || wd.DirectionType != "Offshore") {
			continue
		}
		if r.MinRating > 0 {
			rt, hasRating := ratingIdx[wv.Timestamp]
			if !hasRating || ratingValue(rt) < r.MinRating {
				continue
			}
		}
		// All set thresholds satisfied at this point.
		m.Matched = true
		m.At = localTime(wv.Timestamp, wv.UTCOffset, "Mon 15:04")
		m.SurfMax = wv.Surf.Max
		if hasSwell {
			m.Period = sw.Period
		}
		if hasWind {
			m.WindKts = wd.Speed
			m.WindType = wd.DirectionType
		}
		if rt, ok := ratingIdx[wv.Timestamp]; ok {
			m.Rating = rt.Rating.Key
		}
		return m, nil
	}
	m.Reason = "no forecast point in the window met all thresholds"
	return m, nil
}

func newNovelAlertRunCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var days int
	var failOnMatch bool

	cmd := &cobra.Command{
		Use:   "run [name]",
		Short: "Evaluate stored alert rules against a fresh forecast; with --fail-on-match, exit 8 when any fires.",
		Long: "Fetches a fresh forecast for each stored rule and reports which fire.\n\n" +
			"Use this command to cron-evaluate condition rules. For an interactive one-time look use 'now'/'windows'; this is for unattended runs.",
		Example: strings.Trim(`
  surfline-pp-cli alert run
  surfline-pp-cli alert run dawn --fail-on-match || notify-send "Surf is on"`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,8"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would evaluate stored alert rules against a fresh forecast")
				return nil
			}
			resolved := dbPath
			if resolved == "" {
				resolved = defaultDBPath(surflineDBName)
			}
			if _, statErr := os.Stat(resolved); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no alerts yet; add one with: surfline-pp-cli alert add <name> --spot <id> ...\n")
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), `{"matched":[],"evaluated":0,"fetch_failures":[]}`)
				}
				return nil
			}
			if cliutil.IsDogfoodEnv() {
				days = 1
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := openSurflineStore(ctx, dbPath)
			if err != nil {
				return err
			}
			filter := ""
			if len(args) > 0 {
				filter = args[0]
			}
			rules, err := listAlertRules(ctx, db, filter)
			_ = db.Close()
			if err != nil {
				return fmt.Errorf("loading alerts: %w", err)
			}
			if len(rules) == 0 {
				if filter != "" {
					return notFoundErr(fmt.Errorf("no alert named %q", filter))
				}
				fmt.Fprintln(cmd.ErrOrStderr(), "no alerts stored")
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), `{"matched":[],"evaluated":0,"fetch_failures":[]}`)
				}
				return nil
			}

			view := alertRunView{Matched: []alertMatch{}, FetchFailures: []string{}}
			anyMatch := false
			for _, r := range rules {
				m, err := evaluateRule(cmd, flags, r, days)
				if err != nil {
					view.FetchFailures = append(view.FetchFailures, err.Error())
					continue
				}
				view.Evaluated++
				if m.Matched {
					anyMatch = true
				}
				view.Matched = append(view.Matched, m)
			}
			if len(view.FetchFailures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d rule(s) failed to fetch; evaluated %d\n", len(view.FetchFailures), view.Evaluated)
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
					return err
				}
			} else {
				tw := newTabWriter(cmd.OutOrStdout())
				fmt.Fprintln(tw, "ALERT\tSPOT\tFIRES\tWHEN\tSURF\tPERIOD\tWIND\tRATING")
				for _, m := range view.Matched {
					fires := "no"
					when := "-"
					if m.Matched {
						fires = green("YES")
						when = m.At
					}
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%.0fft\t%.0fs\t%.0fkt %s\t%s\n",
						m.Name, truncate(firstNonEmpty(m.SpotName, m.SpotID), 20), fires, when, m.SurfMax, m.Period, m.WindKts, m.WindType, m.Rating)
				}
				_ = tw.Flush()
			}

			if failOnMatch && anyMatch {
				return &cliError{code: 8, err: fmt.Errorf("%d alert(s) fired", countMatched(view.Matched))}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/surfline-pp-cli/data.db)")
	cmd.Flags().IntVar(&days, "days", 2, "Forecast horizon in days to evaluate")
	cmd.Flags().BoolVar(&failOnMatch, "fail-on-match", false, "Exit 8 when any alert fires (for cron `... || notify`)")
	return cmd
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func countMatched(ms []alertMatch) int {
	n := 0
	for _, m := range ms {
		if m.Matched {
			n++
		}
	}
	return n
}
