// Copyright 2026 rushyant-m. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/mvanhorn/printing-press-library/library/other/bse-filings/internal/bseutil"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// defaultThesisTerms are the guidance keywords thesis-drift tracks unless the
// user overrides them with --terms.
var defaultThesisTerms = []string{"margin", "demand", "debt", "guidance", "growth", "pricing"}

// ftsQuote wraps a user phrase as an FTS5 string literal so phrases with
// spaces match as a phrase and special characters don't break the query.
func ftsQuote(phrase string) string {
	return `"` + strings.ReplaceAll(phrase, `"`, `""`) + `"`
}

func snippet(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// cleanFilingTitle de-slugs BSE's NEWSSUB category labels, which arrive
// underscore-joined (e.g. "Award_of_Order_Receipt_of_Order"). Spaces read
// as prose without altering titles that are already spaced.
func cleanFilingTitle(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "_", " "))
}

// ---- concall-grep ----------------------------------------------------------

func newConcallGrepCmd(flags *rootFlags) *cobra.Command {
	var scrip, sector, quarter, since string

	cmd := &cobra.Command{
		Use:   "concall-grep [query]",
		Short: "Full-text search a phrase across stored concall transcripts in your portfolio.",
		Long: strings.Trim(`
Search the parsed concall paragraphs (populated by 'concall') with SQLite FTS5
and return the scrip, quarter, filing date, and matching paragraph snippet.
Scoped to your holdings; narrow further with --scrip, --sector, or --quarter.`, "\n"),
		Example: strings.Trim(`
  bse-filings-pp-cli concall-grep "capex" --scrip 500325
  bse-filings-pp-cli concall-grep "margin" --sector Energy`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			phrase := strings.Join(args, " ")

			s, err := openBSEStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()

			// Join FTS to holdings so the sector filter and holdings-only
			// default come for free.
			query := `
				SELECT f.scrip_code, COALESCE(h.scrip_name,''), COALESCE(h.sector,''),
				       f.quarter, f.body, COALESCE(c.filed_at,'')
				FROM concall_chunks_fts f
				JOIN holdings h ON h.scrip_code = f.scrip_code
				LEFT JOIN concall_chunks c
				       ON c.filing_id = f.filing_id AND c.body = f.body
				WHERE concall_chunks_fts MATCH ?`
			condArgs := []any{ftsQuote(phrase)}
			if scrip != "" {
				query += " AND f.scrip_code = ?"
				condArgs = append(condArgs, strings.TrimSpace(scrip))
			}
			if sector != "" {
				query += " AND LOWER(h.sector) = LOWER(?)"
				condArgs = append(condArgs, sector)
			}
			if quarter != "" {
				query += " AND LOWER(f.quarter) = LOWER(?)"
				condArgs = append(condArgs, quarter)
			}
			var sinceCutoff time.Time
			if since != "" {
				sinceCutoff, err = parseSinceDuration(since)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --since value %q: %w", since, err))
				}
			}
			query += " ORDER BY c.filed_at DESC LIMIT 200"

			rows, err := s.Query(query, condArgs...)
			if err != nil {
				return err
			}
			defer rows.Close()

			type hit struct {
				ScripCode string `json:"scrip_code"`
				ScripName string `json:"scrip_name"`
				Sector    string `json:"sector"`
				Quarter   string `json:"quarter"`
				FiledAt   string `json:"filed_at"`
				Snippet   string `json:"snippet"`
			}
			hits := []hit{}
			for rows.Next() {
				var h hit
				var filed string
				if err := rows.Scan(&h.ScripCode, &h.ScripName, &h.Sector, &h.Quarter, &h.Snippet, &filed); err != nil {
					continue
				}
				if !sinceCutoff.IsZero() {
					if t, ok := bseutil.ParseBSEDate(filed); ok && t.Before(sinceCutoff) {
						continue
					}
				}
				h.FiledAt = filed
				h.Snippet = snippet(h.Snippet, 240)
				hits = append(hits, h)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			return flags.printJSON(cmd, hits)
		},
	}
	cmd.Flags().StringVar(&scrip, "scrip", "", "Restrict to one scrip code.")
	cmd.Flags().StringVar(&sector, "sector", "", "Restrict to one sector.")
	cmd.Flags().StringVar(&quarter, "quarter", "", "Restrict to one quarter label (e.g. Q1 FY27).")
	cmd.Flags().StringVar(&since, "since", "", "Only paragraphs filed within this window (e.g. 365d).")
	return cmd
}

// ---- thesis-drift ----------------------------------------------------------

func newThesisDriftCmd(flags *rootFlags) *cobra.Command {
	var termsCSV string
	var last int
	var all bool

	cmd := &cobra.Command{
		Use:   "thesis-drift [scrip]",
		Short: "Per-quarter frequency of guidance terms across a holding's stored concalls.",
		Long: strings.Trim(`
For a scrip (or every holding with --all), count how often each term appears
in each quarter's concall transcript and emit a matrix (rows = quarter, cols =
term) so you can see which themes are rising or falling. Requires concalls to
be parsed first via 'concall <scrip>'.`, "\n"),
		Example: strings.Trim(`
  bse-filings-pp-cli thesis-drift 500325 --terms margin,demand --last 4
  bse-filings-pp-cli thesis-drift --all`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !all {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) > 0 {
				if err := requireNumericScrip(args[0]); err != nil {
					return err
				}
			}

			terms := defaultThesisTerms
			if termsCSV != "" {
				terms = nil
				for _, t := range strings.Split(termsCSV, ",") {
					if t = strings.TrimSpace(t); t != "" {
						terms = append(terms, t)
					}
				}
			}

			s, err := openBSEStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()

			var scrips []string
			if all {
				holdings, herr := s.ListHoldings()
				if herr != nil {
					return herr
				}
				for _, h := range holdings {
					scrips = append(scrips, h.ScripCode)
				}
			} else {
				scrips = []string{strings.TrimSpace(args[0])}
			}

			type matrixRow struct {
				Quarter string         `json:"quarter"`
				Counts  map[string]int `json:"counts"`
			}
			type scripDrift struct {
				ScripCode string      `json:"scrip_code"`
				Terms     []string    `json:"terms"`
				Matrix    []matrixRow `json:"matrix"`
				Note      string      `json:"note,omitempty"`
			}

			out := []scripDrift{}
			for _, sc := range scrips {
				// Group paragraph bodies by quarter for this scrip.
				rows, qerr := s.Query(
					`SELECT COALESCE(quarter,''), body FROM concall_chunks WHERE scrip_code = ? ORDER BY filed_at`,
					sc,
				)
				if qerr != nil {
					return qerr
				}
				byQuarter := map[string][]string{}
				var order []string
				for rows.Next() {
					var q, body string
					if rows.Scan(&q, &body) != nil {
						continue
					}
					if _, seen := byQuarter[q]; !seen {
						order = append(order, q)
					}
					byQuarter[q] = append(byQuarter[q], body)
				}
				rows.Close()

				sd := scripDrift{ScripCode: sc, Terms: terms}
				if len(order) == 0 {
					sd.Note = fmt.Sprintf("no concall data — run `concall %s` first", sc)
					out = append(out, sd)
					continue
				}
				// Keep only the last N quarters (order is oldest-first).
				if last > 0 && len(order) > last {
					order = order[len(order)-last:]
				}
				for _, q := range order {
					sd.Matrix = append(sd.Matrix, matrixRow{Quarter: q, Counts: bseutil.CountTerms(byQuarter[q], terms)})
				}
				out = append(out, sd)
			}

			// Single-scrip without data: surface the honest note as the
			// primary message, not buried in an array.
			if !all && len(out) == 1 && len(out[0].Matrix) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), out[0].Note)
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&termsCSV, "terms", "", "Comma-separated terms to count (default: margin,demand,debt,guidance,growth,pricing).")
	cmd.Flags().IntVar(&last, "last", 4, "Keep only the most recent N quarters.")
	cmd.Flags().BoolVar(&all, "all", false, "Run across every holding instead of one scrip.")
	return cmd
}

// ---- cross -----------------------------------------------------------------

func newCrossCmd(flags *rootFlags) *cobra.Command {
	var minHoldings int
	var since string
	var holdingsOnly bool

	cmd := &cobra.Command{
		Use:   "cross [query]",
		Short: "Find a phrase appearing across two or more holdings, grouped by sector.",
		Long: strings.Trim(`
FTS-match a phrase across every holding's stored concall paragraphs, group the
matching holdings by sector, and keep sectors where at least --min-holdings
distinct holdings matched — a sector-wide shift detector.`, "\n"),
		Example: strings.Trim(`
  bse-filings-pp-cli cross "demand recovery" --min-holdings 2
  bse-filings-pp-cli cross "capex" --since 365d`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			phrase := strings.Join(args, " ")

			s, err := openBSEStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()

			var sinceCutoff time.Time
			if since != "" {
				sinceCutoff, err = parseSinceDuration(since)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --since value %q: %w", since, err))
				}
			}

			// --holdings-only (default) requires the chunk's scrip to be a tracked
			// holding; turning it off widens to any synced chunk (sector falls
			// back to '(unknown)' for non-holdings).
			holdingsJoin := "JOIN holdings h ON h.scrip_code = f.scrip_code"
			if !holdingsOnly {
				holdingsJoin = "LEFT JOIN holdings h ON h.scrip_code = f.scrip_code"
			}
			rows, err := s.Query(`
				SELECT f.scrip_code, COALESCE(h.scrip_name,''), COALESCE(h.sector,'(unknown)'),
				       f.body, COALESCE(c.filed_at,'')
				FROM concall_chunks_fts f
				`+holdingsJoin+`
				LEFT JOIN concall_chunks c
				       ON c.filing_id = f.filing_id AND c.body = f.body
				WHERE concall_chunks_fts MATCH ?`, ftsQuote(phrase))
			if err != nil {
				return err
			}
			defer rows.Close()

			type holdMatch struct {
				ScripCode string `json:"scrip_code"`
				ScripName string `json:"scrip_name"`
				Snippet   string `json:"snippet"`
			}
			// sector -> scrip -> first snippet
			sectorHoldings := map[string]map[string]holdMatch{}
			for rows.Next() {
				var scrip, name, sector, body, filed string
				if rows.Scan(&scrip, &name, &sector, &body, &filed) != nil {
					continue
				}
				if !sinceCutoff.IsZero() {
					if t, ok := bseutil.ParseBSEDate(filed); ok && t.Before(sinceCutoff) {
						continue
					}
				}
				if sectorHoldings[sector] == nil {
					sectorHoldings[sector] = map[string]holdMatch{}
				}
				if _, seen := sectorHoldings[sector][scrip]; !seen {
					sectorHoldings[sector][scrip] = holdMatch{ScripCode: scrip, ScripName: name, Snippet: snippet(body, 200)}
				}
			}
			if err := rows.Err(); err != nil {
				return err
			}

			type sectorGroup struct {
				Sector   string      `json:"sector"`
				Count    int         `json:"holdings_matched"`
				Holdings []holdMatch `json:"holdings"`
			}
			groups := []sectorGroup{}
			for sector, holds := range sectorHoldings {
				if len(holds) < minHoldings {
					continue
				}
				g := sectorGroup{Sector: sector, Count: len(holds)}
				for _, hm := range holds {
					g.Holdings = append(g.Holdings, hm)
				}
				sort.Slice(g.Holdings, func(i, j int) bool { return g.Holdings[i].ScripCode < g.Holdings[j].ScripCode })
				groups = append(groups, g)
			}
			sort.Slice(groups, func(i, j int) bool {
				if groups[i].Count != groups[j].Count {
					return groups[i].Count > groups[j].Count
				}
				return groups[i].Sector < groups[j].Sector
			})
			return flags.printJSON(cmd, groups)
		},
	}
	cmd.Flags().IntVar(&minHoldings, "min-holdings", 2, "Keep only sectors where at least this many holdings matched.")
	cmd.Flags().StringVar(&since, "since", "", "Only paragraphs filed within this window (e.g. 365d).")
	cmd.Flags().BoolVar(&holdingsOnly, "holdings-only", true, "Restrict the sweep to tracked holdings (default); --holdings-only=false widens to any synced chunk.")
	return cmd
}

// ---- stale -----------------------------------------------------------------

func newStaleCmd(flags *rootFlags) *cobra.Command {
	var noFilingSinceFlag string

	cmd := &cobra.Command{
		Use:   "stale",
		Short: "List holdings with no filing activity in the last N days (silence as a signal).",
		Long: strings.Trim(`
For each holding, find the most recent synced filing date and list the holdings
whose latest filing is older than --no-filing-since days, or that have no synced
filings at all. Local query — run 'sync' first to populate filing dates.`, "\n"),
		Example:     "  bse-filings-pp-cli stale --no-filing-since 90",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			noFilingSince, err := parseDaysFlag(noFilingSinceFlag)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --no-filing-since value: %w", err))
			}
			s, err := openBSEStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()
			if err := seedHoldingsIfEmpty(s); err != nil {
				return err
			}

			cutoff := time.Now().AddDate(0, 0, -noFilingSince)
			rows, err := s.Query(`
				SELECT h.scrip_code, COALESCE(h.scrip_name,''), COALESCE(h.sector,''),
				       MAX(a.news_dt) AS latest
				FROM holdings h
				LEFT JOIN announcements a ON a.scrip_cd = h.scrip_code
				GROUP BY h.scrip_code
				ORDER BY h.scrip_code`)
			if err != nil {
				return err
			}
			defer rows.Close()

			type staleHolding struct {
				ScripCode   string `json:"scrip_code"`
				ScripName   string `json:"scrip_name"`
				Sector      string `json:"sector"`
				LatestFiled string `json:"latest_filed,omitempty"`
				Reason      string `json:"reason"`
			}
			out := []staleHolding{}
			for rows.Next() {
				var sc, name, sector string
				var latest sql.NullString
				if rows.Scan(&sc, &name, &sector, &latest) != nil {
					continue
				}
				if !latest.Valid || latest.String == "" {
					out = append(out, staleHolding{ScripCode: sc, ScripName: name, Sector: sector, Reason: "no_filings"})
					continue
				}
				t, ok := bseutil.ParseBSEDate(latest.String)
				if !ok || t.Before(cutoff) {
					out = append(out, staleHolding{
						ScripCode: sc, ScripName: name, Sector: sector,
						LatestFiled: latest.String, Reason: "stale",
					})
				}
			}
			if err := rows.Err(); err != nil {
				return err
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&noFilingSinceFlag, "no-filing-since", "90", "Flag holdings whose latest filing is older than this window: a day count (90) or duration (90d, 12w).")
	return cmd
}

// ---- critical --------------------------------------------------------------

func newCriticalCmd(flags *rootFlags) *cobra.Command {
	var days int
	var scrip string

	cmd := &cobra.Command{
		Use:   "critical",
		Short: "List critical-news (Regulation 30 material) filings across holdings within a window.",
		Long: strings.Trim(`
Local query over synced announcements: list filings flagged criticalnews=1
within the last N days, scoped to your holdings (or one --scrip). Prints the
scrip, title, date, and attachment URL. Run 'sync' first to populate filings.`, "\n"),
		Example: strings.Trim(`
  bse-filings-pp-cli critical --days 7
  bse-filings-pp-cli critical --scrip 500325 --days 365 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
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

			cutoff := time.Now().AddDate(0, 0, -days)
			query := `
				SELECT a.scrip_cd, COALESCE(a.newssub, a.headline, ''), COALESCE(a.news_dt,''),
				       COALESCE(a.attachmentname,'')
				FROM announcements a
				JOIN holdings h ON h.scrip_code = a.scrip_cd
				WHERE a.criticalnews = 1`
			var condArgs []any
			if scrip != "" {
				query += " AND a.scrip_cd = ?"
				condArgs = append(condArgs, strings.TrimSpace(scrip))
			}
			query += " ORDER BY a.news_dt DESC"

			rows, err := s.Query(query, condArgs...)
			if err != nil {
				return err
			}
			defer rows.Close()

			type criticalFiling struct {
				ScripCode  string `json:"scrip_code"`
				Title      string `json:"title"`
				Date       string `json:"date"`
				Attachment string `json:"attachment_url,omitempty"`
			}
			out := []criticalFiling{}
			for rows.Next() {
				var sc, title, date, attach string
				if rows.Scan(&sc, &title, &date, &attach) != nil {
					continue
				}
				if t, ok := bseutil.ParseBSEDate(date); !ok || t.Before(cutoff) {
					continue
				}
				cf := criticalFiling{ScripCode: sc, Title: snippet(cleanFilingTitle(title), 120), Date: date}
				if attach != "" {
					cf.Attachment = "https://www.bseindia.com/xml-data/corpfiling/AttachLive/" + attach
				}
				out = append(out, cf)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().IntVar(&days, "days", 7, "Window in days to look back for critical filings.")
	cmd.Flags().StringVar(&scrip, "scrip", "", "Restrict to one scrip code.")
	return cmd
}
