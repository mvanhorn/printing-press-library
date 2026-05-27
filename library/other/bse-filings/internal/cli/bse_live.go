// Copyright 2026 rushyant-m. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/mvanhorn/printing-press-library/library/other/bse-filings/internal/bseutil"
	"github.com/mvanhorn/printing-press-library/library/other/bse-filings/internal/cliutil"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// ---- due-soon --------------------------------------------------------------

func newDueSoonCmd(flags *rootFlags) *cobra.Command {
	var withinFlag string
	var kindsCSV string

	cmd := &cobra.Command{
		Use:   "due-soon",
		Short: "Holdings with results, board meetings, or AGM due in the next N days (live).",
		Long: strings.Trim(`
Merge forthcoming-results and forthcoming-corp-action feeds from BSE, filter to
your holdings, and window to the next N days. Live API call.

--kind selects which feeds to query: results (Corpforthresults), board / agm
(DefaultData corp actions). Default queries results only.`, "\n"),
		Example: strings.Trim(`
  bse-filings-pp-cli due-soon --within 7
  bse-filings-pp-cli due-soon --within 30 --kind results,board --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			within, err := parseDaysFlag(withinFlag)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --within value: %w", err))
			}

			kinds := map[string]bool{}
			for _, k := range strings.Split(kindsCSV, ",") {
				if k = strings.TrimSpace(strings.ToLower(k)); k != "" {
					kinds[k] = true
				}
			}
			if len(kinds) == 0 {
				kinds["results"] = true
			}

			s, err := openBSEStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()
			if err := seedHoldingsIfEmpty(s); err != nil {
				return err
			}
			held, err := holdingScripSet(s)
			if err != nil {
				return err
			}

			type dueItem struct {
				ScripCode string `json:"scrip_code"`
				Name      string `json:"name"`
				Kind      string `json:"kind"`
				Date      string `json:"date"`
				when      time.Time
			}

			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.ErrOrStderr(), "verify: would query BSE forthcoming-results / corp-action feeds")
				return flags.printJSON(cmd, []dueItem{})
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			now := time.Now()
			horizon := now.AddDate(0, 0, within)
			items := []dueItem{}

			if kinds["results"] {
				data, derr := c.Get("/Corpforthresults/w", map[string]string{"scripcode": "", "fromdate": "", "todate": ""})
				if derr != nil {
					return classifyAPIError(derr, flags)
				}
				var arr []struct {
					ScripCode   string `json:"scrip_Code"`
					ShortName   string `json:"short_name"`
					LongName    string `json:"Long_Name"`
					MeetingDate string `json:"meeting_date"`
				}
				_ = json.Unmarshal(data, &arr)
				for _, r := range arr {
					if !held[r.ScripCode] {
						continue
					}
					t, ok := bseutil.ParseBSEDate(r.MeetingDate)
					if !ok || t.Before(now.AddDate(0, 0, -1)) || t.After(horizon) {
						continue
					}
					items = append(items, dueItem{ScripCode: r.ScripCode, Name: r.LongName, Kind: "results", Date: r.MeetingDate, when: t})
				}
			}

			if kinds["board"] || kinds["agm"] {
				data, derr := c.Get("/DefaultData/w", map[string]string{"scripcode": "", "segment": "Equity", "Fdate": "", "TDate": "", "Purposecode": ""})
				if derr != nil {
					return classifyAPIError(derr, flags)
				}
				var arr []struct {
					ScripCode json.Number `json:"scrip_code"`
					LongName  string      `json:"long_name"`
					ExDate    string      `json:"Ex_date"`
					Purpose   string      `json:"Purpose"`
				}
				_ = json.Unmarshal(data, &arr)
				for _, r := range arr {
					sc := r.ScripCode.String()
					if !held[sc] {
						continue
					}
					purpose := strings.ToLower(r.Purpose)
					kind := "board"
					if strings.Contains(purpose, "agm") || strings.Contains(purpose, "annual general") {
						kind = "agm"
					}
					if !kinds[kind] {
						continue
					}
					t, ok := bseutil.ParseBSEDate(r.ExDate)
					if !ok || t.Before(now.AddDate(0, 0, -1)) || t.After(horizon) {
						continue
					}
					items = append(items, dueItem{ScripCode: sc, Name: r.LongName, Kind: kind, Date: r.ExDate, when: t})
				}
			}

			sort.Slice(items, func(i, j int) bool { return items[i].when.Before(items[j].when) })
			return flags.printJSON(cmd, items)
		},
	}
	cmd.Flags().StringVar(&withinFlag, "within", "7", "Look-ahead window: a day count (7) or duration (7d, 2w).")
	cmd.Flags().StringVar(&kindsCSV, "kind", "results", "Comma-separated event kinds: results, board, agm.")
	return cmd
}

// ---- outcomes --------------------------------------------------------------

func newOutcomesCmd(flags *rootFlags) *cobra.Command {
	var scrip, quarter string
	var beat, miss bool

	cmd := &cobra.Command{
		Use:   "outcomes",
		Short: "Pull results filings for holdings and tag beat/miss where computable.",
		Long: strings.Trim(`
For each holding (or one --scrip), pull result-category announcements from the
local store and fetch the detailed-financials snapshot from BSE, store the
parsed numbers into results_outcomes, and print a table joining the filing to
the numbers with a beat/miss column.

Beat/miss is best-effort: when the snapshot endpoint returns no comparable
numbers, beat_miss is "n/a" and the row still shows what is available — no
fabricated verdicts.`, "\n"),
		Example: strings.Trim(`
  bse-filings-pp-cli outcomes --scrip 500325
  bse-filings-pp-cli outcomes --quarter "Q4 FY26"`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			s, err := openBSEStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()
			if err := seedHoldingsIfEmpty(s); err != nil {
				return err
			}

			// Which scrips to consider.
			var scrips []string
			if scrip != "" {
				scrips = []string{strings.TrimSpace(scrip)}
			} else {
				holdings, herr := s.ListHoldings()
				if herr != nil {
					return herr
				}
				for _, h := range holdings {
					scrips = append(scrips, h.ScripCode)
				}
			}

			type outcomeRow struct {
				ScripCode string `json:"scrip_code"`
				FilingID  string `json:"filing_id"`
				Quarter   string `json:"quarter"`
				Date      string `json:"date"`
				Revenue   string `json:"revenue"`
				EBITDA    string `json:"ebitda"`
				PAT       string `json:"pat"`
				BeatMiss  string `json:"beat_miss"`
			}

			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.ErrOrStderr(), "verify: would fetch TabResults_PAR financials per holding")
				return flags.printJSON(cmd, []outcomeRow{})
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			out := []outcomeRow{}
			for _, sc := range scrips {
				// Result-category filings from the local store. Match the
				// BSE category names that carry result announcements.
				q := `SELECT newsid, COALESCE(news_dt,''), COALESCE(quarter_id,''), COALESCE(newssub, headline, '')
				      FROM announcements
				      WHERE scrip_cd = ?
				        AND (announcement_type = 'R'
				             OR LOWER(COALESCE(newssub,'')) LIKE '%financial result%'
				             OR LOWER(COALESCE(newssub,'')) LIKE '%results%')
				      ORDER BY news_dt DESC LIMIT 8`
				rows, qerr := s.Query(q, sc)
				if qerr != nil {
					return qerr
				}
				type filing struct {
					id, date, qid string
				}
				var filings []filing
				for rows.Next() {
					var f filing
					var sub string
					if rows.Scan(&f.id, &f.date, &f.qid, &sub) != nil {
						continue
					}
					filings = append(filings, f)
				}
				rows.Close()
				if len(filings) == 0 {
					continue
				}

				// Fetch the detailed-financials snapshot once per scrip.
				rev, ebitda, pat := fetchResultNumbers(c, sc)

				for _, f := range filings {
					qLabel := f.qid
					if qLabel == "" {
						if t, ok := bseutil.ParseBSEDate(f.date); ok {
							qLabel = bseutil.QuarterFromDate(t)
						}
					}
					if quarter != "" && !strings.EqualFold(qLabel, quarter) {
						continue
					}
					// Beat/miss requires a comparable baseline we don't have
					// from the available endpoints — keep honest "n/a".
					beatMiss := "n/a"
					if beat && beatMiss != "beat" {
						continue
					}
					if miss && beatMiss != "miss" {
						continue
					}
					_ = s.UpsertResultsOutcome(f.id, sc, qLabel, rev, ebitda, pat, beatMiss)
					out = append(out, outcomeRow{
						ScripCode: sc, FilingID: f.id, Quarter: qLabel, Date: f.date,
						Revenue: rev, EBITDA: ebitda, PAT: pat, BeatMiss: beatMiss,
					})
				}
			}
			if (beat || miss) && len(out) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "note: beat/miss tagging is unavailable — BSE's results-snapshot endpoint returns no comparable baseline, so --beat/--miss match nothing. Drop the flag to see result filings and any revenue/PAT figures.")
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&scrip, "scrip", "", "Restrict to one scrip code.")
	cmd.Flags().StringVar(&quarter, "quarter", "", "Restrict to one quarter label (e.g. Q4 FY26).")
	cmd.Flags().BoolVar(&beat, "beat", false, "Show only filings tagged beat (none until a baseline source exists).")
	cmd.Flags().BoolVar(&miss, "miss", false, "Show only filings tagged miss (none until a baseline source exists).")
	return cmd
}

// fetchResultNumbers calls the detailed-financials snapshot endpoint and tries
// to pull revenue/EBITDA/PAT out of whatever shape it returns. The endpoint
// frequently returns an empty body for large caps; in that case all three come
// back "" and the caller records "n/a". pp:client-call — real TabResults_PAR
// request.
func fetchResultNumbers(c interface {
	Get(string, map[string]string) (json.RawMessage, error)
}, scrip string) (revenue, ebitda, pat string) {
	data, err := c.Get("/TabResults_PAR/w", map[string]string{"scripcode": scrip, "tabtype": "1"})
	if err != nil {
		return "", "", ""
	}
	// The endpoint may return "" (empty JSON string), {} or an array. Try the
	// common field names without forcing a fixed schema.
	var rows []map[string]any
	if json.Unmarshal(data, &rows) != nil || len(rows) == 0 {
		var obj map[string]any
		if json.Unmarshal(data, &obj) == nil && len(obj) > 0 {
			rows = []map[string]any{obj}
		}
	}
	if len(rows) == 0 {
		return "", "", ""
	}
	pick := func(m map[string]any, keys ...string) string {
		for _, k := range keys {
			for mk, v := range m {
				if strings.EqualFold(mk, k) && v != nil {
					return fmt.Sprintf("%v", v)
				}
			}
		}
		return ""
	}
	r := rows[0]
	revenue = pick(r, "Revenue", "TotalIncome", "Total_Income", "NetSales", "Sales")
	ebitda = pick(r, "EBITDA", "OperatingProfit", "PBIDT")
	pat = pick(r, "PAT", "NetProfit", "ProfitAfterTax", "Net_Profit")
	return revenue, ebitda, pat
}
