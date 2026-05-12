package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// ReleaseRow is one row in the `releases next` output.
type ReleaseRow struct {
	Date   string `json:"date"`
	Time   string `json:"time"`
	Survey string `json:"survey"`
	Title  string `json:"title"`
	Period string `json:"period,omitempty"`
	URL    string `json:"url"`
}

func newReleasesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "releases",
		Short: "Query the upcoming BLS release calendar (CPI, Employment Situation, JOLTS, PPI, ECI, productivity).",
	}
	cmd.AddCommand(newReleasesNextCmd(flags))
	return cmd
}

func newReleasesNextCmd(flags *rootFlags) *cobra.Command {
	var surveyFilter, withinDur string
	var watch bool
	var limit int
	cmd := &cobra.Command{
		Use:   "next",
		Short: "List upcoming BLS releases with date, time, and news-release URL.",
		Long: `Reads the locally-curated 2026 BLS release calendar. BLS publishes the
calendar only as HTML behind Akamai bot protection, so the local table is
the only programmatic surface for "what's printing next." Filter by
survey or window with --survey/--within; pass --watch to poll until the
next release window opens (useful in CI / scheduled jobs).`,
		Example: `  bls-pp-cli releases next
  bls-pp-cli releases next --survey CPI --within 14d
  bls-pp-cli releases next --survey CE --watch`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openBLSStore(cmd.Context(), "")
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer func() { _ = db.Close() }()

			window := time.Duration(0)
			if withinDur != "" {
				window, err = parseDurationLoose(withinDur)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --within value %q: %w", withinDur, err))
				}
			}

			rows, err := queryReleases(cmd.Context(), db.DB(), surveyFilter, window, limit)
			if err != nil {
				return err
			}

			if watch && len(rows) > 0 {
				first := rows[0]
				if t, perr := time.Parse(time.RFC3339, first.Date); perr == nil {
					now := time.Now()
					if t.After(now) {
						fmt.Fprintf(cmd.OutOrStdout(), "watching %s at %s (%s) — sleeping %s\n", first.Title, first.Date, first.Time, t.Sub(now).Round(time.Minute))
					}
				}
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				raw, _ := json.Marshal(rows)
				return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				m := make([]map[string]any, 0, len(rows))
				for _, r := range rows {
					m = append(m, map[string]any{
						"date":   r.Date,
						"time":   r.Time,
						"survey": r.Survey,
						"title":  r.Title,
						"period": r.Period,
						"url":    r.URL,
					})
				}
				if len(m) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "(no upcoming releases match)")
					return nil
				}
				return printAutoTable(cmd.OutOrStdout(), m)
			}
			raw, _ := json.Marshal(rows)
			return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
		},
	}
	cmd.Flags().StringVar(&surveyFilter, "survey", "", "Filter by survey: CPI, CE (employment), JT (JOLTS), WP (PPI), CI (ECI), PR (productivity), or two-letter abbr.")
	cmd.Flags().StringVar(&withinDur, "within", "", "Only include releases within this duration from now (e.g. 24h, 14d, 30d).")
	cmd.Flags().BoolVar(&watch, "watch", false, "Poll until the next release window opens (no-op under PRINTING_PRESS_VERIFY).")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of releases to return.")
	return cmd
}

// queryReleases pulls upcoming events from bls_releases, applying optional
// survey and time-window filters. The survey filter accepts both the
// canonical abbreviation (CU, CE, etc.) and friendly aliases ("CPI",
// "employment", "JOLTS", "PPI", "ECI", "productivity").
func queryReleases(ctx context.Context, db *sql.DB, surveyFilter string, within time.Duration, limit int) ([]ReleaseRow, error) {
	now := time.Now().UTC()
	abbr := normalizeSurveyAbbr(surveyFilter)
	q := `SELECT release_date, release_time, survey, title, COALESCE(period,''), COALESCE(url,'')
		FROM bls_releases
		WHERE release_date >= ?`
	args := []any{now}
	if abbr != "" {
		q += " AND UPPER(survey) = UPPER(?)"
		args = append(args, abbr)
	}
	if within > 0 {
		q += " AND release_date <= ?"
		args = append(args, now.Add(within))
	}
	q += " ORDER BY release_date ASC LIMIT ?"
	args = append(args, limit)
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query releases: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []ReleaseRow
	for rows.Next() {
		var date time.Time
		var r ReleaseRow
		if err := rows.Scan(&date, &r.Time, &r.Survey, &r.Title, &r.Period, &r.URL); err != nil {
			return nil, err
		}
		r.Date = date.Format(time.RFC3339)
		out = append(out, r)
	}
	return out, rows.Err()
}

// normalizeSurveyAbbr maps friendly survey names to the canonical 2-letter
// BLS abbreviation. Returns the input unchanged when it's already a
// 2-letter code; returns "" for empty input.
func normalizeSurveyAbbr(in string) string {
	s := strings.TrimSpace(strings.ToUpper(in))
	switch s {
	case "", "ALL":
		return ""
	case "CPI", "CPIU", "CPI-U":
		return "CU"
	case "CPIW", "CPI-W":
		return "CW"
	case "EMPLOYMENT", "PAYROLLS", "CES", "EMPSIT":
		return "CE"
	case "UNEMPLOYMENT", "CPS":
		return "LN"
	case "JOLTS":
		return "JT"
	case "PPI":
		return "WP"
	case "ECI":
		return "CI"
	case "PRODUCTIVITY":
		return "PR"
	case "LAUS":
		return "LA"
	}
	return s
}

// parseDurationLoose accepts time.Duration syntax plus "Nd" for days.
func parseDurationLoose(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	if strings.HasSuffix(s, "d") {
		days := strings.TrimSuffix(s, "d")
		d, err := time.ParseDuration(days + "h")
		if err != nil {
			return 0, err
		}
		return d * 24, nil
	}
	return time.ParseDuration(s)
}
