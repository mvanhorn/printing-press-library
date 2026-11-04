package cli

// pp:data-source live

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/internal/cliutil"
	"github.com/spf13/cobra"
)

// SEC requests share one conservative limiter across endpoint families.
var secLimiter = cliutil.NewAdaptiveLimiter(2)
var secDigits = regexp.MustCompile(`^[0-9]{1,10}$`)
var secAccession = regexp.MustCompile(`^[0-9]{10}-[0-9]{2}-[0-9]{6}$`)
var secConcept = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)

func secFetch(ctx context.Context, u string) ([]byte, error) {
	parsed, parseErr := url.Parse(u)
	if parseErr != nil || parsed.Scheme != "https" || !secHost(parsed.Hostname()) {
		return nil, fmt.Errorf("SEC requests require an official HTTPS SEC host")
	}
	ua := strings.TrimSpace(os.Getenv("SEC_USER_AGENT"))
	if ua == "" {
		return nil, usageErr(fmt.Errorf("set SEC_USER_AGENT to your application name and contact email before SEC requests"))
	}
	httpClient, limiter := secRequestSettings(ctx)
	for attempt := 0; attempt < 3; attempt++ {
		if err := limiter.Wait(ctx); err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", ua)
		req.Header.Set("Accept", "application/json,text/html,*/*")
		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == 429 {
			retry := cliutil.RetryAfter(resp)
			_ = resp.Body.Close()
			limiter.OnRateLimit()
			if attempt == 2 {
				return nil, &cliutil.RateLimitError{URL: u, RetryAfter: retry}
			}
			if retry < time.Second {
				retry = time.Second
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(retry):
			}
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("SEC HTTP %d for %s; check SEC_USER_AGENT and retry later for fair-access blocks", resp.StatusCode, u)
		}
		b, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20+1))
		closeErr := resp.Body.Close()
		if err == nil {
			err = closeErr
		}
		if len(b) > 64<<20 {
			return nil, fmt.Errorf("SEC response exceeded 64 MiB limit")
		}
		if err == nil {
			limiter.OnSuccess()
		}
		return b, err
	}
	return nil, fmt.Errorf("SEC retry limit reached")
}
func secJSON(ctx context.Context, u string) (map[string]any, error) {
	b, e := secFetch(ctx, u)
	if e != nil {
		return nil, e
	}
	var d map[string]any
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.UseNumber()
	e = decoder.Decode(&d)
	if e != nil {
		return nil, e
	}
	if v, ok := d["errorType"]; ok {
		return nil, fmt.Errorf("SEC returned application error %v; retry later", v)
	}
	if v, ok := d["error"]; ok {
		return nil, fmt.Errorf("SEC returned application error: %v", v)
	}
	return d, e
}
func secCIK(ctx context.Context, input string) (string, error) {
	v := strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(input)), "CIK")
	if secDigits.MatchString(v) {
		n, _ := strconv.ParseUint(v, 10, 64)
		if n == 0 {
			return "", usageErr(fmt.Errorf("CIK must be positive"))
		}
		return fmt.Sprintf("%010d", n), nil
	}
	if v == "" {
		return "", usageErr(fmt.Errorf("provide a ticker or CIK, for example AAPL or 0000320193"))
	}
	d, e := secJSON(ctx, "https://www.sec.gov/files/company_tickers.json")
	if e != nil {
		return "", e
	}
	for _, r := range d {
		if m, ok := r.(map[string]any); ok && strings.EqualFold(fmt.Sprint(m["ticker"]), v) {
			n, err := strconv.ParseUint(fmt.Sprint(m["cik_str"]), 10, 64)
			if err != nil || n == 0 {
				return "", fmt.Errorf("invalid CIK in SEC ticker directory")
			}
			return fmt.Sprintf("%010d", n), nil
		}
	}
	return "", fmt.Errorf("ticker %q not found in SEC ticker directory; pass a CIK instead", input)
}
func secSubmission(ctx context.Context, cik string) (map[string]any, error) {
	return secJSON(ctx, "https://data.sec.gov/submissions/CIK"+cik+".json")
}
func secDate(v string) error {
	if v != "" {
		if _, e := time.Parse("2006-01-02", v); e != nil {
			return usageErr(fmt.Errorf("date %q must use YYYY-MM-DD", v))
		}
	}
	return nil
}
func secRange(start, end string) error {
	if e := secDate(start); e != nil {
		return e
	}
	if e := secDate(end); e != nil {
		return e
	}
	if start != "" && end != "" && start > end {
		return usageErr(fmt.Errorf("--start must not follow --end"))
	}
	return nil
}
func secRows(m map[string]any) []map[string]any {
	a, _ := m["accessionNumber"].([]any)
	out := make([]map[string]any, 0, len(a))
	for i := range a {
		r := map[string]any{}
		for k, v := range m {
			if arr, ok := v.([]any); ok && i < len(arr) {
				r[k] = arr[i]
			}
		}
		out = append(out, r)
	}
	return out
}
func init() {
	registerNovelCommand(func(root *cobra.Command, f *rootFlags) { root.AddCommand(newGovEdgar(f)) })
}
func newGovEdgar(f *rootFlags) *cobra.Command {
	parent := &cobra.Command{Use: "edgar", Short: "Search SEC filings and retrieve company financial facts"}
	makeCmd := func(use, short, example string, run func(*cobra.Command, []string) error) *cobra.Command {
		c := &cobra.Command{Use: use, Short: short, Example: "  government-data-pp-cli " + example, Annotations: map[string]string{"pp:data-source": "live", "mcp:read-only": "true"}}
		c.RunE = func(c *cobra.Command, a []string) error {
			if dryRunOK(f) {
				return writeDryRun(c.OutOrStdout(), f, c.CommandPath())
			}
			h, err := govHTTPClient(f)
			if err != nil {
				return err
			}
			c.SetContext(withSECSettings(c.Context(), h, f.rateLimit))
			return run(c, a)
		}
		return c
	}
	company := &cobra.Command{Use: "company [ticker-or-cik]"}
	*company = *makeCmd("company [ticker-or-cik]", "Resolve a company and show SEC submission metadata", "edgar company 0000320193 --json", func(c *cobra.Command, a []string) error {
		if len(a) != 1 {
			return usageErr(fmt.Errorf("usage: edgar company AAPL or edgar company 0000320193"))
		}
		ctx, cancel := boundCtx(c.Context(), f)
		defer cancel()
		cik, e := secCIK(ctx, a[0])
		if e != nil {
			return e
		}
		d, e := secSubmission(ctx, cik)
		if e != nil {
			return e
		}
		delete(d, "filings")
		d["source"] = "sec-edgar"
		d["source_url"] = "https://data.sec.gov/submissions/CIK" + cik + ".json"
		return f.printJSON(c, d)
	})
	parent.AddCommand(company)
	var forms, start, end string
	var limit, maxPages int
	search := &cobra.Command{Use: "search [query]"}
	*search = *makeCmd("search [query]", "Search filing text with bounded pages and source provenance", "edgar search water --forms 10-K --start 2025-01-01 --end 2025-01-31 --limit 5 --json", func(c *cobra.Command, a []string) error {
		if len(a) != 1 || strings.TrimSpace(a[0]) == "" {
			return usageErr(fmt.Errorf("provide a filing text query"))
		}
		if e := secRange(start, end); e != nil {
			return e
		}
		if limit < 1 || limit > 1000 || maxPages < 1 || maxPages > 100 {
			return usageErr(fmt.Errorf("--limit must be 1..1000 and --max-scan-pages 1..100"))
		}
		ctx, cancel := boundCtx(c.Context(), f)
		defer cancel()
		out := []any{}
		scanned := 0
		total := any(nil)
		timedOut := false
		for page := 0; page < maxPages && len(out) < limit; page++ {
			n := min(100, limit-len(out))
			q := url.Values{"q": {a[0]}, "from": {strconv.Itoa(page * 100)}, "size": {strconv.Itoa(n)}}
			if forms != "" {
				q.Set("forms", forms)
			}
			if start != "" {
				q.Set("startdt", start)
			}
			if end != "" {
				q.Set("enddt", end)
			}
			d, e := secJSON(ctx, "https://efts.sec.gov/LATEST/search-index?"+q.Encode())
			if e != nil {
				return e
			}
			scanned++
			h, ok := d["hits"].(map[string]any)
			if !ok {
				return fmt.Errorf("SEC search returned no hits envelope; retry later")
			}
			rows, _ := h["hits"].([]any)
			total = h["total"]
			if b, _ := d["timed_out"].(bool); b {
				timedOut = true
			}
			out = append(out, rows...)
			if len(rows) < n {
				break
			}
		}
		if len(out) > limit {
			out = out[:limit]
		}
		return f.printJSON(c, map[string]any{"results": out, "scanned_pages": scanned, "total": total, "timed_out": timedOut, "max_scan_pages": maxPages, "note": "Bounded SEC full-text results; total may be a lower bound. Filing documents may yield multiple hits."})
	})
	search.Flags().StringVar(&forms, "forms", "", "Comma-separated SEC form types such as 10-K,10-Q")
	search.Flags().StringVar(&start, "start", "", "First filing date in YYYY-MM-DD format")
	search.Flags().StringVar(&end, "end", "", "Last filing date in YYYY-MM-DD format")
	search.Flags().IntVar(&limit, "limit", 10, "Maximum filing search hits to return")
	search.Flags().IntVar(&maxPages, "max-scan-pages", 3, "Maximum SEC result pages to examine")
	parent.AddCommand(search)
	var filingForm, filingStart, filingEnd string
	var filingLimit, filingPages int
	filings := &cobra.Command{Use: "filings [ticker-or-cik]"}
	*filings = *makeCmd("filings [ticker-or-cik]", "List recent and bounded historical company filings", "edgar filings 0000320193 --form 10-K --limit 5 --json", func(c *cobra.Command, a []string) error {
		if len(a) != 1 {
			return usageErr(fmt.Errorf("provide a ticker or CIK"))
		}
		if e := secRange(filingStart, filingEnd); e != nil {
			return e
		}
		if filingLimit < 1 || filingLimit > 10000 || filingPages < 1 || filingPages > 100 {
			return usageErr(fmt.Errorf("--limit must be 1..10000 and --max-scan-pages 1..100"))
		}
		ctx, cancel := boundCtx(c.Context(), f)
		defer cancel()
		cik, e := secCIK(ctx, a[0])
		if e != nil {
			return e
		}
		d, e := secSubmission(ctx, cik)
		if e != nil {
			return e
		}
		fs, _ := d["filings"].(map[string]any)
		recent, _ := fs["recent"].(map[string]any)
		all := secRows(recent)
		files, _ := fs["files"].([]any)
		scanned := 1
		out := []map[string]any{}
		accept := func(rows []map[string]any) {
			for _, r := range rows {
				date, _ := r["filingDate"].(string)
				form, _ := r["form"].(string)
				if filingForm != "" && form != filingForm || filingStart != "" && date < filingStart || filingEnd != "" && date > filingEnd {
					continue
				}
				r["cik"] = cik
				acc, _ := r["accessionNumber"].(string)
				doc, _ := r["primaryDocument"].(string)
				if secAccession.MatchString(acc) && doc != "" {
					r["primary_document_url"] = secArchive(cik, acc) + url.PathEscape(doc)
				}
				out = append(out, r)
				if len(out) >= filingLimit {
					return
				}
			}
		}
		accept(all)
		for _, v := range files {
			if scanned >= filingPages || len(out) >= filingLimit {
				break
			}
			m, _ := v.(map[string]any)
			name, _ := m["name"].(string)
			if filepath.Base(name) != name || !strings.HasSuffix(name, ".json") {
				return fmt.Errorf("invalid SEC submissions history filename")
			}
			older, e := secJSON(ctx, "https://data.sec.gov/submissions/"+url.PathEscape(name))
			if e != nil {
				return e
			}
			scanned++
			accept(secRows(older))
		}
		return f.printJSON(c, map[string]any{"cik": cik, "results": out, "scanned_pages": scanned, "max_scan_pages": filingPages, "historical_files_available": len(files), "note": "Bounded submission history; widen --max-scan-pages and --limit for older filings."})
	})
	filings.Flags().StringVar(&filingForm, "form", "", "Exact SEC filing form such as 10-K")
	filings.Flags().StringVar(&filingStart, "start", "", "Earliest filing date in YYYY-MM-DD format")
	filings.Flags().StringVar(&filingEnd, "end", "", "Latest filing date in YYYY-MM-DD format")
	filings.Flags().IntVar(&filingLimit, "limit", 20, "Maximum matching company filings to return")
	filings.Flags().IntVar(&filingPages, "max-scan-pages", 3, "Maximum submissions files including recent history")
	parent.AddCommand(filings)
	var taxonomy, concept string
	facts := &cobra.Command{Use: "facts [ticker-or-cik]"}
	*facts = *makeCmd("facts [ticker-or-cik]", "Get company XBRL facts preserving units and filing context", "edgar facts 0000320193 --concept Assets --json", func(c *cobra.Command, a []string) error {
		if len(a) != 1 {
			return usageErr(fmt.Errorf("provide a ticker or CIK"))
		}
		if !secConcept.MatchString(taxonomy) || concept != "" && !secConcept.MatchString(concept) {
			return usageErr(fmt.Errorf("taxonomy and concept must be simple XBRL names"))
		}
		ctx, cancel := boundCtx(c.Context(), f)
		defer cancel()
		cik, e := secCIK(ctx, a[0])
		if e != nil {
			return e
		}
		u := "https://data.sec.gov/api/xbrl/companyfacts/CIK" + cik + ".json"
		if concept != "" {
			u = "https://data.sec.gov/api/xbrl/companyconcept/CIK" + cik + "/" + taxonomy + "/" + concept + ".json"
		}
		d, e := secJSON(ctx, u)
		if e != nil {
			return e
		}
		d["source_url"] = u
		return f.printJSON(c, d)
	})
	facts.Flags().StringVar(&taxonomy, "taxonomy", "us-gaap", "XBRL taxonomy such as us-gaap or dei")
	facts.Flags().StringVar(&concept, "concept", "", "Optional exact XBRL concept such as Assets")
	parent.AddCommand(facts)
	var accession, document, output string
	documents := &cobra.Command{Use: "documents [ticker-or-cik]"}
	*documents = *makeCmd("documents [ticker-or-cik]", "List filing archive documents or download a selected document", "edgar documents 0000320193 --accession 0000320193-25-000079 --json", func(c *cobra.Command, a []string) error {
		if len(a) != 1 || !secAccession.MatchString(accession) {
			return usageErr(fmt.Errorf("provide ticker/CIK and --accession in 0000320193-25-000079 format"))
		}
		if (document == "") != (output == "") {
			return usageErr(fmt.Errorf("download requires both --document (exact listed name) and --output"))
		}
		ctx, cancel := boundCtx(c.Context(), f)
		defer cancel()
		cik, e := secCIK(ctx, a[0])
		if e != nil {
			return e
		}
		base := secArchive(cik, accession)
		d, e := secJSON(ctx, base+"index.json")
		if e != nil {
			return e
		}
		if document == "" {
			d["source_url"] = base
			return f.printJSON(c, d)
		}
		dir, _ := d["directory"].(map[string]any)
		items, _ := dir["item"].([]any)
		found := false
		for _, v := range items {
			m, _ := v.(map[string]any)
			if m["name"] == document {
				found = true
			}
		}
		if !found || filepath.Base(document) != document {
			return usageErr(fmt.Errorf("--document must exactly match a document listed by edgar documents"))
		}
		b, e := secFetch(ctx, base+url.PathEscape(document))
		if e != nil {
			return e
		}
		file, e := os.CreateTemp(filepath.Dir(output), ".sec-download-*")
		if e != nil {
			return e
		}
		tmp := file.Name()
		defer os.Remove(tmp)
		_, e = file.Write(b)
		ce := file.Close()
		if e != nil {
			return e
		}
		if ce != nil {
			return ce
		}
		if e = os.Link(tmp, output); e != nil {
			return fmt.Errorf("save download without overwrite: %w", e)
		}
		return f.printJSON(c, map[string]any{"output": output, "bytes": len(b), "source_url": base + url.PathEscape(document)})
	})
	documents.Annotations["mcp:read-only-unless-flags"] = "document,output"
	delete(documents.Annotations, "mcp:read-only")
	documents.Flags().StringVar(&accession, "accession", "", "SEC accession number identifying one filing")
	documents.Flags().StringVar(&document, "document", "", "Exact archive document filename to download")
	documents.Flags().StringVar(&output, "output", "", "New destination file; existing files are never overwritten")
	parent.AddCommand(documents)
	return parent
}
func secArchive(cik, acc string) string {
	n, _ := strconv.ParseUint(cik, 10, 64)
	return "https://www.sec.gov/Archives/edgar/data/" + strconv.FormatUint(n, 10) + "/" + strings.ReplaceAll(acc, "-", "") + "/"
}

func secHost(host string) bool {
	return host == "www.sec.gov" || host == "data.sec.gov" || host == "efts.sec.gov"
}
