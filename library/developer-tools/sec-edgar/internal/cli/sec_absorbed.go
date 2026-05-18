// Copyright 2026 chris-drit. Licensed under Apache-2.0. See LICENSE.
// Hand-authored absorbed features: companies lookup/search, filings
// list/get/exhibits, feed latest, search (EFTS), status, sic show,
// facts statement. Each command matches a feature in the absorb manifest.

package cli

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// ---------- companies lookup ----------
//
// Resolves a ticker (or company-name fragment) to its CIK + name by reading
// the SEC's company_tickers.json. Backs both `companies lookup <ticker>` and
// `companies search "<query>"`.

type tickerRow struct {
	CIK    int64  `json:"cik_str"`
	Ticker string `json:"ticker"`
	Title  string `json:"title"`
	Padded string `json:"-"`
}

func fetchTickerMap(c clientLike) ([]tickerRow, error) {
	var raw map[string]struct {
		CIK    int64  `json:"cik_str"`
		Ticker string `json:"ticker"`
		Title  string `json:"title"`
	}
	if err := fetchSECJSON(c, "https://www.sec.gov/files/company_tickers.json", &raw); err != nil {
		return nil, err
	}
	out := make([]tickerRow, 0, len(raw))
	for _, v := range raw {
		out = append(out, tickerRow{
			CIK:    v.CIK,
			Ticker: v.Ticker,
			Title:  v.Title,
			Padded: fmt.Sprintf("%010d", v.CIK),
		})
	}
	return out, nil
}

func newCompaniesLookupCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "lookup <ticker>",
		Short:       "Resolve a ticker symbol to its CIK and company name",
		Example:     "  sec-edgar-pp-cli companies lookup AAPL --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			rows, err := fetchTickerMap(c)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			needle := strings.ToUpper(strings.TrimSpace(args[0]))
			for _, r := range rows {
				if r.Ticker == needle {
					out := map[string]any{
						"ticker": r.Ticker,
						"cik":    r.Padded,
						"name":   r.Title,
					}
					return printJSONOrTable(cmd, flags, []map[string]any{out})
				}
			}
			return usageErr(fmt.Errorf("ticker %q not found in SEC company_tickers.json; try 'companies search' for fuzzy matching", args[0]))
		},
	}
	return cmd
}

func newCompaniesSearchCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "search <query>",
		Short:       "Fuzzy-search the SEC company directory by name or ticker substring",
		Example:     "  sec-edgar-pp-cli companies search 'apple' --limit 5 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			rows, err := fetchTickerMap(c)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			needle := strings.ToLower(strings.TrimSpace(strings.Join(args, " ")))
			matches := make([]map[string]any, 0, 32)
			for _, r := range rows {
				name := strings.ToLower(r.Title)
				tick := strings.ToLower(r.Ticker)
				if strings.Contains(name, needle) || strings.Contains(tick, needle) {
					matches = append(matches, map[string]any{
						"ticker": r.Ticker,
						"cik":    r.Padded,
						"name":   r.Title,
					})
				}
			}
			sort.SliceStable(matches, func(i, j int) bool {
				return strings.ToLower(matches[i]["name"].(string)) < strings.ToLower(matches[j]["name"].(string))
			})
			if limit > 0 && len(matches) > limit {
				matches = matches[:limit]
			}
			return printJSONOrTable(cmd, flags, matches)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "Max results to return (0 = no limit)")
	return cmd
}

// ---------- filings list / get / exhibits ----------

type submissionRecent struct {
	AccessionNumber []string `json:"accessionNumber"`
	FilingDate      []string `json:"filingDate"`
	ReportDate      []string `json:"reportDate"`
	Form            []string `json:"form"`
	Items           []string `json:"items"`
	PrimaryDocument []string `json:"primaryDocument"`
	IsXBRL          []int    `json:"isXBRL"`
}

// PATCH(greptile P1): submissionFilesRef points at an older page of this
// CIK's filing history. SEC submissions JSON puts the most recent ~400
// filings inline under filings.recent; everything older sits in a list of
// page refs under filings.files. Each ref includes the date range it
// covers, so callers that only need certain periods can skip pages they
// know are out of range.
type submissionFilesRef struct {
	Name        string `json:"name"`
	FilingCount int    `json:"filingCount"`
	FilingFrom  string `json:"filingFrom"`
	FilingTo    string `json:"filingTo"`
}

type submissionPayload struct {
	CIK     string   `json:"cik"`
	Name    string   `json:"name"`
	SIC     string   `json:"sic"`
	SICDesc string   `json:"sicDescription"`
	Tickers []string `json:"tickers"`
	Filings struct {
		Recent submissionRecent     `json:"recent"`
		Files  []submissionFilesRef `json:"files"`
	} `json:"filings"`
}

func fetchSubmissions(c clientLike, cik string) (*submissionPayload, error) {
	url := fmt.Sprintf("https://data.sec.gov/submissions/CIK%s.json", cik)
	var p submissionPayload
	if err := fetchSECJSON(c, url, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// PATCH(greptile P1): fetchSubmissionPage retrieves one of the older pages
// referenced by submissionPayload.Filings.Files. Each page JSON has the
// same shape as the inline `recent` block — parallel arrays of accession
// numbers, forms, dates, etc. — so callers can scan it the same way.
func fetchSubmissionPage(c clientLike, name string) (*submissionRecent, error) {
	url := fmt.Sprintf("https://data.sec.gov/submissions/%s", name)
	var r submissionRecent
	if err := fetchSECJSON(c, url, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func newFilingsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "filings",
		Short: "Filing-level views (list per CIK, fetch, list exhibits)",
	}
	cmd.AddCommand(newFilingsListCmd(flags))
	cmd.AddCommand(newFilingsGetCmd(flags))
	cmd.AddCommand(newFilingsExhibitsCmd(flags))
	return cmd
}

func newFilingsListCmd(flags *rootFlags) *cobra.Command {
	var (
		formCSV string
		since   string
		until   string
		limit   int
	)
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List a company's recent filings, optionally filtered by form type and date window",
		Example:     "  sec-edgar-pp-cli filings list --cik 0000320193 --form 10-K --since 365d --limit 5 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			rawCIK, _ := cmd.Flags().GetString("cik")
			if strings.TrimSpace(rawCIK) == "" {
				return usageErr(fmt.Errorf("--cik is required"))
			}
			cik, err := padCIK(rawCIK)
			if err != nil {
				return usageErr(err)
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			p, err := fetchSubmissions(c, cik)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			formFilter := map[string]struct{}{}
			for _, f := range parseCSV(formCSV) {
				formFilter[strings.ToUpper(f)] = struct{}{}
			}
			now := time.Now().UTC()
			var sinceTime, untilTime time.Time
			if since != "" {
				sinceTime, err = parseSince(since, now)
				if err != nil {
					return usageErr(err)
				}
			}
			if until != "" {
				untilTime, err = parseSince(until, now)
				if err != nil {
					return usageErr(err)
				}
			}

			r := p.Filings.Recent
			n := len(r.AccessionNumber)
			out := make([]map[string]any, 0, n)
			for i := 0; i < n; i++ {
				form := r.Form[i]
				if len(formFilter) > 0 {
					if _, ok := formFilter[strings.ToUpper(form)]; !ok {
						continue
					}
				}
				fdRaw := r.FilingDate[i]
				if !sinceTime.IsZero() {
					if fd, fErr := time.Parse("2006-01-02", fdRaw); fErr == nil && fd.Before(sinceTime) {
						continue
					}
				}
				if !untilTime.IsZero() {
					if fd, fErr := time.Parse("2006-01-02", fdRaw); fErr == nil && fd.After(untilTime) {
						continue
					}
				}
				items := ""
				if i < len(r.Items) {
					items = r.Items[i]
				}
				row := map[string]any{
					"accession":   r.AccessionNumber[i],
					"form":        form,
					"filing_date": fdRaw,
					"report_date": r.ReportDate[i],
					"items":       items,
					"primary_doc": r.PrimaryDocument[i],
					"filing_url":  archiveBase(cik, r.AccessionNumber[i]) + r.PrimaryDocument[i],
					"company":     p.Name,
					"cik":         cik,
				}
				out = append(out, row)
				if limit > 0 && len(out) >= limit {
					break
				}
			}
			return printJSONOrTable(cmd, flags, out)
		},
	}
	cmd.Flags().String("cik", "", "10-digit zero-padded CIK (required)")
	cmd.Flags().StringVar(&formCSV, "form", "", "Comma-separated form types to filter (e.g. 10-K,10-Q,8-K)")
	cmd.Flags().StringVar(&since, "since", "", "Earliest filing date (YYYY-MM-DD, Nd/Nh, or 'last friday')")
	cmd.Flags().StringVar(&until, "until", "", "Latest filing date")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max filings to return (0 = no limit)")
	return cmd
}

func newFilingsGetCmd(flags *rootFlags) *cobra.Command {
	var cikRaw string
	cmd := &cobra.Command{
		Use:         "get <accession>",
		Short:       "Get the index page URL and primary-document URL for one filing",
		Example:     "  sec-edgar-pp-cli filings get 0000320193-24-000123 --cik 0000320193 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			accession := strings.TrimSpace(args[0])
			if strings.TrimSpace(cikRaw) == "" {
				return usageErr(fmt.Errorf("--cik is required (10-digit zero-padded)"))
			}
			cik, err := padCIK(cikRaw)
			if err != nil {
				return usageErr(err)
			}
			if dryRunOK(flags) {
				return nil
			}
			base := archiveBase(cik, accession)
			out := map[string]any{
				"accession":   accession,
				"cik":         cik,
				"archive_url": base,
				"index_html":  base + accession + "-index.htm",
				"index_json":  base + "index.json",
			}
			return printJSONOrTable(cmd, flags, []map[string]any{out})
		},
	}
	cmd.Flags().StringVar(&cikRaw, "cik", "", "10-digit zero-padded CIK (required)")
	return cmd
}

type indexJSON struct {
	Directory struct {
		Item []struct {
			Name string `json:"name"`
			Type string `json:"type"`
			Size string `json:"size"`
		} `json:"item"`
	} `json:"directory"`
}

func newFilingsExhibitsCmd(flags *rootFlags) *cobra.Command {
	var cikRaw, exhibitFilter string
	cmd := &cobra.Command{
		Use:         "exhibits <accession>",
		Short:       "List every exhibit and document in a filing (10-K + EX-10, EX-99.1, etc.)",
		Example:     "  sec-edgar-pp-cli filings exhibits 0000320193-24-000123 --cik 0000320193 --exhibit-type EX-10 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			accession := strings.TrimSpace(args[0])
			if strings.TrimSpace(cikRaw) == "" {
				return usageErr(fmt.Errorf("--cik is required"))
			}
			cik, err := padCIK(cikRaw)
			if err != nil {
				return usageErr(err)
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			base := archiveBase(cik, accession)
			var idx indexJSON
			if err := fetchSECJSON(c, base+"index.json", &idx); err != nil {
				return classifyAPIError(err, flags)
			}
			out := make([]map[string]any, 0, len(idx.Directory.Item))
			for _, it := range idx.Directory.Item {
				if exhibitFilter != "" && !strings.Contains(strings.ToUpper(it.Name), strings.ToUpper(exhibitFilter)) {
					continue
				}
				row := map[string]any{
					"name": it.Name,
					"type": it.Type,
					"size": it.Size,
					"url":  base + it.Name,
				}
				out = append(out, row)
			}
			return printJSONOrTable(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&cikRaw, "cik", "", "10-digit zero-padded CIK (required)")
	cmd.Flags().StringVar(&exhibitFilter, "exhibit-type", "", "Filter exhibit name substring (case-insensitive, e.g. EX-10, EX-99.1)")
	return cmd
}

// ---------- search (EFTS full-text) ----------

func newSecSearchCmd(flags *rootFlags) *cobra.Command {
	var (
		formCSV string
		cikCSV  string
		start   string
		end     string
		limit   int
	)
	cmd := &cobra.Command{
		Use:         "fts <phrase>",
		Short:       "Full-text search every SEC filing since 2001 via efts.sec.gov",
		Long:        "Full-text search every SEC filing since 2001 via efts.sec.gov. Use --form to restrict form type, --cik to restrict filers, --start/--end for date window.",
		Example:     "  sec-edgar-pp-cli fts 'going concern' --form 10-K --start 2024-01-01 --end 2024-03-31 --json --limit 10",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ciks := []string{}
			for _, raw := range parseCSV(cikCSV) {
				p, err := padCIK(raw)
				if err != nil {
					return usageErr(err)
				}
				ciks = append(ciks, p)
			}
			q := EFTSQuery{
				Q:     strings.Join(args, " "),
				Forms: parseCSV(formCSV),
				CIKs:  ciks,
				Start: start,
				End:   end,
			}
			var resp EFTSResponse
			if err := fetchSECJSON(c, q.URL(), &resp); err != nil {
				return classifyAPIError(err, flags)
			}
			hits := resp.Flatten()
			if limit > 0 && len(hits) > limit {
				hits = hits[:limit]
			}
			rows := make([]map[string]any, 0, len(hits))
			for _, h := range hits {
				row := map[string]any{
					"accession":  h.Accession,
					"form":       h.Form,
					"file_date":  h.FileDate,
					"period":     h.PeriodEnding,
					"display":    strings.Join(h.DisplayNames, " | "),
					"filing_url": h.FilingURL,
				}
				if len(h.Items) > 0 {
					row["items"] = strings.Join(h.Items, ",")
				}
				rows = append(rows, row)
			}
			meta := map[string]any{
				"total_hits": resp.Hits.Total.Value,
				"returned":   len(rows),
			}
			return printJSONOrTableWithMeta(cmd, flags, rows, meta)
		},
	}
	cmd.Flags().StringVar(&formCSV, "form", "", "Form types to restrict to (e.g. 10-K,10-Q,8-K)")
	cmd.Flags().StringVar(&cikCSV, "cik", "", "Restrict to these CIKs (comma-separated)")
	cmd.Flags().StringVar(&start, "start", "", "Earliest file date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&end, "end", "", "Latest file date (YYYY-MM-DD)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max hits to return (0 = no limit; EFTS still caps at ~10000)")
	return cmd
}

// ---------- feed latest (Atom getcurrent) ----------

var atomEntryRE = regexp.MustCompile(`(?s)<entry>(.*?)</entry>`)
var atomFieldRE = regexp.MustCompile(`(?s)<title>(.*?)</title>.*?<link rel="alternate".*?href="(.*?)".*?<summary[^>]*>(.*?)</summary>.*?<updated>(.*?)</updated>`)
var atomTitleFormRE = regexp.MustCompile(`^([A-Z0-9/.-]+(?:\s+[A-Z0-9/.-]+)*)\s+-\s+(.+?)\s+\((\d{10})\)\s+\((Issuer|Filer|Reporting)\)\s*$`)
var atomItemRE = regexp.MustCompile(`Items:?\s+([0-9.,\s]+)`)

func newFeedCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "feed",
		Short: "Latest filings feed (SEC Atom getcurrent)",
	}
	cmd.AddCommand(newFeedLatestCmd(flags))
	return cmd
}

func newFeedLatestCmd(flags *rootFlags) *cobra.Command {
	var (
		count int
		form  string
	)
	cmd := &cobra.Command{
		Use:         "latest",
		Short:       "Fetch the latest N filings from the live Atom feed",
		Example:     "  sec-edgar-pp-cli feed latest --count 20 --form 8-K --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			url := fmt.Sprintf("https://www.sec.gov/cgi-bin/browse-edgar?action=getcurrent&type=%s&owner=include&count=%d&output=atom",
				form, count)
			body, err := fetchSECRaw(c, url)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			entries := parseAtomFeed(string(body))
			return printJSONOrTable(cmd, flags, entries)
		},
	}
	cmd.Flags().IntVar(&count, "count", 40, "Number of entries to return (max 100)")
	cmd.Flags().StringVar(&form, "form", "", "Filter to one form type (e.g. 8-K) - applied server-side")
	return cmd
}

// parseAtomFeed extracts {form, company, cik, accession, filed, filing_url}
// rows from the SEC's getcurrent Atom XML using regex (kept dependency-free).
func parseAtomFeed(xml string) []map[string]any {
	out := []map[string]any{}
	for _, ent := range atomEntryRE.FindAllStringSubmatch(xml, -1) {
		block := ent[1]
		fields := atomFieldRE.FindStringSubmatch(block)
		if fields == nil {
			continue
		}
		title := htmlUnescape(fields[1])
		link := fields[2]
		summary := htmlUnescape(fields[3])
		updated := fields[4]
		row := map[string]any{
			"title":      title,
			"filing_url": link,
			"updated":    updated,
		}
		if m := atomTitleFormRE.FindStringSubmatch(title); m != nil {
			row["form"] = m[1]
			row["company"] = m[2]
			row["cik"] = m[3]
			row["role"] = m[4]
		}
		if accIdx := strings.Index(summary, "AccNo:"); accIdx > 0 {
			rest := strings.TrimSpace(summary[accIdx+len("AccNo:"):])
			if i := strings.IndexAny(rest, " <"); i > 0 {
				row["accession"] = rest[:i]
			}
		}
		if m := atomItemRE.FindStringSubmatch(summary); m != nil {
			row["items"] = strings.TrimSpace(strings.ReplaceAll(m[1], " ", ""))
		}
		if filedIdx := strings.Index(summary, "Filed:"); filedIdx > 0 {
			rest := strings.TrimSpace(summary[filedIdx+len("Filed:"):])
			if i := strings.IndexAny(rest, " <"); i > 0 {
				row["filed"] = rest[:i]
			}
		}
		out = append(out, row)
	}
	return out
}

func htmlUnescape(s string) string {
	r := strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", `"`,
		"&#39;", "'",
		"&apos;", "'",
		"&nbsp;", " ",
	)
	return r.Replace(s)
}

// ---------- status (operational health) ----------

func newStatusCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "status",
		Short:       "Probe data.sec.gov, www.sec.gov, and efts.sec.gov for reachability",
		Example:     "  sec-edgar-pp-cli status --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			rows := []map[string]any{}
			probes := []struct {
				name string
				url  string
			}{
				{"data.sec.gov", "https://data.sec.gov/submissions/CIK0000320193.json"},
				{"www.sec.gov", "https://www.sec.gov/files/company_tickers.json"},
				{"efts.sec.gov", "https://efts.sec.gov/LATEST/search-index?q=apple&forms=10-K&from=0"},
			}
			for _, p := range probes {
				start := time.Now()
				_, err := c.Get(p.url, nil)
				ms := time.Since(start).Milliseconds()
				status := "ok"
				detail := ""
				if err != nil {
					status = "error"
					detail = err.Error()
				}
				rows = append(rows, map[string]any{
					"host":       p.name,
					"status":     status,
					"latency_ms": ms,
					"detail":     detail,
				})
			}
			return printJSONOrTable(cmd, flags, rows)
		},
	}
	return cmd
}

// ---------- sic show ----------

// sicTable is a curated subset of the most common SIC codes plus their
// titles. The full SEC list has ~440 codes; this slice covers ~the top 80
// by filing volume across NYSE/Nasdaq tickers (manually compiled from
// www.sec.gov/info/edgar/siccodes.htm). Use this for offline lookups;
// the values shipping in research-grade reference data should be
// considered authoritative.
var sicTable = map[string]string{
	"0100": "Agricultural Production - Crops",
	"0200": "Agricultural Production - Livestock",
	"0700": "Agricultural Services",
	"0800": "Forestry",
	"0900": "Fishing, Hunting & Trapping",
	"1000": "Metal Mining",
	"1040": "Gold Mining",
	"1311": "Crude Petroleum & Natural Gas",
	"1389": "Services-Oil & Gas Field Services",
	"1400": "Mining & Quarrying of Nonmetallic Minerals",
	"1531": "Operative Builders",
	"1623": "Water, Sewer, Pipeline, Construction",
	"1731": "Electrical Work",
	"2000": "Food & Kindred Products",
	"2024": "Ice Cream & Frozen Desserts",
	"2080": "Beverages",
	"2086": "Bottled & Canned Soft Drinks",
	"2090": "Food & Kindred Products NEC",
	"2200": "Textile Mill Products",
	"2300": "Apparel & Other Finished Products",
	"2400": "Lumber & Wood Products",
	"2500": "Furniture & Fixtures",
	"2600": "Paper & Allied Products",
	"2700": "Printing & Publishing",
	"2731": "Books: Publishing or Publishing & Printing",
	"2834": "Pharmaceutical Preparations",
	"2836": "Biological Products (No Diagnostic Substances)",
	"2840": "Soap, Detergents, Cleansing Preparations",
	"2860": "Industrial Organic Chemicals",
	"2911": "Petroleum Refining",
	"3140": "Footwear (No Rubber)",
	"3310": "Steel Works, Blast Furnaces & Rolling Mills",
	"3334": "Primary Production of Aluminum",
	"3411": "Metal Cans",
	"3559": "Special Industry Machinery NEC",
	"3571": "Electronic Computers",
	"3572": "Computer Storage Devices",
	"3576": "Computer Communications Equipment",
	"3577": "Computer Peripheral Equipment NEC",
	"3585": "Refrigeration & Service Industry Machinery",
	"3651": "Household Audio & Video Equipment",
	"3661": "Telephone & Telegraph Apparatus",
	"3663": "Radio & TV Broadcasting & Communications Equipment",
	"3669": "Communications Equipment NEC",
	"3672": "Printed Circuit Boards",
	"3674": "Semiconductors & Related Devices",
	"3711": "Motor Vehicles & Passenger Car Bodies",
	"3714": "Motor Vehicle Parts & Accessories",
	"3721": "Aircraft",
	"3812": "Search, Detection, Navigation, Guidance Instruments",
	"3826": "Laboratory Analytical Instruments",
	"3841": "Surgical & Medical Instruments & Apparatus",
	"3845": "Electromedical & Electrotherapeutic Apparatus",
	"4011": "Railroads, Line-Haul Operating",
	"4213": "Trucking (No Local)",
	"4412": "Deep Sea Foreign Transportation of Freight",
	"4512": "Air Transportation, Scheduled",
	"4813": "Telephone Communications (No Radiotelephone)",
	"4832": "Radio Broadcasting Stations",
	"4833": "Television Broadcasting Stations",
	"4899": "Communications Services NEC",
	"4911": "Electric Services",
	"4922": "Natural Gas Transmission",
	"4924": "Natural Gas Distribution",
	"4931": "Electric & Other Services Combined",
	"4955": "Hazardous Waste Management",
	"5000": "Wholesale-Durable Goods",
	"5045": "Wholesale-Computers & Computer Peripheral Equipment",
	"5172": "Wholesale-Petroleum & Petroleum Products",
	"5200": "Retail-Building Materials, Hardware, Garden Supply",
	"5311": "Retail-Variety Stores",
	"5331": "Retail-Drug Stores & Proprietary Stores",
	"5400": "Retail-Food Stores",
	"5411": "Retail-Grocery Stores",
	"5412": "Retail-Convenience Stores",
	"5500": "Retail-Auto Dealers & Gasoline Stations",
	"5651": "Retail-Family Clothing Stores",
	"5700": "Retail-Home Furniture, Furnishings & Equipment Stores",
	"5731": "Retail-Radio, TV, Consumer Electronics Stores",
	"5812": "Retail-Eating Places",
	"5912": "Retail-Drug Stores And Proprietary Stores",
	"5961": "Retail-Catalog, Mail-Order Houses",
	"5990": "Retail-Retail Stores NEC",
	"6020": "State Commercial Banks",
	"6021": "National Commercial Banks",
	"6022": "State Commercial Banks NEC",
	"6029": "Commercial Banks NEC",
	"6035": "Savings Institution, Federally Chartered",
	"6099": "Functions Related to Depository Banking NEC",
	"6141": "Personal Credit Institutions",
	"6153": "Short-Term Business Credit Institutions",
	"6189": "Asset-Backed Securities",
	"6199": "Finance Services",
	"6200": "Security & Commodity Brokers, Dealers, Exchanges",
	"6211": "Security Brokers, Dealers, and Flotation Companies",
	"6221": "Commodity Contracts Brokers and Dealers",
	"6311": "Life Insurance",
	"6321": "Accident & Health Insurance",
	"6331": "Fire, Marine, Casualty Insurance",
	"6411": "Insurance Agents, Brokers, Service",
	"6500": "Real Estate",
	"6770": "Blank Checks",
	"6798": "Real Estate Investment Trusts",
	"7000": "Hotels, Rooming Houses, Camps, Other Lodging",
	"7011": "Hotels & Motels",
	"7200": "Services-Personal Services",
	"7311": "Services-Advertising",
	"7370": "Services-Computer Services",
	"7371": "Services-Computer Services",
	"7372": "Services-Prepackaged Software",
	"7374": "Services-Computer Processing & Data Preparation",
	"7389": "Services-Business Services NEC",
	"7812": "Services-Motion Picture & Video Tape Production",
	"7841": "Services-Video Tape Rental",
	"8000": "Services-Health Services",
	"8011": "Services-Offices & Clinics of Doctors of Medicine",
	"8060": "Services-Hospitals & Medical Service Plans",
	"8071": "Services-Medical Laboratories",
	"8082": "Services-Home Health Care Services",
	"8200": "Services-Educational Services",
	"8300": "Services-Social Services",
	"8741": "Services-Management Services",
	"8742": "Services-Management Consulting Services",
	"8744": "Services-Facilities Support Management Services",
	"8888": "Foreign Governments",
	"9995": "Non-Classifiable Establishments",
}

func newSicCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sic",
		Short: "Standard Industrial Classification (SIC) code lookup",
	}
	cmd.AddCommand(newSicShowCmd(flags))
	return cmd
}

func newSicShowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "show <code>",
		Short:       "Print the title for one SIC code (e.g. 3571 -> Electronic Computers)",
		Example:     "  sec-edgar-pp-cli sic show 7372 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			code := strings.TrimSpace(args[0])
			pad := code
			for len(pad) < 4 {
				pad = "0" + pad
			}
			title, ok := sicTable[pad]
			if !ok {
				return usageErr(fmt.Errorf("SIC code %q not in embedded table; see https://www.sec.gov/info/edgar/siccodes.htm for the full list", code))
			}
			out := map[string]any{"sic": pad, "title": title}
			return printJSONOrTable(cmd, flags, []map[string]any{out})
		},
	}
	return cmd
}

// ---------- facts statement ----------
//
// Pulls /api/xbrl/companyfacts/CIK<cik>.json and groups facts into one of
// the three standard us-gaap statement shapes (balance sheet, income, cash
// flow) using a curated tag set.

var statementTags = map[string][]string{
	"balance": {
		"Assets", "AssetsCurrent", "CashAndCashEquivalentsAtCarryingValue",
		"Inventory", "AccountsReceivableNetCurrent",
		"Liabilities", "LiabilitiesCurrent", "AccountsPayableCurrent",
		"LongTermDebt", "LongTermDebtNoncurrent",
		"StockholdersEquity", "RetainedEarningsAccumulatedDeficit",
		"CommonStockValue",
	},
	"income": {
		"Revenues", "RevenueFromContractWithCustomerExcludingAssessedTax",
		"CostOfRevenue", "GrossProfit",
		"OperatingExpenses", "OperatingIncomeLoss",
		"InterestExpense", "IncomeTaxExpenseBenefit",
		"NetIncomeLoss",
		"EarningsPerShareBasic", "EarningsPerShareDiluted",
	},
	"cashflow": {
		"NetCashProvidedByUsedInOperatingActivities",
		"NetCashProvidedByUsedInInvestingActivities",
		"NetCashProvidedByUsedInFinancingActivities",
		"DepreciationDepletionAndAmortization",
		"CashAndCashEquivalentsPeriodIncreaseDecrease",
		"PaymentsToAcquirePropertyPlantAndEquipment",
		"PaymentsOfDividends",
	},
}

type companyFacts struct {
	CIK        int                              `json:"cik"`
	EntityName string                           `json:"entityName"`
	Facts      map[string]map[string]factDetail `json:"facts"`
}
type factDetail struct {
	Label       string                       `json:"label"`
	Description string                       `json:"description"`
	Units       map[string][]factObservation `json:"units"`
}
type factObservation struct {
	Start string  `json:"start"`
	End   string  `json:"end"`
	Val   float64 `json:"val"`
	Accn  string  `json:"accn"`
	FY    int     `json:"fy"`
	FP    string  `json:"fp"`
	Form  string  `json:"form"`
	Filed string  `json:"filed"`
	Frame string  `json:"frame,omitempty"`
}

func newFactsStatementCmd(flags *rootFlags) *cobra.Command {
	var (
		cikRaw     string
		kind       string
		periodsRaw string
	)
	cmd := &cobra.Command{
		Use:         "statement",
		Short:       "Print balance sheet, income statement, or cash-flow XBRL facts for the last N periods",
		Example:     "  sec-edgar-pp-cli facts statement --cik 0000320193 --kind income --periods last4 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if strings.TrimSpace(cikRaw) == "" {
				return usageErr(fmt.Errorf("--cik is required"))
			}
			cik, err := padCIK(cikRaw)
			if err != nil {
				return usageErr(err)
			}
			tags, ok := statementTags[strings.ToLower(kind)]
			if !ok {
				return usageErr(fmt.Errorf("--kind must be one of: balance, income, cashflow"))
			}
			periods, err := parsePeriodsCount(periodsRaw)
			if err != nil {
				return usageErr(err)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			url := fmt.Sprintf("https://data.sec.gov/api/xbrl/companyfacts/CIK%s.json", cik)
			var cf companyFacts
			if err := fetchSECJSON(c, url, &cf); err != nil {
				return classifyAPIError(err, flags)
			}
			rows := []map[string]any{}
			usgaap := cf.Facts["us-gaap"]
			for _, tag := range tags {
				fd, exists := usgaap[tag]
				if !exists {
					continue
				}
				var usd []factObservation
				if obs, ok := fd.Units["USD"]; ok {
					usd = obs
				} else if obs, ok := fd.Units["USD/shares"]; ok {
					usd = obs
				} else {
					for _, obs := range fd.Units {
						usd = obs
						break
					}
				}
				if len(usd) == 0 {
					continue
				}
				sort.SliceStable(usd, func(i, j int) bool {
					return usd[i].End > usd[j].End
				})
				take := usd
				if periods > 0 && len(take) > periods {
					take = take[:periods]
				}
				for _, o := range take {
					rows = append(rows, map[string]any{
						"tag":       tag,
						"label":     fd.Label,
						"end":       o.End,
						"value":     o.Val,
						"fiscal":    fmt.Sprintf("%d-%s", o.FY, o.FP),
						"form":      o.Form,
						"accession": o.Accn,
						"filed":     o.Filed,
					})
				}
			}
			meta := map[string]any{
				"company": cf.EntityName,
				"cik":     cik,
				"kind":    kind,
			}
			return printJSONOrTableWithMeta(cmd, flags, rows, meta)
		},
	}
	cmd.Flags().StringVar(&cikRaw, "cik", "", "10-digit zero-padded CIK (required)")
	cmd.Flags().StringVar(&kind, "kind", "income", "Statement kind: balance | income | cashflow")
	cmd.Flags().StringVar(&periodsRaw, "periods", "last4", "Most recent N periods per tag: 'lastN', integer 'N', or 0/all for unlimited")
	return cmd
}

// parsePeriodsCount accepts "lastN", a bare integer "N", or "all"/"0" for
// unlimited, returning the int N (0 means no truncation).
func parsePeriodsCount(raw string) (int, error) {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" || s == "all" || s == "0" {
		return 0, nil
	}
	if strings.HasPrefix(s, "last") {
		s = strings.TrimPrefix(s, "last")
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("--periods must be 'lastN', integer N, or 'all'; got %q", raw)
	}
	if n < 0 {
		return 0, fmt.Errorf("--periods must be non-negative; got %d", n)
	}
	return n, nil
}

// ---------- shared output helpers ----------

func printJSONOrTable(cmd *cobra.Command, flags *rootFlags, rows []map[string]any) error {
	return printJSONOrTableWithMeta(cmd, flags, rows, nil)
}

func printJSONOrTableWithMeta(cmd *cobra.Command, flags *rootFlags, rows []map[string]any, meta map[string]any) error {
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		out := map[string]any{
			"results": rows,
		}
		if len(meta) > 0 {
			out["meta"] = meta
		}
		buf, err := json.Marshal(out)
		if err != nil {
			return err
		}
		if flags.selectFields != "" {
			// filterFields operates on the inner "results" array
			inner, _ := json.Marshal(rows)
			filtered := filterFields(inner, flags.selectFields)
			out["results"] = json.RawMessage(filtered)
			buf, err = json.Marshal(out)
			if err != nil {
				return err
			}
		}
		return printOutput(cmd.OutOrStdout(), buf, true)
	}
	if len(rows) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "(no rows)")
		return nil
	}
	return printAutoTable(cmd.OutOrStdout(), rows)
}

// Compile-time check that strconv is referenced from non-padCIK paths.
var _ = strconv.Itoa
