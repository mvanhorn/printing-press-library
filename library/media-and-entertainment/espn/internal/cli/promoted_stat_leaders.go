// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written promoted command added via amend (2026-05-29).
// PATCH(amend-2026-05-29: add stat-leaders for the ESPN stats / dailyleaders surface)

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// newStatLeadersCmd wires `espn-pp-cli stat-leaders <sport> <league> --stat <name>`,
// exposing ESPN's stats-leaderboard surface — the data behind
// espn.com/<league>/stats and .../stats/dailyleaders. It ranks athletes by a
// single chosen statistic (home runs, ERA, passing yards, ...), which is the
// shape the stats-leaderboard page presents.
//
// This complements the existing `leaders` command: `leaders` lists each athlete
// with a few headline values from their first category; `stat-leaders` produces
// a true single-stat ranking sorted server-side by that stat.
//
// Source: the same stable common/v3 statistics/byathlete endpoint `leaders`
// uses, with the server-side `sort=<category>.<stat>:desc` qualifier plus
// optional `season`/`seasontype`. Each top-level category carries a `names`
// array that is index-aligned with every athlete's `values` array, so a named
// stat resolves to a column without guessing.
func newStatLeadersCmd(flags *rootFlags) *cobra.Command {
	var (
		stat       string
		season     int
		seasonType string
		limit      int
	)

	cmd := &cobra.Command{
		Use:         "stat-leaders <sport> <league>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Statistical leaderboard ranked by a single stat",
		Long: "Rank athletes by a single statistic — the stats-leaderboard surface " +
			"behind espn.com/<league>/stats and .../stats/dailyleaders.\n\n" +
			"Use --stat to choose the stat to rank by (e.g. homeRuns, RBIs, avg, ERA, " +
			"strikeouts for baseball; passingYards, sacks for football). Run without " +
			"--stat to list the available stats for the sport, then re-run with one.",
		Example: `  espn-pp-cli stat-leaders baseball mlb --stat homeRuns
  espn-pp-cli stat-leaders baseball mlb --stat avg --season 2024 --limit 10
  espn-pp-cli stat-leaders football nfl --stat passingYards`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("league is required\nUsage: stat-leaders <sport> <league> --stat <name> [--season N] [--seasontype regular|pre|playoffs] [--limit N]"))
			}
			sport, league := args[0], args[1]

			endpoint := fmt.Sprintf("https://site.web.api.espn.com/apis/common/v3/sports/%s/%s/statistics/byathlete", sport, league)
			q := url.Values{}
			if season > 0 {
				q.Set("season", fmt.Sprintf("%d", season))
			}
			code := seasonTypeCode(seasonType)
			if code < 0 {
				return usageErr(fmt.Errorf("invalid --seasontype %q: valid values are pre, regular, or playoffs", seasonType))
			}
			q.Set("seasontype", fmt.Sprintf("%d", code))
			// Always fetch a generous window so the client-side ranking is
			// correct regardless of the display --limit. ESPN's server-side
			// sort for this endpoint is unreliable (it returns a default
			// composite order), so a small fetch window would miss true
			// leaders; we fetch wide, rank locally, then slice to --limit.
			q.Set("limit", "200")
			if stat != "" {
				// Pass the server-side sort hint too (harmless even though we
				// re-rank locally — it nudges relevant athletes into the window).
				body, err := espnHTTPGet(flags.timeout, endpoint+"?"+q.Encode())
				if err != nil {
					return err
				}
				if sortKey, ok := resolveStatSortKey(body, stat); ok {
					q.Set("sort", sortKey+":desc")
					body, err = espnHTTPGet(flags.timeout, endpoint+"?"+q.Encode())
					if err != nil {
						return err
					}
				} else {
					return usageErr(fmt.Errorf("stat %q not found for %s/%s. Run without --stat to list available stats.", stat, sport, league))
				}
				if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
					return writeJSON(cmd.OutOrStdout(), body)
				}
				return renderStatLeaders(cmd.OutOrStdout(), body, stat, limit)
			}

			body, err := espnHTTPGet(flags.timeout, endpoint+"?"+q.Encode())
			if err != nil {
				return err
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				// Emit a structured discovery envelope rather than the raw
				// byathlete payload, so piped/--agent consumers can see the
				// rankable stat names by category (the same information the
				// table view surfaces) without parsing every athlete row.
				return writeAvailableStatsJSON(cmd.OutOrStdout(), body, sport, league)
			}
			return listAvailableStats(cmd.OutOrStdout(), body)
		},
	}

	cmd.Flags().StringVar(&stat, "stat", "", "Statistic to rank by (e.g. homeRuns, avg, ERA). Omit to list available stats.")
	cmd.Flags().IntVar(&season, "season", 0, "Season year (default: current season)")
	cmd.Flags().StringVar(&seasonType, "seasontype", "regular", "Season type: pre, regular, or playoffs")
	cmd.Flags().IntVar(&limit, "limit", 10, "Max leaders to show (0 = all fetched)")
	return cmd
}

// seasonTypeCode maps a friendly season-type token to ESPN's numeric code.
func seasonTypeCode(s string) int {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "pre", "preseason":
		return 1
	case "", "regular", "reg":
		return 2
	case "post", "postseason", "playoffs":
		return 3
	default:
		// Unrecognized token. Return a sentinel so the caller can reject it
		// with a usage error rather than silently serving regular-season data
		// for a near-miss like "playoff" (singular) or "post-season".
		return -1
	}
}

// writeJSON re-encodes a raw response as indented JSON (matches the JSON
// output idiom used by the other promoted commands).
func writeJSON(w io.Writer, body []byte) error {
	var raw json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(raw)
}

// statCategoryMeta is the index-alignment metadata ESPN attaches to each
// top-level category: names[i] is the stat at position i of every athlete's
// values array for that category.
type statByAthleteResponse struct {
	RequestedSeason struct {
		Name string `json:"name"`
		Year int    `json:"year"`
	} `json:"requestedSeason"`
	Categories []struct {
		Name         string   `json:"name"`
		Names        []string `json:"names"`
		DisplayNames []string `json:"displayNames"`
	} `json:"categories"`
	Athletes []struct {
		Athlete struct {
			DisplayName string `json:"displayName"`
			Team        struct {
				Abbreviation string `json:"abbreviation"`
			} `json:"team"`
			Position struct {
				Abbreviation string `json:"abbreviation"`
			} `json:"position"`
		} `json:"athlete"`
		Categories []struct {
			Name   string    `json:"name"`
			Values []float64 `json:"values"`
		} `json:"categories"`
	} `json:"athletes"`
}

// resolveStatSortKey finds the "<category>.<stat>" sort key for a requested
// stat name (case-insensitive against both the API names and displayNames).
func resolveStatSortKey(body []byte, stat string) (string, bool) {
	var resp statByAthleteResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", false
	}
	want := strings.ToLower(strings.TrimSpace(stat))
	for _, c := range resp.Categories {
		for i, name := range c.Names {
			disp := ""
			if i < len(c.DisplayNames) {
				disp = c.DisplayNames[i]
			}
			if strings.ToLower(name) == want || strings.ToLower(disp) == want {
				return c.Name + "." + name, true
			}
		}
	}
	return "", false
}

// statColumn locates the (category, index, displayName) for a stat name.
func statColumn(resp *statByAthleteResponse, stat string) (catName string, idx int, display string, ok bool) {
	want := strings.ToLower(strings.TrimSpace(stat))
	for _, c := range resp.Categories {
		for i, name := range c.Names {
			disp := name
			if i < len(c.DisplayNames) && c.DisplayNames[i] != "" {
				disp = c.DisplayNames[i]
			}
			if strings.ToLower(name) == want || (i < len(c.DisplayNames) && strings.ToLower(c.DisplayNames[i]) == want) {
				return c.Name, i, disp, true
			}
		}
	}
	return "", 0, "", false
}

// renderStatLeaders prints a ranked single-stat leaderboard. limit caps the
// rows (0 = all returned).
func renderStatLeaders(w io.Writer, body []byte, stat string, limit int) error {
	var resp statByAthleteResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("parsing stat leaders: %w", err)
	}
	catName, idx, display, ok := statColumn(&resp, stat)
	if !ok {
		fmt.Fprintf(w, "Stat %q not found in response.\n", stat)
		return nil
	}
	if len(resp.Athletes) == 0 {
		fmt.Fprintln(w, "No leaders found (the season may not be underway yet — try --season with a completed year).")
		return nil
	}

	// Rank client-side by the chosen stat (descending). ESPN's server-side
	// `sort=<category>.<stat>:desc` is unreliable for this endpoint — it
	// returns athletes in a default composite order, so e.g. a 41-HR hitter
	// can appear below a 38-HR hitter. Sorting here guarantees a correct
	// leaderboard regardless of the response order.
	type row struct {
		name, team, pos string
		val             float64
	}
	rows := make([]row, 0, len(resp.Athletes))
	for _, a := range resp.Athletes {
		val, has := athleteStatValue(a.Categories, catName, idx)
		if !has {
			continue
		}
		rows = append(rows, row{
			name: a.Athlete.DisplayName,
			team: a.Athlete.Team.Abbreviation,
			pos:  a.Athlete.Position.Abbreviation,
			val:  val,
		})
	}
	if len(rows) == 0 {
		fmt.Fprintln(w, "No leaders found for that stat.")
		return nil
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].val > rows[j].val })

	season := resp.RequestedSeason.Name
	if season != "" {
		fmt.Fprintf(w, "%s\n\n", bold(fmt.Sprintf("%s leaders — %s", display, season)))
	} else {
		fmt.Fprintf(w, "%s\n\n", bold(display+" leaders"))
	}

	tw := newTabWriter(w)
	fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
		bold("RANK"), bold("PLAYER"), bold("TEAM"), bold("POS"), bold(display))
	for i, r := range rows {
		if limit > 0 && i >= limit {
			break
		}
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\n",
			i+1, truncate(r.name, 24), r.team, r.pos, formatStatValue(r.val))
	}
	return tw.Flush()
}

// athleteStatValue extracts the value at idx within the athlete's matching
// category.
func athleteStatValue(cats []struct {
	Name   string    `json:"name"`
	Values []float64 `json:"values"`
}, catName string, idx int) (float64, bool) {
	for _, c := range cats {
		if c.Name == catName {
			if idx >= 0 && idx < len(c.Values) {
				return c.Values[idx], true
			}
			return 0, false
		}
	}
	return 0, false
}

// formatStatValue renders a stat value, dropping the trailing .0 for whole
// counts (62, 58) while keeping rate stats (0.322, 1.159) intact.
func formatStatValue(v float64) string {
	if v == float64(int64(v)) {
		return fmt.Sprintf("%d", int64(v))
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.3f", v), "0"), ".")
}

// availableStatEntry is one rankable stat in the discovery envelope.
type availableStatEntry struct {
	Stat        string `json:"stat"`
	Description string `json:"description,omitempty"`
}

// availableStatsEnvelope is the structured JSON returned by the no--stat path
// in machine-output mode. It mirrors what listAvailableStats prints for a TTY:
// the rankable stat names grouped by category, plus a flat category list.
type availableStatsEnvelope struct {
	Sport          string                          `json:"sport"`
	League         string                          `json:"league"`
	Categories     []string                        `json:"categories"`
	AvailableStats map[string][]availableStatEntry `json:"available_stats"`
}

// writeAvailableStatsJSON emits the stat-discovery envelope for the no--stat
// path. Without this, piped/--agent callers received the raw byathlete payload
// (every athlete + values) and could not tell which stat names --stat accepts.
func writeAvailableStatsJSON(w io.Writer, body []byte, sport, league string) error {
	var resp statByAthleteResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("parsing stats: %w", err)
	}
	out := availableStatsEnvelope{
		Sport:          sport,
		League:         league,
		Categories:     make([]string, 0, len(resp.Categories)),
		AvailableStats: make(map[string][]availableStatEntry, len(resp.Categories)),
	}
	for _, c := range resp.Categories {
		if len(c.Names) == 0 {
			continue
		}
		entries := make([]availableStatEntry, 0, len(c.Names))
		for i, name := range c.Names {
			disp := ""
			if i < len(c.DisplayNames) {
				disp = c.DisplayNames[i]
			}
			entries = append(entries, availableStatEntry{Stat: name, Description: disp})
		}
		out.Categories = append(out.Categories, c.Name)
		out.AvailableStats[c.Name] = entries
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// listAvailableStats prints the stat names the user can pass to --stat,
// grouped by category, when --stat is omitted.
func listAvailableStats(w io.Writer, body []byte) error {
	var resp statByAthleteResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("parsing stats: %w", err)
	}
	if len(resp.Categories) == 0 {
		fmt.Fprintln(w, "No stat categories returned for that sport/league.")
		return nil
	}
	fmt.Fprintln(w, "Pass one of these to --stat (then re-run):")
	fmt.Fprintln(w)
	for _, c := range resp.Categories {
		if len(c.Names) == 0 {
			continue
		}
		fmt.Fprintf(w, "%s\n", bold(c.Name))
		tw := newTabWriter(w)
		fmt.Fprintf(tw, "%s\t%s\n", bold("STAT"), bold("DESCRIPTION"))
		for i, name := range c.Names {
			disp := ""
			if i < len(c.DisplayNames) {
				disp = c.DisplayNames[i]
			}
			fmt.Fprintf(tw, "%s\t%s\n", name, disp)
		}
		if err := tw.Flush(); err != nil {
			return err
		}
		fmt.Fprintln(w)
	}
	return nil
}
