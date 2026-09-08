// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/payments/mufap/internal/client"
	"github.com/mvanhorn/printing-press-library/library/payments/mufap/internal/mufap"
)

// MUFAP fetch helpers.
//
// This file is the single place that knows MUFAP's request shapes, so the
// novel commands never re-derive them. Two facts make that worth centralising:
//
//  1. MUFAP uses FOUR different date encodings across its surfaces
//     (YYYY-MM-DD in the HTML query string, "Mon DD, YYYY" in the rendered
//     Validity Date column, M-YYYY on the allocation endpoint, YYYY on the
//     unit-holder endpoint). Sending the wrong one returns HTTP 200 with an
//     empty table, or HTTP 500 -- never a parameter error.
//  2. The JSON envelope's `message` field reads "No data found" even on fully
//     populated responses, so no caller may gate on it.
//
// Kept in a separate hand-authored file so `generate --force` preserves it.

// mufapDailyTabs maps a friendly tab name to MUFAP's numeric tab parameter.
var mufapDailyTabs = map[string]int{
	"returns": 1,
	"nav":     2,
	"pricing": 3,
	"payout":  4,
	"ter":     5,
}

// MUFAPDailyTabNames lists the selectable daily tabs in a stable order.
var MUFAPDailyTabNames = []string{"returns", "nav", "pricing", "payout", "ter"}

// mufapResourceForTab is the store resource key for a daily tab.
func mufapResourceForTab(tab string) string { return "daily-" + tab }

// mufapFetchHTML performs a GET whose response is an HTML document.
//
// The generated client rejects HTML by default (it expects JSON and reports
// "API returned HTML instead of JSON"), so the HTML-response header is set to
// opt this request out of that check.
func mufapFetchHTML(ctx context.Context, c *client.Client, path string, params map[string]string) (string, error) {
	raw, err := c.GetWithHeadersNoCache(ctx, path, params, map[string]string{
		client.HTMLResponseHeader: "true",
	})
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// mufapFetchDaily returns the parsed daily panel for one tab and one date.
//
// An explicit date returns a clean single-validity-date panel. Omitting the
// date returns MUFAP's "latest available per fund" view, which mixes many
// validity dates and must not be treated as one observation.
func mufapFetchDaily(ctx context.Context, c *client.Client, tab, date string) (*mufap.Table, error) {
	n, ok := mufapDailyTabs[tab]
	if !ok {
		return nil, fmt.Errorf("unknown tab %q: want one of %s", tab, strings.Join(MUFAPDailyTabNames, ", "))
	}
	params := map[string]string{"tab": strconv.Itoa(n)}
	if date != "" {
		params["AMCId"] = "0"
		params["fundId"] = "0"
		params["datefrom"] = date
		params["datetill"] = date
	}
	doc, err := mufapFetchHTML(ctx, c, "/Industry/IndustryStatDaily", params)
	if err != nil {
		return nil, fmt.Errorf("daily %s %s: %w", tab, date, err)
	}
	t, err := mufap.ParseTable(doc)
	if err != nil {
		return nil, fmt.Errorf("daily %s %s: %w", tab, date, err)
	}
	return t, nil
}

// mufapFetchMonthly returns the parsed monthly net-assets panel for one date.
//
// The visible control on this page is an <input type="month" id="filterDate">,
// but filterDate is NOT the query parameter: passing it returns HTTP 200 with
// a byte-identical snapshot for every month. The real parameters are
// datefrom/datetill, read from the page's own window.location.assign call.
func mufapFetchMonthly(ctx context.Context, c *client.Client, date string) (*mufap.Table, error) {
	params := map[string]string{"tab": "1"}
	if date != "" {
		params["AMCId"] = "0"
		params["fundId"] = "0"
		params["datefrom"] = date
		params["datetill"] = date
	}
	doc, err := mufapFetchHTML(ctx, c, "/Industry/IndustryStatMonthly", params)
	if err != nil {
		return nil, fmt.Errorf("monthly %s: %w", date, err)
	}
	t, err := mufap.ParseTable(doc)
	if err != nil {
		return nil, fmt.Errorf("monthly %s: %w", date, err)
	}
	return t, nil
}

// mufapEnvelope is MUFAP's JSON wrapper.
//
// Message is captured but deliberately never used as a success test: it reads
// "No data found" on fully populated responses.
type mufapEnvelope struct {
	StatusCode string          `json:"statusCode"`
	Message    string          `json:"message"`
	Data       json.RawMessage `json:"data"`
}

// mufapPostJSON posts a plain JSON body and returns the unwrapped data field.
//
// Payload encryption is disabled on MUFAP's public pages
// (API_PAYLOAD_ENCRYPTION_ENABLED = 'false'), so the documented plain-JSON
// fallback applies and no AES envelope is constructed.
func mufapPostJSON(ctx context.Context, c *client.Client, path string, body any) (json.RawMessage, error) {
	raw, _, err := c.Post(ctx, path, body)
	if err != nil {
		return nil, err
	}
	var env mufapEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("%s: decoding envelope: %w", path, err)
	}
	return env.Data, nil
}

// MUFAPAMc is one asset management company.
type MUFAPAMc struct {
	AMCId   string `json:"AMCId"`
	AMCDesc string `json:"AMC_Desc"`
	AMC     int    `json:"AMC"`
}

// Name returns the AMC's display name with MUFAP's column padding removed.
func (a MUFAPAMc) Name() string { return strings.TrimSpace(a.AMCDesc) }

// mufapFetchAMCs lists every AMC. AMCId is a GUID, not an integer.
func mufapFetchAMCs(ctx context.Context, c *client.Client) ([]MUFAPAMc, error) {
	data, err := mufapPostJSON(ctx, c, "/AMC/GetAMCList", map[string]any{})
	if err != nil {
		return nil, fmt.Errorf("amc list: %w", err)
	}
	out := make([]MUFAPAMc, 0)
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("amc list: decoding: %w", err)
	}
	return out, nil
}

// MUFAPFund is one fund under an AMC.
//
// Both identifiers are kept because MUFAP requires different ones in
// different places: FundID (a GUID) identifies the fund in listings, while the
// allocation endpoint requires the integer Fund code and returns HTTP 500 for
// the GUID.
type MUFAPFund struct {
	FundID      string `json:"FundID"`
	Fund        int    `json:"fund"`
	FundDesc    string `json:"Fund_Desc"`
	CatID       int    `json:"CatId"`
	CatDesc     string `json:"Cat_Desc"`
	SubCatName  string `json:"SubCategoryName"`
	Pricing     string `json:"PricingMechanism"`
	IsMatured   any    `json:"IsMatured"`
	FundTypeRaw int    `json:"fund_type"`
}

// Name returns the fund's display name with MUFAP's column padding removed.
func (f MUFAPFund) Name() string { return strings.TrimSpace(f.FundDesc) }

// Category returns the fund's category with padding removed.
func (f MUFAPFund) Category() string { return strings.TrimSpace(f.CatDesc) }

// mufapFetchFunds lists the funds managed by one AMC.
func mufapFetchFunds(ctx context.Context, c *client.Client, amcID string) ([]MUFAPFund, error) {
	data, err := mufapPostJSON(ctx, c, "/TopHolding/GetFundNameByAMC", map[string]any{"AMCId": amcID})
	if err != nil {
		return nil, fmt.Errorf("funds for AMC %s: %w", amcID, err)
	}
	out := make([]MUFAPFund, 0)
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("funds for AMC %s: decoding: %w", amcID, err)
	}
	return out, nil
}

// mufapFetchAllocation returns one fund's asset allocation for one month.
//
// month must already be in M-YYYY form (use mufap.MonthKey). fundCode must be
// the integer fund code; the GUID returns HTTP 500 and a non-numeric value
// returns HTTP 200 with an empty table, so both mistakes fail differently and
// neither surfaces as a parameter error.
//
// The data field is a JSON *string* containing {"Table":[...]}, so it needs a
// second decode.
func mufapFetchAllocation(ctx context.Context, c *client.Client, fundCode int, month string) (map[string]any, error) {
	data, err := mufapPostJSON(ctx, c, "/Industry/GetAssetAllDetailbyId", map[string]any{
		"ID":   strconv.Itoa(fundCode),
		"Date": month,
	})
	if err != nil {
		return nil, fmt.Errorf("allocation %d %s: %w", fundCode, month, err)
	}
	var inner string
	if err := json.Unmarshal(data, &inner); err != nil {
		return nil, fmt.Errorf("allocation %d %s: data was not the expected JSON string: %w", fundCode, month, err)
	}
	var wrapper struct {
		Table []map[string]any `json:"Table"`
	}
	if err := json.Unmarshal([]byte(inner), &wrapper); err != nil {
		return nil, fmt.Errorf("allocation %d %s: decoding inner table: %w", fundCode, month, err)
	}
	if len(wrapper.Table) == 0 {
		// A genuinely absent fund-month, not an error: MUFAP predates
		// allocation reporting for many funds. The caller records it as a
		// zero-row observation rather than a failed fetch.
		return nil, nil
	}
	return wrapper.Table[0], nil
}

// mufapAllocNumber reads a numeric field from an allocation row.
//
// MUFAP returns these as JSON numbers, as quoted strings, or as null
// depending on the field and the vintage, so a plain float64 type assertion
// silently drops most historical rows.
func mufapAllocNumber(row map[string]any, key string) (float64, bool) {
	v, ok := row[key]
	if !ok || v == nil {
		return 0, false
	}
	switch t := v.(type) {
	case float64:
		return t, true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	case string:
		return mufap.ParseNumber(t)
	}
	return 0, false
}

// mufapAllocString reads a string field from an allocation row, trimming
// MUFAP's fixed-width column padding.
func mufapAllocString(row map[string]any, key string) string {
	v, ok := row[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	}
	return fmt.Sprint(v)
}

// --- Row identity ---------------------------------------------------------
//
// Fund name is NOT unique within a date. Measured on 2026-09-04, tab=returns:
// 388 rows collapse to 339 distinct fund names -- 49 collisions, 12.6%. Two
// causes, and only one of them is an upstream defect:
//
//   1. VPS pension funds legitimately reuse one name across sub-funds.
//      "ABL Pension Fund" is three genuinely different series -- VPS-Money
//      Market (NAV 293.7521), VPS-Debt (388.4842) and VPS-Equity (647.2226).
//   2. One true duplicate: "Pak Qatar Daily Dividend Plan" appears twice with
//      identical sector, category, rating and NAV.
//
// Sector|Category|Fund Name leaves exactly one collision out of 388, so it is
// the key; the residual duplicate is disambiguated by occurrence. Keying on
// the bare name would silently drop 49 rows per date through the
// PRIMARY KEY (resource, date, row_key) upsert, with no error anywhere.

// mufapNameColumns are the fund-identifying headers, in probe order. MUFAP
// labels the column "Fund Name" on tab=returns and plain "Fund" on nav,
// pricing, payout and ter, so neither name alone covers all five tabs.
var mufapNameColumns = []string{"Fund Name", "Fund"}

// MUFAPRowName returns the fund name of a parsed row, whichever header the
// tab happens to use. Empty when the row carries neither column.
func MUFAPRowName(row map[string]string) string {
	for _, h := range mufapNameColumns {
		if v := strings.TrimSpace(row[h]); v != "" {
			return v
		}
	}
	return ""
}

// MUFAPRowKey builds the stable within-date identity for a row:
// Sector|Category|Fund Name. Absent components are elided rather than
// substituted, so a tab that omits one still yields a deterministic key.
func MUFAPRowKey(row map[string]string) string {
	parts := make([]string, 0, 3)
	for _, h := range []string{"Sector", "Category"} {
		if v := strings.TrimSpace(row[h]); v != "" {
			parts = append(parts, v)
		}
	}
	if n := MUFAPRowName(row); n != "" {
		parts = append(parts, n)
	}
	return strings.Join(parts, "|")
}

// MUFAPValidityColumn returns the header carrying each row's OWN observation
// date.
//
// tab=payout has NO "Validity Date" column at all -- its date column is
// "Payout Date". Code that looks only for "Validity Date" stores nothing for
// that tab and reports it as an upstream page change, so both spellings are
// probed here once rather than special-cased at each call site.
func MUFAPValidityColumn(headers []string) string {
	for _, want := range []string{"Validity Date", "Payout Date"} {
		for _, h := range headers {
			if strings.EqualFold(strings.TrimSpace(h), want) {
				return h
			}
		}
	}
	return ""
}

// mufapDateFilteredTabs records which daily tabs actually honour the
// datefrom/datetill query parameters.
//
// Measured on 2026-09-04: tab=returns returned 388 rows for the explicit date
// while tab=pricing and tab=ter both returned 551. Those two are current
// reference data, not a dated panel, so their row counts must never be read
// as a universe width for the requested date.
var mufapDateFilteredTabs = map[string]bool{
	"returns": true,
	"nav":     true,
	"payout":  true,
	"pricing": false,
	"ter":     false,
}

// MUFAPTabIsDateFiltered reports whether a tab's rows belong to the requested
// date. Unknown tabs are treated as dated, matching the default expectation.
func MUFAPTabIsDateFiltered(tab string) bool {
	filtered, ok := mufapDateFilteredTabs[tab]
	return !ok || filtered
}
