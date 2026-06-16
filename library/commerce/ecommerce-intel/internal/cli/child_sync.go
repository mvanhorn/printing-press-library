package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/ecommerce-intel/internal/store"
	"github.com/mvanhorn/printing-press-library/library/internal/intelcli"
)

type childSyncOptions struct {
	Shop           string
	KlaviyoAccount string
	Site           string
	GAProperty     string
	AhrefsTarget   string
	StartDate      string
	EndDate        string
	AhrefsDate     string
	Limit          int
}

type childSourceDef struct {
	name   string
	binary string
	args   []string
	env    []string
}

type childEntities struct {
	Products   []store.Product
	Pages      []store.Page
	Categories []store.Category
	Emails     []store.EmailFlow
}

func syncFromChildCLIs(profile, source string, base store.DataSet, opts childSyncOptions) (store.DataSet, error) {
	defs := childSourceDefs(opts)
	switch source {
	case "all":
		if missing := missingChildSources(defs, "shopify", "klaviyo", "ga4", "gsc", "ahrefs"); len(missing) > 0 {
			return store.DataSet{}, fmt.Errorf("--source all requires shopify, klaviyo, ga4, gsc, and ahrefs configuration; missing %s", strings.Join(missing, ", "))
		}
	case "shopify", "klaviyo", "ga4", "gsc", "ahrefs":
		def, ok := findChildSource(defs, source)
		if !ok {
			return store.DataSet{}, fmt.Errorf("source %q is not configured; %s", source, sourceConfigHint(source))
		}
		defs = []childSourceDef{def}
	default:
		return store.DataSet{}, fmt.Errorf("unknown source %q (want all, shopify, klaviyo, ga4, gsc, or ahrefs)", source)
	}

	if base.Profile == "" {
		base.Profile = profile
	}
	used := []string{}
	inputHashes := map[string]string{}
	sourceVersions := map[string]string{}
	for _, def := range defs {
		entities, command, outputHash, childVersion, err := runChildSource(def)
		if err != nil {
			return store.DataSet{}, err
		}
		used = append(used, def.name)
		inputHashes[def.name] = outputHash
		sourceVersions[def.name] = childVersion
		mergeChildEntities(&base, entities, def.name, command)
	}
	base.Profile = profile
	base.Source = "child-cli:" + strings.Join(used, "+")
	base.SyncedAt = time.Now().UTC()
	start, end := defaultDateRange(opts.StartDate, opts.EndDate)
	base.Provenance = store.DataProvenance{
		SchemaVersion:         "ecommerce-intel.provenance/v1",
		DateRange:             store.DateRange{StartDate: start, EndDate: end},
		SourceCommandVersions: sourceVersions,
		InputHashes:           inputHashes,
	}
	return base, nil
}

func childSourceDefs(opts childSyncOptions) []childSourceDef {
	start, end := defaultDateRange(opts.StartDate, opts.EndDate)
	if opts.Limit <= 0 {
		opts.Limit = 1000
	}
	shop := firstNonEmpty(opts.Shop, os.Getenv("SHOPIFY_SHOP"))
	klaviyoAccount := firstNonEmpty(opts.KlaviyoAccount, os.Getenv("KLAVIYO_ACCOUNT"))
	gaProperty := firstNonEmpty(opts.GAProperty, os.Getenv("GA4_PROPERTY_ID"))
	site := firstNonEmpty(opts.Site, os.Getenv("GSC_SITE_URL"))
	ahrefsTarget := firstNonEmpty(opts.AhrefsTarget, os.Getenv("AHREFS_TARGET"), os.Getenv("AHREFS_PROJECT"))
	ahrefsDate := firstNonEmpty(opts.AhrefsDate, end)

	defs := []childSourceDef{}
	if shop != "" {
		defs = append(defs, childSourceDef{name: "shopify", binary: "shopify-pp-cli", args: []string{"products", "list", "--agent", "--all", "--data-source", "live"}, env: []string{"SHOPIFY_SHOP=" + shop}})
	}
	if klaviyoAccount != "" || os.Getenv("KLAVIYO_API_KEY") != "" {
		env := []string{}
		if klaviyoAccount != "" {
			env = append(env, "KLAVIYO_ACCOUNT="+klaviyoAccount)
		}
		defs = append(defs, childSourceDef{name: "klaviyo", binary: "klaviyo-pp-cli", args: []string{"report", "flow-comparison", "--agent", "--data-source", "live"}, env: env})
	}
	if gaProperty != "" {
		defs = append(defs, childSourceDef{name: "ga4", binary: "google-analytics-pp-cli", args: []string{"top-pages", "--agent", "--property", gaProperty, "--start", start, "--end", end, "--limit", strconv.Itoa(opts.Limit)}})
	}
	if site != "" {
		defs = append(defs, childSourceDef{name: "gsc", binary: "google-search-console-pp-cli", args: []string{"webmasters", "query-search-analytics", site, "--agent", "--dimensions", "[\"page\"]", "--start-date", start, "--end-date", end, "--type", "WEB", "--row-limit", strconv.Itoa(opts.Limit)}})
	}
	if ahrefsTarget != "" {
		defs = append(defs, childSourceDef{name: "ahrefs", binary: "ahrefs-pp-cli", args: []string{"site-explorer", "top-pages", "--agent", "--target", ahrefsTarget, "--date", ahrefsDate, "--mode", "subdomains", "--protocol", "both", "--limit", strconv.Itoa(opts.Limit), "--select", "url,sum_traffic,keywords,referring_domains,top_keyword"}})
	}
	return defs
}

func findChildSource(defs []childSourceDef, source string) (childSourceDef, bool) {
	for _, def := range defs {
		if def.name == source {
			return def, true
		}
	}
	return childSourceDef{}, false
}

func missingChildSources(defs []childSourceDef, sources ...string) []string {
	missing := []string{}
	for _, source := range sources {
		if _, ok := findChildSource(defs, source); !ok {
			missing = append(missing, source)
		}
	}
	return missing
}

func sourceConfigHint(source string) string {
	switch source {
	case "shopify":
		return "provide --shop, a saved profile Shopify shop, or SHOPIFY_SHOP"
	case "klaviyo":
		return "provide --klaviyo-account, a saved profile Klaviyo account, KLAVIYO_ACCOUNT, or KLAVIYO_API_KEY"
	case "ga4":
		return "provide --ga-property, a saved profile GA property, or GA4_PROPERTY_ID"
	case "gsc":
		return "provide --site, a saved profile GSC property/site, or GSC_SITE_URL"
	case "ahrefs":
		return "provide --ahrefs-target, a saved profile Ahrefs target, AHREFS_TARGET, or AHREFS_PROJECT"
	default:
		return "provide source flags, a saved profile, or source env vars"
	}
}

func defaultDateRange(start, end string) (string, string) {
	return intelcli.DefaultDateRange(start, end)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func runChildSource(def childSourceDef) (childEntities, string, string, string, error) {
	command := def.binary + " " + strings.Join(def.args, " ")
	c := exec.Command(def.binary, def.args...)
	if len(def.env) > 0 {
		c.Env = append(os.Environ(), def.env...)
	}
	b, err := c.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return childEntities{}, command, "", "", fmt.Errorf("%s failed: %w: %s", def.name, err, strings.TrimSpace(string(ee.Stderr)))
		}
		return childEntities{}, command, "", "", fmt.Errorf("%s failed: %w", def.name, err)
	}
	if err := validateChildSchema(def.name, b); err != nil {
		return childEntities{}, command, "", "", err
	}
	entities, err := parseChildEntities(def.name, b)
	if err != nil {
		return childEntities{}, command, "", "", fmt.Errorf("%s returned invalid JSON: %w", def.name, err)
	}
	return entities, command, intelcli.HashBytes(b), intelcli.ChildCLIVersion(def.binary), nil
}

func validateChildSchema(source string, b []byte) error {
	var payload map[string]any
	if err := json.Unmarshal(b, &payload); err != nil {
		return err
	}
	version := intelcli.ExtractSchemaVersion(payload)
	supported := map[string][]string{
		"shopify": {"shopify.products/v1", "shopify.rows/v1"},
		"klaviyo": {"klaviyo.flows/v1", "klaviyo.rows/v1"},
		"ga4":     {"google-analytics.top-pages/v1", "google-analytics.rows/v1"},
		"gsc":     {"google-search-console.search-analytics/v1", "google-search-console.rows/v1"},
		"ahrefs":  {"ahrefs.top-pages/v1", "ahrefs.rows/v1"},
	}
	if !intelcli.SupportedSchema(source, version, supported) {
		return fmt.Errorf("%s child CLI output missing unsupported schema_version %q; confidence and movers require one of %s", source, version, strings.Join(supported[source], ", "))
	}
	return nil
}

func parseChildEntities(source string, b []byte) (childEntities, error) {
	var v any
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return childEntities{}, err
	}
	rows := findRows(v)
	out := childEntities{}
	for _, row := range rows {
		m, ok := row.(map[string]any)
		if !ok {
			continue
		}
		flat := flattenMap(m, "")
		switch source {
		case "shopify":
			if p := productFromChildMap(flat); p.Handle != "" || p.ID != "" || p.URL != "" {
				out.Products = append(out.Products, p)
			}
		case "klaviyo":
			if e := emailFromChildMap(flat); e.Name != "" {
				out.Emails = append(out.Emails, e)
			}
			if p := emailProductFromChildMap(flat); p.Handle != "" || p.ID != "" || p.URL != "" {
				out.Products = append(out.Products, p)
			}
		case "ga4", "gsc", "ahrefs":
			if p := pageFromChildMap(source, flat); p.URL != "" {
				out.Pages = append(out.Pages, p)
				if prod := pageProductFromChildMap(source, flat, p); prod.Handle != "" || prod.ID != "" || prod.URL != "" {
					out.Products = append(out.Products, prod)
				}
				if cat := categoryFromPage(p); cat.Handle != "" {
					out.Categories = append(out.Categories, cat)
				}
			}
		}
	}
	return out, nil
}

func findRows(v any) []any {
	switch x := v.(type) {
	case []any:
		return x
	case map[string]any:
		for _, key := range []string{"products", "pages", "flows", "emails", "rows", "items", "results", "data"} {
			if rows, ok := x[key].([]any); ok {
				return rows
			}
		}
		for _, key := range []string{"results", "data", "response", "result", "meta"} {
			if nested, ok := x[key]; ok {
				if rows := findRows(nested); len(rows) > 0 {
					return rows
				}
			}
		}
	}
	return nil
}

func flattenMap(in map[string]any, prefix string) map[string]any {
	out := map[string]any{}
	for k, v := range in {
		key := normKey(k)
		if prefix != "" {
			key = prefix + "." + key
		}
		if m, ok := v.(map[string]any); ok {
			for nk, nv := range flattenMap(m, key) {
				out[nk] = nv
			}
			continue
		}
		if arr, ok := v.([]any); ok {
			for i, item := range arr {
				out[fmt.Sprintf("%s.%d", key, i)] = item
				if m, ok := item.(map[string]any); ok {
					for nk, nv := range flattenMap(m, fmt.Sprintf("%s.%d", key, i)) {
						out[nk] = nv
					}
				}
			}
		}
		out[key] = v
	}
	return out
}

func productFromChildMap(m map[string]any) store.Product {
	handle := firstString(m, "handle", "product_handle", "product.handle")
	title := firstString(m, "title", "name", "product_title", "product.title")
	rawURL := firstString(m, "url", "online_store_url", "onlineStoreUrl", "product_url", "canonical_url")
	if rawURL == "" && handle != "" {
		rawURL = "/products/" + handle
	}
	inventory := firstInt(m, "inventory", "inventory_quantity", "total_inventory", "variants.0.inventoryquantity", "variants.0.inventory_quantity")
	price := firstFloat(m, "price", "min_price", "variants.0.price", "variants.0.node.price")
	return store.Product{ID: firstString(m, "id", "product_id", "admin_graphql_api_id"), Handle: handle, Title: title, URL: rawURL, Vendor: firstString(m, "vendor"), Category: firstString(m, "category", "product_type", "producttype"), Status: firstString(m, "status"), Price: price, Inventory: inventory}
}

func emailFromChildMap(m map[string]any) store.EmailFlow {
	name := firstString(m, "name", "flow", "flow_name", "flowname", "campaign", "campaign_name")
	return store.EmailFlow{Name: name, Type: firstString(m, "type", "kind"), Revenue: firstFloat(m, "revenue", "attributed_revenue", "total_revenue"), Recipients: firstInt(m, "recipients", "delivered", "sends", "sent"), OpenRate: firstFloat(m, "open_rate", "openrate"), ClickRate: firstFloat(m, "click_rate", "clickrate", "clicks_rate"), ConversionRate: firstFloat(m, "conversion_rate", "conversionrate"), AttributedSales: firstInt(m, "attributed_sales", "orders", "conversions"), NeedsAction: firstBool(m, "needs_action")}
}

func emailProductFromChildMap(m map[string]any) store.Product {
	handle := firstString(m, "product_handle", "handle", "product.handle", "item_name", "itemname")
	id := firstString(m, "product_id", "item_id", "itemid", "sku")
	if handle == "" && id == "" {
		return store.Product{}
	}
	return store.Product{ID: id, Handle: handle, Title: firstString(m, "product_title", "product_name", "name"), EmailRevenue: firstFloat(m, "email_revenue", "revenue", "attributed_revenue"), EmailClicks: firstInt(m, "email_clicks", "clicks")}
}

func pageFromChildMap(source string, m map[string]any) store.Page {
	rawURL := firstString(m, "url", "page", "pageurl", "page_url", "landingpageplusquerystring", "landing_page_plus_query_string", "landingpage", "landing_page", "landingpagepath", "landing_page_path", "path", "pagepath", "page_path", "keys.0", "dimensionvalues.0.value")
	p := store.Page{URL: rawURL, Title: firstString(m, "title", "pagetitle", "page_title"), Type: pageType(rawURL)}
	switch source {
	case "ga4":
		p.Sessions = firstInt(m, "sessions", "ga4.sessions")
		p.PreviousSessions = firstInt(m, "previoussessions", "previous_sessions", "priorsessions", "sessions_previous")
		p.Revenue = firstFloat(m, "revenue", "totalrevenue", "total_revenue", "purchase_revenue", "purchaserevenue")
		p.PreviousRevenue = firstFloat(m, "previousrevenue", "previous_revenue", "priorrevenue", "revenue_previous")
	case "gsc":
		p.SearchClicks = firstInt(m, "clicks", "gsc.clicks")
		p.PreviousClicks = firstInt(m, "previousclicks", "previous_clicks", "priorclicks", "clicks_previous")
		p.SearchPosition = firstFloat(m, "position", "avgposition", "avg_position", "averageposition", "average_position")
	case "ahrefs":
		p.Backlinks = firstInt(m, "backlinks", "backlink_count", "backlinkcount")
		p.RefDomains = firstInt(m, "refdomains", "ref_domains", "referringdomains", "referring_domains", "domains")
	}
	return p
}

func pageProductFromChildMap(source string, m map[string]any, p store.Page) store.Product {
	handle := firstNonEmpty(firstString(m, "product_handle", "handle", "item_name", "itemname"), productHandleFromURL(p.URL))
	id := firstString(m, "product_id", "item_id", "itemid", "sku")
	if handle == "" && id == "" {
		return store.Product{}
	}
	prod := store.Product{ID: id, Handle: handle, Title: firstString(m, "product_title", "product_name", "title"), URL: p.URL, Category: collectionFromURL(p.URL)}
	switch source {
	case "ga4":
		prod.Sessions, prod.PreviousSessions, prod.Revenue, prod.PreviousRevenue = p.Sessions, p.PreviousSessions, p.Revenue, p.PreviousRevenue
	case "gsc":
		prod.SearchClicks, prod.PreviousClicks, prod.SearchPosition = p.SearchClicks, p.PreviousClicks, p.SearchPosition
	case "ahrefs":
		prod.Backlinks, prod.RefDomains = p.Backlinks, p.RefDomains
	}
	return prod
}

func categoryFromPage(p store.Page) store.Category {
	handle := firstNonEmpty(p.PrimaryCollection, collectionFromURL(p.URL))
	if handle == "" {
		return store.Category{}
	}
	return store.Category{Handle: handle, Title: titleFromHandle(handle), URL: "/collections/" + handle, Revenue: p.Revenue, PreviousRevenue: p.PreviousRevenue, Sessions: p.Sessions, SearchClicks: p.SearchClicks}
}

func mergeChildEntities(dst *store.DataSet, src childEntities, source, command string) {
	for _, p := range src.Products {
		stampProduct(&p, source, command)
		upsertProduct(dst, p, source, command)
	}
	for _, p := range src.Pages {
		stampPage(&p, source, command)
		upsertPage(dst, p, source, command)
	}
	for _, c := range src.Categories {
		stampCategory(&c, source, command)
		upsertCategory(dst, c, source, command)
	}
	for _, e := range src.Emails {
		stampEmail(&e, source, command)
		upsertEmail(dst, e, source, command)
	}
}

func upsertProduct(d *store.DataSet, src store.Product, source, command string) {
	key := productKey(src)
	for i := range d.Products {
		if productKey(d.Products[i]) == key && key != "" {
			mergeProduct(&d.Products[i], src, source, command)
			return
		}
	}
	d.Products = append(d.Products, src)
}

func mergeProduct(dst *store.Product, src store.Product, source, command string) {
	if dst.ID == "" {
		dst.ID = src.ID
	}
	if dst.Handle == "" {
		dst.Handle = src.Handle
	}
	if dst.Title == "" {
		dst.Title = src.Title
	}
	if dst.URL == "" {
		dst.URL = src.URL
	}
	if dst.Vendor == "" {
		dst.Vendor = src.Vendor
	}
	if dst.Category == "" {
		dst.Category = src.Category
	}
	if dst.Status == "" {
		dst.Status = src.Status
	}
	switch source {
	case "shopify":
		if src.Price != 0 {
			dst.Price = src.Price
		}
		dst.Inventory = src.Inventory
	case "klaviyo":
		if src.EmailRevenue != 0 {
			dst.EmailRevenue = src.EmailRevenue
		}
		if src.EmailClicks != 0 {
			dst.EmailClicks = src.EmailClicks
		}
	case "ga4":
		dst.Sessions, dst.PreviousSessions, dst.Revenue, dst.PreviousRevenue = src.Sessions, src.PreviousSessions, src.Revenue, src.PreviousRevenue
	case "gsc":
		dst.SearchClicks, dst.PreviousClicks, dst.SearchPosition = src.SearchClicks, src.PreviousClicks, src.SearchPosition
	case "ahrefs":
		dst.Backlinks, dst.RefDomains = src.Backlinks, src.RefDomains
	}
	stampProduct(dst, source, command)
}

func upsertPage(d *store.DataSet, src store.Page, source, command string) {
	key := pageKey(src.URL)
	for i := range d.Pages {
		if pageKey(d.Pages[i].URL) == key && key != "" {
			mergePage(&d.Pages[i], src, source, command)
			return
		}
	}
	d.Pages = append(d.Pages, src)
}

func mergePage(dst *store.Page, src store.Page, source, command string) {
	if dst.Title == "" {
		dst.Title = src.Title
	}
	if dst.Type == "" {
		dst.Type = src.Type
	}
	if dst.PrimaryCollection == "" {
		dst.PrimaryCollection = src.PrimaryCollection
	}
	switch source {
	case "ga4":
		dst.Sessions, dst.PreviousSessions, dst.Revenue, dst.PreviousRevenue = src.Sessions, src.PreviousSessions, src.Revenue, src.PreviousRevenue
	case "gsc":
		dst.SearchClicks, dst.PreviousClicks, dst.SearchPosition = src.SearchClicks, src.PreviousClicks, src.SearchPosition
	case "ahrefs":
		dst.Backlinks, dst.RefDomains = src.Backlinks, src.RefDomains
	}
	stampPage(dst, source, command)
}

func upsertCategory(d *store.DataSet, src store.Category, source, command string) {
	key := categoryKey(src)
	for i := range d.Categories {
		if categoryKey(d.Categories[i]) == key && key != "" {
			if src.Revenue != 0 {
				d.Categories[i].Revenue = src.Revenue
			}
			if src.PreviousRevenue != 0 {
				d.Categories[i].PreviousRevenue = src.PreviousRevenue
			}
			if src.Sessions != 0 {
				d.Categories[i].Sessions = src.Sessions
			}
			if src.SearchClicks != 0 {
				d.Categories[i].SearchClicks = src.SearchClicks
			}
			stampCategory(&d.Categories[i], source, command)
			return
		}
	}
	d.Categories = append(d.Categories, src)
}

func upsertEmail(d *store.DataSet, src store.EmailFlow, source, command string) {
	key := strings.ToLower(strings.TrimSpace(src.Name))
	for i := range d.Emails {
		if strings.ToLower(strings.TrimSpace(d.Emails[i].Name)) == key && key != "" {
			if src.Revenue != 0 {
				d.Emails[i].Revenue = src.Revenue
			}
			if src.Recipients != 0 {
				d.Emails[i].Recipients = src.Recipients
			}
			if src.OpenRate != 0 {
				d.Emails[i].OpenRate = src.OpenRate
			}
			if src.ClickRate != 0 {
				d.Emails[i].ClickRate = src.ClickRate
			}
			if src.ConversionRate != 0 {
				d.Emails[i].ConversionRate = src.ConversionRate
			}
			if src.AttributedSales != 0 {
				d.Emails[i].AttributedSales = src.AttributedSales
			}
			d.Emails[i].NeedsAction = src.NeedsAction
			stampEmail(&d.Emails[i], source, command)
			return
		}
	}
	d.Emails = append(d.Emails, src)
}

func stampProduct(p *store.Product, source, command string) { stampSources(&p.Source, source, command) }
func stampPage(p *store.Page, source, command string)       { stampSources(&p.Source, source, command) }
func stampCategory(c *store.Category, source, command string) {
	stampSources(&c.Source, source, command)
}
func stampEmail(e *store.EmailFlow, source, command string) { stampSources(&e.Source, source, command) }

func stampSources(s *store.MetricSources, source, command string) {
	e := store.SourceEvidence{Synced: true, ChildCLICommand: command}
	switch source {
	case "shopify":
		s.Shopify = e
	case "klaviyo":
		s.Klaviyo = e
	case "ga4":
		s.GA4 = e
	case "gsc":
		s.GSC = e
	case "ahrefs":
		s.Ahrefs = e
	}
}

func productKey(p store.Product) string {
	for _, v := range []string{p.Handle, productHandleFromURL(p.URL), p.ID} {
		if strings.TrimSpace(v) != "" {
			return strings.ToLower(strings.TrimSpace(v))
		}
	}
	return ""
}

func pageKey(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if u, err := url.Parse(raw); err == nil && u.Path != "" {
		path := strings.TrimRight(u.Path, "/")
		if path == "" {
			path = "/"
		}
		return strings.ToLower(path)
	}
	path := strings.TrimRight(raw, "/")
	if path == "" {
		path = "/"
	}
	return strings.ToLower(path)
}

func categoryKey(c store.Category) string {
	return strings.ToLower(firstNonEmpty(c.Handle, collectionFromURL(c.URL)))
}

func productHandleFromURL(raw string) string {
	parts := pathParts(raw)
	for i, part := range parts {
		if part == "products" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

func collectionFromURL(raw string) string {
	parts := pathParts(raw)
	for i, part := range parts {
		if part == "collections" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

func titleFromHandle(handle string) string {
	parts := strings.Fields(strings.ReplaceAll(handle, "-", " "))
	for i, part := range parts {
		if len(part) == 0 {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

func pageType(raw string) string {
	parts := pathParts(raw)
	if len(parts) == 0 {
		return "page"
	}
	return strings.TrimSuffix(parts[0], "s")
}

func pathParts(raw string) []string {
	if u, err := url.Parse(raw); err == nil && u.Path != "" {
		raw = u.Path
	}
	parts := []string{}
	for _, part := range strings.Split(strings.Trim(raw, "/"), "/") {
		if part != "" {
			parts = append(parts, strings.ToLower(part))
		}
	}
	return parts
}

func normKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	repl := strings.NewReplacer(" ", "_", "-", "_", "/", "_")
	return repl.Replace(s)
}

func firstString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := m[normKey(key)]; ok {
			switch x := v.(type) {
			case string:
				return strings.TrimSpace(x)
			case json.Number:
				return x.String()
			case float64:
				return strconv.FormatFloat(x, 'f', -1, 64)
			}
		}
	}
	return ""
}

func firstInt(m map[string]any, keys ...string) int {
	for _, key := range keys {
		if v, ok := m[normKey(key)]; ok {
			return int(firstNumeric(v))
		}
	}
	return 0
}

func firstFloat(m map[string]any, keys ...string) float64 {
	for _, key := range keys {
		if v, ok := m[normKey(key)]; ok {
			return firstNumeric(v)
		}
	}
	return 0
}

func firstBool(m map[string]any, keys ...string) bool {
	for _, key := range keys {
		if v, ok := m[normKey(key)]; ok {
			switch x := v.(type) {
			case bool:
				return x
			case string:
				return strings.EqualFold(x, "true") || x == "1" || strings.EqualFold(x, "yes")
			}
		}
	}
	return false
}

func firstNumeric(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case json.Number:
		f, _ := x.Float64()
		return f
	case string:
		x = strings.TrimSpace(strings.TrimSuffix(x, "%"))
		f, _ := strconv.ParseFloat(strings.ReplaceAll(x, ",", ""), 64)
		return f
	default:
		return 0
	}
}
