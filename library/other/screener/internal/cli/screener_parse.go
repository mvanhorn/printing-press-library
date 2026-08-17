// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command support: Screener.in HTML page parsing shared by the
// hand-authored compare/qtrend/overlap/rank/insider-flow commands.
// generate --force preserves implemented bodies.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/screener/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/screener/internal/cliutil"
)

// screenerTopRatios maps a top-ratios label to its value.
// Parsed from the #top-ratios block: "Market Cap ₹ 4,75,422 Cr. Current
// Price ₹ 1,172 High / Low ₹ 1,728 / 982 Stock P/E 15.2 ..."
type screenerTopRatios struct {
	MarketCap     float64 `json:"market_cap_cr,omitempty"`
	CurrentPrice  float64 `json:"current_price,omitempty"`
	HighLow       string  `json:"high_low,omitempty"`
	StockPE       float64 `json:"stock_pe,omitempty"`
	BookValue     float64 `json:"book_value,omitempty"`
	DividendYield float64 `json:"dividend_yield_pct,omitempty"`
	ROCE          float64 `json:"roce_pct,omitempty"`
	ROE           float64 `json:"roe_pct,omitempty"`
	FaceValue     float64 `json:"face_value,omitempty"`
}

// screenerFinRow is one labelled row of a financial table (quarterly,
// P&L, balance sheet, cash flow, ratios, shareholding). Values are
// keyed by the period column (e.g. "Jun 2026" or "Mar 2026").
type screenerFinRow struct {
	Label  string             `json:"label"`
	Values map[string]float64 `json:"values,omitempty"`
	Raw    map[string]string  `json:"raw,omitempty"`
}

// screenerFinTable is a parsed financial table with its period headers.
type screenerFinTable struct {
	Title   string           `json:"title,omitempty"`
	Periods []string         `json:"periods,omitempty"`
	Rows    []screenerFinRow `json:"rows"`
}

// screenerAnalysis is the machine-generated pros/cons block.
type screenerAnalysis struct {
	Pros []string `json:"pros,omitempty"`
	Cons []string `json:"cons,omitempty"`
}

// screenerShareholding is the latest shareholding pattern row (percentages).
type screenerShareholding struct {
	Promoters    float64 `json:"promoters_pct,omitempty"`
	FIIs         float64 `json:"fii_pct,omitempty"`
	DIIs         float64 `json:"dii_pct,omitempty"`
	Government   float64 `json:"government_pct,omitempty"`
	Public       float64 `json:"public_pct,omitempty"`
	Others       float64 `json:"others_pct,omitempty"`
	Shareholders float64 `json:"shareholders_count,omitempty"`
}

var (
	reTopRatioBlock   = regexp.MustCompile(`(?s)id="top-ratios".*?</ul>`)
	reFinSection      = regexp.MustCompile(`(?s)<section id="([a-z0-9-]+)".*?</section>`)
	reFinTable        = regexp.MustCompile(`(?s)<table[^>]*>(.*?)</table>`)
	reFinRows         = regexp.MustCompile(`(?s)<tr[^>]*>(.*?)</tr>`)
	reFinCells        = regexp.MustCompile(`(?s)<t[dh][^>]*>(.*?)</t[dh]>`)
	reStripTags       = regexp.MustCompile(`<[^>]+>`)
	reStripWhitespace = regexp.MustCompile(`\s+`)
	reNumber          = regexp.MustCompile(`-?[\d,]+(?:\.\d+)?`)
)

// parseScreenerTopRatios extracts the key-metrics header block.
func parseScreenerTopRatios(html string) screenerTopRatios {
	var out screenerTopRatios
	m := reTopRatioBlock.FindString(html)
	if m == "" {
		return out
	}
	// Each ratio is an <li> with a <span class="name"> label and a
	// value span. Parse per-li to keep label/value pairs aligned.
	lis := regexp.MustCompile(`(?s)<li[^>]*>(.*?)</li>`).FindAllStringSubmatch(m, -1)
	for _, li := range lis {
		body := li[1]
		nameRe := regexp.MustCompile(`(?s)<span class="name"[^>]*>(.*?)</span>`)
		nm := nameRe.FindStringSubmatch(body)
		if len(nm) != 2 {
			continue
		}
		label := strings.ToLower(cleanText(nm[1]))
		valText := cleanText(nameRe.ReplaceAllString(body, " "))
		val := parseNum(valText)
		switch {
		case label == "market cap":
			out.MarketCap = val
		case label == "current price":
			out.CurrentPrice = val
		case label == "high / low":
			out.HighLow = strings.TrimSpace(valText)
		case label == "stock p/e":
			out.StockPE = val
		case label == "book value":
			out.BookValue = val
		case label == "dividend yield":
			out.DividendYield = val
		case label == "roce":
			out.ROCE = val
		case label == "roe":
			out.ROE = val
		case label == "face value":
			out.FaceValue = val
		}
	}
	return out
}

// parseScreenerFinTable parses the first data table inside a section.
func parseScreenerFinTable(html string, sectionID string) screenerFinTable {
	var out screenerFinTable
	out.Rows = make([]screenerFinRow, 0)
	// Fall back to scanning the whole page if the section anchor is absent.
	sectionHTML := html
	for _, m := range reFinSection.FindAllStringSubmatch(html, -1) {
		if len(m) == 2 && m[1] == sectionID {
			sectionHTML = m[0]
			break
		}
	}
	if sectionID != "" && sectionHTML == html {
		alt := regexp.MustCompile(`(?s)id="` + regexp.QuoteMeta(sectionID) + `"`).FindString(html)
		if alt != "" {
			sectionHTML = alt
		}
	}
	tbl := reFinTable.FindString(sectionHTML)
	if tbl == "" {
		return out
	}
	trs := reFinRows.FindAllStringSubmatch(tbl, -1)
	for _, tr := range trs {
		cells := reFinCells.FindAllStringSubmatch(tr[1], -1)
		if len(cells) == 0 {
			continue
		}
		first := cleanText(cells[0][1])
		isHeader := strings.EqualFold(first, "S.No.") || strings.Contains(strings.ToLower(first), "particulars") || first == "Label" || first == "Company" || first == "Name"
		allTH := true
		for _, c := range cells {
			if !strings.HasPrefix(c[0], "<th") {
				allTH = false
				break
			}
		}
		if allTH && len(cells) > 1 {
			for _, c := range cells {
				h := cleanText(c[1])
				if h == "" || h == "S.No." || strings.EqualFold(h, "Particulars") || h == "Label" || h == "Company" || h == "Name" {
					continue
				}
				out.Periods = append(out.Periods, h)
			}
			continue
		}
		if isHeader && len(cells) > 1 {
			continue
		}
		if len(cells) < 2 {
			continue
		}
		label := cleanText(cells[0][1])
		if label == "" || strings.EqualFold(label, "No.") {
			continue
		}
		row := screenerFinRow{Label: label, Values: map[string]float64{}, Raw: map[string]string{}}
		for idx, c := range cells[1:] {
			val := cleanText(c[1])
			period := ""
			if idx < len(out.Periods) {
				period = out.Periods[idx]
			} else {
				period = strconv.Itoa(idx)
			}
			row.Raw[period] = val
			row.Values[period] = parseNum(val)
		}
		out.Rows = append(out.Rows, row)
	}
	return out
}

// parseScreenerAnalysis extracts the pros/cons list.
func parseScreenerAnalysis(html string) screenerAnalysis {
	var out screenerAnalysis
	sec := extractSection(html, "analysis")
	if sec == "" {
		return out
	}
	pros := regexp.MustCompile(`(?s)PROS(.*?)(?:CONS|</section>|$)`).FindStringSubmatch(sec)
	if len(pros) == 2 {
		for _, p := range parseListItems(pros[1]) {
			out.Pros = append(out.Pros, p)
		}
	}
	cons := regexp.MustCompile(`(?s)CONS(.*?)(?:</section>|$)`).FindStringSubmatch(sec)
	if len(cons) == 2 {
		for _, p := range parseListItems(cons[1]) {
			out.Cons = append(out.Cons, p)
		}
	}
	return out
}

// parseScreenerShareholding extracts the latest shareholding percentages.
func parseScreenerShareholding(html string) screenerShareholding {
	var out screenerShareholding
	sec := extractSection(html, "shareholding")
	if sec == "" {
		return out
	}
	tbl := reFinTable.FindString(sec)
	if tbl == "" {
		return out
	}
	trs := reFinRows.FindAllStringSubmatch(tbl, -1)
	for _, tr := range trs {
		cells := reFinCells.FindAllStringSubmatch(tr[1], -1)
		if len(cells) < 2 {
			continue
		}
		label := cleanText(cells[0][1])
		vals := make([]float64, 0, len(cells)-1)
		for _, c := range cells[1:] {
			vals = append(vals, parseNum(cleanText(c[1])))
		}
		latest := 0.0
		if len(vals) > 0 {
			latest = vals[len(vals)-1]
		}
		switch strings.ToLower(label) {
		case "promoters":
			out.Promoters = latest
		case "fiis":
			out.FIIs = latest
		case "diis":
			out.DIIs = latest
		case "government":
			out.Government = latest
		case "public":
			out.Public = latest
		case "others":
			out.Others = latest
		case "no. of shareholders":
			out.Shareholders = latest
		}
	}
	return out
}

// parseListItems extracts bullet-ish text items (li or newline-separated).
func parseListItems(html string) []string {
	var out []string
	lis := regexp.MustCompile(`(?s)<li[^>]*>(.*?)</li>`).FindAllStringSubmatch(html, -1)
	if len(lis) > 0 {
		for _, m := range lis {
			t := cleanText(m[1])
			if t != "" {
				out = append(out, t)
			}
		}
		return out
	}
	text := cleanText(html)
	for _, part := range strings.Split(text, "\n") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// cleanText strips HTML tags and entities, collapsing whitespace.
// Unlike the generated cleanHTMLText (which only unescapes entities and
// collapses whitespace), this removes markup so table labels and values
// come out as plain text.
func cleanText(s string) string {
	s = reStripTags.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "\u00a0", " ")
	return cleanHTMLText(s)
}

// printNovelJSON emits structured JSON for novel commands. It applies an
// explicit --select filter only, and deliberately skips the auto-compact
// that --agent triggers: compactFields' data-driven keep rule strips
// fields present in fewer than 80% of rows, which silently drops
// heterogeneous fields like YOY percentages that only appear on some
// quarters. Agents that want narrowed output pass --select explicitly.
func printNovelJSON(w io.Writer, v any, flags *rootFlags) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if flags.selectFields != "" {
		raw = filterFields(raw, flags.selectFields)
	}
	if flags.csv {
		return printOutputWithFlagsMeta(w, raw, flags, map[string]any{"source": "local"})
	}
	return printOutput(w, raw, true)
}

// getWithRateRetry performs an HTTP GET with a single paced retry on rate
// limiting. Screener.in throttles burst access; the site's Retry-After
// window (~0.4-1s) clears within one retry, so retrying once after the
// advertised delay is the polite, correct behavior. Other errors pass
// through unchanged.
func getWithRateRetry(ctx context.Context, c *client.Client, path string, params map[string]string) ([]byte, error) {
	data, err := c.Get(ctx, path, params)
	if err == nil {
		return data, nil
	}
	var rle *cliutil.RateLimitError
	if errors.As(err, &rle) && rle.RetryAfter > 0 && rle.RetryAfter <= 5*time.Second {
		select {
		case <-time.After(rle.RetryAfter):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		data, err = c.Get(ctx, path, params)
		if err == nil {
			return data, nil
		}
	}
	return data, err
}

// extractSection returns the HTML between a section anchor and the next
// section (or end of content). Falls back to the section element itself.
func extractSection(html, id string) string {
	re := regexp.MustCompile(`(?s)<section[^>]*id="` + regexp.QuoteMeta(id) + `"[^>]*>(.*?)</section>`)
	if m := re.FindStringSubmatch(html); len(m) == 2 {
		return m[1]
	}
	re2 := regexp.MustCompile(`(?s)id="` + regexp.QuoteMeta(id) + `"(.*?)(?:<section|</main>)`)
	if m := re2.FindStringSubmatch(html); len(m) == 2 {
		return m[1]
	}
	if i := strings.Index(html, `id="`+id+`"`); i >= 0 {
		return html[i:]
	}
	return ""
}

// parseNum parses an Indian-formatted number ("4,75,422", "1,172.50", "24%").
func parseNum(s string) float64 {
	s = strings.TrimSpace(s)
	m := reNumber.FindString(s)
	if m == "" {
		return 0
	}
	m = strings.ReplaceAll(m, ",", "")
	v, err := strconv.ParseFloat(m, 64)
	if err != nil {
		return 0
	}
	return v
}

// screenRow is one row of the standard screen/peer result table
// (S.No., Name, CMP, P/E, Mar Cap, Div Yld, NP Qtr, Qtr Profit Var,
// Sales Qtr, Qtr Sales Var, ROCE).
type screenRow struct {
	Rank          int     `json:"rank"`
	Name          string  `json:"name"`
	Symbol        string  `json:"symbol,omitempty"`
	URL           string  `json:"url,omitempty"`
	CMP           float64 `json:"cmp,omitempty"`
	PE            float64 `json:"pe,omitempty"`
	MarketCap     float64 `json:"market_cap_cr,omitempty"`
	DividendYield float64 `json:"dividend_yield_pct,omitempty"`
	NPQtr         float64 `json:"np_qtr_cr,omitempty"`
	QtrProfitVar  float64 `json:"qtr_profit_var_pct,omitempty"`
	SalesQtr      float64 `json:"sales_qtr_cr,omitempty"`
	QtrSalesVar   float64 `json:"qtr_sales_var_pct,omitempty"`
	ROCE          float64 `json:"roce_pct,omitempty"`
}

var reCompanyURL = regexp.MustCompile(`href="(/company/[^"]+)"`)

// parseScreenTable parses the standard screen/peer result table.
func parseScreenTable(html string) []screenRow {
	var out []screenRow
	tbl := reFinTable.FindString(html)
	if tbl == "" {
		return out
	}
	headers := []string{}
	trs := reFinRows.FindAllStringSubmatch(tbl, -1)
	for _, tr := range trs {
		cells := reFinCells.FindAllStringSubmatch(tr[1], -1)
		if len(cells) == 0 {
			continue
		}
		allTH := true
		for _, c := range cells {
			if !strings.HasPrefix(c[0], "<th") {
				allTH = false
				break
			}
		}
		if allTH {
			for _, c := range cells {
				headers = append(headers, cleanText(c[1]))
			}
			continue
		}
		if len(cells) < 3 {
			continue
		}
		var row screenRow
		first := cleanText(cells[0][1])
		if first == "" || strings.Contains(strings.ToLower(first), "median") {
			continue
		}
		if r, err := strconv.Atoi(strings.TrimSuffix(first, ".")); err == nil {
			row.Rank = r
		}
		nameCell := cells[1][1]
		row.Name = cleanText(nameCell)
		if m := reCompanyURL.FindStringSubmatch(nameCell); len(m) == 2 {
			row.URL = m[1]
			parts := strings.Split(strings.Trim(m[1], "/"), "/")
			if len(parts) >= 2 {
				row.Symbol = strings.ToUpper(parts[1])
			}
		}
		vals := make([]float64, 0, len(cells)-2)
		for _, c := range cells[2:] {
			vals = append(vals, parseNum(cleanText(c[1])))
		}
		for i, v := range vals {
			h := ""
			if i+2 < len(headers) {
				h = strings.ToLower(headers[i+2])
			}
			switch {
			case strings.Contains(h, "cmp"):
				row.CMP = v
			case strings.Contains(h, "p/e"):
				row.PE = v
			case strings.Contains(h, "mar cap"):
				row.MarketCap = v
			case strings.Contains(h, "div yld"):
				row.DividendYield = v
			case strings.Contains(h, "np qtr"):
				row.NPQtr = v
			case strings.Contains(h, "qtr profit var"):
				row.QtrProfitVar = v
			case strings.Contains(h, "sales qtr") && !strings.Contains(h, "var"):
				row.SalesQtr = v
			case strings.Contains(h, "qtr sales var"):
				row.QtrSalesVar = v
			case strings.Contains(h, "roce"):
				row.ROCE = v
			}
		}
		out = append(out, row)
	}
	return out
}
