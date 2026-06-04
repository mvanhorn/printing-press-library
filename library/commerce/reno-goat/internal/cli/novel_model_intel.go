// Novel command: compound model intelligence for renovation selections.

package cli

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/reno-goat/internal/cliutil"
	"github.com/spf13/cobra"
)

type modelIntelResult struct {
	Query      string             `json:"query"`
	Total      int                `json:"total"`
	Models     []modelIntelRecord `json:"models"`
	Blocked    []modelProbe       `json:"blocked,omitempty"`
	Notes      []string           `json:"notes,omitempty"`
	SourcesRun []string           `json:"sources_run,omitempty"`
}

type modelIntelRecord struct {
	Model        string              `json:"model"`
	Brand        string              `json:"brand,omitempty"`
	Title        string              `json:"title,omitempty"`
	Category     string              `json:"category,omitempty"`
	BestPrice    float64             `json:"best_price,omitempty"`
	BestSource   string              `json:"best_source,omitempty"`
	ProductURL   string              `json:"product_url,omitempty"`
	ImageURL     string              `json:"image_url,omitempty"`
	Specs        []modelSpecDocument `json:"specs,omitempty"`
	Offers       []modelOffer        `json:"offers,omitempty"`
	ProbeStatus  []modelProbe        `json:"probe_status,omitempty"`
	SourceFields []string            `json:"source_fields,omitempty"`
}

type modelOffer struct {
	Source   string  `json:"source"`
	Title    string  `json:"title,omitempty"`
	Price    float64 `json:"price,omitempty"`
	Regular  float64 `json:"regular,omitempty"`
	URL      string  `json:"url,omitempty"`
	Evidence string  `json:"evidence,omitempty"`
}

type modelSpecDocument struct {
	Kind   string `json:"kind"`
	URL    string `json:"url"`
	Source string `json:"source,omitempty"`
}

type modelProbe struct {
	Source string `json:"source"`
	URL    string `json:"url"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

func newModelIntelCmd(flags *rootFlags) *cobra.Command {
	var (
		category     string
		limit        int
		room         string
		searchOffers bool
		sources      string
		probePages   bool
	)

	cmd := &cobra.Command{
		Use:   "model-intel <query-or-model>",
		Short: "Discover model numbers, spec docs, and model-page offers for installed selections.",
		Long: `Model intelligence runs the compound lookup Reno Goat needs for installed-selection decisions:
discover product rows, extract model numbers, keep spec/install document links, probe
predictable manufacturer/retailer model pages for prices or transport blockers, and
use labeled search-result snippets as fallback model and offer evidence. In auto mode,
Reno Goat infers installed-selection categories from the query before falling back to
appliance-oriented discovery.`,
		Example: `  reno-goat-pp-cli model-intel "36 induction cooktop" --json
  reno-goat-pp-cli model-intel "mini split heat pump" --category hvac --sources routed --json
  reno-goat-pp-cli model-intel JOESC330RM --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			result, err := runModelIntel(cmd.Context(), flags, args[0], limit, sources, category, room, probePages, searchOffers)
			if err != nil {
				return err
			}
			if flags.asJSON || flags.agent {
				return flags.printJSON(cmd, result)
			}
			return renderModelIntelTable(cmd, flags, result)
		},
	}

	cmd.Flags().StringVar(&category, "category", "", "Comma-separated category routing buckets: plumbing, electrical, hvac, flooring, hardware, materials, appliances, furniture, decor")
	cmd.Flags().IntVar(&limit, "limit", 12, "Maximum model records to return")
	cmd.Flags().StringVar(&room, "room", "", "Room shortcut that expands to categories: bathroom, kitchen, bedroom, living, dining, outdoor")
	cmd.Flags().BoolVar(&searchOffers, "search-offers", true, "Add labeled search-result model and price evidence from readable search pages")
	cmd.Flags().StringVar(&sources, "sources", "auto", "Discovery sources: auto, routed, active, or comma-separated source names such as ge-appliances,bosch")
	cmd.Flags().BoolVar(&probePages, "probe-pages", true, "Probe predictable manufacturer and retailer model pages")
	return cmd
}

func runModelIntel(ctx context.Context, flags *rootFlags, query string, limit int, sources, category, room string, probePages, searchOffers bool) (modelIntelResult, error) {
	if limit <= 0 {
		limit = 12
	}
	httpClient := &http.Client{Timeout: flags.timeout}
	selected, categories, resolvedRoom, err := resolveModelIntelSources(query, sources, category, room)
	if err != nil {
		return modelIntelResult{}, usageErr(err)
	}

	result := modelIntelResult{
		Query:      query,
		SourcesRun: selected,
		Notes: []string{
			"Model intelligence is a compound workflow: category search, model extraction, spec-document capture, and model-page probing.",
			"Blocked model pages are reported as evidence instead of silently dropping retailer coverage.",
		},
	}
	if len(categories) > 0 {
		result.Notes = append(result.Notes, "Category routing used: "+strings.Join(categories, ", "))
	}
	if resolvedRoom != "" {
		result.Notes = append(result.Notes, "Room routing used: "+resolvedRoom)
	}
	records := map[string]*modelIntelRecord{}

	if looksLikeModelNumber(query) {
		model := strings.ToUpper(strings.TrimSpace(query))
		records[model] = &modelIntelRecord{Model: model}
		if searchOffers {
			result.SourcesRun = appendUniqueStrings(result.SourcesRun, "brave-search")
			result.Notes = append(result.Notes, "Exact-model search-result offer discovery used; snippet-derived prices are labeled as search evidence, not direct retailer API results.")
			enrichExactModelSearchOffers(ctx, httpClient, records[model])
		}
	} else if searchOffers {
		result.SourcesRun = appendUniqueStrings(result.SourcesRun, "brave-search")
		result.Notes = append(result.Notes, "Category search-result model discovery used; search-derived prices are labeled as fallback evidence, not direct retailer API results.")
		for _, rec := range discoverBraveCategoryProducts(ctx, httpClient, query, maxInt(limit, 8)) {
			if rec.Model == "" {
				continue
			}
			existing := records[rec.Model]
			if existing == nil {
				copy := rec
				records[rec.Model] = &copy
				continue
			}
			mergeModelRecord(existing, rec)
		}
	}
	if !looksLikeModelNumber(query) || sources != "auto" || category != "" || room != "" {
		discovered, errs := discoverModelRows(ctx, httpClient, query, limit, selected)
		for _, fanoutErr := range errs {
			result.Notes = append(result.Notes, modelIntelFanoutNote(fanoutErr))
		}
		for _, rec := range discovered {
			if rec.Model == "" {
				continue
			}
			existing := records[rec.Model]
			if existing == nil {
				copy := rec
				records[rec.Model] = &copy
				continue
			}
			mergeModelRecord(existing, rec)
		}
	}
	if searchOffers {
		enrichUnpricedDiscoverySearchOffers(ctx, httpClient, records, limit)
	}

	var out []modelIntelRecord
	for _, rec := range records {
		finalizeBestOffer(rec)
		out = append(out, *rec)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].BestPrice > 0 && out[j].BestPrice == 0 {
			return true
		}
		if out[i].BestPrice == 0 && out[j].BestPrice > 0 {
			return false
		}
		if out[i].BestPrice != out[j].BestPrice {
			return out[i].BestPrice < out[j].BestPrice
		}
		return out[i].Model < out[j].Model
	})
	if len(out) > limit {
		out = out[:limit]
	}
	if out == nil {
		out = []modelIntelRecord{}
	}
	for i := range out {
		enrichModelProductPage(ctx, httpClient, &out[i])
		enrichModelOfferPages(ctx, httpClient, &out[i], 3)
		if probePages {
			probeModelPages(ctx, httpClient, &out[i])
		}
		finalizeBestOffer(&out[i])
	}
	result.Models = out
	result.Total = len(out)
	for _, rec := range out {
		for _, probe := range rec.ProbeStatus {
			if probe.Status == "blocked" || probe.Status == "not_found" {
				result.Blocked = append(result.Blocked, probe)
			}
		}
	}
	return result, nil
}

func modelIntelFanoutNote(err cliutil.FanoutError) string {
	reason := ""
	if err.Err != nil {
		reason = err.Err.Error()
		if idx := strings.Index(reason, "\n"); idx >= 0 {
			reason = reason[:idx]
		}
		reason = truncate(reason, 160)
	}
	if reason == "" {
		return "Source warning: " + err.Source
	}
	return "Source warning: " + err.Source + ": " + reason
}

func discoverModelRows(ctx context.Context, httpClient *http.Client, query string, limit int, sources []string) ([]modelIntelRecord, []cliutil.FanoutError) {
	type sourceRows struct {
		Rows []modelIntelRecord
	}
	results, errs := cliutil.FanoutRun(
		ctx,
		sources,
		func(s string) string { return s },
		func(ctx context.Context, source string) (sourceRows, error) {
			switch source {
			case "bosch":
				rows, err := discoverBoschModels(ctx, httpClient, query, limit)
				return sourceRows{Rows: rows}, err
			case "broan-nutone":
				rows, err := discoverBroanNuToneModels(ctx, httpClient, query, limit)
				return sourceRows{Rows: rows}, err
			default:
				products, err := searchSource(ctx, httpClient, source, query, maxInt(limit*2, 12))
				if err != nil {
					return sourceRows{}, err
				}
				var rows []modelIntelRecord
				for _, p := range products {
					if !modelIntelProductMatchesQuery(query, p) {
						continue
					}
					if rec := modelRecordFromProduct(p); rec.Model != "" && modelIntelUsefulRecord(rec) {
						rows = append(rows, rec)
					}
				}
				return sourceRows{Rows: rows}, nil
			}
		},
		cliutil.WithConcurrency(len(sources)),
	)
	var rows []modelIntelRecord
	for _, result := range results {
		rows = append(rows, result.Value.Rows...)
	}
	return rows, errs
}

func modelRecordFromProduct(p NormalizedProduct) modelIntelRecord {
	rawID := strings.ToUpper(strings.TrimSpace(p.ID))
	model := ""
	if productIDIsSelectionSKU(p) {
		if !isDigitsOnly(rawID) {
			model = normalizeSelectionSKU(p.ID)
		}
	} else if looksLikeModelNumber(rawID) {
		model = rawID
	}
	if model == "" {
		model = extractBestModel(p.Title)
	}
	if model == "" {
		model = extractModelFromProductURL(p.URL)
	}
	if model == "" && productIDIsSelectionSKU(p) {
		model = normalizeSelectionSKU(p.ID)
	}
	rec := modelIntelRecord{
		Model:      model,
		Brand:      p.Brand,
		Title:      p.Title,
		Category:   p.Category,
		ProductURL: p.URL,
		ImageURL:   p.ImageURL,
	}
	if p.PriceMin > 0 {
		rec.Offers = append(rec.Offers, modelOffer{
			Source:   p.Source,
			Price:    p.PriceMin,
			Regular:  p.RegularPriceMin,
			URL:      p.URL,
			Evidence: "normalized product search row",
		})
	}
	rec.Specs = append(rec.Specs, specDocsFromDescription(p.Source, p.Description)...)
	if p.Description != "" {
		rec.SourceFields = append(rec.SourceFields, p.Description)
	}
	return rec
}

const maxDiscoveryOfferFallbackProbes = 4

func enrichUnpricedDiscoverySearchOffers(ctx context.Context, httpClient *http.Client, records map[string]*modelIntelRecord, limit int) {
	for _, rec := range discoveryOfferFallbackCandidates(records, limit) {
		enrichExactModelSearchOffers(ctx, httpClient, rec)
	}
}

func discoveryOfferFallbackCandidates(records map[string]*modelIntelRecord, limit int) []*modelIntelRecord {
	if limit <= 0 {
		limit = 6
	}
	maxProbes := minInt(limit, maxDiscoveryOfferFallbackProbes)
	if maxProbes <= 0 {
		return nil
	}
	var candidates []*modelIntelRecord
	for _, rec := range records {
		if rec == nil || rec.Model == "" || len(rec.Offers) > 0 || !isModelDiscoveryRecord(*rec) {
			continue
		}
		candidates = append(candidates, rec)
	}
	sort.Slice(candidates, func(i, j int) bool {
		leftSource := modelDiscoverySource(candidates[i])
		rightSource := modelDiscoverySource(candidates[j])
		if leftSource != rightSource {
			return leftSource < rightSource
		}
		leftURL := strings.ToLower(candidates[i].ProductURL)
		rightURL := strings.ToLower(candidates[j].ProductURL)
		if leftURL != rightURL {
			return leftURL < rightURL
		}
		leftTitle := strings.ToLower(candidates[i].Title)
		rightTitle := strings.ToLower(candidates[j].Title)
		if leftTitle != rightTitle {
			return leftTitle < rightTitle
		}
		return candidates[i].Model < candidates[j].Model
	})
	if len(candidates) > maxProbes {
		candidates = candidates[:maxProbes]
	}
	return candidates
}

func isModelDiscoveryRecord(rec modelIntelRecord) bool {
	return modelDiscoverySource(&rec) != ""
}

func modelDiscoverySource(rec *modelIntelRecord) string {
	if rec == nil {
		return ""
	}
	for _, field := range rec.SourceFields {
		if strings.HasPrefix(field, "model_discovery:") {
			return strings.TrimSpace(strings.TrimPrefix(field, "model_discovery:"))
		}
	}
	return ""
}

func isDigitsOnly(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func productIDIsSelectionSKU(p NormalizedProduct) bool {
	if p.ID == "" || p.Title == "" || p.URL == "" {
		return false
	}
	switch p.Source {
	case "floor-and-decor", "hardware-hut", "ikea", "rejuvenation", "faucetlist", "plumbtile", "modern-bathroom":
		return true
	default:
		return false
	}
}

func normalizeSelectionSKU(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "-_/ ")
	s = strings.ToUpper(s)
	s = regexp.MustCompile(`[^A-Z0-9._-]+`).ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 80 {
		return ""
	}
	return s
}

func enrichExactModelSearchOffers(ctx context.Context, httpClient *http.Client, rec *modelIntelRecord) {
	if rec.Model == "" {
		return
	}
	body, finalURL, err := fetchModelPage(ctx, httpClient, "https://search.brave.com/search?q="+url.QueryEscape(rec.Model))
	if err != nil {
		rec.ProbeStatus = append(rec.ProbeStatus, modelProbe{Source: "brave-search", URL: "https://search.brave.com/search?q=" + url.QueryEscape(rec.Model), Status: "error", Reason: err.Error()})
		return
	}
	offers := searchResultOffersFromBrave(rec.Model, body)
	if len(offers) == 0 {
		rec.ProbeStatus = append(rec.ProbeStatus, modelProbe{Source: "brave-search", URL: finalURL, Status: "not_found", Reason: "no exact-model price snippets found"})
		return
	}
	rec.Offers = append(rec.Offers, offers...)
	rec.ProbeStatus = append(rec.ProbeStatus, modelProbe{Source: "brave-search", URL: finalURL, Status: "ok", Reason: fmt.Sprintf("%d snippet offer(s)", len(offers))})
}

func discoverBraveCategoryProducts(ctx context.Context, httpClient *http.Client, query string, limit int) []modelIntelRecord {
	body, _, err := fetchModelPage(ctx, httpClient, "https://search.brave.com/search?q="+url.QueryEscape(query))
	if err != nil {
		return nil
	}
	return searchResultProductsFromBrave(query, body, limit)
}

func searchResultProductsFromBrave(query, body string, limit int) []modelIntelRecord {
	queryLower := strings.ToLower(query)
	parts := strings.Split(body, `product:{type:"Product",name:"`)
	seen := map[string]bool{}
	var out []modelIntelRecord
	for idx := 1; idx < len(parts); idx++ {
		part := parts[idx]
		nameRaw, rest, ok := strings.Cut(part, `"`)
		if !ok {
			continue
		}
		name := decodeSearchResultString(nameRaw)
		if !categoryProductMatchesQuery(queryLower, strings.ToLower(name)) {
			continue
		}
		u := firstQuotedField(rest, "url")
		price := parsePriceString(firstQuotedField(rest, "price"))
		if price <= 0 || u == "" {
			continue
		}
		model := extractBestModel(name)
		if model == "" {
			model = extractModelFromProductURL(u)
		}
		if model == "" {
			model = extractBestModel(u)
		}
		if model == "" || !looksLikeModelNumber(model) {
			continue
		}
		if seen[model] {
			continue
		}
		seen[model] = true
		rec := modelIntelRecord{
			Model:      model,
			Brand:      brandFromTitle(name),
			Title:      name,
			ProductURL: u,
			Offers: []modelOffer{{
				Source:   "brave-search:" + hostFromURL(u),
				Title:    name,
				Price:    price,
				URL:      u,
				Evidence: "Brave Search category product result price field",
			}},
		}
		if modelIntelUsefulRecord(rec) {
			out = append(out, rec)
		}
		if len(out) >= limit {
			break
		}
	}
	return out
}

func firstQuotedField(text, field string) string {
	prefix := field + `:"`
	idx := strings.Index(text, prefix)
	if idx < 0 {
		return ""
	}
	rest := text[idx+len(prefix):]
	raw, _, ok := strings.Cut(rest, `"`)
	if !ok {
		return ""
	}
	return decodeSearchResultString(raw)
}

func extractModelFromProductURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	path := strings.ToUpper(parsed.EscapedPath())
	for _, marker := range []string{"SKU-", "MODEL-"} {
		idx := strings.Index(path, marker)
		if idx < 0 {
			continue
		}
		rest := path[idx+len(marker):]
		token, _, _ := strings.Cut(rest, "/")
		token = strings.Trim(token, "-_")
		if looksLikeModelNumber(token) && len(token) <= 32 {
			return token
		}
	}
	for _, part := range reverseStrings(regexp.MustCompile(`[-_/.\s]+`).Split(path, -1)) {
		part = strings.Trim(part, "-_ .")
		if part == "" || strings.EqualFold(part, "HTML") {
			continue
		}
		if modelRegexp.MatchString(part) && len(part) >= 5 && len(part) <= 32 {
			return part
		}
	}
	return ""
}

func reverseStrings(vals []string) []string {
	out := make([]string, 0, len(vals))
	for i := len(vals) - 1; i >= 0; i-- {
		out = append(out, vals[i])
	}
	return out
}

func categoryProductMatchesQuery(queryLower, titleLower string) bool {
	matched := 0
	for _, term := range strings.Fields(queryLower) {
		term = strings.Trim(term, `"'`)
		if len(term) <= 2 {
			continue
		}
		if strings.Contains(titleLower, term) {
			matched++
		}
	}
	return matched >= 2
}

func brandFromTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}
	fields := strings.Fields(title)
	if len(fields) == 0 {
		return ""
	}
	if len(fields) >= 2 && strings.EqualFold(fields[0], "GE") {
		return strings.Join(fields[:2], " ")
	}
	return strings.Trim(fields[0], `"'`)
}

func searchResultOffersFromBrave(model, body string) []modelOffer {
	model = strings.ToUpper(strings.TrimSpace(model))
	if model == "" {
		return nil
	}
	descPattern := regexp.MustCompile(`description:"((?:\\.|[^"])*)"`)
	seen := map[string]bool{}
	var offers []modelOffer
	parts := strings.Split(body, `title:"`)
	for idx := 1; idx < len(parts); idx++ {
		part := parts[idx]
		titleRaw, rest, ok := strings.Cut(part, `",url:"`)
		if !ok {
			continue
		}
		urlRaw, rest, ok := strings.Cut(rest, `"`)
		if !ok {
			continue
		}
		title := cleanSearchResultTitle(decodeSearchResultString(titleRaw))
		u := decodeSearchResultString(urlRaw)
		end := len(rest)
		if next := strings.Index(rest, `title:"`); next >= 0 {
			end = next
		}
		desc := ""
		if descMatch := descPattern.FindStringSubmatch(rest[:end]); len(descMatch) > 1 {
			desc = decodeSearchResultString(descMatch[1])
		}
		anchorText := strings.Join([]string{title, u}, " ")
		if !strings.Contains(strings.ToUpper(anchorText), model) {
			continue
		}
		if urlHasConflictingModel(model, u) {
			continue
		}
		block := rest[:end]
		price := likelySearchSnippetPrice(desc)
		evidence := "Brave Search exact-model result snippet: " + truncate(strings.Join(strings.Fields(stripHTMLTags(desc)), " "), 220)
		if price <= 0 {
			price = braveProductResultPrice(block)
			if price > 0 {
				evidence = "Brave Search exact-model product result price field"
			}
		}
		if price <= 0 {
			price = likelySearchSnippetPrice(title)
		}
		if price <= 0 {
			continue
		}
		key := strings.ToLower(u) + fmt.Sprintf("|%.2f", price)
		if seen[key] {
			continue
		}
		seen[key] = true
		source := "brave-search"
		if host := hostFromURL(u); host != "" {
			source = "brave-search:" + host
		}
		offers = append(offers, modelOffer{
			Source:   source,
			Title:    title,
			Price:    price,
			URL:      u,
			Evidence: evidence,
		})
	}
	return offers
}

func braveProductResultPrice(block string) float64 {
	productIdx := strings.Index(block, "product:{")
	if productIdx < 0 {
		return 0
	}
	productBlock := block[productIdx:]
	if nextResult := strings.Index(productBlock, `title:"`); nextResult > 0 {
		productBlock = productBlock[:nextResult]
	}
	pricePattern := regexp.MustCompile(`price:"([0-9][0-9,]*(?:\.[0-9]+)?)"`)
	if match := pricePattern.FindStringSubmatch(productBlock); len(match) > 1 {
		return parsePriceString(match[1])
	}
	return 0
}

func decodeSearchResultString(s string) string {
	replacements := map[string]string{
		`\"`:     `"`,
		`\/`:     `/`,
		`\u003C`: "<",
		`\u003c`: "<",
		`\u003E`: ">",
		`\u003e`: ">",
		`\u0026`: "&",
	}
	for old, replacement := range replacements {
		s = strings.ReplaceAll(s, old, replacement)
	}
	return html.UnescapeString(s)
}

func cleanSearchResultTitle(title string) string {
	for _, marker := range []string{`",description:`, `",page_age:`, `",profile:`} {
		if idx := strings.Index(title, marker); idx >= 0 {
			title = title[:idx]
		}
	}
	return strings.TrimSpace(title)
}

func likelySearchSnippetPrice(text string) float64 {
	text = regexp.MustCompile(`(?i)retail price\s*:\s*\$[0-9][0-9,]*(?:\.[0-9]{2})?`).ReplaceAllString(text, "")
	lower := strings.ToLower(text)
	if idx := strings.Index(lower, "today"); idx >= 0 {
		windowEnd := minInt(len(text), idx+180)
		window := lower[idx:windowEnd]
		if strings.Contains(window, "price") {
			if price := firstSnippetPrice(text[idx:windowEnd]); price > 0 {
				return price
			}
		}
	}
	for _, marker := range []string{"sale price", "price"} {
		if idx := strings.Index(lower, marker); idx >= 0 {
			windowEnd := minInt(len(text), idx+160)
			if price := firstSnippetPrice(text[idx:windowEnd]); price > 0 {
				return price
			}
		}
	}
	return firstSnippetPrice(text)
}

func firstSnippetPrice(text string) float64 {
	var prices []float64
	for _, match := range priceRegexp.FindAllStringIndex(text, -1) {
		if match[0] > 0 && text[match[0]-1] == '-' {
			continue
		}
		price := parsePriceString(text[match[0]:match[1]])
		if price <= 0 {
			continue
		}
		lowerWindow := strings.ToLower(text[maxInt(0, match[0]-40):minInt(len(text), match[1]+40)])
		if strings.Contains(lowerWindow, "shipping") || strings.Contains(lowerWindow, "fee") || strings.Contains(lowerWindow, "package") {
			continue
		}
		prices = append(prices, price)
	}
	if len(prices) == 0 {
		return 0
	}
	sort.Float64s(prices)
	return prices[0]
}

func hostFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	host := strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
	return host
}

func urlHasConflictingModel(model, rawURL string) bool {
	model = strings.ToUpper(strings.TrimSpace(model))
	urlUpper := strings.ToUpper(rawURL)
	if len(model) > 3 && !strings.Contains(urlUpper, model) && strings.Contains(urlUpper, model[:len(model)-1]) {
		return true
	}
	for _, found := range modelRegexp.FindAllString(urlUpper, -1) {
		if found == model {
			continue
		}
		if commonPrefixLen(found, model) >= minInt(len(model)-1, 7) {
			return true
		}
	}
	return false
}

func commonPrefixLen(a, b string) int {
	n := minInt(len(a), len(b))
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}

func discoverBoschModels(ctx context.Context, httpClient *http.Client, query string, limit int) ([]modelIntelRecord, error) {
	if !containsAny(strings.ToLower(query), "induction", "cooktop", "hob") {
		return nil, nil
	}
	body, url, err := fetchModelPage(ctx, httpClient, "https://www.bosch-home.com/us/en/category/cooking-baking/induction-electric-cooktops/induction-cooktops")
	if err != nil {
		return nil, err
	}
	models := orderedUnique(modelRegexp.FindAllString(body, -1))
	var rows []modelIntelRecord
	for _, model := range models {
		if len(rows) >= limit {
			break
		}
		if !strings.HasPrefix(model, "NIT") {
			continue
		}
		rec := modelIntelRecord{
			Model:      model,
			Brand:      "Bosch",
			Title:      strings.TrimSpace(boschTitleForModel(body, model)),
			Category:   "Induction cooktop",
			ProductURL: boschProductURL(model),
		}
		if rec.Title == "" {
			rec.Title = "Bosch induction cooktop " + model
		}
		if price := priceNearToken(body, model); price > 0 {
			rec.Offers = append(rec.Offers, modelOffer{
				Source:   "bosch",
				Price:    price,
				URL:      url,
				Evidence: "Bosch category page price near model",
			})
		}
		rows = append(rows, rec)
	}
	return rows, nil
}

func discoverBroanNuToneModels(ctx context.Context, httpClient *http.Client, query string, limit int) ([]modelIntelRecord, error) {
	route, category, ok := broanNuToneRouteForQuery(query)
	if !ok {
		return nil, nil
	}
	body, finalURL, err := fetchModelPage(ctx, httpClient, route)
	if err != nil {
		return nil, err
	}
	rows := broanNuToneModelRowsFromHTML(body, finalURL, category, limit)
	if len(rows) == 0 {
		return nil, fmt.Errorf("broan-nutone: no product cards found")
	}
	return rows, nil
}

func broanNuToneRouteForQuery(query string) (string, string, bool) {
	q := strings.ToLower(query)
	switch {
	case containsAny(q, "range hood", "hood insert", "vent hood", "kitchen hood"):
		return "https://www.broan-nutone.com/en-us/search/product?q=range%20hood", "Ventilation > Range Hoods", true
	case containsAny(q, "bath fan", "bathroom fan", "exhaust fan", "ventilation fan"):
		return "https://www.broan-nutone.com/en-us/search/product?q=bath%20fan", "Ventilation > Bath Fans", true
	default:
		return "", "", false
	}
}

func broanNuToneModelRowsFromHTML(body, baseURL, category string, limit int) []modelIntelRecord {
	var rows []modelIntelRecord
	searchFrom := 0
	for {
		idx := strings.Index(body[searchFrom:], `productCard productCard--slim`)
		if idx < 0 {
			break
		}
		start := searchFrom + idx
		cardStart := strings.LastIndex(body[:start], `<div`)
		if cardStart >= 0 {
			start = cardStart
		}
		end := len(body)
		next := strings.Index(body[start+len(`productCard productCard--slim`):], `productCard productCard--slim`)
		if next >= 0 {
			end = start + len(`productCard productCard--slim`) + next
		}
		block := body[start:end]
		if rec := normalizeBroanNuToneModelCard(block, baseURL, category); rec.Model != "" && modelIntelUsefulRecord(rec) && broanNuToneSelectionTitle(rec.Title) {
			rows = append(rows, rec)
			if limit > 0 && len(rows) >= limit {
				break
			}
		}
		searchFrom = end
	}
	return rows
}

func normalizeBroanNuToneModelCard(block, baseURL, category string) modelIntelRecord {
	model := strings.TrimSpace(html.UnescapeString(htmlAttr(block, "id")))
	if !looksLikeModelNumber(model) {
		model = strings.TrimSpace(stripHTMLTags(textBetween(block, `<h4 class="productPanel-infoOptionHeading">Model:</h4>`, `</div>`)))
		model = strings.TrimSpace(regexp.MustCompile(`(?i)^model:\s*`).ReplaceAllString(model, ""))
	}
	model = normalizeSelectionSKU(model)
	if !looksLikeModelNumber(model) {
		return modelIntelRecord{}
	}

	href := broanNuToneProductHref(block)
	title := strings.TrimSpace(html.UnescapeString(htmlAttr(block, "alt")))
	if title == "" {
		title = strings.TrimSpace(stripHTMLTags(textBetween(block, `class="productPanel-heading"`, `</h3>`)))
	}
	rec := modelIntelRecord{
		Model:        model,
		Brand:        broanNuToneBrand(title),
		Title:        strings.Join(strings.Fields(title), " "),
		Category:     category,
		ProductURL:   absoluteURL(baseURL, html.UnescapeString(href)),
		ImageURL:     absoluteURL(baseURL, html.UnescapeString(htmlAttr(block, "src"))),
		SourceFields: []string{"model_discovery: broan-nutone"},
	}
	if rec.Brand == "" {
		rec.Brand = "Broan-NuTone"
	}
	return rec
}

func broanNuToneProductHref(block string) string {
	match := regexp.MustCompile(`(?is)<a[^>]+href="([^"]*/en-us/product/[^"]+)"`).FindStringSubmatch(block)
	if len(match) > 1 {
		return match[1]
	}
	return htmlAttr(block, "href")
}

func broanNuToneBrand(title string) string {
	lower := strings.ToLower(title)
	switch {
	case strings.Contains(lower, "broan-nutone"):
		return "Broan-NuTone"
	case strings.Contains(lower, "nutone"):
		return "NuTone"
	case strings.Contains(lower, "broan"):
		return "Broan"
	default:
		return ""
	}
}

func broanNuToneSelectionTitle(title string) bool {
	lower := strings.ToLower(title)
	if containsAny(lower, "replacement", "motor", "wheel", "reducer", "filter kit", "installation kit", "grille", "cover", "part") {
		return false
	}
	return containsAny(lower, "bath", "exhaust fan", "ventilation fan", "range hood", "hood insert", "under cabinet", "wall mount", "chimney")
}

func probeModelPages(ctx context.Context, httpClient *http.Client, rec *modelIntelRecord) {
	if rec.Model == "" {
		return
	}
	if strings.EqualFold(rec.Brand, "Bosch") || strings.HasPrefix(rec.Model, "NIT") {
		probeBoschProductPage(ctx, httpClient, rec)
	}
	if shouldProbeMoenProductPage(rec) {
		probeMoenProductPage(ctx, httpClient, rec)
	}
	if shouldProbeLevitonProductPage(rec) {
		probeLevitonProductPage(ctx, httpClient, rec)
	}
	if shouldProbeAJMadison(rec) {
		probeAJMadisonModelPage(ctx, httpClient, rec)
	}
}

func enrichModelProductPage(ctx context.Context, httpClient *http.Client, rec *modelIntelRecord) {
	if rec.ProductURL == "" {
		return
	}
	source := modelRecordSource(rec)
	originalURL := rec.ProductURL
	body, finalURL, err := fetchModelPage(ctx, httpClient, rec.ProductURL)
	if err != nil {
		return
	}
	if shouldUseFinalModelURL(rec.Model, originalURL, finalURL) {
		rec.ProductURL = finalURL
	}
	if rec.Title == "" {
		rec.Title = html.UnescapeString(metaContent(body, "title"))
	}
	rec.Specs = appendUniqueSpecs(rec.Specs, filterModelSpecDocs(rec.Model, specDocsFromHTML(source, body, rec.ProductURL)))
	if len(rec.Offers) == 0 && strings.Contains(hostFromURL(rec.ProductURL), "hardwarehut.com") {
		if price := hardwareHutProductPagePrice(body); price > 0 {
			rec.Offers = append(rec.Offers, modelOffer{Source: "hardware-hut", Price: price, URL: rec.ProductURL, Evidence: "Hardware Hut product page schema price"})
		}
	}
}

func enrichModelOfferPages(ctx context.Context, httpClient *http.Client, rec *modelIntelRecord, limit int) {
	if limit <= 0 {
		return
	}
	seen := map[string]bool{}
	if rec.ProductURL != "" {
		seen[rec.ProductURL] = true
	}
	enriched := 0
	for _, offer := range rec.Offers {
		if offer.URL == "" || seen[offer.URL] {
			continue
		}
		seen[offer.URL] = true
		if rec.ProductURL == "" {
			rec.ProductURL = offer.URL
		}
		if rec.Title == "" && offer.Title != "" {
			rec.Title = offer.Title
		}
		body, finalURL, err := fetchModelPage(ctx, httpClient, offer.URL)
		if err != nil {
			status, reason := classifyModelPageFailure(body, err)
			rec.ProbeStatus = append(rec.ProbeStatus, modelProbe{Source: offer.Source, URL: finalURL, Status: status, Reason: reason})
			continue
		}
		if rec.ProductURL == "" && shouldUseFinalModelURL(rec.Model, offer.URL, finalURL) {
			rec.ProductURL = finalURL
		}
		if rec.Title == "" {
			rec.Title = html.UnescapeString(metaContent(body, "title"))
		}
		source := offer.Source
		if source == "" {
			source = hostFromURL(finalURL)
		}
		rec.Specs = appendUniqueSpecs(rec.Specs, filterModelSpecDocs(rec.Model, specDocsFromHTML(source, body, finalURL)))
		enriched++
		if enriched >= limit {
			return
		}
	}
}

func shouldUseFinalModelURL(model, originalURL, finalURL string) bool {
	if finalURL == "" {
		return false
	}
	if originalURL == "" || finalURL == originalURL || model == "" {
		return true
	}
	model = strings.ToUpper(strings.TrimSpace(model))
	finalUpper := strings.ToUpper(finalURL)
	originalUpper := strings.ToUpper(originalURL)
	if strings.Contains(finalUpper, model) {
		return true
	}
	if strings.Contains(originalUpper, model) {
		return false
	}
	return !urlHasConflictingModel(model, finalURL)
}

func classifyModelPageFailure(body string, err error) (string, string) {
	reason := err.Error()
	lower := strings.ToLower(body)
	if containsAny(lower, "px-captcha", "perimeterx", "access to this page has been denied", "access denied", "cloudflare", "just a moment", "vercel security checkpoint", "enable javascript and cookies") {
		return "blocked", truncate(reason, 180)
	}
	return "error", truncate(reason, 180)
}

func modelRecordSource(rec *modelIntelRecord) string {
	if rec.BestSource != "" {
		return rec.BestSource
	}
	for _, offer := range rec.Offers {
		if offer.Source != "" {
			return offer.Source
		}
	}
	for _, spec := range rec.Specs {
		if spec.Source != "" {
			return spec.Source
		}
	}
	return "product-page"
}

func shouldProbeMoenProductPage(rec *modelIntelRecord) bool {
	if rec == nil || rec.Model == "" {
		return false
	}
	haystack := strings.ToLower(strings.Join([]string{rec.Brand, rec.Title}, " "))
	return strings.Contains(haystack, "moen")
}

func probeMoenProductPage(ctx context.Context, httpClient *http.Client, rec *modelIntelRecord) {
	url := moenShopProductURL(rec.Model)
	body, finalURL, err := fetchModelPage(ctx, httpClient, url)
	if err != nil {
		rec.ProbeStatus = append(rec.ProbeStatus, modelProbe{Source: "moen", URL: finalURL, Status: "error", Reason: err.Error()})
		return
	}
	if !moenPageContainsModel(body, rec.Model) {
		rec.ProbeStatus = append(rec.ProbeStatus, modelProbe{Source: "moen", URL: finalURL, Status: "not_found", Reason: "model not present in response"})
		return
	}
	if rec.ProductURL == "" || strings.Contains(hostFromURL(rec.ProductURL), "kbauthority.com") {
		rec.ProductURL = finalURL
	}
	if rec.Title == "" {
		rec.Title = html.UnescapeString(metaContent(body, "og:title"))
	}
	if price := moenShopPriceForModel(body, rec.Model); price > 0 {
		rec.Offers = append(rec.Offers, modelOffer{Source: "moen", Price: price, URL: finalURL, Evidence: "Moen product page price"})
	}
	rec.Specs = appendUniqueSpecs(rec.Specs, filterModelSpecDocs(rec.Model, moenSpecDocsFromHTML(body)))
	rec.ProbeStatus = append(rec.ProbeStatus, modelProbe{Source: "moen", URL: finalURL, Status: "ok"})
}

func moenShopProductURL(model string) string {
	return "https://shop.moen.com/products/" + strings.ToLower(moenBaseModel(model))
}

func moenBaseModel(model string) string {
	model = strings.ToUpper(strings.TrimSpace(model))
	for _, suffix := range []string{"ORB", "BN", "BG", "BL", "NL", "CH", "CP"} {
		if strings.HasSuffix(model, suffix) && len(model) > len(suffix)+3 {
			return strings.TrimSuffix(model, suffix)
		}
	}
	return model
}

func moenPageContainsModel(body, model string) bool {
	upper := strings.ToUpper(body)
	model = strings.ToUpper(strings.TrimSpace(model))
	base := moenBaseModel(model)
	return strings.Contains(upper, model) || (base != "" && strings.Contains(upper, base))
}

func moenShopPriceForModel(body, model string) float64 {
	upper := strings.ToUpper(body)
	model = strings.ToUpper(strings.TrimSpace(model))
	base := moenBaseModel(model)
	for _, token := range orderedUnique([]string{model, base}) {
		idx := strings.Index(upper, `"SKU":"`+token+`"`)
		if idx < 0 {
			idx = strings.Index(upper, `"SKU": "`+token+`"`)
		}
		if idx < 0 {
			idx = strings.Index(upper, `SKU":"`+token+`"`)
		}
		if idx < 0 {
			continue
		}
		start := idx
		end := idx + 1600
		if end > len(body) {
			end = len(body)
		}
		window := body[start:end]
		for _, pattern := range []string{
			`"price"\s*:\s*([0-9]{3,})`,
			`"price"\s*:\s*"([0-9]+(?:\.[0-9]+)?)"`,
			`"amount"\s*:\s*([0-9]+(?:\.[0-9]+)?)`,
		} {
			match := regexp.MustCompile(pattern).FindStringSubmatch(window)
			if len(match) <= 1 {
				continue
			}
			price := parsePriceString(match[1])
			if strings.Contains(pattern, `{3,}`) {
				price = price / 100
			}
			if price > 0 {
				return price
			}
		}
	}
	return priceNearToken(body, model)
}

func moenSpecDocsFromHTML(body string) []modelSpecDocument {
	seen := map[string]bool{}
	var docs []modelSpecDocument
	re := regexp.MustCompile(`https:\\?/\\?/assets\.moen\.com\\?/shared\\?/docs\\?/[^"'<>\\]+?\.pdf|https://assets\.moen\.com/shared/docs/[^"'<>\\]+?\.pdf`)
	for _, raw := range re.FindAllString(body, -1) {
		u := strings.ReplaceAll(raw, `\/`, `/`)
		if seen[u] {
			continue
		}
		seen[u] = true
		kind := "document"
		lower := strings.ToLower(u)
		if strings.Contains(lower, "/product-specifications/") {
			kind = "spec"
		} else if strings.Contains(lower, "/instruction-sheets/") {
			kind = "installation"
		}
		docs = append(docs, modelSpecDocument{Kind: kind, URL: u, Source: "moen"})
	}
	return docs
}

func shouldProbeLevitonProductPage(rec *modelIntelRecord) bool {
	if rec == nil || rec.Model == "" {
		return false
	}
	haystack := strings.ToLower(strings.Join([]string{rec.Brand, rec.Title, rec.ProductURL}, " "))
	return strings.Contains(haystack, "leviton")
}

func probeLevitonProductPage(ctx context.Context, httpClient *http.Client, rec *modelIntelRecord) {
	url := levitonProductURL(rec.Model)
	body, finalURL, err := fetchModelPage(ctx, httpClient, url)
	if err != nil {
		rec.ProbeStatus = append(rec.ProbeStatus, modelProbe{Source: "leviton", URL: finalURL, Status: "error", Reason: err.Error()})
		return
	}
	if !strings.Contains(strings.ToUpper(body), strings.ToUpper(strings.TrimSpace(rec.Model))) {
		rec.ProbeStatus = append(rec.ProbeStatus, modelProbe{Source: "leviton", URL: finalURL, Status: "not_found", Reason: "model not present in response"})
		return
	}
	if rec.ProductURL == "" || strings.Contains(hostFromURL(rec.ProductURL), "1000bulbs.com") {
		rec.ProductURL = finalURL
	}
	if rec.Brand == "" {
		rec.Brand = "Leviton"
	}
	if rec.Title == "" {
		rec.Title = html.UnescapeString(metaContent(body, "og:title"))
	}
	rec.Specs = appendUniqueSpecs(rec.Specs, filterModelSpecDocs(rec.Model, levitonSpecDocsFromHTML(body, finalURL)))
	rec.ProbeStatus = append(rec.ProbeStatus, modelProbe{Source: "leviton", URL: finalURL, Status: "ok"})
}

func levitonProductURL(model string) string {
	return "https://leviton.com/products/" + strings.ToLower(strings.TrimSpace(model))
}

func levitonSpecDocsFromHTML(body, finalURL string) []modelSpecDocument {
	seen := map[string]bool{}
	var docs []modelSpecDocument
	for _, doc := range specDocsFromHTML("leviton", body, finalURL) {
		lower := strings.ToLower(doc.URL)
		switch {
		case strings.Contains(lower, "/product_specification/"):
			doc.Kind = "spec"
		case strings.Contains(lower, "/instruction_sheet/"):
			doc.Kind = "installation"
		default:
			continue
		}
		if containsAny(lower, "french", "-fr.", "-sp.", "cleaning", "case%20study", "case-study", "buying-guide", "application_note", "infographic", "solution-sheet") {
			continue
		}
		if seen[doc.URL] {
			continue
		}
		seen[doc.URL] = true
		docs = append(docs, doc)
	}
	return docs
}

func shouldProbeAJMadison(rec *modelIntelRecord) bool {
	if rec.Title == "" && rec.ProductURL == "" {
		return true
	}
	haystack := strings.ToLower(strings.Join([]string{rec.Brand, rec.Title, rec.ProductURL}, " "))
	if containsAny(haystack, "appliance", "dishwasher", "range", "refrigerator", "cooktop", "oven", "laundry", "washer", "dryer") {
		return true
	}
	if containsAny(strings.ToLower(rec.Brand), "ge", "bosch", "jennair", "jenn-air", "kitchenaid", "whirlpool", "maytag", "thermador", "miele", "lg", "samsung") {
		return true
	}
	return false
}

func probeBoschProductPage(ctx context.Context, httpClient *http.Client, rec *modelIntelRecord) {
	url := boschProductURL(rec.Model)
	body, finalURL, err := fetchModelPage(ctx, httpClient, url)
	if err != nil {
		rec.ProbeStatus = append(rec.ProbeStatus, modelProbe{Source: "bosch", URL: url, Status: "error", Reason: err.Error()})
		return
	}
	if !strings.Contains(strings.ToUpper(body), rec.Model) {
		rec.ProbeStatus = append(rec.ProbeStatus, modelProbe{Source: "bosch", URL: finalURL, Status: "not_found", Reason: "model not present in response"})
		return
	}
	rec.ProductURL = finalURL
	if rec.Brand == "" {
		rec.Brand = "Bosch"
	}
	if rec.Title == "" {
		rec.Title = html.UnescapeString(metaContent(body, "title"))
	}
	if price := priceNearToken(body, rec.Model); price > 0 {
		rec.Offers = append(rec.Offers, modelOffer{Source: "bosch", Price: price, URL: finalURL, Evidence: "Bosch product page price"})
	}
	rec.Specs = appendUniqueSpecs(rec.Specs, filterModelSpecDocs(rec.Model, specDocsFromHTML("bosch", body, finalURL)))
	rec.ProbeStatus = append(rec.ProbeStatus, modelProbe{Source: "bosch", URL: finalURL, Status: "ok"})
}

func probeAJMadisonModelPage(ctx context.Context, httpClient *http.Client, rec *modelIntelRecord) {
	url := "https://www.ajmadison.com/cgi-bin/ajmadison/" + rec.Model + ".html"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		rec.ProbeStatus = append(rec.ProbeStatus, modelProbe{Source: "ajmadison", URL: url, Status: "error", Reason: err.Error()})
		return
	}
	req.Header.Set("Accept", "text/html")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")
	resp, err := httpClient.Do(req)
	if err != nil {
		rec.ProbeStatus = append(rec.ProbeStatus, modelProbe{Source: "ajmadison", URL: url, Status: "error", Reason: err.Error()})
		return
	}
	defer resp.Body.Close()
	bodyBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		rec.ProbeStatus = append(rec.ProbeStatus, modelProbe{Source: "ajmadison", URL: url, Status: "error", Reason: readErr.Error()})
		return
	}
	body := string(bodyBytes)
	if resp.StatusCode == http.StatusForbidden || containsAny(strings.ToLower(body), "px-captcha", "access to this page has been denied", "perimeterx") {
		rec.ProbeStatus = append(rec.ProbeStatus, modelProbe{Source: "ajmadison", URL: url, Status: "blocked", Reason: "PerimeterX/captcha response"})
		return
	}
	if resp.StatusCode >= 400 {
		rec.ProbeStatus = append(rec.ProbeStatus, modelProbe{Source: "ajmadison", URL: url, Status: "not_found", Reason: fmt.Sprintf("HTTP %d", resp.StatusCode)})
		return
	}
	if !strings.Contains(strings.ToUpper(body), rec.Model) {
		rec.ProbeStatus = append(rec.ProbeStatus, modelProbe{Source: "ajmadison", URL: url, Status: "not_found", Reason: "model not present in response"})
		return
	}
	if price := priceNearToken(body, rec.Model); price > 0 {
		rec.Offers = append(rec.Offers, modelOffer{Source: "ajmadison", Price: price, URL: url, Evidence: "AJ Madison model page price"})
	}
	rec.Specs = appendUniqueSpecs(rec.Specs, filterModelSpecDocs(rec.Model, specDocsFromHTML("ajmadison", body, url)))
	rec.ProbeStatus = append(rec.ProbeStatus, modelProbe{Source: "ajmadison", URL: url, Status: "ok"})
}

func hardwareHutProductPagePrice(body string) float64 {
	for _, pattern := range []string{
		`(?is)"offers"\s*:\s*\{[^{}]*"priceCurrency"\s*:\s*"USD"[^{}]*"price"\s*:\s*"?([0-9][0-9.,]*)"?`,
		`(?is)"offers"\s*:\s*\{[^{}]*"price"\s*:\s*"?([0-9][0-9.,]*)"?[^{}]*"priceCurrency"\s*:\s*"USD"`,
		`(?is)"price"\s*:\s*"?([0-9][0-9.,]*)"?[^{}]*"priceCurrency"\s*:\s*"USD"`,
	} {
		for _, match := range regexp.MustCompile(pattern).FindAllStringSubmatch(body, -1) {
			price := parsePriceString(match[1])
			if price > 1 {
				return price
			}
		}
	}
	return 0
}

func renderModelIntelTable(cmd *cobra.Command, flags *rootFlags, result modelIntelResult) error {
	rows := make([][]string, 0, len(result.Models))
	for _, rec := range result.Models {
		price := ""
		if rec.BestPrice > 0 {
			price = fmt.Sprintf("$%.2f", rec.BestPrice)
		}
		rows = append(rows, []string{
			rec.Model,
			rec.Brand,
			rec.Title,
			price,
			rec.BestSource,
			fmt.Sprintf("%d", len(rec.Specs)),
			rec.ProductURL,
		})
	}
	return flags.printTable(cmd, []string{"model", "brand", "title", "best_price", "source", "specs", "url"}, rows)
}

func fetchModelPage(ctx context.Context, httpClient *http.Client, rawURL string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return "", rawURL, err
	}
	req.Header.Set("Accept", "text/html")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", rawURL, err
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return "", resp.Request.URL.String(), readErr
	}
	if resp.StatusCode >= 400 {
		return string(body), resp.Request.URL.String(), fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 120))
	}
	return string(body), resp.Request.URL.String(), nil
}

func mergeModelRecord(dst *modelIntelRecord, src modelIntelRecord) {
	if dst.Brand == "" {
		dst.Brand = src.Brand
	}
	if dst.Title == "" {
		dst.Title = src.Title
	}
	if dst.Category == "" {
		dst.Category = src.Category
	}
	if dst.ProductURL == "" {
		dst.ProductURL = src.ProductURL
	}
	if dst.ImageURL == "" {
		dst.ImageURL = src.ImageURL
	}
	dst.Offers = append(dst.Offers, src.Offers...)
	dst.Specs = appendUniqueSpecs(dst.Specs, src.Specs)
	dst.SourceFields = append(dst.SourceFields, src.SourceFields...)
}

func finalizeBestOffer(rec *modelIntelRecord) {
	for _, offer := range rec.Offers {
		if offer.Price <= 0 {
			continue
		}
		if rec.BestPrice == 0 || offer.Price < rec.BestPrice {
			rec.BestPrice = offer.Price
			rec.BestSource = offer.Source
		}
	}
}

func resolveModelIntelSources(query, sources, category, room string) ([]string, []string, string, error) {
	sources = strings.TrimSpace(sources)
	if sources == "" {
		sources = "auto"
	}
	switch sources {
	case "auto":
		if category != "" || room != "" {
			resolved, categories, resolvedRoom, err := resolveSources(category, room, "")
			return appendModelDiscoverySources(resolved, categories), categories, resolvedRoom, err
		}
		if inferred := inferModelIntelCategories(query); len(inferred) > 0 {
			return appendModelDiscoverySources(resolveSourcesForCategories(inferred), inferred), inferred, "", nil
		}
		return []string{"ge-appliances", "bosch"}, nil, "", nil
	case "routed":
		if category == "" && room == "" {
			return nil, nil, "", fmt.Errorf("--sources routed requires --category or --room")
		}
		resolved, categories, resolvedRoom, err := resolveSources(category, room, "")
		return resolved, categories, resolvedRoom, err
	case "active":
		var out []string
		for _, source := range activeSources() {
			out = append(out, source.Name)
		}
		return out, nil, "", nil
	default:
		out := splitModelIntelCSV(sources)
		for _, source := range out {
			if source == "bosch" || source == "broan-nutone" {
				continue
			}
			cfg := sourceByName(source)
			if cfg == nil {
				return nil, nil, "", fmt.Errorf("unknown model-intel source %q", source)
			}
			if cfg.Status != "active" {
				return nil, nil, "", fmt.Errorf("model-intel source %q is %s, not active", source, cfg.Status)
			}
		}
		return out, nil, "", nil
	}
}

func appendModelDiscoverySources(sources, categories []string) []string {
	if stringSliceContains(categories, "hvac") || stringSliceContains(categories, "appliances") {
		sources = appendUniqueStrings(sources, "broan-nutone")
	}
	return sources
}

func stringSliceContains(vals []string, target string) bool {
	for _, val := range vals {
		if val == target {
			return true
		}
	}
	return false
}

func inferModelIntelCategories(query string) []string {
	q := strings.ToLower(query)
	type rule struct {
		Category string
		Terms    []string
	}
	rules := []rule{
		{Category: "hvac", Terms: []string{"mini split", "minisplit", "heat pump", "ductless", "air handler", "condenser", "dehumidifier", "bath fan", "exhaust fan", "thermostat", "floor register", "air register", "wall register", "ceiling register", "return grille", "register grille", "floor vent", "wall vent", "vent cover", "erv", "hrv"}},
		{Category: "electrical", Terms: []string{"recessed light", "downlight", "can light", "led tape", "under cabinet", "under-cabinet", "dimmer", "driver", "transformer", "sconce", "pendant", "chandelier", "picture light", "picture lights", "art light", "vanity light", "vanity lights", "bath vanity light", "bathroom vanity light", "bathroom vanity lights", "ceiling fan", "towel warmer", "heated towel", "towel radiator", "lighted mirror", "led mirror", "illuminated mirror", "fixture", "bulb"}},
		{Category: "plumbing", Terms: []string{"faucet", "sink", "toilet", "bidet", "bidet seat", "washlet", "shower", "tub", "valve", "rough-in", "rough in", "vanity", "medicine cabinet", "mirror cabinet", "vanity light", "vanity lights", "bath vanity light", "bathroom vanity light", "bathroom vanity lights", "lighted mirror", "led mirror", "illuminated mirror", "grab bar", "ada bar", "safety bar", "robe hook", "robe hooks", "bath hook", "bath hooks", "towel bar", "towel bars", "towel rail", "towel rails", "towel holder", "towel holders", "towel ring", "towel rings", "soap dispenser", "soap dispensers", "lotion dispenser", "lotion dispensers", "towel warmer", "heated towel", "towel radiator", "pot filler", "disposal", "linear drain", "shower drain", "tile drain", "drain grate"}},
		{Category: "flooring", Terms: []string{"tile", "flooring", "floor tile", "wall tile", "mosaic", "paver", "plank", "backsplash", "stone", "floor warming", "heated floor", "radiant floor", "floor heat", "linear drain", "shower drain", "tile drain", "drain grate"}},
		{Category: "hardware", Terms: []string{"hinge", "pull", "knob", "cabinet hardware", "door hardware", "door lever", "door levers", "interior lever", "interior levers", "entry lever", "entry levers", "passage lever", "privacy lever", "handleset", "handle set", "lockset", "lock set", "door knob", "door knobs", "deadbolt", "latch", "handle", "drawer slide", "rail", "grab bar", "ada bar", "safety bar", "robe hook", "robe hooks", "bath hook", "bath hooks", "wardrobe hook", "wardrobe hooks", "towel bar", "towel bars", "towel rail", "towel rails", "towel holder", "towel holders", "register", "vent cover"}},
		{Category: "materials", Terms: []string{"countertop", "slab", "trim", "molding", "panel", "shelf", "threshold"}},
		{Category: "appliances", Terms: []string{"cooktop", "range", "oven", "dishwasher", "refrigerator", "freezer", "microwave", "washer", "dryer", "range hood", "hood insert", "induction"}},
		{Category: "furniture", Terms: []string{"sofa", "chair", "table", "stool", "bed", "dresser", "nightstand", "sectional"}},
		{Category: "decor", Terms: []string{"rug", "mirror", "medicine cabinet", "mirror cabinet", "lighted mirror", "led mirror", "illuminated mirror", "curtain", "shade", "lamp", "vase"}},
	}
	seen := map[string]bool{}
	var out []string
	for _, rule := range rules {
		for _, term := range rule.Terms {
			if strings.Contains(q, term) {
				if !seen[rule.Category] {
					seen[rule.Category] = true
					out = append(out, rule.Category)
				}
				break
			}
		}
	}
	return out
}

func modelIntelUsefulRecord(rec modelIntelRecord) bool {
	if rec.Model == "" || rec.Title == "" {
		return false
	}
	model := strings.ToUpper(rec.Model)
	if containsAny(model, "ENERGY", "STAR", "SERIES", "SEER", "BTU", "LUMEN", "VOLT", "WATT") {
		return false
	}
	if len(model) < 5 {
		return false
	}
	if len(model) > 80 {
		return false
	}
	return rec.BestPrice > 0 || len(rec.Offers) > 0 || len(rec.Specs) > 0 || rec.ProductURL != ""
}

func modelIntelProductMatchesQuery(query string, p NormalizedProduct) bool {
	q := strings.ToLower(query)
	if strings.Contains(q, "ceiling fan") {
		haystack := strings.ToLower(strings.Join([]string{p.Title, p.Category, p.URL}, " "))
		if containsAny(haystack, "headlight", "fog light", "conversion kit", "appliance bulb", "light bulb", "lamp", "mount for", "wall mount", "ceiling mount") {
			return false
		}
		return strings.Contains(haystack, "ceiling fan") || strings.Contains(haystack, "indoor ceiling fans") || strings.Contains(haystack, "outdoor ceiling fans")
	}
	if strings.Contains(q, "thermostat") {
		haystack := strings.ToLower(strings.Join([]string{p.Title, p.Category, p.Description, p.URL}, " "))
		if containsAny(haystack, "replacement defrost", "replacement part", "service part", "compatible with the following") {
			return false
		}
		return strings.Contains(haystack, "thermostat")
	}
	if modelIntelQueryLooksLikeHVACRegister(q) {
		haystack := strings.ToLower(strings.Join([]string{p.Title, p.Category, p.Description, p.URL}, " "))
		return containsAny(haystack, "floor register", "wall register", "ceiling register", "air register", "register louver", "register damper", "register grille", "register grill", "floor grille", "floor grill", "return grille", "return grill", "vent cover", "floor vent", "wall vent", "air vent", "diffuser")
	}
	if modelIntelQueryLooksLikeLinearDrain(q) {
		haystack := strings.ToLower(strings.Join([]string{p.Title, p.Category, p.Description, p.URL}, " "))
		if containsAny(haystack, "drain liner", "drain plug", "plug drain", "condensate drain", "dishwasher drain", "refrigerator drain", "washer drain") {
			return false
		}
		return containsAny(haystack, "linear drain", "shower drain", "tile-in drain", "tile insert shower", "tileable grate", "linear shower drain", "kerdi-line", "drain body")
	}
	if modelIntelQueryLooksLikeShowerNiche(q) {
		haystack := strings.ToLower(strings.Join([]string{p.Title, p.Category, p.Description, p.URL}, " "))
		if containsAny(haystack, "shower trim", "valve trim", "control trim", "plr handle", "volume control", "diverter trim", "tub spout") {
			return false
		}
		return containsAny(haystack, "shower niche", "wall niche", "niche with frame", "niche divider", "rectangle niche", "square niche", "double niche", "single niche", "hydro ban niche", "kerdi niche", "shower niches", "wall shelves")
	}
	if modelIntelQueryLooksLikeMedicineCabinet(q) {
		haystack := strings.ToLower(strings.Join([]string{p.Title, p.Category, p.Description, p.URL}, " "))
		if containsAny(haystack, "shelf kit", "risers", "riser set", "organizer", "organiser", "storage bin", "kitchen cabinet", "kitchen cabinets", "display cabinet", "storage cabinet", "cabinet with doors", "cabinet with 2 doors", "hutches", "cupboards") {
			return false
		}
		return containsAny(haystack, "medicine cabinet", "mirror cabinet", "medicine cabinets with mirror", "mirror cabinets", "bathroom mirror cabinet")
	}
	if modelIntelQueryLooksLikeGrabBar(q) {
		haystack := strings.ToLower(strings.Join([]string{p.Title, p.Category, p.Description, p.URL}, " "))
		if containsAny(haystack, "water heater", "water dispenser", "dishwasher", "clip-on handle", "clip on handle", "cabinet handle", "towel rail", "suspension rail", "rail with hooks", "step stool", "stop bar", "shower glass clamp", "shower door hinge") {
			return false
		}
		return containsAny(haystack, "grab bar", "ada compliant grab", "safety bar")
	}
	if modelIntelQueryLooksLikeRobeHook(q) {
		haystack := strings.ToLower(strings.Join([]string{p.Title, p.Category, p.Description, p.URL}, " "))
		if containsAny(haystack, "wall hook", "door hook", "hook for door", "suction cup", "purse hook", "ceiling hook", "undercounter", "pot lid", "robe", "bathrobe") && !containsAny(haystack, "robe hook", "robe hooks", "wardrobe hook", "wardrobe hooks") {
			return false
		}
		return containsAny(haystack, "robe hook", "robe hooks", "wardrobe hook", "wardrobe hooks", "bath hook", "bath hooks") || (containsAny(haystack, "double hook") && containsAny(haystack, "bathroom", "bath"))
	}
	if modelIntelQueryLooksLikeTowelBar(q) {
		haystack := strings.ToLower(strings.Join([]string{p.Title, p.Category, p.Description, p.URL}, " "))
		if containsAny(haystack, "kitchen", "sektion", "cleaning cabinet") && !containsAny(haystack, "towel bar", "towel bars") {
			return false
		}
		if containsAny(haystack, "dishwasher", "dry boost", "sanitize cycle", "hook", "hanger", "clothes hanger", "kitchen organization", "kitchen wall organizers", "hultarp", "kungsfors", "nereby") && !containsAny(haystack, "towel bar", "towel bars", "towel rail", "towel rails", "towel holder", "towel holders") {
			return false
		}
		return containsAny(haystack, "towel bar", "towel bars", "towel rail", "towel rails", "towel holder", "towel holders")
	}
	if modelIntelQueryLooksLikeTowelRing(q) {
		haystack := strings.ToLower(strings.Join([]string{p.Title, p.Category, p.Description, p.URL}, " "))
		if containsAny(haystack, "hook", "clip", "rack", "hand towel", "bathroom textiles", "spoonrest", "spoon rest", "kitchen", "småstad", "smastad", "towel hanger") && !containsAny(haystack, "towel ring", "towel rings") {
			return false
		}
		return containsAny(haystack, "towel ring", "towel rings")
	}
	if modelIntelQueryLooksLikeSoapDispenser(q) {
		haystack := strings.ToLower(strings.Join([]string{p.Title, p.Category, p.Description, p.URL}, " "))
		titleHaystack := strings.ToLower(strings.Join([]string{p.Title, p.Description, p.URL}, " "))
		if containsAny(titleHaystack, "soap dish", "bathroom set") {
			return false
		}
		if containsAny(haystack, "washer", "washing machine", "dish brush", "dish-washing brush", "brush refill", "drying mat", "dish rack", "oil/vinegar", "oil vinegar", "soap dish", "bathroom set", "spice jars") && !containsAny(titleHaystack, "soap dispenser", "soap dispensers", "lotion dispenser", "lotion dispensers", "soap/lotion") {
			return false
		}
		return containsAny(titleHaystack, "soap dispenser", "soap dispensers", "lotion dispenser", "lotion dispensers", "soap/lotion dispenser", "soap and lotion dispenser")
	}
	if modelIntelQueryLooksLikeTowelWarmer(q) {
		haystack := strings.ToLower(strings.Join([]string{p.Title, p.Category, p.Description, p.URL}, " "))
		if containsAny(haystack, "steam closet", "fabric refresh", "led bulb", "towel ring", "towel hook", "towel rail", "towel rack", "towel hanger", "towel holder", "towel with hood", "switch", "mounting kit", "replacement element", "extension", "valve housing", "spare parts", "accessories", "timer for tower warmers") {
			return false
		}
		return containsAny(haystack, "towel warmer", "towel warmers", "heated towel", "towel radiator")
	}
	if modelIntelQueryLooksLikeShowerDoor(q) {
		haystack := strings.ToLower(strings.Join([]string{p.Title, p.Category, p.Description, p.URL}, " "))
		if containsAny(haystack, "door handle", "handle escutcheon", "escutcheons", "door pull", "shower components", "glass clamp", "door hinge", "hinge only", "replacement seal", "sweep") {
			return false
		}
		return containsAny(haystack, "shower door", "shower doors", "shower screen", "shower screens", "frameless screen", "shower panel", "bypass sliding", "bypass shower", "hinged shower", "sliding shower")
	}
	if modelIntelQueryLooksLikeLightedMirror(q) {
		haystack := strings.ToLower(strings.Join([]string{p.Title, p.Category, p.Description, p.URL}, " "))
		hasMirrorLighting := containsAny(haystack, "led mirror", "lighted mirror", "illuminated mirror", "illuminated mirrors", "mirror with built-in light", "built-in light", "integrated lighting", "lighted mirrors")
		if containsAny(haystack, "led bulb", "light bulb", "incandescent", "wall lamp", "wall sconce", "vanity & wall light", "bathroom vanity lights", "plain mirror", "wall mirror", "vanity mirror", "table mirror", "mirror with shelf") && !hasMirrorLighting {
			return false
		}
		return hasMirrorLighting
	}
	if modelIntelQueryLooksLikeVanityLight(q) {
		haystack := strings.ToLower(strings.Join([]string{p.Title, p.Category, p.Description, p.URL}, " "))
		hasVanityLighting := containsAny(haystack, "vanity light", "vanity lights", "bathroom vanity light", "bathroom vanity lights", "bath vanity light", "bath vanity lights", "bath bar")
		if containsAny(haystack, "light bulb", "led bulb", "incandescent", "festoon", "cars, trucks", "vehicle lighting") {
			return false
		}
		if containsAny(haystack, "outdoor", "single sconce", "wall sconce", "bathroom vanity", "vanity cabinet", "countertop") && !hasVanityLighting {
			return false
		}
		return hasVanityLighting
	}
	if modelIntelQueryLooksLikePictureLight(q) {
		haystack := strings.ToLower(strings.Join([]string{p.Title, p.Category, p.Description, p.URL}, " "))
		if containsAny(haystack, "light bulb", "led bulb", "incandescent", "t5", "t8", "pygmy", "intermediate base", "european base", "shatter resistant", "tubular", "kitchen hub", "range hood", "wall mount hood", "refrigerator") {
			return false
		}
		return containsAny(haystack, "picture light", "picture lights", "art light")
	}
	if modelIntelQueryLooksLikeBidetSeat(q) {
		haystack := strings.ToLower(strings.Join([]string{p.Title, p.Category, p.Description, p.URL}, " "))
		if containsAny(haystack, "one-piece toilet", "two-piece toilet", "high-efficiency toilet", "toilet tank", "toilet bowl") && !containsAny(haystack, "bidet", "washlet") {
			return false
		}
		return containsAny(haystack, "bidet seat", "bidet seats", "bidet toilet seat", "electronic bidet", "electric bidet", "washlet")
	}
	if modelIntelQueryLooksLikePotFiller(q) {
		haystack := strings.ToLower(strings.Join([]string{p.Title, p.Category, p.Description, p.URL}, " "))
		if containsAny(haystack, "handle kit", "repair kit", "replacement", "cartridge", "spout kit", "parts") {
			return false
		}
		return containsAny(haystack, "pot filler", "potfiller")
	}
	if modelIntelQueryLooksLikeShowerValve(q) {
		haystack := strings.ToLower(strings.Join([]string{p.Title, p.Category, p.Description, p.URL}, " "))
		hasValve := containsAny(haystack, "shower valve", "valve trim", "control trim", "pressure balance", "pressure balanced", "thermostatic valve", "transfer valve", "diverter valve", "rough-in valve", "rough in valve", "m-core", "m core", "tub/shower valve", "tub and shower valve", "rough-in", "rough in")
		if containsAny(haystack, "floor tile", "wall tile", "marble tile", "mosaic", "porcelain tile", "ceramic tile", "shower door", "shower niche", "shower head", "shower panel", "soap dish", "grab bar", "towel", "robe hook") && !hasValve {
			return false
		}
		return hasValve
	}
	if modelIntelQueryLooksLikeShowerHead(q) {
		haystack := strings.ToLower(strings.Join([]string{p.Title, p.Category, p.Description, p.URL}, " "))
		if containsAny(haystack, "tub and shower", "tub & shower", "shower combination", "shower trim", "valve trim", "control trim", "diverter trim", "diverter bathcock", "bathcock", "riser", "rough-in", "rough in", "shower arm", "shower hose", "slide bar", "grab bar", "wall supply", "drop elbow") {
			return false
		}
		return containsAny(haystack, "shower head", "shower heads", "showerhead", "spray head", "rain shower", "rainshower", "hand shower", "handshower")
	}
	if modelIntelQueryLooksLikeShowerPanel(q) {
		haystack := strings.ToLower(strings.Join([]string{p.Title, p.Category, p.Description, p.URL}, " "))
		if containsAny(haystack, "hook", "curtain", "load in crate", "crate", "shower door handle", "glass clamp", "door hinge", "trim", "valve") {
			return false
		}
		return containsAny(haystack, "shower wall panel", "shower and wall panel", "waterproof resilient shower", "wall panel", "shower panel", "shower panels")
	}
	if modelIntelQueryLooksLikeCabinetPull(q) {
		haystack := strings.ToLower(strings.Join([]string{p.Title, p.Category, p.Description, p.URL}, " "))
		if containsAny(haystack, "screw", "machine screws", "collection", "clip-on", "clip on", "kids storage", "småstad", "smastad", "knob", "hinge", "drawer slide", "rail") {
			return false
		}
		return containsAny(haystack, "cabinet pull", "drawer pull", "appliance pull", "cabinet handle", "drawer handle", "centers cabinet pull", "center-to-center cabinet pull")
	}
	if modelIntelQueryLooksLikeDoorHinge(q) {
		haystack := strings.ToLower(strings.Join([]string{p.Title, p.Category, p.Description, p.URL}, " "))
		if containsAny(haystack, "door stop", "hinge pin door stop", "pocket door slides", "cabinet hinge", "cabinet hinges", "shower door hinge", "glass shower door", "glass-to-glass shower", "wardrobe", "pax", "komplement", "småstad", "smastad", "kitchen cabinet", "super hinge") {
			return false
		}
		return containsAny(haystack, "door hinge", "door hinges", "residential duty", "radius corner")
	}
	if modelIntelQueryLooksLikeDoorLeverOrLockset(q) {
		haystack := strings.ToLower(strings.Join([]string{p.Title, p.Category, p.Description, p.URL}, " "))
		hasDoorSet := containsAny(haystack, "door lever", "door levers", "interior lever", "entry lever", "passage lever", "privacy lever", "handleset", "handle set", "lockset", "lock set", "door knob", "door knobs", "deadbolt", "electronic lock", "pushbutton lock", "key in locks", "key-in locks")
		if containsAny(haystack, "cabinet", "drawer", "appliance pull", "shower door", "glass clamp", "door hinge", "door stop", "kick plate", "threshold", "weatherstripping", "strike plate", "latchbolt", "replacement", "repair part", "key blank", "padlock") && !hasDoorSet {
			return false
		}
		return hasDoorSet
	}
	return true
}

func modelIntelQueryLooksLikeHVACRegister(q string) bool {
	return containsAny(q, "floor register", "air register", "wall register", "ceiling register", "return grille", "register grille", "floor vent", "wall vent", "vent cover")
}

func modelIntelQueryLooksLikeLinearDrain(q string) bool {
	return containsAny(q, "linear drain", "shower drain", "tile drain", "drain grate")
}

func modelIntelQueryLooksLikeShowerNiche(q string) bool {
	return containsAny(q, "shower niche", "wall niche", "tile niche", "niche shelf")
}

func modelIntelQueryLooksLikeMedicineCabinet(q string) bool {
	return containsAny(q, "medicine cabinet", "mirror cabinet")
}

func modelIntelQueryLooksLikeGrabBar(q string) bool {
	return containsAny(q, "grab bar", "ada bar", "safety bar")
}

func modelIntelQueryLooksLikeRobeHook(q string) bool {
	return containsAny(q, "robe hook", "robe hooks", "bath hook", "bath hooks", "wardrobe hook", "wardrobe hooks")
}

func modelIntelQueryLooksLikeTowelBar(q string) bool {
	return containsAny(q, "towel bar", "towel bars", "towel rail", "towel rails", "towel holder", "towel holders")
}

func modelIntelQueryLooksLikeTowelRing(q string) bool {
	return containsAny(q, "towel ring", "towel rings")
}

func modelIntelQueryLooksLikeSoapDispenser(q string) bool {
	return containsAny(q, "soap dispenser", "soap dispensers", "lotion dispenser", "lotion dispensers")
}

func modelIntelQueryLooksLikeTowelWarmer(q string) bool {
	return containsAny(q, "towel warmer", "heated towel", "towel radiator")
}

func modelIntelQueryLooksLikeShowerDoor(q string) bool {
	return containsAny(q, "shower door", "shower screen", "frameless shower screen")
}

func modelIntelQueryLooksLikeLightedMirror(q string) bool {
	return containsAny(q, "lighted mirror", "led mirror", "illuminated mirror")
}

func modelIntelQueryLooksLikeVanityLight(q string) bool {
	return containsAny(q, "vanity light", "vanity lights", "bath vanity light", "bathroom vanity light", "bathroom vanity lights")
}

func modelIntelQueryLooksLikePictureLight(q string) bool {
	return containsAny(q, "picture light", "picture lights", "art light")
}

func modelIntelQueryLooksLikeBidetSeat(q string) bool {
	return containsAny(q, "bidet seat", "bidet toilet seat", "electronic bidet", "electric bidet", "washlet")
}

func modelIntelQueryLooksLikePotFiller(q string) bool {
	return containsAny(q, "pot filler", "potfiller")
}

func modelIntelQueryLooksLikeShowerValve(q string) bool {
	return containsAny(q, "shower valve", "valve trim", "control trim", "rough-in valve", "rough in valve", "pressure balance", "thermostatic valve", "diverter valve", "transfer valve", "m-core", "m core")
}

func modelIntelQueryLooksLikeShowerHead(q string) bool {
	return containsAny(q, "shower head", "showerhead", "rain shower", "rainshower", "hand shower", "handshower")
}

func modelIntelQueryLooksLikeShowerPanel(q string) bool {
	return containsAny(q, "shower panel", "shower wall panel", "wall panel")
}

func modelIntelQueryLooksLikeCabinetPull(q string) bool {
	return containsAny(q, "cabinet pull", "drawer pull", "appliance pull")
}

func modelIntelQueryLooksLikeDoorHinge(q string) bool {
	return containsAny(q, "door hinge", "door hinges")
}

func modelIntelQueryLooksLikeDoorLeverOrLockset(q string) bool {
	return containsAny(q, "door lever", "door levers", "interior lever", "interior levers", "entry lever", "entry levers", "passage lever", "privacy lever", "handleset", "handle set", "lockset", "lock set", "door knob", "door knobs", "deadbolt")
}

func splitModelIntelCSV(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func appendUniqueStrings(existing []string, more ...string) []string {
	seen := map[string]bool{}
	for _, val := range existing {
		seen[val] = true
	}
	for _, val := range more {
		if val == "" || seen[val] {
			continue
		}
		seen[val] = true
		existing = append(existing, val)
	}
	return existing
}

var modelRegexp = regexp.MustCompile(`\b[A-Z]{2,}[A-Z0-9-]*[0-9][A-Z0-9-]*\b`)
var priceRegexp = regexp.MustCompile(`\$[0-9][0-9,]*(?:\.[0-9]{2})?`)

func looksLikeModelNumber(s string) bool {
	s = strings.ToUpper(strings.TrimSpace(s))
	if len(s) < 5 || strings.Contains(s, " ") {
		return false
	}
	return modelRegexp.MatchString(s)
}

func extractBestModel(text string) string {
	for _, match := range modelRegexp.FindAllString(strings.ToUpper(text), -1) {
		if len(match) >= 5 && len(match) <= 32 && !containsAny(match, "ENERGY", "STAR", "SERIES", "SEER", "BTU", "LUMEN", "VOLT", "WATT") {
			return match
		}
	}
	return ""
}

func orderedUnique(vals []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, val := range vals {
		val = strings.ToUpper(strings.TrimSpace(val))
		if val == "" || seen[val] {
			continue
		}
		seen[val] = true
		out = append(out, val)
	}
	return out
}

func priceNearToken(body, token string) float64 {
	idx := strings.Index(strings.ToUpper(body), strings.ToUpper(token))
	if idx < 0 {
		return 0
	}
	end := idx + 20000
	if end > len(body) {
		end = len(body)
	}
	start := idx - 2000
	if start < 0 {
		start = 0
	}
	for _, raw := range priceRegexp.FindAllString(body[start:end], -1) {
		if price := parsePriceString(raw); price > 10 {
			return price
		}
	}
	return 0
}

func boschProductURL(model string) string {
	return "https://www.bosch-home.com/us/en/product/" + strings.ToUpper(model)
}

func boschTitleForModel(body, model string) string {
	idx := strings.Index(strings.ToUpper(body), strings.ToUpper(model))
	if idx < 0 {
		return ""
	}
	start := idx - 240
	if start < 0 {
		start = 0
	}
	end := idx + 240
	if end > len(body) {
		end = len(body)
	}
	window := stripHTMLTags(html.UnescapeString(body[start:end]))
	window = strings.Join(strings.Fields(window), " ")
	if strings.Contains(window, model) {
		return window
	}
	return ""
}

func specDocsFromDescription(source, desc string) []modelSpecDocument {
	var docs []modelSpecDocument
	for _, line := range strings.Split(desc, "\n") {
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		val = strings.TrimSpace(val)
		if !strings.HasPrefix(val, "http") {
			continue
		}
		kind := strings.TrimSpace(key)
		if strings.Contains(kind, "installation") {
			kind = "installation"
		} else if strings.Contains(kind, "quick_specs") || strings.Contains(kind, "spec") {
			kind = "spec"
		}
		docs = append(docs, modelSpecDocument{Kind: kind, URL: val, Source: source})
	}
	return docs
}

func specDocsFromHTML(source, body, baseURL string) []modelSpecDocument {
	var docs []modelSpecDocument
	for _, raw := range regexp.MustCompile(`(?i)(?:https?://|/)[^"'\\\s<>]+\.pdf(?:\?[^"'\\\s<>]*)?`).FindAllString(body, -1) {
		u := strings.ReplaceAll(html.UnescapeString(raw), `\u0026`, "&")
		u = absoluteURL(baseURL, u)
		kind := "document"
		lower := strings.ToLower(u)
		if strings.Contains(lower, "spec") {
			kind = "spec"
		} else if strings.Contains(lower, "install") {
			kind = "installation"
		}
		docs = append(docs, modelSpecDocument{Kind: kind, URL: u, Source: source})
	}
	return docs
}

func filterModelSpecDocs(model string, docs []modelSpecDocument) []modelSpecDocument {
	model = strings.ToUpper(strings.TrimSpace(model))
	var out []modelSpecDocument
	for _, doc := range docs {
		lowerURL := strings.ToLower(doc.URL)
		if containsAny(lowerURL, "official-rules", "review-and-win", "sweepstakes", "privacy", "terms", "financing", "rebate", "creditapplication", "credit-application", "credit_application") {
			continue
		}
		upperURL := strings.ToUpper(doc.URL)
		if model != "" && !strings.Contains(upperURL, model) && urlHasConflictingModel(model, doc.URL) {
			continue
		}
		out = append(out, doc)
	}
	return out
}

func absoluteURL(baseURL, href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	parsed, err := url.Parse(href)
	if err != nil {
		return href
	}
	if parsed.IsAbs() {
		return parsed.String()
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return href
	}
	return base.ResolveReference(parsed).String()
}

func appendUniqueSpecs(existing, more []modelSpecDocument) []modelSpecDocument {
	seen := map[string]bool{}
	for _, doc := range existing {
		seen[doc.URL] = true
	}
	for _, doc := range more {
		if doc.URL == "" || seen[doc.URL] {
			continue
		}
		seen[doc.URL] = true
		existing = append(existing, doc)
	}
	return existing
}

func metaContent(body, name string) string {
	pattern := regexp.MustCompile(`<meta[^>]+name=["']` + regexp.QuoteMeta(name) + `["'][^>]+content=["']([^"']+)["']`)
	if m := pattern.FindStringSubmatch(body); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}
