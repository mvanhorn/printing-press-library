// pp-zillow — Zillow Zestimate + deal intelligence CLI for Kapowsin Business Solutions
// Sources: Zillow public search + detail pages (Chrome TLS via surf)
// Built: 2026-05-08 | Kapowsin PLAN 016 community contribution
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/enetx/surf"
)

// Property holds extracted Zillow listing data
type Property struct {
	ZPID            string  `json:"zpid"`
	Address         string  `json:"address"`
	City            string  `json:"city"`
	State           string  `json:"state"`
	Zip             string  `json:"zip"`
	Beds            float64 `json:"beds"`
	Baths           float64 `json:"baths"`
	Sqft            float64 `json:"sqft"`
	HomeType        string  `json:"home_type"`
	StatusType      string  `json:"status"`
	ListPrice       float64 `json:"list_price"`
	Zestimate       float64 `json:"zestimate"`
	RentZestimate   float64 `json:"rent_zestimate"`
	TaxAssessed     float64 `json:"tax_assessed_value"`
	DaysOnZillow    float64 `json:"days_on_zillow"`
	DetailURL       string  `json:"detail_url"`
	ZestimateGapPct float64 `json:"zestimate_gap_pct,omitempty"`
}

func newClient() *http.Client {
	builder := surf.NewClient().Builder().Impersonate().Chrome()
	return builder.Timeout(20 * time.Second).Build().Unwrap().Std()
}

func fetchPage(client *http.Client, pageURL, referer string) (string, int, error) {
	req, err := http.NewRequest("GET", pageURL, nil)
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return string(body), resp.StatusCode, err
}

// extractJSON pulls __NEXT_DATA__ script JSON from Zillow HTML
func extractNextData(html string) map[string]interface{} {
	re := regexp.MustCompile(`<script id="__NEXT_DATA__" type="application/json">([^<]+)</script>`)
	m := re.FindStringSubmatch(html)
	if len(m) < 2 {
		return nil
	}
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(m[1]), &result); err != nil {
		return nil
	}
	return result
}

// extractFloat safely gets a float from an interface{}
func extractFloat(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	}
	return 0
}

// extractString safely gets a string
func extractString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// parseListings walks the __NEXT_DATA__ JSON tree to find listing objects with zestimate
func parseListings(data map[string]interface{}) []Property {
	raw, _ := json.Marshal(data)
	return parseListingsFromJSON(string(raw))
}

// parseListingsFromJSON uses regex to extract all property objects from JSON blob
func parseListingsFromJSON(jsonStr string) []Property {
	// Extract all objects that have both "zpid" and "zestimate"
	// Strategy: find each "zestimate": occurrence, walk back to find the object
	var props []Property
	seen := make(map[string]bool)

	zpidRe := regexp.MustCompile(`"zpid"\s*:\s*"?(\d+)"?`)
	priceRe := regexp.MustCompile(`"(?:price|unformattedPrice)"\s*:\s*(\d+)`)
	zestRe := regexp.MustCompile(`"zestimate"\s*:\s*(\d+)`)
	rentZestRe := regexp.MustCompile(`"rentZestimate"\s*:\s*(\d+)`)
	taxRe := regexp.MustCompile(`"taxAssessedValue"\s*:\s*(\d+(?:\.\d+)?)`)
	bedsRe := regexp.MustCompile(`"beds(?:rooms)?"\s*:\s*(\d+)`)
	bathsRe := regexp.MustCompile(`"bath(?:rooms|s)?"\s*:\s*(\d+)`)
	sqftRe := regexp.MustCompile(`"(?:area|livingArea)"\s*:\s*(\d+)`)
	addrRe := regexp.MustCompile(`"address"\s*:\s*"([^"]+)"`)
	cityRe := regexp.MustCompile(`"(?:addressCity|city)"\s*:\s*"([^"]+)"`)
	stateRe := regexp.MustCompile(`"(?:addressState|state)"\s*:\s*"([A-Z]{2})"`)
	zipRe := regexp.MustCompile(`"(?:addressZipcode|zipcode)"\s*:\s*"(\d{5})"`)
	typeRe := regexp.MustCompile(`"homeType"\s*:\s*"([^"]+)"`)
	statusRe := regexp.MustCompile(`"statusType"\s*:\s*"([^"]+)"`)
	daysRe := regexp.MustCompile(`"daysOnZillow"\s*:\s*(\d+)`)
	urlRe := regexp.MustCompile(`"detailUrl"\s*:\s*"([^"]+)"`)

	// Find all zestimate positions and extract surrounding context (~3000 chars)
	zestMatches := zestRe.FindAllStringIndex(jsonStr, -1)
	for _, loc := range zestMatches {
		start := loc[0] - 2000
		if start < 0 {
			start = 0
		}
		end := loc[1] + 1000
		if end > len(jsonStr) {
			end = len(jsonStr)
		}
		chunk := jsonStr[start:end]

		// Extract zpid — skip duplicates
		zpidMatch := zpidRe.FindStringSubmatch(chunk)
		if len(zpidMatch) < 2 {
			continue
		}
		zpid := zpidMatch[1]
		if seen[zpid] {
			continue
		}

		// Must have a zestimate value > 0
		zestMatch := zestRe.FindStringSubmatch(chunk)
		if len(zestMatch) < 2 {
			continue
		}
		zest, _ := strconv.ParseFloat(zestMatch[1], 64)
		if zest == 0 {
			continue
		}

		seen[zpid] = true
		p := Property{ZPID: zpid, Zestimate: zest}

		// Price
		if m := priceRe.FindStringSubmatch(chunk); len(m) >= 2 {
			p.ListPrice, _ = strconv.ParseFloat(m[1], 64)
		}
		if m := rentZestRe.FindStringSubmatch(chunk); len(m) >= 2 {
			p.RentZestimate, _ = strconv.ParseFloat(m[1], 64)
		}
		if m := taxRe.FindStringSubmatch(chunk); len(m) >= 2 {
			p.TaxAssessed, _ = strconv.ParseFloat(m[1], 64)
		}
		if m := bedsRe.FindStringSubmatch(chunk); len(m) >= 2 {
			p.Beds, _ = strconv.ParseFloat(m[1], 64)
		}
		if m := bathsRe.FindStringSubmatch(chunk); len(m) >= 2 {
			p.Baths, _ = strconv.ParseFloat(m[1], 64)
		}
		if m := sqftRe.FindStringSubmatch(chunk); len(m) >= 2 {
			p.Sqft, _ = strconv.ParseFloat(m[1], 64)
		}
		if m := addrRe.FindStringSubmatch(chunk); len(m) >= 2 {
			p.Address = m[1]
		}
		if m := cityRe.FindStringSubmatch(chunk); len(m) >= 2 {
			p.City = m[1]
		}
		if m := stateRe.FindStringSubmatch(chunk); len(m) >= 2 {
			p.State = m[1]
		}
		if m := zipRe.FindStringSubmatch(chunk); len(m) >= 2 {
			p.Zip = m[1]
		}
		if m := typeRe.FindStringSubmatch(chunk); len(m) >= 2 {
			p.HomeType = m[1]
		}
		if m := statusRe.FindStringSubmatch(chunk); len(m) >= 2 {
			p.StatusType = m[1]
		}
		if m := daysRe.FindStringSubmatch(chunk); len(m) >= 2 {
			p.DaysOnZillow, _ = strconv.ParseFloat(m[1], 64)
		}
		if m := urlRe.FindStringSubmatch(chunk); len(m) >= 2 {
			u := m[1]
			if !strings.HasPrefix(u, "http") {
				u = "https://www.zillow.com" + u
			}
			p.DetailURL = u
		}

		// Compute gap
		if p.ListPrice > 0 && p.Zestimate > 0 {
			p.ZestimateGapPct = math.Round(((p.Zestimate-p.ListPrice)/p.ListPrice)*100*10) / 10
		}

		if p.ListPrice > 0 || p.Address != "" {
			props = append(props, p)
		}
	}

	return props
}

func cityToSlug(city string) string {
	c := strings.ToLower(strings.TrimSpace(city))
	c = strings.ReplaceAll(c, ", ", "-") // "Tacoma, WA" → "tacoma-wa"
	c = strings.ReplaceAll(c, ",", "-")  // "Tacoma,WA" → "tacoma-wa"
	c = strings.ReplaceAll(c, " ", "-")  // remaining spaces
	c = strings.Trim(c, "-")
	return c
}

func formatMoney(v float64) string {
	if v == 0 {
		return "—"
	}
	s := fmt.Sprintf("%.0f", v)
	n := len(s)
	if n <= 3 {
		return "$" + s
	}
	var b strings.Builder
	b.WriteString("$")
	start := n % 3
	if start == 0 {
		start = 3
	}
	b.WriteString(s[:start])
	for i := start; i < n; i += 3 {
		b.WriteByte(',')
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

func gapArrow(pct float64) string {
	if pct > 0 {
		return fmt.Sprintf("▲ +%.1f%%", pct)
	} else if pct < 0 {
		return fmt.Sprintf("▼ %.1f%%", pct)
	}
	return "→ 0%"
}

// cmdSearch fetches Zillow search results for a city
func cmdSearch(city string, limit int, jsonOut bool) {
	client := newClient()
	slug := cityToSlug(city)

	// Try city-state URL formats
	urls := []string{
		fmt.Sprintf("https://www.zillow.com/homes/%s_rb/", url.PathEscape(slug)),
		fmt.Sprintf("https://www.zillow.com/%s/", slug),
	}

	var html string
	var status int
	var err error
	for _, u := range urls {
		html, status, err = fetchPage(client, u, "https://www.google.com/")
		if err == nil && status == 200 && strings.Contains(html, `"zestimate":`) {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if status != 200 {
		fmt.Fprintf(os.Stderr, "HTTP %d — Zillow blocked this request\n", status)
		os.Exit(1)
	}

	props := parseListingsFromJSON(html)
	if len(props) > limit {
		props = props[:limit]
	}

	if jsonOut {
		json.NewEncoder(os.Stdout).Encode(props)
		return
	}

	fmt.Printf("🏠 Zillow — %s (%d listings)\n\n", city, len(props))
	for _, p := range props {
		gap := ""
		if p.ZestimateGapPct != 0 {
			gap = fmt.Sprintf("  Zest: %s (%s vs list)", formatMoney(p.Zestimate), gapArrow(p.ZestimateGapPct))
		}
		rent := ""
		if p.RentZestimate > 0 {
			rent = fmt.Sprintf("  Rent est: %s/mo", formatMoney(p.RentZestimate))
		}
		beds := ""
		if p.Beds > 0 {
			beds = fmt.Sprintf("  %.0fbd/%.0fba", p.Beds, p.Baths)
		}
		sqft := ""
		if p.Sqft > 0 {
			sqft = fmt.Sprintf("  %.0f sqft", p.Sqft)
		}
		dom := ""
		if p.DaysOnZillow > 0 {
			dom = fmt.Sprintf("  %gd on market", p.DaysOnZillow)
		}
		fmt.Printf("• %s\n  List: %s%s%s%s%s%s\n\n",
			p.Address, formatMoney(p.ListPrice), beds, sqft, dom, gap, rent)
	}
}

// cmdZestimate fetches a single property's Zestimate by URL or address
func cmdZestimate(target string, jsonOut bool) {
	client := newClient()

	var pageURL string
	if strings.HasPrefix(target, "http") {
		pageURL = target
	} else {
		// Search for it
		slug := cityToSlug(target)
		pageURL = "https://www.zillow.com/homes/" + url.PathEscape(slug) + "_rb/"
	}

	html, status, err := fetchPage(client, pageURL, "https://www.zillow.com/")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if status != 200 {
		fmt.Fprintf(os.Stderr, "HTTP %d\n", status)
		os.Exit(1)
	}

	props := parseListingsFromJSON(html)
	if len(props) == 0 {
		fmt.Println("No Zestimate data found for this property")
		os.Exit(1)
	}

	p := props[0]
	if jsonOut {
		json.NewEncoder(os.Stdout).Encode(p)
		return
	}

	fmt.Printf("🏠 %s\n\n", p.Address)
	fmt.Printf("  List price:      %s\n", formatMoney(p.ListPrice))
	fmt.Printf("  Zestimate:       %s  (%s vs list)\n", formatMoney(p.Zestimate), gapArrow(p.ZestimateGapPct))
	if p.RentZestimate > 0 {
		fmt.Printf("  Rent Zestimate:  %s/mo\n", formatMoney(p.RentZestimate))
	}
	if p.TaxAssessed > 0 {
		fmt.Printf("  Tax assessed:    %s\n", formatMoney(p.TaxAssessed))
	}
	if p.Beds > 0 {
		fmt.Printf("  Beds/Baths:      %.0f bd / %.0f ba\n", p.Beds, p.Baths)
	}
	if p.Sqft > 0 {
		fmt.Printf("  Living area:     %.0f sqft\n", p.Sqft)
	}
	if p.DaysOnZillow > 0 {
		fmt.Printf("  Days on Zillow:  %.0f\n", p.DaysOnZillow)
	}
	if p.DetailURL != "" {
		fmt.Printf("  Zillow URL:      %s\n", p.DetailURL)
	}
}

// cmdDeals finds properties where Zestimate > list price by minGapPct
func cmdDeals(city string, minGapPct float64, limit int, jsonOut bool) {
	client := newClient()
	slug := cityToSlug(city)

	urls := []string{
		fmt.Sprintf("https://www.zillow.com/homes/%s_rb/", url.PathEscape(slug)),
		fmt.Sprintf("https://www.zillow.com/%s/", slug),
	}

	var html string
	var status int
	var err error
	for _, u := range urls {
		html, status, err = fetchPage(client, u, "https://www.google.com/")
		if err == nil && status == 200 && strings.Contains(html, `"zestimate":`) {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if err != nil || status != 200 {
		fmt.Fprintf(os.Stderr, "Fetch failed: HTTP %d %v\n", status, err)
		os.Exit(1)
	}

	props := parseListingsFromJSON(html)

	// Filter: Zestimate > list price by minGapPct
	var deals []Property
	for _, p := range props {
		if p.ZestimateGapPct >= minGapPct && p.ListPrice > 0 {
			deals = append(deals, p)
		}
	}

	// Sort by gap descending
	sort.Slice(deals, func(i, j int) bool {
		return deals[i].ZestimateGapPct > deals[j].ZestimateGapPct
	})

	if len(deals) > limit {
		deals = deals[:limit]
	}

	if jsonOut {
		json.NewEncoder(os.Stdout).Encode(deals)
		return
	}

	if len(deals) == 0 {
		fmt.Printf("No deals found in %s where Zestimate > list price by %.0f%%+\n", city, minGapPct)
		return
	}

	fmt.Printf("💰 Deals in %s — Zestimate > List Price by ≥%.0f%% (%d found)\n\n", city, minGapPct, len(deals))
	for i, p := range deals {
		rent := ""
		if p.RentZestimate > 0 {
			rent = fmt.Sprintf("  Rent est: %s/mo", formatMoney(p.RentZestimate))
		}
		sqft := ""
		if p.Sqft > 0 {
			sqft = fmt.Sprintf("  %.0f sqft", p.Sqft)
		}
		fmt.Printf("%d. %s\n   List: %s  →  Zest: %s  (%s)\n   %.0fbd/%.0fba%s%s\n   %s\n\n",
			i+1, p.Address,
			formatMoney(p.ListPrice), formatMoney(p.Zestimate), gapArrow(p.ZestimateGapPct),
			p.Beds, p.Baths, sqft, rent,
			p.DetailURL)
	}
}

// cmdCompare side-by-side comparison of multiple properties
func cmdCompare(targets []string, jsonOut bool) {
	client := newClient()
	var props []Property

	for _, t := range targets {
		html, status, err := fetchPage(client, t, "https://www.zillow.com/")
		if err != nil || status != 200 {
			fmt.Fprintf(os.Stderr, "Warning: could not fetch %s (HTTP %d)\n", t, status)
			continue
		}
		ps := parseListingsFromJSON(html)
		if len(ps) > 0 {
			props = append(props, ps[0])
		}
		time.Sleep(800 * time.Millisecond) // rate limit
	}

	if jsonOut {
		json.NewEncoder(os.Stdout).Encode(props)
		return
	}

	fmt.Printf("📊 Property Comparison (%d properties)\n\n", len(props))
	for i, p := range props {
		fmt.Printf("%d. %s\n", i+1, p.Address)
		fmt.Printf("   List: %s  |  Zest: %s  |  Gap: %s\n",
			formatMoney(p.ListPrice), formatMoney(p.Zestimate), gapArrow(p.ZestimateGapPct))
		if p.RentZestimate > 0 {
			fmt.Printf("   Rent Zest: %s/mo  |  Tax Assessed: %s\n",
				formatMoney(p.RentZestimate), formatMoney(p.TaxAssessed))
		}
		if p.Beds > 0 {
			fmt.Printf("   %.0fbd/%.0fba  %.0f sqft\n", p.Beds, p.Baths, p.Sqft)
		}
		fmt.Println()
	}
}

func usage() {
	fmt.Println(`pp-zillow — Zillow Zestimate + deal intelligence CLI
Uses Chrome TLS fingerprinting to access Zillow data

Commands:
  search <city>             List homes with Zestimate for a city
  zestimate <url|address>   Zestimate for a specific property
  deals <city>              Properties where Zestimate > list price
  compare <url> [url...]    Side-by-side comparison with Zestimates

Flags:
  --limit N      Max results (default: 20)
  --gap N        Min Zestimate-vs-price gap % for deals (default: 15)
  --json         Structured JSON output

Examples:
  pp-zillow search "Tacoma, WA"
  pp-zillow deals "Federal Way, WA" --gap 20
  pp-zillow zestimate https://www.zillow.com/homedetails/.../12345_zpid/
  pp-zillow compare https://zillow.com/... https://zillow.com/...
`)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(0)
	}

	cmd := os.Args[1]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "JSON output")
	limit := fs.Int("limit", 20, "Max results")
	gap := fs.Float64("gap", 15.0, "Min deal gap %")

	// splitArgs separates flags (--key val) from positional args so users
	// can mix: pp-zillow deals "Tacoma, WA" --gap 20
	splitArgs := func(rawArgs []string) (flags []string, positional []string) {
		for i := 0; i < len(rawArgs); i++ {
			if strings.HasPrefix(rawArgs[i], "-") {
				flags = append(flags, rawArgs[i])
				// peek: if next is not a flag it's the value
				if i+1 < len(rawArgs) && !strings.HasPrefix(rawArgs[i+1], "-") {
					i++
					flags = append(flags, rawArgs[i])
				}
			} else {
				positional = append(positional, rawArgs[i])
			}
		}
		return
	}

	switch cmd {
	case "search":
		flags, pos := splitArgs(os.Args[2:])
		fs.Parse(flags)
		if len(pos) == 0 {
			fmt.Fprintln(os.Stderr, "Usage: pp-zillow search <city>")
			os.Exit(1)
		}
		cmdSearch(strings.Join(pos, " "), *limit, *jsonOut)

	case "zestimate":
		flags, pos := splitArgs(os.Args[2:])
		fs.Parse(flags)
		if len(pos) == 0 {
			fmt.Fprintln(os.Stderr, "Usage: pp-zillow zestimate <url|address>")
			os.Exit(1)
		}
		cmdZestimate(strings.Join(pos, " "), *jsonOut)

	case "deals":
		flags, pos := splitArgs(os.Args[2:])
		fs.Parse(flags)
		if len(pos) == 0 {
			fmt.Fprintln(os.Stderr, "Usage: pp-zillow deals <city>")
			os.Exit(1)
		}
		cmdDeals(strings.Join(pos, " "), *gap, *limit, *jsonOut)

	case "compare":
		flags, pos := splitArgs(os.Args[2:])
		fs.Parse(flags)
		if len(pos) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: pp-zillow compare <url1> <url2> [...]")
			os.Exit(1)
		}
		cmdCompare(pos, *jsonOut)

	case "help", "--help", "-h":
		usage()

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		usage()
		os.Exit(1)
	}

	_ = extractFloat
	_ = extractString
	_ = parseListings
}
