// Package cli — nsdl_sync.go is a hand-authored companion to the generated
// sync.go. NSDL/CDSL have no API: every resource here is a server-rendered
// ASP.NET GridView report page, not a paginated JSON REST collection, so the
// generic REST sync machinery in sync.go does not apply. sync.go's
// syncResource intercepts to nsdlSyncHandlers before running its generic
// pagination logic (see the dispatch hook at the top of that function).
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/fpi-india/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/fpi-india/internal/nsdl"
	"github.com/mvanhorn/printing-press-library/library/other/fpi-india/internal/store"
)

// nsdlSyncClient is the same method set syncResource's generic REST path
// declares its client parameter with. Defining it as a named interface lets
// nsdlSyncHandlers share that exact call-site value without changing the
// generated function's signature.
type nsdlSyncClient interface {
	Get(context.Context, string, map[string]string) (json.RawMessage, error)
	RequestBaseURL() string
	LastContentType() string
	RateLimit() float64
}

type nsdlSyncHandlerFunc func(ctx context.Context, c nsdlSyncClient, db *store.Store, resource string, syncEvents io.Writer, started time.Time) syncResult

var nsdlSyncHandlers = map[string]nsdlSyncHandlerFunc{
	"net_investment": syncNetInvestment,
	"auc":            syncAUC,
	"trades":         syncTrades,
	"sector":         syncSector,
	"registry":       syncRegistry,
	"limits":         syncLimitsHandler,
	"cdsl_reports":   syncCDSLReports,
}

func emitSyncStart(syncEvents io.Writer, resource string) {
	if !humanFriendly {
		fmt.Fprintf(syncEvents, `{"event":"sync_start","resource":"%s"}`+"\n", resource)
	}
}

func syncFail(resource string, err error, started time.Time) syncResult {
	return syncResult{Resource: resource, Err: err, Duration: time.Since(started)}
}

func syncOK(resource string, count int, started time.Time) syncResult {
	return syncResult{Resource: resource, Count: count, Duration: time.Since(started)}
}

// syncNetInvestment fetches NSDL's Yearwise.aspx for FY, CY, and CY-monthly
// (for the last 3 calendar years, to seed monthly history without an
// unbounded fan-out) in both INR and USD, parses every period row, and
// upserts each as its own resources record keyed by period+type+currency —
// so the full 1992-93-to-present series lands in one sync call.
func syncNetInvestment(ctx context.Context, c nsdlSyncClient, db *store.Store, resource string, syncEvents io.Writer, started time.Time) syncResult {
	emitSyncStart(syncEvents, resource)
	var items []json.RawMessage

	fetchSeries := func(rptType, periodType, currency string) error {
		params := map[string]string{"RptType": rptType}
		if currency == "USD" {
			params["CurrVal"] = "USD"
		}
		body, err := c.Get(ctx, "/web/Reports/Yearwise.aspx", params)
		if err != nil {
			return err
		}
		rows, err := nsdl.ParseNetInvestment(body, periodType, currency)
		if err != nil {
			return err
		}
		for _, r := range rows {
			data, err := json.Marshal(r)
			if err != nil {
				continue
			}
			items = append(items, data)
		}
		return nil
	}

	if err := fetchSeries("5", "fy", "INR"); err != nil {
		return syncFail(resource, fmt.Errorf("fetching FY series: %w", err), started)
	}
	if err := fetchSeries("5", "fy", "USD"); err != nil {
		return syncFail(resource, fmt.Errorf("fetching FY USD series: %w", err), started)
	}
	if err := fetchSeries("6", "cy", "INR"); err != nil {
		return syncFail(resource, fmt.Errorf("fetching CY series: %w", err), started)
	}
	if err := fetchSeries("6", "cy", "USD"); err != nil {
		return syncFail(resource, fmt.Errorf("fetching CY USD series: %w", err), started)
	}

	// Monthly breakdown: drill into the current and two prior calendar
	// years. NSDL only exposes monthly detail per-year-selected, so a full
	// historical monthly backfill would mean one request per year since
	// 1992 — disproportionate for data most transcendence commands consume
	// at fortnight/FY/CY granularity. Three years covers "this year vs last
	// year vs the year before" trend/yoy queries without an unbounded fan-out.
	currentYear := time.Now().Year()
	for _, y := range []int{currentYear, currentYear - 1, currentYear - 2} {
		yearStr := fmt.Sprintf("%d", y)
		body, err := c.Get(ctx, "/web/Reports/Yearwise.aspx", map[string]string{"RptType": "6", "year": yearStr})
		if err != nil {
			continue // best-effort: a missing/unpublished year should not fail the whole sync
		}
		rows, err := nsdl.ParseNetInvestment(body, "cy_monthly", "INR")
		if err != nil {
			continue
		}
		for _, r := range rows {
			r.Period = yearStr + " " + r.Period
			data, err := json.Marshal(r)
			if err != nil {
				continue
			}
			items = append(items, data)
		}
	}

	// Attach a stable id before upserting: db.UpsertBatch extracts an id via
	// generic field-name fallbacks, none of which exist on NetInvestmentRow.
	for i, raw := range items {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err != nil {
			continue
		}
		var periodType, period, currency string
		_ = json.Unmarshal(obj["period_type"], &periodType)
		_ = json.Unmarshal(obj["period"], &period)
		_ = json.Unmarshal(obj["currency"], &currency)
		obj["id"] = mustMarshal(periodType + "-" + period + "-" + currency)
		out, err := json.Marshal(obj)
		if err == nil {
			items[i] = out
		}
	}

	stored, _, err := db.UpsertBatch(resource, items)
	if err != nil {
		return syncFail(resource, err, started)
	}
	return syncOK(resource, stored, started)
}

// syncAUC fetches NSDL's country-wise and category-wise assets-under-custody
// reports (ReportDetail.aspx?RepID=14|18). Each stored record is tagged with
// today's sync date so repeated syncs accumulate a time series the auc trend
// command can diff — NSDL's page itself only ever shows the current snapshot.
func syncAUC(ctx context.Context, c nsdlSyncClient, db *store.Store, resource string, syncEvents io.Writer, started time.Time) syncResult {
	emitSyncStart(syncEvents, resource)
	syncDate := time.Now().Format("2006-01-02")
	var items []json.RawMessage

	fetchOne := func(repID, by string) error {
		body, err := c.Get(ctx, "/web/Reports/ReportDetail.aspx", map[string]string{"RepID": repID})
		if err != nil {
			return err
		}
		recs, err := nsdl.ParseAUC(body)
		if err != nil {
			return err
		}
		for i, rec := range recs {
			key := recordKey(rec, "Country", "Category")
			rec["by"] = by
			rec["synced_date"] = syncDate
			rec["id"] = by + "-" + key + "-" + syncDate
			data, err := json.Marshal(rec)
			if err != nil {
				continue
			}
			items = append(items, data)
			_ = i
		}
		return nil
	}

	if err := fetchOne("14", "country"); err != nil {
		return syncFail(resource, fmt.Errorf("fetching AUC country: %w", err), started)
	}
	if err := fetchOne("18", "category"); err != nil {
		return syncFail(resource, fmt.Errorf("fetching AUC category: %w", err), started)
	}

	stored, _, err := db.UpsertBatch(resource, items)
	if err != nil {
		return syncFail(resource, err, started)
	}
	return syncOK(resource, stored, started)
}

// syncTrades fetches the trade-wise equity and debt report pages. These
// live under NSDL's StaticReports path family, which rejects Go's stock
// net/http TLS fingerprint even with a browser User-Agent set — see
// browserFingerprintHTTPGet's doc comment — so they go through a
// Chrome-impersonating client instead of the standard one every other
// resource in this file uses.
func syncTrades(ctx context.Context, c nsdlSyncClient, db *store.Store, resource string, syncEvents io.Writer, started time.Time) syncResult {
	emitSyncStart(syncEvents, resource)
	var items []json.RawMessage

	fetchOne := func(path, asset string) error {
		body, err := nsdl.FetchStaticReport(ctx, c.RequestBaseURL(), path)
		if err != nil {
			return err
		}
		recs, err := nsdl.ParseTrades(body)
		if err != nil {
			return err
		}
		for i, rec := range recs {
			rec["asset"] = asset
			rec["id"] = fmt.Sprintf("%s-%d", asset, i)
			data, err := json.Marshal(rec)
			if err != nil {
				continue
			}
			items = append(items, data)
		}
		return nil
	}

	if err := fetchOne("/web/StaticReports/FIITradeWise2008/FIITradeWise2008.htm", "equity"); err != nil {
		return syncFail(resource, fmt.Errorf("fetching trade-wise equity: %w", err), started)
	}
	if err := fetchOne("/web/StaticReports/FIITradeWiseDebt/FIITradeWiseDebt.htm", "debt"); err != nil {
		return syncFail(resource, fmt.Errorf("fetching trade-wise debt: %w", err), started)
	}

	stored, _, err := db.UpsertBatch(resource, items)
	if err != nil {
		return syncFail(resource, err, started)
	}
	return syncOK(resource, stored, started)
}

// syncCDSLReports fetches the current-month daily FPI report from both
// NSDL (Monthly.aspx, primary) and CDSL (FIIMonthly, cross-check) — both
// render the same rowspan GridView shape as their respective "latest"
// pages, one row-group per already-published day this month, broken out
// by asset class and investment route (gross purchases/sales, not just
// net). This is the only historical archive of that granular daily
// breakdown either source exposes: NSDL's own Yearwise/CY-series
// endpoints only ever publish pre-aggregated net figures per period,
// never the daily route-level detail. Both sources reset their monthly
// view at the start of each calendar month, so repeated syncs across
// month boundaries are what build up a rolling multi-month local
// history; a single sync only captures whatever days the current month
// has published so far.
func syncCDSLReports(ctx context.Context, c nsdlSyncClient, db *store.Store, resource string, syncEvents io.Writer, started time.Time) syncResult {
	emitSyncStart(syncEvents, resource)

	var items []json.RawMessage

	addSource := func(recs []map[string]string, source string) {
		for _, rec := range recs {
			// recordKey picks the first non-empty candidate; every row
			// shares the same Reporting Date, so the id must combine
			// source + date + asset + route explicitly or every row on a
			// given day (and same-day rows from the two sources) would
			// collide.
			key := strings.Join([]string{
				source,
				rec["Reporting Date"],
				rec["Debt/Debt-VRR/Equity/Hybrid"],
				rec["Investment Route"],
			}, "-")
			rec["source"] = source
			rec["id"] = key
			data, jerr := json.Marshal(rec)
			if jerr != nil {
				continue
			}
			items = append(items, data)
		}
	}

	nsdlBody, nsdlErr := c.Get(ctx, "/web/Reports/Monthly.aspx", nil)
	if nsdlErr == nil {
		if recs, perr := nsdl.ParseGenericRecords(nsdlBody); perr == nil {
			addSource(recs, "nsdl")
		} else {
			nsdlErr = perr
		}
	}

	cdslBody, cdslErr := nsdl.FetchStaticReport(ctx, "https://www.cdslindia.com", "/eservices/publications/FIIMonthly")
	if cdslErr == nil {
		if recs, perr := nsdl.ParseGenericRecords(cdslBody); perr == nil {
			addSource(recs, "cdsl")
		} else {
			cdslErr = perr
		}
	}

	if nsdlErr != nil && cdslErr != nil {
		return syncFail(resource, fmt.Errorf("fetching NSDL monthly report: %w; fetching CDSL monthly report: %w", nsdlErr, cdslErr), started)
	}

	stored, _, err := db.UpsertBatch(resource, items)
	if err != nil {
		return syncFail(resource, err, started)
	}
	return syncOK(resource, stored, started)
}

// syncSector fetches the fortnightly sector-wise landing page for its list
// of dated snapshot URLs, then syncs the most recent N fortnights. Every
// sync run stores one record per (sector, period) pair, so repeated syncs
// build the history sector rotation needs to rank period-over-period
// deltas — NSDL never exposes more than one fortnight's absolute figures at
// a time.
func syncSector(ctx context.Context, c nsdlSyncClient, db *store.Store, resource string, syncEvents io.Writer, started time.Time) syncResult {
	emitSyncStart(syncEvents, resource)
	listingBody, err := c.Get(ctx, "/web/Reports/FPI_Fortnightly_Selection.aspx", nil)
	if err != nil {
		return syncFail(resource, fmt.Errorf("fetching sector period listing: %w", err), started)
	}
	periods := nsdl.ParseSectorPeriods(listingBody)
	if len(periods) == 0 {
		return syncFail(resource, fmt.Errorf("no fortnightly periods found on sector listing page"), started)
	}

	// Sync the most recent 3 fortnights: enough for sector rotation's
	// period-over-period ranking without a 347-request fan-out on every
	// sync call.
	limit := 3
	if len(periods) < limit {
		limit = len(periods)
	}

	var items []json.RawMessage
	for _, p := range periods[:limit] {
		// StaticReports paths reject Go's stock net/http TLS fingerprint
		// (see browserFingerprintHTTPGet's doc comment); the landing page
		// fetched above is a plain Reports/ path and is unaffected.
		body, err := nsdl.FetchStaticReport(ctx, c.RequestBaseURL(), p.Path)
		if err != nil {
			continue // best-effort: one broken period link should not fail the whole sync
		}
		recs, err := nsdl.ParseSectorSnapshot(body)
		if err != nil {
			continue
		}
		for i, rec := range recs {
			sectorName := recordKey(rec, "Sectors", "Sector")
			rec["period_label"] = p.Label
			rec["id"] = p.Label + "-" + sectorName
			data, err := json.Marshal(rec)
			if err != nil {
				continue
			}
			items = append(items, data)
			_ = i
		}
	}

	stored, _, err := db.UpsertBatch(resource, items)
	if err != nil {
		return syncFail(resource, err, started)
	}
	return syncOK(resource, stored, started)
}

// syncRegistry fetches the FPI category-wise registration counts and
// DDP-pendency reports. RegisteredFIISAFPI.aspx (the raw FPI name list) is a
// search form requiring an ASP.NET WebForms POST with VIEWSTATE replay — a
// different engineering surface from the GET-based reports every other
// resource here uses — and is not synced; see the README/SKILL Known Gaps
// section.
func syncRegistry(ctx context.Context, c nsdlSyncClient, db *store.Store, resource string, syncEvents io.Writer, started time.Time) syncResult {
	emitSyncStart(syncEvents, resource)
	var items []json.RawMessage

	addRecords := func(body []byte, kind string) error {
		recs, err := nsdl.ParseRegistry(body)
		if err != nil {
			return err
		}
		for i, rec := range recs {
			rec["kind"] = kind
			rec["id"] = fmt.Sprintf("%s-%d", kind, i)
			data, err := json.Marshal(rec)
			if err != nil {
				continue
			}
			items = append(items, data)
		}
		return nil
	}

	// FPI_Registration_Data.aspx is a plain Reports page (unaffected by the
	// StaticReports family's TLS-fingerprint bot mitigation), so the
	// generated client's plain GET is fine here.
	categoryBody, err := c.Get(ctx, "/web/Reports/FPI_Registration_Data.aspx", nil)
	if err != nil {
		return syncFail(resource, fmt.Errorf("fetching registration categories: %w", err), started)
	}
	if err := addRecords(categoryBody, "category"); err != nil {
		return syncFail(resource, fmt.Errorf("fetching registration categories: %w", err), started)
	}

	// DDP_Pendency_Report.htm lives under /web/StaticReports/, the same
	// family that rejects Go's stock net/http TLS fingerprint (see
	// browserclient.go) — route it through the Chrome-fingerprint fetch
	// like sector/trades, not the plain client.
	pendencyBody, err := nsdl.FetchStaticReport(ctx, c.RequestBaseURL(), "/web/StaticReports/DDP_Pendency_Report/DDP_Pendency_Report.htm")
	if err != nil {
		return syncFail(resource, fmt.Errorf("fetching DDP pendency: %w", err), started)
	}
	if err := addRecords(pendencyBody, "pendency"); err != nil {
		return syncFail(resource, fmt.Errorf("fetching DDP pendency: %w", err), started)
	}

	stored, _, err := db.UpsertBatch(resource, items)
	if err != nil {
		return syncFail(resource, err, started)
	}
	return syncOK(resource, stored, started)
}

// syncLimitsHandler fetches the per-ISIN foreign investment limit data.
// This is real structured JSON (not an HTML table), but the upstream
// endpoint is a POST with a fixed placeholder body, which the generic REST
// sync path (GET-only) cannot express — so it is synced here instead,
// reusing the generated typed UpsertLimits path.
func syncLimitsHandler(ctx context.Context, c nsdlSyncClient, db *store.Store, resource string, syncEvents io.Writer, started time.Time) syncResult {
	emitSyncStart(syncEvents, resource)
	cc, ok := c.(*client.Client)
	if !ok {
		return syncFail(resource, fmt.Errorf("limits sync requires a live HTTP client"), started)
	}
	data, _, err := cc.PostWithParams(ctx, "/web/Reports/DefaultAPI_Reports.aspx",
		map[string]string{"action": "GetFilmReport_Detail"},
		map[string]string{"uName": "s"})
	if err != nil {
		return syncFail(resource, err, started)
	}
	// The endpoint returns a JSON array of per-ISIN records; UpsertLimits
	// stores one object at a time, so each array element is upserted
	// individually.
	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err != nil {
		return syncFail(resource, fmt.Errorf("decoding limits response: %w", err), started)
	}
	stored := 0
	for _, item := range arr {
		// UpsertLimits requires an "id" key (checks id/Id/ID/uuid/slug/name)
		// and populates the typed "limits" table columns via a snake_case
		// lookup (film_isin, film_issuer, ...). The upstream payload uses
		// SCREAMING_SNAKE keys (FILM_ISIN, FILM_ISSUER, ...), which neither
		// lookup matches, so both an "id" and lowercased key copies are
		// added before storing.
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(item, &obj); err != nil {
			continue
		}
		isin := ""
		_ = json.Unmarshal(obj["FILM_ISIN"], &isin)
		if isin == "" {
			continue
		}
		lowered := make(map[string]json.RawMessage, len(obj)+1)
		for k, v := range obj {
			lowered[k] = v
			lowered[strings.ToLower(k)] = v
		}
		lowered["id"] = mustMarshal(isin)
		withID, err := json.Marshal(lowered)
		if err != nil {
			continue
		}
		if err := db.UpsertLimits(withID); err != nil {
			continue
		}
		stored++
	}
	return syncOK(resource, stored, started)
}

// recordKey pulls the first non-empty value from a set of candidate map
// keys, for building a stable id when a report's column naming may vary
// slightly (Sectors/Sector, Country/Category).
func recordKey(rec map[string]string, candidates ...string) string {
	for _, k := range candidates {
		if v, ok := rec[k]; ok && v != "" {
			return v
		}
	}
	return "unknown"
}

func mustMarshal(s string) json.RawMessage {
	data, _ := json.Marshal(s)
	return data
}
