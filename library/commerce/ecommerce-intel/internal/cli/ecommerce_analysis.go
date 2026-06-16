package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/mvanhorn/printing-press-library/library/commerce/ecommerce-intel/internal/store"
	"github.com/spf13/cobra"
)

func agentCommands() []map[string]any {
	return []map[string]any{
		{"name": "agent-context", "safe_for_agents": true, "description": "machine-readable CLI context"},
		{"name": "doctor", "safe_for_agents": true, "description": "local readiness, env presence, and optional child binary discovery"},
		{"name": "sources doctor", "safe_for_agents": true, "description": "source-specific child adapter status without printing secrets"},
		{"name": "profile save/list/show/delete", "safe_for_agents": true, "description": "manage local profile metadata"},
		{"name": "sync", "safe_for_agents": true, "description": "load embedded fixture, local JSON import, or opt-in child CLI sync"},
		{"name": "movers", "safe_for_agents": true, "description": "diff latest snapshot against the previous snapshot for commerce climbers, droppers, new Strike-Zone entrants, and new revenue-at-risk"},
		{"name": "dashboard", "safe_for_agents": true, "description": "commerce KPI overview"},
		{"name": "opportunities", "safe_for_agents": true, "description": "prioritized revenue, SEO, GEO, email, and inventory opportunities"},
		{"name": "action-plan", "safe_for_agents": true, "description": "7-day ecommerce action plan"},
		{"name": "money-pages", "safe_for_agents": true, "description": "rank pages by revenue and search/link context"},
		{"name": "money-products", "safe_for_agents": true, "description": "rank products by revenue and margin"},
		{"name": "query-revenue", "safe_for_agents": true, "description": "find revenue by product/page/category query"},
		{"name": "explain-drop", "safe_for_agents": true, "description": "explain revenue/search/session drops"},
		{"name": "product-actions", "safe_for_agents": true, "description": "PDP/product action recommendations"},
		{"name": "category-actions", "safe_for_agents": true, "description": "collection/category action recommendations"},
		{"name": "email-actions", "safe_for_agents": true, "description": "Klaviyo/email action recommendations"},
		{"name": "inventory-risk", "safe_for_agents": true, "description": "stockout and demand risk"},
		{"name": "source-coverage", "safe_for_agents": true, "description": "audit product/page/category/email evidence across Shopify, Klaviyo, GA4, GSC, and Ahrefs"},
		{"name": "merchandising-link-plan", "safe_for_agents": true, "description": "recommend collection-to-PDP and PDP-to-PDP links by category, revenue, and link equity"},
		{"name": "experiment-plan", "safe_for_agents": true, "description": "turn one product into PDP offer, content, schema, and measurement tests"},
		{"name": "forecast-impact", "safe_for_agents": true, "description": "estimate revenue upside from conversion, CTR, and inventory gaps"},
		{"name": "restock-winners", "safe_for_agents": true, "description": "rank high-margin, high-velocity products before stockout or decay"},
		{"name": "cannibalization", "safe_for_agents": true, "description": "detect products competing for the same query/category"},
		{"name": "category-clusters", "safe_for_agents": true, "description": "aggregate revenue, sessions, clicks, backlinks, and decay by collection"},
		{"name": "digest weekly", "safe_for_agents": true, "description": "weekly ecommerce summary"},
		{"name": "geo-audit", "safe_for_agents": true, "description": "answer-engine readiness audit"},
	}
}

func sourceCoverageCmd(f *rootFlags) *cobra.Command {
	var limit int
	var missingOnly bool
	c := &cobra.Command{Use: "source-coverage", Short: "Audit evidence coverage across Shopify, Klaviyo, GA4, GSC, and Ahrefs", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		rows := []map[string]any{}
		counts := map[string]int{"shopify": 0, "klaviyo": 0, "ga4": 0, "gsc": 0, "ahrefs": 0, "complete": 0, "partial": 0}
		add := func(entity, target string, sources store.MetricSources) {
			row := coverageRow(entity, target, sources)
			for _, source := range sourceNames() {
				if row[source+"_present"].(bool) {
					counts[source]++
				}
			}
			if len(row["missing_sources"].([]string)) == 0 {
				counts["complete"]++
			} else {
				counts["partial"]++
			}
			if missingOnly && len(row["missing_sources"].([]string)) == 0 {
				return
			}
			rows = append(rows, row)
		}
		for _, p := range d.Products {
			add("product", first(p.Handle, p.Title, p.URL, p.ID), p.Source)
		}
		for _, p := range d.Pages {
			add("page", p.URL, p.Source)
		}
		for _, c := range d.Categories {
			add("category", c.Handle, c.Source)
		}
		for _, e := range d.Emails {
			add("email_flow", e.Name, e.Source)
		}
		sort.Slice(rows, func(i, j int) bool {
			return len(rows[i]["missing_sources"].([]string)) > len(rows[j]["missing_sources"].([]string))
		})
		if limit > 0 && len(rows) > limit {
			rows = rows[:limit]
		}
		result := map[string]any{"profile": d.Profile, "entities": len(d.Products) + len(d.Pages) + len(d.Categories) + len(d.Emails), "summary": counts, "rows": rows}
		lines := []string{"entity\ttarget\tcoverage\tmissing_sources"}
		for _, row := range rows {
			lines = append(lines, fmt.Sprintf("%s\t%s\t%.2f\t%s", row["entity"], row["target"], row["coverage_score"], strings.Join(row["missing_sources"].([]string), ",")))
		}
		return out(cmd, f, result, strings.Join(lines, "\n")+"\n")
	}}
	c.Flags().IntVar(&limit, "limit", 25, "Rows to return")
	c.Flags().BoolVar(&missingOnly, "missing-only", false, "Only show entities missing one or more sources")
	return c
}

func merchandisingLinkPlanCmd(f *rootFlags) *cobra.Command {
	var limit int
	c := &cobra.Command{Use: "merchandising-link-plan", Short: "Recommend collection and PDP internal links", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		rows := merchandisingLinks(d)
		if limit > 0 && len(rows) > limit {
			rows = rows[:limit]
		}
		lines := []string{"from_url\tto_url\tanchor\treason"}
		for _, row := range rows {
			lines = append(lines, fmt.Sprintf("%s\t%s\t%s\t%s", row["from_url"], row["to_url"], row["anchor"], row["reason"]))
		}
		return out(cmd, f, rows, strings.Join(lines, "\n")+"\n")
	}}
	c.Flags().IntVar(&limit, "limit", 10, "Rows to return")
	return c
}

func experimentPlanCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "experiment-plan <product>", Args: cobra.ExactArgs(1), Short: "Create PDP experiments for one product", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		p, ok := findProduct(d.Products, args[0])
		if !ok {
			return fmt.Errorf("no product matching %q in profile %q", args[0], f.profile)
		}
		plan := productExperimentPlan(p)
		if err := st(f).AppendLearning(d.Profile, fmt.Sprintf("experiment-plan generated for %s; success metric: %v", first(p.Handle, p.Title), plan["primary_success_metric"])); err != nil {
			return err
		}
		return out(cmd, f, plan, fmt.Sprintf("Experiment plan: %s\nsuccess metric: %s\n", first(p.Handle, p.Title), plan["primary_success_metric"]))
	}}
}

func forecastImpactCmd(f *rootFlags) *cobra.Command {
	var limit int
	c := &cobra.Command{Use: "forecast-impact", Short: "Estimate revenue upside from conversion, CTR, and inventory gaps", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		rows := forecastRows(d)
		if limit > 0 && len(rows) > limit {
			rows = rows[:limit]
		}
		lines := []string{"product\test_revenue_gain\tprimary_gap"}
		for _, row := range rows {
			lines = append(lines, fmt.Sprintf("%s\t%.2f\t%s", row["product"], row["estimated_revenue_gain"], row["primary_gap"]))
		}
		return out(cmd, f, rows, strings.Join(lines, "\n")+"\n")
	}}
	c.Flags().IntVar(&limit, "limit", 10, "Rows to return")
	return c
}

func restockWinnersCmd(f *rootFlags) *cobra.Command {
	var limit int
	c := &cobra.Command{Use: "restock-winners", Short: "Rank high-value products to restock or refresh", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		rows := restockWinners(d)
		if limit > 0 && len(rows) > limit {
			rows = rows[:limit]
		}
		lines := []string{"target\taction\timpact"}
		for _, r := range rows {
			target := fmt.Sprint(firstAny(r, "product", "target"))
			lines = append(lines, fmt.Sprintf("%s\t%s\t%v", target, r["action"], r["impact"]))
		}
		return out(cmd, f, rows, strings.Join(lines, "\n")+"\n")
	}}
	c.Flags().IntVar(&limit, "limit", 10, "Rows to return")
	return c
}

func cannibalizationCmd(f *rootFlags) *cobra.Command {
	var limit int
	c := &cobra.Command{Use: "cannibalization", Short: "Detect products competing for the same query/category", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		rows := cannibalizationRows(d)
		if limit > 0 && len(rows) > limit {
			rows = rows[:limit]
		}
		lines := []string{"topic\tproducts\ttotal_revenue\tcanonical"}
		for _, row := range rows {
			lines = append(lines, fmt.Sprintf("%s\t%d\t%.2f\t%s", row["topic"], row["product_count"], row["total_revenue"], row["canonical_product"]))
		}
		return out(cmd, f, rows, strings.Join(lines, "\n")+"\n")
	}}
	c.Flags().IntVar(&limit, "limit", 10, "Rows to return")
	return c
}

func categoryClustersCmd(f *rootFlags) *cobra.Command {
	var limit int
	c := &cobra.Command{Use: "category-clusters", Short: "Summarize revenue, sessions, search, links, and decay by collection", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		rows := categoryClusterRows(d)
		if limit > 0 && len(rows) > limit {
			rows = rows[:limit]
		}
		lines := []string{"category\tproducts\trevenue\tlost_revenue\ttop_product"}
		for _, row := range rows {
			lines = append(lines, fmt.Sprintf("%s\t%d\t%.2f\t%.2f\t%s", row["category"], row["products"], row["revenue"], row["lost_revenue"], row["top_product"]))
		}
		return out(cmd, f, rows, strings.Join(lines, "\n")+"\n")
	}}
	c.Flags().IntVar(&limit, "limit", 10, "Rows to return")
	return c
}

func sourceNames() []string { return []string{"shopify", "klaviyo", "ga4", "gsc", "ahrefs"} }

func coverageRow(entity, target string, sources store.MetricSources) map[string]any {
	present := map[string]bool{"shopify": sources.Shopify.Synced, "klaviyo": sources.Klaviyo.Synced, "ga4": sources.GA4.Synced, "gsc": sources.GSC.Synced, "ahrefs": sources.Ahrefs.Synced}
	missing := []string{}
	for _, source := range sourceNames() {
		if !present[source] {
			missing = append(missing, source)
		}
	}
	row := map[string]any{"entity": entity, "target": target, "missing_sources": missing, "coverage_score": float64(5-len(missing)) / 5, "next_action": coverageAction(missing)}
	for source, ok := range present {
		row[source+"_present"] = ok
	}
	return row
}

func coverageAction(missing []string) string {
	if len(missing) == 0 {
		return "coverage complete"
	}
	return "sync " + strings.Join(missing, ",") + " evidence before relying on combined prioritization"
}

func merchandisingLinks(d store.DataSet) []map[string]any {
	rows := []map[string]any{}
	byCategory := map[string][]store.Product{}
	for _, p := range d.Products {
		byCategory[strings.ToLower(first(p.Category, collectionFromURL(p.URL), "uncategorized"))] = append(byCategory[strings.ToLower(first(p.Category, collectionFromURL(p.URL), "uncategorized"))], p)
	}
	for _, c := range d.Categories {
		products := byCategory[strings.ToLower(c.Handle)]
		sort.Slice(products, func(i, j int) bool { return productValue(products[i]) > productValue(products[j]) })
		for _, p := range products {
			if p.URL == "" || c.URL == "" {
				continue
			}
			rows = append(rows, map[string]any{"from_url": c.URL, "to_url": p.URL, "anchor": first(p.Title, p.Handle), "category": c.Handle, "score": productValue(p) + float64(c.SearchClicks), "reason": "collection has category intent and product has revenue, margin, or inventory upside"})
			break
		}
	}
	ps := append([]store.Product(nil), d.Products...)
	sort.Slice(ps, func(i, j int) bool { return productValue(ps[i]) > productValue(ps[j]) })
	for i, src := range ps {
		for _, dst := range ps {
			if src.Handle == dst.Handle || src.Category != dst.Category || src.URL == "" || dst.URL == "" {
				continue
			}
			rows = append(rows, map[string]any{"from_url": src.URL, "to_url": dst.URL, "anchor": first(dst.Title, dst.Handle), "category": dst.Category, "score": productValue(dst) + float64(src.RefDomains*10), "reason": "same category PDP with complementary revenue/search/link value"})
			break
		}
		if i >= 10 {
			break
		}
	}
	sort.Slice(rows, func(i, j int) bool { return asFloat(rows[i]["score"]) > asFloat(rows[j]["score"]) })
	return rows
}

func productExperimentPlan(p store.Product) map[string]any {
	return map[string]any{
		"product":                first(p.Handle, p.Title, p.ID),
		"url":                    p.URL,
		"category":               p.Category,
		"primary_gap":            productGap(p),
		"primary_success_metric": experimentMetric(p),
		"title_tests":            []string{"lead with the highest-intent product use case", "test benefit plus material/spec proof", "add availability or shipping promise when inventory supports demand"},
		"offer_tests":            []string{"test bundle, threshold, or guarantee framing", "compare full-price value against markdown/bundle alternative"},
		"schema_tests":           []string{"validate Product, Offer, AggregateRating, Review, FAQPage, and Breadcrumb schema", "add SKU/GTIN and availability fields where present"},
		"measurement":            map[string]any{"baseline_revenue": p.Revenue, "baseline_conversion_rate": p.ConversionRate, "baseline_clicks": p.SearchClicks, "minimum_runtime": "14 days or one full weekly cycle", "follow_up_commands": []string{"ecommerce-intel-pp-cli forecast-impact --profile <profile>", "ecommerce-intel-pp-cli source-coverage --profile <profile> --missing-only"}},
	}
}

func forecastRows(d store.DataSet) []map[string]any {
	rows := []map[string]any{}
	for _, p := range d.Products {
		revenuePerSession := 0.0
		if p.Sessions > 0 {
			revenuePerSession = p.Revenue / float64(p.Sessions)
		}
		crGap := maxf(0, .04-p.ConversionRate)
		ctrGain := float64(p.SearchClicks) * maxf(0, expectedCTR(p.SearchPosition)-.03)
		stockGain := 0.0
		if p.Inventory < 15 || p.DaysOfStock < 7 {
			stockGain = maxf(p.Revenue*.2, float64(p.UnitsSold)*p.Price*.1)
		}
		estimated := float64(p.Sessions)*crGap*p.Price + ctrGain*revenuePerSession + stockGain
		if estimated <= 0 {
			continue
		}
		rows = append(rows, map[string]any{"product": first(p.Handle, p.Title), "estimated_revenue_gain": estimated, "conversion_gap": crGap, "estimated_click_gain": ctrGain, "inventory_protection": stockGain, "primary_gap": productGap(p), "recommended_validation_cmd": "ecommerce-intel-pp-cli experiment-plan --profile <profile> " + strconv.Quote(first(p.Handle, p.Title))})
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i]["estimated_revenue_gain"].(float64) > rows[j]["estimated_revenue_gain"].(float64)
	})
	return rows
}

func restockWinners(d store.DataSet) []map[string]any {
	rows := []map[string]any{}
	for _, p := range d.Products {
		score := restockScore(p)
		if score <= 0 {
			continue
		}
		rows = append(rows, map[string]any{"type": "restock", "product": p.Handle, "target": p.Handle, "action": restockAction(p), "impact": score, "revenue": p.Revenue, "inventory": p.Inventory, "days_of_stock": p.DaysOfStock, "gross_margin": p.GrossMargin})
	}
	sortRows(rows)
	return rows
}

func cannibalizationRows(d store.DataSet) []map[string]any {
	groups := map[string][]store.Product{}
	for _, p := range d.Products {
		groups[productTopic(p)] = append(groups[productTopic(p)], p)
	}
	rows := []map[string]any{}
	for topic, ps := range groups {
		if len(ps) < 2 {
			continue
		}
		sort.Slice(ps, func(i, j int) bool { return productValue(ps[i]) > productValue(ps[j]) })
		total := 0.0
		products := []map[string]any{}
		for _, p := range ps {
			total += p.Revenue
			products = append(products, map[string]any{"handle": p.Handle, "title": p.Title, "revenue": p.Revenue, "clicks": p.SearchClicks, "position": p.SearchPosition})
		}
		rows = append(rows, map[string]any{"topic": topic, "product_count": len(ps), "products": products, "total_revenue": total, "canonical_product": ps[0].Handle, "recommended_action": "choose a canonical PDP, consolidate overlapping query intent, and cross-link substitutes", "score": total + float64(len(ps))*100})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i]["score"].(float64) > rows[j]["score"].(float64) })
	return rows
}

func categoryClusterRows(d store.DataSet) []map[string]any {
	type agg struct {
		products, sessions, clicks, backlinks int
		revenue, lostRevenue                  float64
		top                                   string
	}
	clusters := map[string]*agg{}
	for _, p := range d.Products {
		key := strings.ToLower(first(p.Category, collectionFromURL(p.URL), "uncategorized"))
		if clusters[key] == nil {
			clusters[key] = &agg{}
		}
		a := clusters[key]
		a.products++
		a.sessions += p.Sessions
		a.clicks += p.SearchClicks
		a.backlinks += p.Backlinks
		a.revenue += p.Revenue
		a.lostRevenue += maxf(0, p.PreviousRevenue-p.Revenue)
		if a.top == "" || p.Revenue > productRevenueByHandle(d.Products, a.top) {
			a.top = p.Handle
		}
	}
	rows := []map[string]any{}
	for category, a := range clusters {
		rows = append(rows, map[string]any{"category": category, "products": a.products, "sessions": a.sessions, "search_clicks": a.clicks, "backlinks": a.backlinks, "revenue": a.revenue, "lost_revenue": a.lostRevenue, "top_product": a.top, "recommended_next": clusterAction(a.lostRevenue, a.revenue), "score": a.revenue + a.lostRevenue + float64(a.clicks)})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i]["score"].(float64) > rows[j]["score"].(float64) })
	return rows
}

func findProduct(products []store.Product, q string) (store.Product, bool) {
	q = strings.ToLower(strings.TrimSpace(q))
	for _, p := range products {
		if strings.ToLower(p.Handle) == q || strings.ToLower(p.ID) == q || strings.Contains(strings.ToLower(p.URL), q) || strings.Contains(strings.ToLower(p.Title), q) {
			return p, true
		}
	}
	return store.Product{}, false
}

func productGap(p store.Product) string {
	switch {
	case p.Inventory < 15 || p.DaysOfStock < 7:
		return "inventory"
	case p.ConversionRate < .025:
		return "conversion"
	case p.SearchPosition > 5 || p.SearchClicks < p.PreviousClicks:
		return "search"
	case !p.HasStructuredData || !p.HasProductFacts || !p.HasFAQ:
		return "geo"
	default:
		return "optimization"
	}
}

func experimentMetric(p store.Product) string {
	if p.Revenue > 0 {
		return "product revenue and conversion rate"
	}
	if p.SearchClicks > 0 {
		return "organic clicks and PDP conversion rate"
	}
	return "sessions and add-to-cart rate"
}

func productValue(p store.Product) float64 {
	return p.Revenue + p.GrossMargin*float64(max(1, p.UnitsSold)) + float64(p.SearchClicks*2) + float64(p.RefDomains*15)
}

func restockScore(p store.Product) float64 {
	if p.Inventory >= 15 && p.DaysOfStock >= 14 {
		return 0
	}
	return p.Revenue + p.GrossMargin*float64(max(1, p.UnitsSold)) + float64(p.SearchClicks) + p.EmailRevenue
}

func restockAction(p store.Product) string {
	if p.Inventory <= 0 {
		return "restock immediately and route demand to substitute PDPs until available"
	}
	if p.DaysOfStock < 7 {
		return "expedite reorder and throttle paid/email demand before stockout"
	}
	return "refresh merchandising and confirm replenishment before demand spike"
}

func productTopic(p store.Product) string {
	return normalizeTopic(first(p.Category, p.Title, p.Handle))
}

func normalizeTopic(s string) string {
	parts := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	stop := map[string]bool{"a": true, "an": true, "and": true, "best": true, "for": true, "of": true, "or": true, "the": true, "to": true, "with": true}
	out := []string{}
	for _, part := range parts {
		if part != "" && !stop[part] {
			out = append(out, part)
		}
		if len(out) == 3 {
			break
		}
	}
	if len(out) == 0 {
		return "uncategorized"
	}
	return strings.Join(out, " ")
}

func expectedCTR(position float64) float64 {
	switch {
	case position <= 0:
		return 0
	case position <= 1:
		return .28
	case position <= 2:
		return .15
	case position <= 3:
		return .10
	case position <= 5:
		return .05
	case position <= 10:
		return .03
	default:
		return .015
	}
}

func productRevenueByHandle(products []store.Product, handle string) float64 {
	for _, p := range products {
		if p.Handle == handle {
			return p.Revenue
		}
	}
	return 0
}

func clusterAction(lostRevenue, revenue float64) string {
	if lostRevenue > 0 {
		return "refresh PDPs and inspect conversion/search decay"
	}
	if revenue > 0 {
		return "protect high-value category with internal links, schema, and restock planning"
	}
	return "expand category coverage after source sync"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
