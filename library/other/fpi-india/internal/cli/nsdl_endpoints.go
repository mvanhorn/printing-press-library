package cli

import (
	"context"
	"encoding/json"

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
// chrome. Returns ok=false for endpoints with no bespoke parser, letting the
// caller fall through to the generic extractor.
func nsdlTransformHTMLEndpoint(ctx context.Context, endpoint, baseURL string, rawBody []byte, params map[string]string) (json.RawMessage, bool) {
	if path, ok := staticReportEndpointPaths[endpoint]; ok {
		fresh, err := nsdl.FetchStaticReport(ctx, baseURL, path)
		if err != nil {
			// The generated command's own pre-fetch (rawBody) is the WAF
			// rejection page for this StaticReports-backed endpoint, not
			// real content — parsing it would silently return an empty or
			// garbage result instead of surfacing the fetch failure. Fail
			// closed rather than falling through to parse a rejection page.
			return nil, false
		}
		rawBody = fresh
	}
	switch endpoint {
	case "net_investment.fy":
		currency := "INR"
		if params["CurrVal"] == "USD" {
			currency = "USD"
		}
		rows, err := nsdl.ParseNetInvestment(rawBody, "fy", currency)
		if err != nil {
			return nil, false
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
			return nil, false
		}
		return marshalOrFalse(rows)
	case "net_investment.quarterly", "net_investment.latest":
		recs, err := nsdl.ParseGenericRecords(rawBody)
		if err != nil {
			return nil, false
		}
		return marshalOrFalse(recs)
	case "auc.country", "auc.category":
		recs, err := nsdl.ParseAUC(rawBody)
		if err != nil {
			return nil, false
		}
		return marshalOrFalse(recs)
	case "trades.equity", "trades.debt":
		recs, err := nsdl.ParseTrades(rawBody)
		if err != nil {
			return nil, false
		}
		return marshalOrFalse(recs)
	case "registry.list", "registry.categories", "registry.pendency":
		recs, err := nsdl.ParseRegistry(rawBody)
		if err != nil {
			return nil, false
		}
		return marshalOrFalse(recs)
	case "sector.list":
		periods := nsdl.ParseSectorPeriods(rawBody)
		return marshalOrFalse(periods)
	default:
		return nil, false
	}
}

func marshalOrFalse(v any) (json.RawMessage, bool) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, false
	}
	return data, true
}
