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

	"github.com/mvanhorn/printing-press-library/library/commerce/creativefabrica/internal/algolia"
	"github.com/mvanhorn/printing-press-library/library/commerce/creativefabrica/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/creativefabrica/internal/config"
	"github.com/spf13/cobra"
)

// newAlgoliaClient builds the catalog client, honoring the CREATIVEFABRICA_BASE_URL
// override (used by the verifier/mock server) via the generated config loader.
func newAlgoliaClient(flags *rootFlags) *algolia.Client {
	c := algolia.New(flags.timeout, flags.rateLimit)
	c.DryRun = flags.dryRun
	if cfg, err := config.Load(flags.configPath); err == nil && cfg.BaseURL != "" {
		// Only override when an explicit non-default base URL is set (mock/test).
		if cfg.BaseURL != "https://"+algolia.DefaultAppID+"-dsn.algolia.net" {
			c.BaseURL = cfg.BaseURL
		}
	}
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

func (q catalogQuery) request() algolia.SearchRequest {
	limit := q.limit
	if limit <= 0 {
		limit = 20
	}
	// When local post-filters are active, over-fetch so the post-filter still
	// has enough rows to satisfy the requested limit.
	hitsPerPage := limit
	if len(q.formats) > 0 || q.noSubscription {
		hitsPerPage = clampInt(limit*4, limit, 100)
	}
	return algolia.SearchRequest{
		IndexName:   q.index(),
		Query:       q.query,
		Page:        q.page,
		HitsPerPage: hitsPerPage,
		Filters:     q.serverFilters(),
	}
}

// applyLocalFilters applies the format and subscription-free filters that
// Algolia has no server facet for, then truncates to limit.
func (q catalogQuery) applyLocalFilters(hits []algolia.Hit) []algolia.Hit {
	limit := q.limit
	if limit <= 0 {
		limit = 20
	}
	out := hits[:0:0]
	for _, h := range hits {
		if q.noSubscription && !h.OutsideSubscription {
			continue
		}
		if len(q.formats) > 0 && !hitMatchesFormat(h, q.formats) {
			continue
		}
		out = append(out, h)
		if len(out) >= limit {
			break
		}
	}
	return out
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
	results, err := c.Search(ctx, q.request())
	if err != nil {
		return apiErr(err)
	}
	if len(results) == 0 {
		return printProducts(cmd, flags, nil)
	}
	hits := q.applyLocalFilters(results[0].Hits)
	return printProducts(cmd, flags, toViews(hits))
}

// fetchAllForDesigner pages a designer's catalog (server-filtered) up to
// maxScanPages, returning every hit. Used by designer-stats/compare.
func fetchAllForDesigner(ctx context.Context, c *algolia.Client, designer string, maxScanPages int) ([]algolia.Hit, int, error) {
	q := catalogQuery{designer: designer, sortBy: "newest", limit: 100}
	var all []algolia.Hit
	nbHits := 0
	for page := 0; page < maxScanPages; page++ {
		req := q.request()
		req.Page = page
		req.HitsPerPage = 100
		results, err := c.Search(ctx, req)
		if err != nil {
			return all, nbHits, err
		}
		if len(results) == 0 {
			break
		}
		nbHits = results[0].NbHits
		all = append(all, results[0].Hits...)
		if len(results[0].Hits) == 0 || page+1 >= results[0].NbPages {
			break
		}
	}
	return all, nbHits, nil
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
