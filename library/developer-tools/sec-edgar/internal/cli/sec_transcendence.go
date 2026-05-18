// Copyright 2026 chris-drit. Licensed under Apache-2.0. See LICENSE.
// Hand-authored transcendence commands. Each maps 1:1 to a row in the
// Phase 1.5 absorb manifest's Transcendence table.

package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// ---------- watchlist items (8-K Item cross-CIK pivot) ----------

func newWatchlistCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watchlist",
		Short: "Coverage-list operations (cross-CIK pivots over filings)",
	}
	cmd.AddCommand(newWatchlistItemsCmd(flags))
	return cmd
}

func newWatchlistItemsCmd(flags *rootFlags) *cobra.Command {
	var (
		cikIn   string
		cikCSV  string
		itemCSV string
		since   string
		until   string
		formCSV string
		limit   int
	)
	cmd := &cobra.Command{
		Use:         "items",
		Short:       "Cross-CIK 8-K filter by Item code (or other forms with --form)",
		Long:        "For every CIK in --cik-in (file, one per line) or --cik (CSV), pull recent filings and emit rows where Item code matches --item. The cross-CIK pivot is the leverage: no SEC endpoint does this in one call.",
		Example:     "  sec-edgar-pp-cli watchlist items --cik 0000320193,0000789019,0001652044 --item 2.05,5.02,4.02 --since 30d --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cikIn == "" && cikCSV == "" {
				return cmd.Help()
			}
			ciks, err := loadCIKWatchlist(cikIn, cikCSV)
			if err != nil {
				return usageErr(err)
			}
			if len(ciks) == 0 {
				return usageErr(fmt.Errorf("provide --cik-in <file> (one CIK per line) or --cik <csv>"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			now := time.Now().UTC()
			var sinceTime time.Time
			if since != "" {
				sinceTime, err = parseSince(since, now)
				if err != nil {
					return usageErr(err)
				}
			}
			var untilTime time.Time
			if until != "" {
				untilTime, err = parseSince(until, now)
				if err != nil {
					return usageErr(err)
				}
			}
			wantItems := map[string]struct{}{}
			for _, it := range parseCSV(itemCSV) {
				wantItems[normalizeItem(it)] = struct{}{}
			}
			formFilter := map[string]struct{}{}
			for _, f := range parseCSV(formCSV) {
				formFilter[strings.ToUpper(f)] = struct{}{}
			}
			if len(formFilter) == 0 && len(wantItems) > 0 {
				formFilter["8-K"] = struct{}{}
				formFilter["8-K/A"] = struct{}{}
			}
			// PATCH(greptile P1): scan Recent AND any older Filings.Files pages
			// whose [FilingFrom, FilingTo] overlaps the requested window.
			// Previously only Recent was searched; for active filers
			// (~400-entry cap) this silently dropped older filings inside
			// the window with no warning.
			window := periodRange{start: sinceTime, end: untilTime}
			rows := []map[string]any{}
			for _, cik := range ciks {
				p, err := fetchSubmissions(c, cik)
				if err != nil {
					// Don't fail the whole watchlist for one bad CIK.
					rows = append(rows, map[string]any{
						"cik":   cik,
						"error": err.Error(),
					})
					continue
				}
				scanPage := func(r *submissionRecent) {
					for i := range r.AccessionNumber {
						form := r.Form[i]
						if len(formFilter) > 0 {
							if _, ok := formFilter[strings.ToUpper(form)]; !ok {
								continue
							}
						}
						fdRaw := r.FilingDate[i]
						if !sinceTime.IsZero() {
							if fd, e := time.Parse("2006-01-02", fdRaw); e == nil && fd.Before(sinceTime) {
								continue
							}
						}
						if !untilTime.IsZero() {
							if fd, e := time.Parse("2006-01-02", fdRaw); e == nil && fd.After(untilTime) {
								continue
							}
						}
						items := ""
						if i < len(r.Items) {
							items = r.Items[i]
						}
						if len(wantItems) > 0 {
							if items == "" {
								continue
							}
							hit := false
							for _, it := range parseCSV(items) {
								if _, ok := wantItems[normalizeItem(it)]; ok {
									hit = true
									break
								}
							}
							if !hit {
								continue
							}
						}
						rows = append(rows, map[string]any{
							"cik":         cik,
							"company":     p.Name,
							"accession":   r.AccessionNumber[i],
							"form":        form,
							"filing_date": fdRaw,
							"items":       items,
							"filing_url":  archiveBase(cik, r.AccessionNumber[i]) + r.PrimaryDocument[i],
						})
					}
				}
				scanPage(&p.Filings.Recent)
				for _, ref := range p.Filings.Files {
					if !filingsPageOverlaps(ref, window) {
						continue
					}
					page, perr := fetchSubmissionPage(c, ref.Name)
					if perr != nil {
						rows = append(rows, map[string]any{
							"cik":   cik,
							"error": fmt.Sprintf("fetching submissions page %s: %s", ref.Name, perr),
						})
						continue
					}
					scanPage(page)
				}
			}
			sort.SliceStable(rows, func(i, j int) bool {
				di, _ := rows[i]["filing_date"].(string)
				dj, _ := rows[j]["filing_date"].(string)
				return di > dj
			})
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			meta := map[string]any{
				"cik_count": len(ciks),
				"matches":   len(rows),
				"items":     parseCSV(itemCSV),
				"since":     since,
			}
			return printJSONOrTableWithMeta(cmd, flags, rows, meta)
		},
	}
	cmd.Flags().StringVar(&cikIn, "cik-in", "", "Path to a file with one CIK per line")
	cmd.Flags().StringVar(&cikCSV, "cik", "", "Comma-separated CIK list (alternative to --cik-in)")
	cmd.Flags().StringVar(&itemCSV, "item", "", "8-K Item codes to match (e.g. 2.05,5.02,4.02)")
	cmd.Flags().StringVar(&formCSV, "form", "", "Form types to restrict to (defaults to 8-K when --item is set)")
	cmd.Flags().StringVar(&since, "since", "30d", "Earliest filing date (YYYY-MM-DD or Nd)")
	cmd.Flags().StringVar(&until, "until", "", "Latest filing date")
	cmd.Flags().IntVar(&limit, "limit", 100, "Max matches to return (0 = no limit)")
	return cmd
}

func loadCIKWatchlist(filePath, csv string) ([]string, error) {
	ciks := []string{}
	for _, raw := range parseCSV(csv) {
		p, err := padCIK(raw)
		if err != nil {
			return nil, err
		}
		ciks = append(ciks, p)
	}
	if filePath != "" {
		f, err := os.Open(filePath)
		if err != nil {
			return nil, fmt.Errorf("opening watchlist %q: %w", filePath, err)
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			p, err := padCIK(line)
			if err != nil {
				return nil, fmt.Errorf("watchlist %q: %w", filePath, err)
			}
			ciks = append(ciks, p)
		}
		if err := sc.Err(); err != nil {
			return nil, err
		}
	}
	// De-dup
	seen := map[string]struct{}{}
	out := make([]string, 0, len(ciks))
	for _, c := range ciks {
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		out = append(out, c)
	}
	return out, nil
}

func normalizeItem(it string) string {
	t := strings.TrimSpace(it)
	// Allow "2.05" or "Item 2.05" or "item 2.05"
	t = strings.TrimPrefix(strings.ToLower(t), "item")
	t = strings.TrimSpace(t)
	return t
}

// ---------- insider-cluster ----------

func newInsiderClusterCmd(flags *rootFlags) *cobra.Command {
	var (
		within      string
		minInsiders int
		code        string
		since       string
		limit       int
	)
	cmd := &cobra.Command{
		Use:         "insider-cluster",
		Short:       "Flag issuers with N+ Form 4 filings in a rolling window",
		Long:        "Uses EFTS to find Form 4 filings in the --since window, groups by issuer, and emits clusters where the count of distinct accessions within any rolling --within-day span is >= --min-insiders. EFTS does not tag which CIK on a filing is the issuer vs. reporting insider, so the threshold is filings-per-issuer rather than distinct-insider count; the output's distinct_filers field reflects the non-issuer CIKs observed across the window (best-effort, may underreport).",
		Example:     "  sec-edgar-pp-cli insider-cluster --within 5d --min-insiders 3 --since 30d --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			now := time.Now().UTC()
			// PATCH(greptile P2): the --code (transaction code P/S/A) filter
			// is not implemented in v1 — it would require fetching each
			// Form 4 XML to read the underlying transaction. Until that
			// lands, fail loudly to stderr so users don't silently get
			// unfiltered results.
			if code != "" {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"warning: --code=%s is not applied in v1 (Form 4 XML fetching not implemented); results are unfiltered by transaction code.\n",
					code)
			}
			sinceTime, err := parseSince(since, now)
			if err != nil {
				return usageErr(err)
			}
			withinDuration, err := parseSince(within, now)
			if err != nil {
				return usageErr(fmt.Errorf("--within must be Nd or Nh: %w", err))
			}
			// PATCH(greptile P2): warn when --within parses to less than 24h.
			// Clustering math operates on whole-day buckets; sub-day values
			// were previously rounded up to 1 day silently.
			windowHours := now.Sub(withinDuration).Hours()
			windowDays := int(windowHours / 24)
			if windowDays < 1 {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"warning: --within=%s parses to %.1fh (< 24h); rounding up to 1 day for the clustering window.\n",
					within, windowHours)
				windowDays = 1
			}
			// PATCH(greptile P1): delegate paginated EFTS fetch to the shared
			// fetchAllEFTSHits helper. Behavior preserved: same efTSMaxFetch
			// cap, same stderr truncation warning. Previously fixed inline.
			hits, totalAvailable, truncated, err := fetchAllEFTSHits(c, EFTSQuery{
				Forms: []string{"4"},
				Start: sinceTime.Format("2006-01-02"),
				End:   now.Format("2006-01-02"),
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if truncated {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"warning: insider-cluster fetched %d of %d available Form 4 filings in --since=%s; clustering may omit issuers. Narrow --since to reduce truncation.\n",
					len(hits), totalAvailable, since)
			}
			// Group filings by issuer. EFTS Form 4 returns one row per filing
			// with `ciks` listing every CIK on the filing (issuer + every
			// reporting insider). EFTS doesn't tag which CIK is the issuer;
			// the only reliable invariant is that the issuer appears on every
			// row for a given company. We use the first CIK as a best-effort
			// issuer key — it's stable per filing series.
			//
			// A real "cluster" is N+ DISTINCT FILINGS (accessions) within the
			// rolling window. Multi-insider co-filings (a single accession
			// listing 10 reporting persons) still count as one filing event.
			type filingEvent struct {
				Accession string
				Date      string
				Filers    []string // CIKs after the first (assumed reporting insiders)
			}
			type issuerGroup struct {
				IssuerCIK  string
				IssuerName string
				Filings    []filingEvent
			}
			byIssuer := map[string]*issuerGroup{}
			for _, h := range hits {
				if len(h.CIKs) == 0 {
					continue
				}
				issuer := h.CIKs[0]
				name := ""
				if len(h.DisplayNames) > 0 {
					name = h.DisplayNames[0]
				}
				row, ok := byIssuer[issuer]
				if !ok {
					row = &issuerGroup{IssuerCIK: issuer, IssuerName: name}
					byIssuer[issuer] = row
				}
				if row.IssuerName == "" && name != "" {
					row.IssuerName = name
				}
				filers := []string{}
				for _, ck := range h.CIKs[1:] {
					if ck != issuer {
						filers = append(filers, ck)
					}
				}
				row.Filings = append(row.Filings, filingEvent{
					Accession: h.Accession,
					Date:      h.FileDate,
					Filers:    filers,
				})
			}
			// For each issuer with >= minInsiders distinct filings overall,
			// find the densest rolling window.
			rows := []map[string]any{}
			for _, r := range byIssuer {
				// Deduplicate filings by accession (EFTS sometimes returns
				// multiple hits per filing when many primary docs exist).
				seenAcc := map[string]int{}
				dedup := make([]filingEvent, 0, len(r.Filings))
				for _, f := range r.Filings {
					if _, ok := seenAcc[f.Accession]; ok {
						continue
					}
					seenAcc[f.Accession] = len(dedup)
					dedup = append(dedup, f)
				}
				if len(dedup) < minInsiders {
					continue
				}
				sort.SliceStable(dedup, func(i, j int) bool {
					return dedup[i].Date < dedup[j].Date
				})
				// Rolling window: find max distinct accessions in any [d_i, d_i + windowDays] span.
				best := 0
				bestStart, bestEnd := "", ""
				bestAccs := []string{}
				bestFilers := map[string]struct{}{}
				for i := range dedup {
					di, _ := time.Parse("2006-01-02", dedup[i].Date)
					count := 0
					accs := []string{}
					filers := map[string]struct{}{}
					winStart, winEnd := dedup[i].Date, dedup[i].Date
					for j := i; j < len(dedup); j++ {
						dj, _ := time.Parse("2006-01-02", dedup[j].Date)
						if dj.Sub(di) > time.Duration(windowDays)*24*time.Hour {
							break
						}
						count++
						accs = append(accs, dedup[j].Accession)
						for _, fc := range dedup[j].Filers {
							filers[fc] = struct{}{}
						}
						winEnd = dedup[j].Date
					}
					if count > best {
						best = count
						bestStart = winStart
						bestEnd = winEnd
						bestAccs = accs
						bestFilers = filers
					}
				}
				if best < minInsiders {
					continue
				}
				filerList := make([]string, 0, len(bestFilers))
				for k := range bestFilers {
					filerList = append(filerList, k)
				}
				sort.Strings(filerList)
				rows = append(rows, map[string]any{
					"issuer_cik":        r.IssuerCIK,
					"issuer":            r.IssuerName,
					"window_start":      bestStart,
					"window_end":        bestEnd,
					"filings_in_window": best,
					"distinct_filers":   len(filerList),
					"accessions":        bestAccs,
					"filer_ciks":        filerList,
				})
			}
			sort.SliceStable(rows, func(i, j int) bool {
				return rows[i]["filings_in_window"].(int) > rows[j]["filings_in_window"].(int)
			})
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			meta := map[string]any{
				"window_days":      windowDays,
				"min_filings":      minInsiders,
				"transaction_code": code,
				"since":            since,
				"clusters":         len(rows),
				"fetched":          len(hits),
				"total_available":  totalAvailable,
				"truncated":        truncated,
				"note":             "Threshold is Form 4 filings per issuer within the rolling --within window, not strict distinct-insider count. EFTS does not tag which CIK on a filing is the issuer vs. reporting insider; distinct_filers reports the non-issuer CIKs observed (best-effort, may underreport). Transaction-code filtering (--code P/S/A) is not yet applied to EFTS results. When truncated=true, fetched < total_available — narrow --since.",
			}
			return printJSONOrTableWithMeta(cmd, flags, rows, meta)
		},
	}
	cmd.Flags().StringVar(&within, "within", "5d", "Rolling window (e.g. 5d, 14d)")
	cmd.Flags().IntVar(&minInsiders, "min-insiders", 3, "Minimum Form 4 filings per issuer within the rolling window required to flag a cluster (see Long for why this is filings rather than strict distinct-insider count)")
	cmd.Flags().StringVar(&code, "code", "", "Form 4 transaction code (P=open-market buy, S=sale). NOT APPLIED in v1: this flag is accepted for future compatibility but does not filter results; a stderr warning is emitted if set.")
	cmd.Flags().StringVar(&since, "since", "90d", "Earliest filing date (YYYY-MM-DD or Nd)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max clusters to return")
	return cmd
}

// ---------- watch (live filing feed with filters) ----------

func newWatchCmd(flags *rootFlags) *cobra.Command {
	var (
		intervalSec int
		formCSV     string
		cikIn       string
		cikCSV      string
		itemCSV     string
		keywordRE   string
		oneShot     bool
		maxIter     int
	)
	cmd := &cobra.Command{
		Use:         "watch",
		Short:       "Stream the SEC Atom getcurrent feed; filter by form, CIK, 8-K Item, and keyword regex",
		Long:        "Polls https://www.sec.gov/cgi-bin/browse-edgar?action=getcurrent every --interval seconds, deduplicates entries, and emits NDJSON for each match. --one-shot exits after one poll (used by smoke tests).",
		Example:     "  sec-edgar-pp-cli watch --form 8-K --item 2.05 --keyword 'going concern' --interval 60 --one-shot --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			// Verify-env safety: in mock test mode, do not actually poll.
			if isVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), `{"watch":"would poll Atom feed","verify_env":true}`)
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// PATCH(greptile P1): the Atom getcurrent URL is constant, so the
			// client's 5-minute response cache would freeze every poll within
			// any 5-min window onto the first snapshot — polls 2..N silently
			// see the same body and emit nothing. A live-streaming command
			// must never serve from cache.
			c.NoCache = true
			ciks, err := loadCIKWatchlist(cikIn, cikCSV)
			if err != nil {
				return usageErr(err)
			}
			cikSet := map[string]struct{}{}
			for _, ck := range ciks {
				cikSet[ck] = struct{}{}
			}
			formSet := map[string]struct{}{}
			for _, f := range parseCSV(formCSV) {
				formSet[strings.ToUpper(f)] = struct{}{}
			}
			itemSet := map[string]struct{}{}
			for _, it := range parseCSV(itemCSV) {
				itemSet[normalizeItem(it)] = struct{}{}
			}
			var keyRe *regexp.Regexp
			if keywordRE != "" {
				keyRe, err = regexp.Compile("(?i)" + keywordRE)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --keyword regex: %w", err))
				}
			}
			// PATCH(greptile P2): bound the dedup set so long-running `watch` cannot grow memory unboundedly.
			// The Atom feed returns the most recent 100 entries; seenCap >> feed window means we never
			// re-emit an entry that could still appear in the feed.
			const seenCap = 4096
			seen := make(map[string]struct{}, seenCap)
			seenOrder := make([]string, 0, seenCap)
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()
			iter := 0
			emitted := 0
			for {
				iter++
				body, err := fetchSECRaw(c, "https://www.sec.gov/cgi-bin/browse-edgar?action=getcurrent&type=&owner=include&count=100&output=atom")
				if err != nil {
					return classifyAPIError(err, flags)
				}
				entries := parseAtomFeed(string(body))
				batchMatches := 0
				for _, e := range entries {
					accession, _ := e["accession"].(string)
					if accession == "" {
						continue
					}
					if _, ok := seen[accession]; ok {
						continue
					}
					// PATCH(greptile P2): FIFO eviction keeps `seen` bounded at seenCap entries.
					if len(seenOrder) >= seenCap {
						delete(seen, seenOrder[0])
						seenOrder = seenOrder[1:]
					}
					seen[accession] = struct{}{}
					seenOrder = append(seenOrder, accession)
					form, _ := e["form"].(string)
					cik, _ := e["cik"].(string)
					items, _ := e["items"].(string)
					title, _ := e["title"].(string)
					if len(formSet) > 0 {
						if _, ok := formSet[strings.ToUpper(form)]; !ok {
							continue
						}
					}
					if len(cikSet) > 0 {
						if _, ok := cikSet[cik]; !ok {
							continue
						}
					}
					if len(itemSet) > 0 {
						matched := false
						for _, it := range parseCSV(items) {
							if _, ok := itemSet[normalizeItem(it)]; ok {
								matched = true
								break
							}
						}
						if !matched {
							continue
						}
					}
					if keyRe != nil && !keyRe.MatchString(title) {
						continue
					}
					out, _ := json.Marshal(e)
					fmt.Fprintln(cmd.OutOrStdout(), string(out))
					emitted++
					batchMatches++
				}
				if oneShot || (maxIter > 0 && iter >= maxIter) {
					// Emit a final summary so JSON output is never empty.
					if emitted == 0 {
						summary, _ := json.Marshal(map[string]any{
							"matches":   0,
							"polled":    len(entries),
							"polled_at": time.Now().UTC().Format(time.RFC3339),
							"note":      "no entries matched the filters in this poll",
						})
						fmt.Fprintln(cmd.OutOrStdout(), string(summary))
					}
					return nil
				}
				_ = batchMatches
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(time.Duration(intervalSec) * time.Second):
				}
			}
		},
	}
	cmd.Flags().IntVar(&intervalSec, "interval", 60, "Seconds between polls")
	cmd.Flags().StringVar(&formCSV, "form", "", "Form types to match (CSV)")
	cmd.Flags().StringVar(&cikIn, "cik-in", "", "Watchlist file (one CIK per line)")
	cmd.Flags().StringVar(&cikCSV, "cik", "", "CIKs to match (CSV)")
	cmd.Flags().StringVar(&itemCSV, "item", "", "8-K Item codes to match (CSV)")
	cmd.Flags().StringVar(&keywordRE, "keyword", "", "Title regex to match (case-insensitive)")
	cmd.Flags().BoolVar(&oneShot, "one-shot", false, "Exit after one poll (recommended for scripting)")
	cmd.Flags().IntVar(&maxIter, "max-iter", 0, "Stop after N iterations (0 = unlimited unless --one-shot)")
	return cmd
}

func isVerifyEnv() bool {
	return os.Getenv("PRINTING_PRESS_VERIFY") != ""
}

// ---------- industry-bench (XBRL peer-group percentiles) ----------

type frameUnit struct {
	Taxonomy    string `json:"taxonomy"`
	Tag         string `json:"tag"`
	CCP         string `json:"ccp"`
	UOM         string `json:"uom"`
	Label       string `json:"label"`
	Description string `json:"description"`
	PTS         int    `json:"pts"`
	Data        []struct {
		Accn       string  `json:"accn"`
		CIK        int     `json:"cik"`
		EntityName string  `json:"entityName"`
		Loc        string  `json:"loc"`
		End        string  `json:"end"`
		Val        float64 `json:"val"`
	} `json:"data"`
}

func newIndustryBenchCmd(flags *rootFlags) *cobra.Command {
	var (
		tag      string
		taxonomy string
		unit     string
		period   string
		sicCSV   string
		statCSV  string
		limit    int
	)
	cmd := &cobra.Command{
		Use:         "industry-bench",
		Short:       "Compute percentile statistics for one XBRL concept across SIC peers",
		Long:        "Fetches https://data.sec.gov/api/xbrl/frames/<taxonomy>/<tag>/<unit>/<period>.json, filters to companies in the requested SIC codes (joined via company_tickers + submissions), and computes percentile stats.",
		Example:     "  sec-edgar-pp-cli industry-bench --tag Revenues --period CY2024Q1 --unit USD --sic 7372 --stat p10,p50,p90 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if tag == "" {
				return cmd.Help()
			}
			if period == "" {
				return usageErr(fmt.Errorf("--period is required (e.g. --period CY2024Q1, CY2024, CY2024Q1I)"))
			}
			// PATCH(greptile P2): the --sic filter is not implemented in v1
			// — the XBRL frame endpoint doesn't carry SIC codes, and the
			// per-CIK submissions lookup needed to derive them would cost
			// thousands of HTTP calls per run. Until a local-store path
			// lands, fail loudly to stderr so users don't silently get
			// frame-wide percentiles when they asked for an SIC slice.
			if sicCSV != "" {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"warning: --sic=%s is not applied in v1 (XBRL frame does not include SIC; per-CIK lookup is too costly); results span the entire frame.\n",
					sicCSV)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			url := fmt.Sprintf("https://data.sec.gov/api/xbrl/frames/%s/%s/%s/%s.json",
				taxonomy, tag, unit, period)
			var fr frameUnit
			if err := fetchSECJSON(c, url, &fr); err != nil {
				return classifyAPIError(err, flags)
			}
			// Build an optional SIC filter via a lookup against submissions for
			// each frame CIK is too expensive (~thousands of API calls). For
			// scope, the v1 implementation honors --sic only when paired with
			// --limit-companies (future): for now we compute stats across the
			// whole frame and report the SIC filter as informational. To
			// actually filter, the user can run sync first; absent a synced
			// SIC table we fall back to all companies in the frame.
			vals := make([]float64, 0, len(fr.Data))
			for _, d := range fr.Data {
				vals = append(vals, d.Val)
			}
			sort.Float64s(vals)
			stats := map[string]any{
				"count":  len(vals),
				"min":    safeIdx(vals, 0),
				"max":    safeIdx(vals, len(vals)-1),
				"median": percentile(vals, 50),
			}
			for _, s := range parseCSV(statCSV) {
				s = strings.ToLower(strings.TrimSpace(s))
				if strings.HasPrefix(s, "p") {
					n, err := strconv.Atoi(strings.TrimPrefix(s, "p"))
					if err == nil && n >= 0 && n <= 100 {
						stats[fmt.Sprintf("p%d", n)] = percentile(vals, float64(n))
					}
				}
			}
			// Top-N issuers by value (descending).
			sort.SliceStable(fr.Data, func(i, j int) bool {
				return fr.Data[i].Val > fr.Data[j].Val
			})
			top := fr.Data
			if limit > 0 && len(top) > limit {
				top = top[:limit]
			}
			leaders := make([]map[string]any, 0, len(top))
			for _, d := range top {
				leaders = append(leaders, map[string]any{
					"cik":       fmt.Sprintf("%010d", d.CIK),
					"name":      d.EntityName,
					"value":     d.Val,
					"accession": d.Accn,
					"end":       d.End,
				})
			}
			meta := map[string]any{
				"tag":        tag,
				"taxonomy":   taxonomy,
				"unit":       unit,
				"period":     period,
				"sic_filter": parseCSV(sicCSV),
				"sic_note":   "v1 emits frame-wide percentiles; pair with `companies search`/`submissions` to narrow by SIC.",
				"stats":      stats,
			}
			return printJSONOrTableWithMeta(cmd, flags, leaders, meta)
		},
	}
	cmd.Flags().StringVar(&tag, "tag", "", "XBRL tag (e.g. Revenues, OperatingIncomeLoss)")
	cmd.Flags().StringVar(&taxonomy, "taxonomy", "us-gaap", "Taxonomy: us-gaap, dei, ifrs-full, srt")
	cmd.Flags().StringVar(&unit, "unit", "USD", "Unit (USD, USD/shares, shares, pure)")
	cmd.Flags().StringVar(&period, "period", "", "Period code (e.g. CY2024Q1, CY2024Q1I, CY2024)")
	cmd.Flags().StringVar(&sicCSV, "sic", "", "Restrict to one or more SIC codes (CSV). NOT APPLIED in v1: this flag is accepted for future compatibility but does not filter the frame; a stderr warning is emitted if set.")
	cmd.Flags().StringVar(&statCSV, "stat", "p10,p50,p90", "Percentiles to compute (CSV, e.g. p10,p25,p50,p75,p90)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max top-N issuers to return")
	return cmd
}

func safeIdx(v []float64, i int) float64 {
	if i < 0 || i >= len(v) {
		return 0
	}
	return v[i]
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	rank := p / 100.0 * float64(len(sorted)-1)
	lo := int(rank)
	hi := lo + 1
	if hi >= len(sorted) {
		return sorted[len(sorted)-1]
	}
	frac := rank - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}

// ---------- cross-section (one concept, N companies, last N periods) ----------

type companyConceptResp struct {
	CIK        int                          `json:"cik"`
	EntityName string                       `json:"entityName"`
	Taxonomy   string                       `json:"taxonomy"`
	Tag        string                       `json:"tag"`
	Label      string                       `json:"label"`
	Units      map[string][]factObservation `json:"units"`
}

func newCrossSectionCmd(flags *rootFlags) *cobra.Command {
	var (
		tag      string
		taxonomy string
		unit     string
		cikCSV   string
		tickers  string
		periods  string
	)
	cmd := &cobra.Command{
		Use:         "cross-section",
		Short:       "One XBRL concept × N companies × last N periods, as a wide pivot",
		Example:     "  sec-edgar-pp-cli cross-section --tag Revenues --cik 0000320193,0000789019 --periods last8 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if tag == "" {
				return cmd.Help()
			}
			if cikCSV == "" && tickers == "" {
				return usageErr(fmt.Errorf("provide --cik (CSV of 10-digit CIKs) or --ticker (CSV of tickers)"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// Resolve tickers → CIKs if --ticker given.
			ciks := []string{}
			for _, raw := range parseCSV(cikCSV) {
				p, err := padCIK(raw)
				if err != nil {
					return usageErr(err)
				}
				ciks = append(ciks, p)
			}
			if tickers != "" {
				rows, err := fetchTickerMap(c)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				lookup := map[string]string{}
				for _, r := range rows {
					lookup[strings.ToUpper(r.Ticker)] = r.Padded
				}
				for _, t := range parseCSV(tickers) {
					if p, ok := lookup[strings.ToUpper(t)]; ok {
						ciks = append(ciks, p)
					} else {
						return usageErr(fmt.Errorf("ticker %q not found", t))
					}
				}
			}
			// Parse periods spec: "last4", "last8", or explicit CSV ("CY2024Q1,CY2024Q2").
			wantLastN := 0
			explicitPeriods := []string{}
			if strings.HasPrefix(periods, "last") {
				n, err := strconv.Atoi(strings.TrimPrefix(periods, "last"))
				if err != nil {
					return usageErr(fmt.Errorf("--periods 'last<N>': bad N: %w", err))
				}
				wantLastN = n
			} else {
				explicitPeriods = parseCSV(periods)
			}
			// Fetch one companyconcept per CIK.
			type cell struct {
				CIK   string
				Name  string
				Frame string
				End   string
				Val   float64
				Form  string
				Accn  string
			}
			cells := []cell{}
			for _, ck := range ciks {
				url := fmt.Sprintf("https://data.sec.gov/api/xbrl/companyconcept/CIK%s/%s/%s.json", ck, taxonomy, tag)
				var resp companyConceptResp
				if err := fetchSECJSON(c, url, &resp); err != nil {
					// Skip CIKs without this concept
					continue
				}
				var obs []factObservation
				if v, ok := resp.Units[unit]; ok {
					obs = v
				} else {
					for _, v := range resp.Units {
						obs = v
						break
					}
				}
				// Prefer rows with non-empty frame (avoids mid-year amendments)
				clean := []factObservation{}
				for _, o := range obs {
					if o.Frame != "" {
						clean = append(clean, o)
					}
				}
				if len(clean) == 0 {
					clean = obs
				}
				sort.SliceStable(clean, func(i, j int) bool {
					return clean[i].End > clean[j].End
				})
				take := clean
				if wantLastN > 0 && len(take) > wantLastN {
					take = take[:wantLastN]
				}
				if len(explicitPeriods) > 0 {
					wanted := map[string]struct{}{}
					for _, p := range explicitPeriods {
						wanted[p] = struct{}{}
					}
					filtered := []factObservation{}
					for _, o := range clean {
						if _, ok := wanted[o.Frame]; ok {
							filtered = append(filtered, o)
						}
					}
					take = filtered
				}
				for _, o := range take {
					cells = append(cells, cell{
						CIK: ck, Name: resp.EntityName,
						Frame: o.Frame, End: o.End, Val: o.Val,
						Form: o.Form, Accn: o.Accn,
					})
				}
			}
			// Pivot: one row per CIK, one column per frame.
			frames := []string{}
			frameSet := map[string]struct{}{}
			for _, c := range cells {
				if _, ok := frameSet[c.Frame]; !ok {
					frameSet[c.Frame] = struct{}{}
					frames = append(frames, c.Frame)
				}
			}
			sort.Strings(frames)
			byCIK := map[string]map[string]float64{}
			names := map[string]string{}
			for _, c := range cells {
				if _, ok := byCIK[c.CIK]; !ok {
					byCIK[c.CIK] = map[string]float64{}
				}
				byCIK[c.CIK][c.Frame] = c.Val
				names[c.CIK] = c.Name
			}
			rows := []map[string]any{}
			for _, ck := range ciks {
				row := map[string]any{
					"cik":  ck,
					"name": names[ck],
				}
				for _, f := range frames {
					if v, ok := byCIK[ck][f]; ok {
						row[f] = v
					} else {
						row[f] = nil
					}
				}
				rows = append(rows, row)
			}
			meta := map[string]any{
				"tag":       tag,
				"taxonomy":  taxonomy,
				"unit":      unit,
				"frames":    frames,
				"companies": len(ciks),
			}
			return printJSONOrTableWithMeta(cmd, flags, rows, meta)
		},
	}
	cmd.Flags().StringVar(&tag, "tag", "", "XBRL tag (required, e.g. Revenues)")
	cmd.Flags().StringVar(&taxonomy, "taxonomy", "us-gaap", "Taxonomy")
	cmd.Flags().StringVar(&unit, "unit", "USD", "Unit (USD, USD/shares, shares)")
	cmd.Flags().StringVar(&cikCSV, "cik", "", "CSV of 10-digit CIKs")
	cmd.Flags().StringVar(&tickers, "ticker", "", "CSV of tickers (resolved via company_tickers.json)")
	cmd.Flags().StringVar(&periods, "periods", "last4", "'lastN' or CSV of explicit frame codes (e.g. CY2024Q1,CY2024Q2)")
	return cmd
}

// ---------- restatements ----------

func newRestatementsCmd(flags *rootFlags) *cobra.Command {
	var (
		since string
		limit int
	)
	cmd := &cobra.Command{
		Use:         "restatements",
		Short:       "Find 8-K Item 4.02 (non-reliance) and 10-K/A, 10-Q/A amendments in a date window",
		Example:     "  sec-edgar-pp-cli restatements --since 90d --json --limit 20",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			now := time.Now().UTC()
			sinceTime, err := parseSince(since, now)
			if err != nil {
				return usageErr(err)
			}
			// PATCH(greptile P1): page through each EFTS query rather than
			// reading a single un-paginated response (which silently capped
			// each form type at the EFTS default of 10 hits). Truncation is
			// surfaced once at the end via stderr.
			rows := []map[string]any{}
			amendForms := []string{"10-K/A", "10-Q/A", "20-F/A"}
			truncatedForms := []string{}
			for _, form := range amendForms {
				fetched, total, truncated, err := fetchAllEFTSHits(c, EFTSQuery{
					Forms: []string{form},
					Start: sinceTime.Format("2006-01-02"),
					End:   now.Format("2006-01-02"),
				})
				if err != nil {
					return classifyAPIError(err, flags)
				}
				for _, h := range fetched {
					rows = append(rows, hitToRow(h, "amendment"))
				}
				if truncated {
					truncatedForms = append(truncatedForms,
						fmt.Sprintf("%s (%d of %d)", form, len(fetched), total))
				}
			}
			eightK, eightKTotal, eightKTruncated, err := fetchAllEFTSHits(c, EFTSQuery{
				Q:     `"non-reliance"`,
				Forms: []string{"8-K"},
				Start: sinceTime.Format("2006-01-02"),
				End:   now.Format("2006-01-02"),
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			for _, h := range eightK {
				// Confirm it carries Item 4.02 if items are populated.
				ok := false
				if len(h.Items) == 0 {
					ok = true
				} else {
					for _, it := range h.Items {
						if strings.Contains(it, "4.02") {
							ok = true
							break
						}
					}
				}
				if ok {
					rows = append(rows, hitToRow(h, "8-K item 4.02"))
				}
			}
			if eightKTruncated {
				truncatedForms = append(truncatedForms,
					fmt.Sprintf("8-K non-reliance (%d of %d)", len(eightK), eightKTotal))
			}
			if len(truncatedForms) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"warning: restatements truncated EFTS results for [%s] in --since=%s; results may omit filings. Narrow --since to reduce truncation.\n",
					strings.Join(truncatedForms, ", "), since)
			}
			sort.SliceStable(rows, func(i, j int) bool {
				di, _ := rows[i]["file_date"].(string)
				dj, _ := rows[j]["file_date"].(string)
				return di > dj
			})
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			meta := map[string]any{
				"window_since": since,
				"matches":      len(rows),
			}
			return printJSONOrTableWithMeta(cmd, flags, rows, meta)
		},
	}
	cmd.Flags().StringVar(&since, "since", "90d", "Window start (e.g. 90d, 30d, 2024-01-01)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max rows to return")
	return cmd
}

func hitToRow(h EFTSHit, kind string) map[string]any {
	cik := ""
	if len(h.CIKs) > 0 {
		cik = h.CIKs[0]
	}
	disp := ""
	if len(h.DisplayNames) > 0 {
		disp = h.DisplayNames[0]
	}
	return map[string]any{
		"kind":       kind,
		"form":       h.Form,
		"accession":  h.Accession,
		"file_date":  h.FileDate,
		"cik":        cik,
		"company":    disp,
		"items":      strings.Join(h.Items, ","),
		"filing_url": h.FilingURL,
	}
}

// ---------- late-filers ----------

func newLateFilersCmd(flags *rootFlags) *cobra.Command {
	var (
		since   string
		formCSV string
		limit   int
	)
	cmd := &cobra.Command{
		Use:         "late-filers",
		Short:       "Find NT 10-K, NT 10-Q, NT 20-F notifications in a date window",
		Long:        "Surfaces companies that filed an NT (Notification of Late Filing) form — the SEC's 'we'll miss our deadline' signal.",
		Example:     "  sec-edgar-pp-cli late-filers --since 60d --form 10-K --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			now := time.Now().UTC()
			sinceTime, err := parseSince(since, now)
			if err != nil {
				return usageErr(err)
			}
			wanted := parseCSV(formCSV)
			if len(wanted) == 0 {
				wanted = []string{"10-K", "10-Q", "20-F"}
			}
			ntForms := make([]string, 0, len(wanted))
			for _, w := range wanted {
				ntForms = append(ntForms, "NT "+strings.ToUpper(w))
			}
			// PATCH(greptile P1): page through EFTS rather than reading a
			// single un-paginated response (which silently capped at the
			// EFTS default of 10 hits regardless of window).
			fetched, totalAvailable, truncated, err := fetchAllEFTSHits(c, EFTSQuery{
				Forms: ntForms,
				Start: sinceTime.Format("2006-01-02"),
				End:   now.Format("2006-01-02"),
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			rows := []map[string]any{}
			for _, h := range fetched {
				rows = append(rows, hitToRow(h, "late_filing_notice"))
			}
			if truncated {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"warning: late-filers fetched %d of %d available NT filings in --since=%s; results may omit filings. Narrow --since to reduce truncation.\n",
					len(fetched), totalAvailable, since)
			}
			sort.SliceStable(rows, func(i, j int) bool {
				di, _ := rows[i]["file_date"].(string)
				dj, _ := rows[j]["file_date"].(string)
				return di > dj
			})
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			meta := map[string]any{
				"window_since": since,
				"forms":        ntForms,
				"matches":      len(rows),
			}
			return printJSONOrTableWithMeta(cmd, flags, rows, meta)
		},
	}
	cmd.Flags().StringVar(&since, "since", "60d", "Window start")
	cmd.Flags().StringVar(&formCSV, "form", "", "Base form types (CSV, default 10-K,10-Q,20-F — the NT prefix is added)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Max rows to return")
	return cmd
}

// ---------- holdings-delta ----------

type infoTableEntry struct {
	XMLName      xml.Name `xml:"infoTable"`
	NameOfIssuer string   `xml:"nameOfIssuer"`
	TitleOfClass string   `xml:"titleOfClass"`
	CUSIP        string   `xml:"cusip"`
	Value        int64    `xml:"value"`
	ShrsOrPrnAmt struct {
		Shares int64  `xml:"sshPrnamt"`
		Type   string `xml:"sshPrnamtType"`
	} `xml:"shrsOrPrnAmt"`
}

type infoTableDoc struct {
	XMLName xml.Name         `xml:"informationTable"`
	Entries []infoTableEntry `xml:"infoTable"`
}

func newHoldingsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "holdings",
		Short: "Institutional and fund holdings (13F, N-PORT, N-PX)",
	}
	cmd.AddCommand(newHoldingsDeltaCmd(flags))
	return cmd
}

func newHoldingsDeltaCmd(flags *rootFlags) *cobra.Command {
	var (
		cikRaw string
		period string
		vs     string
		limit  int
	)
	cmd := &cobra.Command{
		Use:         "delta",
		Short:       "Diff a filer's 13F holdings across two periods (ADD/EXIT/INCREASE/DECREASE)",
		Long:        "Locates the 13F-HR filings for --period and --vs by reading the filer's submissions, downloads each filing's INFORMATION TABLE XML, and emits per-issuer deltas.",
		Example:     "  sec-edgar-pp-cli holdings delta --filer-cik 0001067983 --period 2024Q4 --vs 2024Q3 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cikRaw == "" {
				return usageErr(fmt.Errorf("--filer-cik is required"))
			}
			cik, err := padCIK(cikRaw)
			if err != nil {
				return usageErr(err)
			}
			if period == "" || vs == "" {
				return usageErr(fmt.Errorf("--period and --vs are required (e.g. 2024Q4 and 2024Q3)"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			currentAcc, err := find13FAccession(c, cik, period)
			if err != nil {
				return err
			}
			priorAcc, err := find13FAccession(c, cik, vs)
			if err != nil {
				return err
			}
			currentTable, err := fetch13FInformationTable(c, cik, currentAcc)
			if err != nil {
				return fmt.Errorf("fetching current %s 13F (%s): %w", period, currentAcc, err)
			}
			priorTable, err := fetch13FInformationTable(c, cik, priorAcc)
			if err != nil {
				return fmt.Errorf("fetching prior %s 13F (%s): %w", vs, priorAcc, err)
			}
			rows := diff13F(currentTable, priorTable)
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			meta := map[string]any{
				"filer_cik":         cik,
				"period_current":    period,
				"period_prior":      vs,
				"accession_current": currentAcc,
				"accession_prior":   priorAcc,
				"deltas":            len(rows),
			}
			return printJSONOrTableWithMeta(cmd, flags, rows, meta)
		},
	}
	cmd.Flags().StringVar(&cikRaw, "filer-cik", "", "10-digit zero-padded CIK of the 13F filer (institution)")
	cmd.Flags().StringVar(&period, "period", "", "Reporting period (YYYYQN, e.g. 2024Q4)")
	cmd.Flags().StringVar(&vs, "vs", "", "Comparison period (YYYYQN)")
	cmd.Flags().IntVar(&limit, "limit", 200, "Max delta rows to return (0 = no limit)")
	return cmd
}

// PATCH(greptile P1): scan `Filings.Recent` first (covers the most recent
// ~400 filings), then fall through to older `Filings.Files` pages when the
// requested period falls outside Recent. Previously this only searched
// Recent, so any quarter older than the active filer's recency window
// silently returned "no 13F-HR filing found" with no indication that the
// real cause was an unfetched history page rather than a true absence.
func find13FAccession(c clientLike, cik, period string) (string, error) {
	p, err := fetchSubmissions(c, cik)
	if err != nil {
		return "", err
	}
	wantDate := period13FDateRange(period)
	if wantDate.start.IsZero() {
		return "", fmt.Errorf("unrecognized period %q (expected YYYYQN, e.g. 2024Q4)", period)
	}
	if acc := scan13FAccession(&p.Filings.Recent, wantDate); acc != "" {
		return acc, nil
	}
	for _, ref := range p.Filings.Files {
		// Each older page advertises the date range it covers. Skip pages
		// whose [FilingFrom, FilingTo] cannot overlap our target quarter so
		// large filers don't pay the cost of fetching every page.
		if !filingsPageOverlaps(ref, wantDate) {
			continue
		}
		page, err := fetchSubmissionPage(c, ref.Name)
		if err != nil {
			return "", fmt.Errorf("fetching submissions page %s: %w", ref.Name, err)
		}
		if acc := scan13FAccession(page, wantDate); acc != "" {
			return acc, nil
		}
	}
	return "", fmt.Errorf("no 13F-HR filing found for CIK %s in period %s", cik, period)
}

func scan13FAccession(r *submissionRecent, wantDate periodRange) string {
	for i := range r.AccessionNumber {
		form := strings.ToUpper(r.Form[i])
		if form != "13F-HR" && form != "13F-HR/A" {
			continue
		}
		rd := r.ReportDate[i]
		t, err := time.Parse("2006-01-02", rd)
		if err != nil {
			continue
		}
		if t.Before(wantDate.start) || t.After(wantDate.end) {
			continue
		}
		return r.AccessionNumber[i]
	}
	return ""
}

// filingsPageOverlaps reports whether an older-submissions page's date
// range [FilingFrom, FilingTo] could plausibly contain a filing in
// wantDate. Pages with unparseable ranges are conservatively assumed to
// overlap so we never silently skip a candidate. A zero start or end on
// wantDate is treated as unbounded — used by watchlist items where
// `--until` is optional.
func filingsPageOverlaps(ref submissionFilesRef, wantDate periodRange) bool {
	from, fromErr := time.Parse("2006-01-02", ref.FilingFrom)
	to, toErr := time.Parse("2006-01-02", ref.FilingTo)
	if fromErr != nil || toErr != nil {
		return true
	}
	if !wantDate.start.IsZero() && to.Before(wantDate.start) {
		return false
	}
	if !wantDate.end.IsZero() && from.After(wantDate.end) {
		return false
	}
	return true
}

type periodRange struct {
	start, end time.Time
}

func period13FDateRange(period string) periodRange {
	p := strings.ToUpper(strings.TrimSpace(period))
	if len(p) < 6 || p[4] != 'Q' {
		return periodRange{}
	}
	year, err := strconv.Atoi(p[:4])
	if err != nil {
		return periodRange{}
	}
	q, err := strconv.Atoi(p[5:])
	if err != nil {
		return periodRange{}
	}
	// PATCH(greptile P1): reject quarters outside [1,4]. Without this,
	// strings like `2024Q99` parse cleanly and time.Date silently
	// normalizes `month := (99-1)*3+1 = 295` into a date range in
	// roughly year 2048 rather than returning an empty range.
	if q < 1 || q > 4 {
		return periodRange{}
	}
	month := (q-1)*3 + 1
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 3, -1)
	return periodRange{start: start, end: end}
}

func fetch13FInformationTable(c clientLike, cik, accession string) (map[string]infoTableEntry, error) {
	base := archiveBase(cik, accession)
	var idx indexJSON
	if err := fetchSECJSON(c, base+"index.json", &idx); err != nil {
		return nil, err
	}
	// 13F filings always have two XML files: `primary_doc.xml` (cover page,
	// small) and an info-table XML whose name is filer-chosen (often a
	// numeric ID like 39042.xml). Skip primary_doc.xml and try each
	// remaining XML; the first one whose root element is <informationTable>
	// wins.
	candidates := []string{}
	for _, it := range idx.Directory.Item {
		l := strings.ToLower(it.Name)
		if strings.HasSuffix(l, ".xml") && l != "primary_doc.xml" {
			candidates = append(candidates, it.Name)
		}
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no candidate information-table XML found in %s", base)
	}
	var body []byte
	var lastErr error
	for _, name := range candidates {
		raw, err := fetchSECRaw(c, base+name)
		if err != nil {
			lastErr = err
			continue
		}
		if strings.Contains(string(raw[:min(len(raw), 4096)]), "informationTable") {
			body = raw
			break
		}
	}
	if body == nil {
		if lastErr != nil {
			return nil, fmt.Errorf("no XML in %s contained <informationTable> (last fetch error: %w)", base, lastErr)
		}
		return nil, fmt.Errorf("no XML in %s contained <informationTable>", base)
	}
	// The information table file may have an XML declaration with a different
	// charset; xml.Decoder needs CharsetReader for non-UTF-8. Most filings are
	// UTF-8; force-skip the unknown-charset path with a passthrough.
	dec := xml.NewDecoder(strings.NewReader(string(body)))
	dec.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		return input, nil
	}
	var doc infoTableDoc
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("parsing 13F XML: %w", err)
	}
	out := map[string]infoTableEntry{}
	for _, e := range doc.Entries {
		key := e.CUSIP
		if key == "" {
			key = e.NameOfIssuer + "|" + e.TitleOfClass
		}
		// Aggregate when a CUSIP appears multiple times (different
		// investment-discretion buckets, etc).
		if prev, ok := out[key]; ok {
			prev.Value += e.Value
			prev.ShrsOrPrnAmt.Shares += e.ShrsOrPrnAmt.Shares
			out[key] = prev
		} else {
			out[key] = e
		}
	}
	return out, nil
}

func diff13F(current, prior map[string]infoTableEntry) []map[string]any {
	seen := map[string]struct{}{}
	rows := []map[string]any{}
	classify := func(curr, prev infoTableEntry, key string) {
		seen[key] = struct{}{}
		shCurr := curr.ShrsOrPrnAmt.Shares
		shPrev := prev.ShrsOrPrnAmt.Shares
		var classification string
		var deltaShares int64
		switch {
		case shPrev == 0 && shCurr > 0:
			classification = "ADD"
			deltaShares = shCurr
		case shPrev > 0 && shCurr == 0:
			classification = "EXIT"
			deltaShares = -shPrev
		case shCurr > shPrev:
			classification = "INCREASE"
			deltaShares = shCurr - shPrev
		case shCurr < shPrev:
			classification = "DECREASE"
			deltaShares = shCurr - shPrev
		default:
			classification = "UNCHANGED"
		}
		issuer := curr.NameOfIssuer
		if issuer == "" {
			issuer = prev.NameOfIssuer
		}
		titleOfClass := curr.TitleOfClass
		if titleOfClass == "" {
			titleOfClass = prev.TitleOfClass
		}
		cusip := curr.CUSIP
		if cusip == "" {
			cusip = prev.CUSIP
		}
		rows = append(rows, map[string]any{
			"classification": classification,
			"issuer":         issuer,
			"title_of_class": titleOfClass,
			"cusip":          cusip,
			"shares_current": shCurr,
			"shares_prior":   shPrev,
			"shares_delta":   deltaShares,
			"value_current":  curr.Value,
			"value_prior":    prev.Value,
		})
	}
	for k, curr := range current {
		prev, ok := prior[k]
		if !ok {
			prev = infoTableEntry{}
		}
		classify(curr, prev, k)
	}
	for k, prev := range prior {
		if _, ok := seen[k]; ok {
			continue
		}
		classify(infoTableEntry{}, prev, k)
	}
	// Sort by abs(shares_delta) desc, EXITs/ADDs to the top.
	sort.SliceStable(rows, func(i, j int) bool {
		di := absInt64(rows[i]["shares_delta"].(int64))
		dj := absInt64(rows[j]["shares_delta"].(int64))
		return di > dj
	})
	// Drop unchanged.
	filtered := rows[:0]
	for _, r := range rows {
		if r["classification"].(string) != "UNCHANGED" {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

func absInt64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
