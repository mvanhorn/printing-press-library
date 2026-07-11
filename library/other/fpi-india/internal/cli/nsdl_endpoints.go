package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/other/fpi-india/internal/nsdl"
)

// staticReportEndpointPaths lists the endpoints backed by NSDL's
// StaticReports path family, which rejects Go's stock net/http TLS
// fingerprint (see nsdl.browserFingerprintHTTPGet's doc comment). The
// generated command's own fetch (plain net/http) already ran by the time
// nsdlTransformHTMLEndpoint sees rawBody, and for these paths that fetch
// returns a WAF rejection page, not real content — so rawBody is discarded
// and the endpoint is re-fetched through the Chrome-fingerprint client
// instead of being parsed as-is.
var staticReportEndpointPaths = map[string]string{
	"trades.equity":     "/web/StaticReports/FIITradeWise2008/FIITradeWise2008.htm",
	"trades.debt":       "/web/StaticReports/FIITradeWiseDebt/FIITradeWiseDebt.htm",
	"registry.pendency": "/web/StaticReports/DDP_Pendency_Report/DDP_Pendency_Report.htm",
}

// nsdlTransformHTMLEndpoint replaces the generic html_extract page-metadata
// post-processing (title/description/links) with real structured records
// for the typed endpoint commands backed by an NSDL/CDSL report page. The
// generated command files call this before falling back to
// extractHTMLResponse, so a live "net-investment fy --json" (or any other
// covered endpoint) returns the same parsed rows sync stores, not raw page
// chrome.
//
// Returns (nil, false, nil) for endpoints with no bespoke parser, letting
// the caller fall through to the generic extractor on the original
// response body — that body is real content for those endpoints.
//
// Returns (nil, false, err) for a static-report-backed endpoint whose
// Chrome-fingerprint re-fetch failed. Callers MUST return err rather than
// falling through: the caller's original pre-fetch body for these
// endpoints is the WAF rejection page, and running it through the generic
// extractor would silently return parsed rejection-page metadata as if it
// were a real (if generic) result instead of surfacing the fetch failure.
func nsdlTransformHTMLEndpoint(ctx context.Context, endpoint, baseURL string, rawBody []byte, params map[string]string) (json.RawMessage, bool, error) {
	isStaticReport := false
	if path, ok := staticReportEndpointPaths[endpoint]; ok {
		isStaticReport = true
		fresh, err := nsdl.FetchStaticReport(ctx, baseURL, path)
		if err != nil {
			return nil, false, fmt.Errorf("fetching %s: %w", endpoint, err)
		}
		rawBody = fresh
	}
	// parseFail turns a parser error into the right return for this
	// endpoint: a hard error for a static-report-backed endpoint (rawBody
	// is the freshly re-fetched real content at this point, not the WAF
	// rejection page — a parse failure here means the page shape changed,
	// not "unsupported endpoint", and must not let the caller fall back
	// to the plain client's original rejected body), or the ordinary
	// "no bespoke parser" fallback signal otherwise.
	parseFail := func(err error) (json.RawMessage, bool, error) {
		if isStaticReport {
			return nil, false, fmt.Errorf("parsing %s: %w", endpoint, err)
		}
		return nil, false, nil
	}
	switch endpoint {
	case "net_investment.fy":
		currency := "INR"
		if params["CurrVal"] == "USD" {
			currency = "USD"
		}
		rows, err := nsdl.ParseNetInvestment(rawBody, "fy", currency)
		if err != nil {
			return parseFail(err)
		}
		return marshalOrFalse(rows)
	case "net_investment.cy":
		currency := "INR"
		if params["CurrVal"] == "USD" {
			currency = "USD"
		}
		periodType := "cy"
		if params["year"] != "" {
			periodType = "cy_monthly"
		}
		rows, err := nsdl.ParseNetInvestment(rawBody, periodType, currency)
		if err != nil {
			return parseFail(err)
		}
		return marshalOrFalse(rows)
	case "net_investment.quarterly", "net_investment.latest":
		recs, err := nsdl.ParseGenericRecords(rawBody)
		if err != nil {
			return parseFail(err)
		}
		return marshalOrFalse(recs)
	case "auc.country", "auc.category":
		recs, err := nsdl.ParseAUC(rawBody)
		if err != nil {
			return parseFail(err)
		}
		return marshalOrFalse(recs)
	case "trades.equity", "trades.debt":
		recs, err := nsdl.ParseTrades(rawBody)
		if err != nil {
			return parseFail(err)
		}
		return marshalOrFalse(recs)
	case "registry.list", "registry.categories", "registry.pendency":
		recs, err := nsdl.ParseRegistry(rawBody)
		if err != nil {
			return parseFail(err)
		}
		return marshalOrFalse(recs)
	case "sector.list":
		periods := nsdl.ParseSectorPeriods(rawBody)
		return marshalOrFalse(periods)
	default:
		return nil, false, nil
	}
}

func marshalOrFalse(v any) (json.RawMessage, bool, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, false, nil
	}
	return data, true, nil
}
