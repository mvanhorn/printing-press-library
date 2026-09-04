// Hand-authored catalog helpers shared by the Creative Fabrica commands
// (find, free, pod, deals, designer*, new-since, tags, categories, types,
// product). Not generated; safe across regen.
package cli

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/creativefabrica/internal/algolia"
	"github.com/mvanhorn/printing-press-library/library/commerce/creativefabrica/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/creativefabrica/internal/config"
	"github.com/spf13/cobra"
)

// NewCatalogClient builds the credential-aware catalog client, honoring
// CREATIVEFABRICA_BASE_URL / config base_url (verifier, mock, or alternate host).
func NewCatalogClient(timeout time.Duration, rateLimit float64, configPath string) *algolia.Client {
	c := algolia.New(timeout, rateLimit)
	if cfg, err := config.Load(configPath); err == nil && cfg.BaseURL != "" {
		if cfg.BaseURL != "https://"+algolia.DefaultAppID+"-dsn.algolia.net" {
			c.BaseURL = cfg.BaseURL
		}
	}
	return c
}

func newAlgoliaClient(flags *rootFlags) *algolia.Client {
	c := NewCatalogClient(flags.timeout, flags.rateLimit, flags.configPath)
	c.DryRun = flags.dryRun
	return c
}

// catalogQuery is the shared filter set for catalog searches. Server-side
// filters map to Algolia facets/numeric filters; localFormat and noSubscription
// are applied locally because they are not server facets.
type catalogQuery struct {
	query          string
	itemType       string
	category       string
	designer       string // numeric id or designer name
	formats        []string
	pod            bool
	free           bool
	onSale         bool
	noSubscription bool
	maxPrice       float64
	sortBy         string // "relevance" | "newest"
	page           int
	limit          int
}

func (q catalogQuery) index() string {
	if strings.EqualFold(q.sortBy, "newest") {
		return algolia.IndexNewest
	}
	return algolia.IndexRelevance
}

// serverFilters builds the Algolia `filters` expression from the server-side
// facet/numeric filters. Format and subscription filters are excluded (applied
// locally in applyLocalFilters).
func (q catalogQuery) serverFilters() string {
	var f []string
	if q.itemType != "" {
		f = append(f, fmt.Sprintf("type:%s", quoteFacet(q.itemType)))
	}
	if q.category != "" {
		f = append(f, fmt.Sprintf("category:%s", quoteFacet(q.category)))
	}
	if q.designer != "" {
		if id, err := strconv.Atoi(q.designer); err == nil {
			f = append(f, fmt.Sprintf("designer.designerId:%d", id))
		} else {
			f = append(f, fmt.Sprintf("designer.designerName:%s", quoteFacet(q.designer)))
		}
	}
	if q.pod {
		f = append(f, "hasPod:true")
	}
	if q.free {
		f = append(f, "isFree:true")
	}
	if q.onSale {
		f = append(f, "hasPromotions:true")
	}
	if q.maxPrice > 0 {
		f = append(f, fmt.Sprintf("price <= %s", strconv.FormatFloat(q.maxPrice, 'f', -1, 64)))
	}
	return strings.Join(f, " AND ")
}

func (q catalogQuery) localFiltersActive() bool {
	return len(q.formats) > 0 || q.noSubscription
}

func (q catalogQuery) request() algolia.SearchRequest {
	limit := q.limit
	if limit <= 0 {
		limit = 20
	}
	return algolia.SearchRequest{
		IndexName:   q.index(),
		Query:       q.query,
		Page:        q.page,
		HitsPerPage: clampInt(limit, 1, 100),
		Filters:     q.serverFilters(),
	}
}

func (q catalogQuery) matchLocal(h algolia.Hit) bool {
	if q.noSubscription && !h.OutsideSubscription {
		return false
	}
	if len(q.formats) > 0 && !hitMatchesFormat(h, q.formats) {
		return false
	}
	return true
}

// applyLocalFilters applies the format and subscription-free filters that
// Algolia has no server facet for, then truncates to limit.
func (q catalogQuery) applyLocalFilters(hits []algolia.Hit) []algolia.Hit {
	limit := q.limit
	if limit <= 0 {
		limit = 20
	}
	return appendLocalMatches(nil, hits, q, limit)
}

func appendLocalMatches(dst []algolia.Hit, hits []algolia.Hit, q catalogQuery, limit int) []algolia.Hit {
	for _, h := range hits {
		if !q.matchLocal(h) {
			continue
		}
		dst = append(dst, h)
		if len(dst) >= limit {
			return dst
		}
	}
	return dst
}

// maxLocalScanPages is the Algolia page budget when local post-filters are
// active. 10 pages × 100 hits covers the typical 1000-hit window.
const maxLocalScanPages = 10

func fetchCatalogHits(ctx context.Context, c *algolia.Client, q catalogQuery) ([]algolia.Hit, error) {
	limit := q.limit
	if limit <= 0 {
		limit = 20
	}
	if !q.localFiltersActive() {
		results, err := c.Search(ctx, q.request())
		if err != nil {
			return nil, err
		}
		if len(results) == 0 {
			return nil, nil
		}
		hits := results[0].Hits
		if len(hits) > limit {
			hits = hits[:limit]
		}
		return hits, nil
	}
	pages := maxLocalScanPages
	if cliutil.IsDogfoodEnv() && pages > 2 {
		pages = 2
	}
	var out []algolia.Hit
	start := q.page
	for page := start; page < start+pages; page++ {
		req := q.request()
		req.Page = page
		req.HitsPerPage = 100
		results, err := c.Search(ctx, req)
		if err != nil {
			return out, err
		}
		if len(results) == 0 || len(results[0].Hits) == 0 {
			break
		}
		out = appendLocalMatches(out, results[0].Hits, q, limit)
		if len(out) >= limit || page+1 >= results[0].NbPages {
			break
		}
	}
	return out, nil
}

// hitMatchesFormat reports whether any requested file format token appears in
// the hit's tags, title, or description. Format is not an Algolia facet, so it
// must be matched against free text.
func hitMatchesFormat(h algolia.Hit, formats []string) bool {
	hay := strings.ToLower(h.NameEN + " " + h.DescriptionEN + " " + strings.Join(h.Tags, " ") + " " + strings.Join(h.Category, " "))
	for _, f := range formats {
		f = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(f, ".")))
		if f == "" {
			continue
		}
		// Word-ish match: ".svg", "svg ", "svg file", "svg)".
		if strings.Contains(hay, "."+f) || strings.Contains(hay, f+" ") ||
			strings.HasSuffix(hay, f) || strings.Contains(hay, f+",") || strings.Contains(hay, f+")") {
			return true
		}
	}
	return false
}

func quoteFacet(v string) string {
	// Escape backslashes first, then quotes, so a value containing a backslash
	// or quote cannot break out of the quoted Algolia facet and inject filter
	// clauses.
	v = strings.ReplaceAll(v, `\`, `\\`)
	v = strings.ReplaceAll(v, `"`, `\"`)
	return `"` + v + `"`
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// productView is the agent-stable JSON shape for a catalog hit. It flattens the
// nested designer object and normalizes the unix date to make --select and
// downstream parsing predictable.
type productView struct {
	ObjectID      string   `json:"objectID"`
	Name          string   `json:"name"`
	Type          string   `json:"type"`
	Category      []string `json:"category"`
	Tags          []string `json:"tags,omitempty"`
	DesignerID    int      `json:"designer_id"`
	Designer      string   `json:"designer"`
	Price         float64  `json:"price"`
	RegularPrice  string   `json:"regular_price,omitempty"`
	IsFree        bool     `json:"is_free"`
	HasPod        bool     `json:"has_pod"`
	OnSale        bool     `json:"on_sale"`
	NoSubRequired bool     `json:"no_subscription_required"`
	URL           string   `json:"url"`
	Image         string   `json:"image,omitempty"`
	Date          int64    `json:"date,omitempty"`
}

// cleanSlice applies cliutil.CleanText to every element of a string slice so
// human-facing fields (category, tags) don't leak raw HTML entities from the
// catalog index (e.g. "Script &amp; Handwritten").
func cleanSlice(in []string) []string {
	if len(in) == 0 {
		return in
	}
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = cliutil.CleanText(s)
	}
	return out
}

func toView(h algolia.Hit) productView {
	return productView{
		ObjectID:      h.ObjectID,
		Name:          cliutil.CleanText(h.NameEN),
		Type:          h.Type,
		Category:      cleanSlice(h.Category),
		Tags:          cleanSlice(h.Tags),
		DesignerID:    h.Designer.DesignerID,
		Designer:      cliutil.CleanText(h.Designer.DesignerName),
		Price:         round2(h.Price.Float()),
		RegularPrice:  h.RegularPrice.String(),
		IsFree:        h.IsFree,
		HasPod:        h.HasPod,
		OnSale:        h.HasPromotions,
		NoSubRequired: h.OutsideSubscription,
		URL:           h.URL,
		Image:         h.Image,
		Date:          h.Date,
	}
}

func toViews(hits []algolia.Hit) []productView {
	out := make([]productView, 0, len(hits))
	for _, h := range hits {
		out = append(out, toView(h))
	}
	return out
}

// printProducts emits the product slice as JSON (honoring --select/--compact/
// --csv) or a human table.
func printProducts(cmd *cobra.Command, flags *rootFlags, views []productView) error {
	if flags.asJSON || flags.agent || !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return flags.printJSON(cmd, views)
	}
	rows := make([][]string, 0, len(views))
	for _, v := range views {
		price := "$" + strconv.FormatFloat(v.Price, 'f', 2, 64)
		if v.IsFree {
			price = "FREE"
		}
		flagsCol := ""
		if v.HasPod {
			flagsCol += "POD "
		}
		if v.OnSale {
			flagsCol += "SALE"
		}
		rows = append(rows, []string{
			truncate(v.Name, 44), v.Type, truncate(v.Designer, 20), price, strings.TrimSpace(flagsCol), v.ObjectID,
		})
	}
	return flags.printTable(cmd, []string{"NAME", "TYPE", "DESIGNER", "PRICE", "", "ID"}, rows)
}

// runCatalogSearch executes a query (server filters + local post-filters) and
// prints the results. Shared by find/free/pod.
func runCatalogSearch(cmd *cobra.Command, flags *rootFlags, q catalogQuery) error {
	if dryRunOK(flags) {
		fmt.Fprintf(cmd.OutOrStdout(), "would search index %s query %q filters %q\n", q.index(), q.query, q.serverFilters())
		return nil
	}
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	c := newAlgoliaClient(flags)
	hits, err := fetchCatalogHits(ctx, c, q)
	if err != nil {
		return apiErr(err)
	}
	return printProducts(cmd, flags, toViews(hits))
}

// fetchAllForDesigner pages a designer's catalog (server-filtered) up to
// maxScanPages, returning every hit plus Algolia facets from the first page.
// Used by designer-stats/compare.
func fetchAllForDesigner(ctx context.Context, c *algolia.Client, designer string, maxScanPages int) ([]algolia.Hit, int, map[string]map[string]int, error) {
	q := catalogQuery{designer: designer, sortBy: "newest", limit: 100}
	var all []algolia.Hit
	nbHits := 0
	var facets map[string]map[string]int
	if maxScanPages <= 0 {
		maxScanPages = 100
	}
	for page := 0; page < maxScanPages; page++ {
		req := q.request()
		req.Page = page
		req.HitsPerPage = 100
		if page == 0 {
			req.Facets = []string{"type", "isFree", "hasPod", "hasPromotions"}
			req.MaxValuesPerFacet = 100
		}
		results, err := c.Search(ctx, req)
		if err != nil {
			return all, nbHits, facets, err
		}
		if len(results) == 0 {
			break
		}
		nbHits = results[0].NbHits
		if page == 0 {
			facets = results[0].Facets
		}
		all = append(all, results[0].Hits...)
		if len(results[0].Hits) == 0 || page+1 >= results[0].NbPages {
			break
		}
	}
	return all, nbHits, facets, nil
}

// sortHitsByDate sorts hits newest-first in place.
func sortHitsByDate(hits []algolia.Hit) {
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].Date > hits[j].Date })
}

// envHint reports whether the catalog key is configured, for doctor/auth status.
func keyConfigured() bool {
	if strings.TrimSpace(os.Getenv("CREATIVEFABRICA_ALGOLIA_API_KEY")) != "" {
		return true
	}
	if _, err := os.Stat(algolia.CredsPath()); err == nil {
		return true
	}
	return false
}
