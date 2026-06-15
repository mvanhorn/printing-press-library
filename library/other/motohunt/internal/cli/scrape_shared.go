// Copyright 2026 richardadonnell. Licensed under Apache-2.0. See LICENSE.
// Hand-written: shared plumbing for the rich goquery commands.

package cli

import (
	"io"
	"net/http"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/motohunt/internal/motohunt"
)

// NOTE: boundCtx lives in helpers.go (provided by the v4.24.0 scaffold). The
// hand-written commands call it for the root --timeout boundary; the duplicate
// that shipped in the v4.2.2 source tree was dropped here to avoid a
// redeclaration against the generated helper.

// siteConfigFor resolves the active --site into a SiteConfig. The
// MOTOHUNT_BASE_URL env override is intentionally NOT honored here: these
// rich commands need the real host + base search path, and verify runs go
// through the --dry-run short-circuit before any network call.
func siteConfigFor(flags *rootFlags) (motohunt.SiteConfig, error) {
	site := "moto"
	if flags != nil && flags.site != "" {
		site = flags.site
	}
	return motohunt.ResolveSite(site)
}

// scrapeClient builds a motohunt.Client whose HTTP transport honors --timeout
// and --rate-limit is left to the caller (these scrapes are page-bounded).
func scrapeClient(flags *rootFlags) *motohunt.Client {
	d := 30 * time.Second
	if flags != nil && flags.timeout > 0 {
		d = flags.timeout
	}
	return motohunt.NewClient(&http.Client{Timeout: d})
}

// printDomainJSON outputs a typed value through the standard pipeline, but with
// a domain-aware compact field set. The generic --compact allowlist (id, name,
// title, status, ...) drops scrape-specific fields like mileage and deal_rating
// that are the whole point of these commands. When --compact is active and the
// user gave no explicit --select, we substitute a domain allowlist so --agent
// still returns the buyer-relevant fields. An explicit --select always wins.
func printDomainJSON(w io.Writer, v any, flags *rootFlags) error {
	if flags != nil && flags.compact && flags.selectFields == "" {
		// Shallow-copy flags so we don't mutate shared state across commands.
		local := *flags
		local.compact = false
		local.selectFields = domainCompactFields
		return printJSONFiltered(w, v, &local)
	}
	return printJSONFiltered(w, v, flags)
}

// domainCompactFields is the buyer-relevant field set surfaced under --agent /
// --compact for listing-shaped output. filterFields keeps any object key that
// matches one of these; rows that lack a field simply omit it.
const domainCompactFields = "id,title,price,mileage,condition,deal_rating,location,dealer,url," +
	"base_msrp,alp,gap_pct,vin,color,stock_number,certified_pre_owned,enriched," +
	"name,slug,section,watch,scanned,new,price_drops,status," +
	"site,q,make,model,style,state,sort,limit,max_pages,created_at,old_price,new_price"
