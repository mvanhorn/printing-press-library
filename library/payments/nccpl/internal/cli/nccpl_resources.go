package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/nccpl/internal/client"
	"github.com/mvanhorn/printing-press-library/library/payments/nccpl/internal/store"
)

// NCCPL resource registry.
//
// Every fact in this file was read out of NCCPL's own page JavaScript (the inline
// script of GET /market-information) rather than inferred. Three details are load
// bearing and are NOT uniform across the API:
//
//  1. Date format differs per endpoint group. The fipi/lipi range endpoints convert
//     to DD/MM/YYYY via the site's own toApiDateFormat(); the sector-wise range
//     endpoints and all single-date endpoints send raw YYYY-MM-DD. Sending the wrong
//     one returns an empty array with HTTP 200 -- a silent-empty bug, not an error.
//
//  2. The response envelope key differs per endpoint. fipi-normal returns `records`
//     while lipi-normal returns `data`; the leverage endpoints use `positions`,
//     SLB uses `rows`, settlement uses `sett`, TFC uses `tfc`, VAR uses `margins`.
//     Assuming symmetry silently returns nothing for the odd ones out.
//
//  3. The flow rows carry no date of their own, so a {fromDate,toDate} call returns
//     one AGGREGATE over the window rather than per-session rows. Daily history
//     therefore requires fromDate == toDate per session. Sync always does that; the
//     generated `fipi data` / `lipi data` commands remain available for ad-hoc
//     period aggregates.

type nccplDateMode int

const (
	nccplSingleDate nccplDateMode = iota // body {date: YYYY-MM-DD}
	nccplRangeDMY                        // body {fromDate,toDate: DD/MM/YYYY}
	nccplRangeISO                        // body {fromDate,toDate: YYYY-MM-DD}
)

type nccplResource struct {
	Name     string        // store + CLI key
	Segment  string        // /api/<Segment>/data
	Mode     nccplDateMode // request encoding for this endpoint group
	Envelope string        // response key holding the row array
	KeyParts []string      // candidate fields composing a stable within-date row key
	Symbol   string        // field holding the instrument code, "" if not per-symbol
	External bool          // served by a non-NCCPL origin; `sync` skips it
}

var nccplResources = []nccplResource{
	{"fipi", "fipi", nccplRangeDMY, "data", []string{"client_type", "segment"}, "", false},
	{"lipi", "lipi", nccplRangeDMY, "data", []string{"client_type", "segment"}, "", false},
	{"fipi-sector", "fipi-sector-wise", nccplRangeISO, "data", []string{"SEC_CODE", "SECTOR_NAME", "CLIENT_TYPE"}, "", false},
	{"lipi-sector", "lipi-sector-wise", nccplRangeISO, "data", []string{"SEC_CODE", "SECTOR_NAME", "CLIENT_TYPE"}, "", false},
	{"fipi-normal", "fipi-normal", nccplSingleDate, "records", []string{"CLIENT_TYPE", "MARKET_TYPE"}, "", false},
	{"lipi-normal", "lipi-normal", nccplSingleDate, "data", []string{"CLIENT_TYPE", "MARKET_TYPE"}, "", false},
	{"mts", "open-positions", nccplSingleDate, "positions", []string{"symbol_code"}, "symbol_code", false},
	{"mts-financiers", "financiers-financees", nccplSingleDate, "records", []string{"symbol_code"}, "symbol_code", false},
	{"mts-force-release", "force-release", nccplSingleDate, "records", []string{"release_date"}, "", false},
	{"mts-top-financiers", "top-15-financiers", nccplSingleDate, "records", []string{"sr"}, "", false},
	{"mts-refinanced", "mts-amount-refinanced", nccplSingleDate, "records", []string{"date"}, "", false},
	{"mfs", "mfs-open-position", nccplSingleDate, "positions", []string{"symbol"}, "symbol", false},
	{"mfs-top", "mfs-top-15-financees-and-financiers", nccplSingleDate, "records", []string{"sr"}, "", false},
	{"msf", "msf-open-position", nccplSingleDate, "positions", []string{"symbol"}, "symbol", false},
	{"msf-top", "msf-top-15-financees-and-financiers", nccplSingleDate, "records", []string{"sr"}, "", false},
	{"slb", "slb-market-information", nccplSingleDate, "rows", []string{"symbol"}, "symbol", false},
	{"var-margins", "var-margins", nccplSingleDate, "margins", []string{"symbol"}, "symbol", false},
	{"settlement-uin", "sett-info-uin-wise", nccplSingleDate, "sett", []string{"symbol"}, "symbol", false},
	{"settlement-cm", "sett-info-cm-wise", nccplSingleDate, "sett", []string{"se", "symbol"}, "symbol", false},
	{"tfc", "un-listed-tfc", nccplSingleDate, "tfc", []string{"tfc_symbol", "settlement_date"}, "tfc_symbol", false},

	// Not an NCCPL origin. FIPI/LIPI sector flows republished by scstrade, reachable
	// over plain HTTPS with no clearance gate. Kept as a first-class resource so the
	// store-reading commands treat it like any other, while the distinct name keeps
	// its provenance explicit rather than silently blending it with NCCPL rows.
	{"flows", "", nccplRangeISO, "d", []string{"FLSectorName", "FLTypeNew"}, "", true},
}

func nccplResourceByName(name string) (nccplResource, bool) {
	for _, r := range nccplResources {
		if r.Name == name {
			return r, true
		}
	}
	return nccplResource{}, false
}

func nccplResourceNames() []string {
	out := make([]string, 0, len(nccplResources))
	for _, r := range nccplResources {
		out = append(out, r.Name)
	}
	return out
}

// nccplPerSymbolResources are the resources whose rows identify an instrument.
func nccplPerSymbolResources() []nccplResource {
	out := make([]nccplResource, 0)
	for _, r := range nccplResources {
		if r.Symbol != "" {
			out = append(out, r)
		}
	}
	return out
}

// nccplToDMY converts YYYY-MM-DD to DD/MM/YYYY, matching the site's toApiDateFormat().
func nccplToDMY(iso string) string {
	t, err := time.Parse("2006-01-02", iso)
	if err != nil {
		return iso
	}
	return t.Format("02/01/2006")
}

// nccplRequestBody builds the POST body for one settlement date.
//
// Range-mode resources are always called with fromDate == toDate, because their rows
// carry no date and a wider window would collapse into a single aggregate.
func nccplRequestBody(r nccplResource, date string) map[string]any {
	switch r.Mode {
	case nccplRangeDMY:
		d := nccplToDMY(date)
		return map[string]any{"fromDate": d, "toDate": d, "type": "101"}
	case nccplRangeISO:
		return map[string]any{"fromDate": date, "toDate": date}
	default:
		return map[string]any{"date": date}
	}
}

// nccplRowKey composes a stable within-date key from the resource's key fields.
// Falls back to the row's ordinal when no key field is present, and disambiguates
// collisions the same way, so an upsert never silently overwrites a sibling row.
func nccplRowKey(r nccplResource, row map[string]any, index int, seen map[string]bool) string {
	parts := make([]string, 0, len(r.KeyParts))
	for _, f := range r.KeyParts {
		if v, ok := row[f]; ok {
			s := strings.TrimSpace(fmt.Sprintf("%v", v))
			if s != "" && s != "<nil>" {
				parts = append(parts, s)
			}
		}
	}
	key := strings.Join(parts, "|")
	if key == "" {
		key = fmt.Sprintf("#%d", index)
	}
	if seen[key] {
		key = fmt.Sprintf("%s#%d", key, index)
	}
	seen[key] = true
	return key
}

// nccplSymbolOf extracts the instrument code from a stored row, "" when absent.
func nccplSymbolOf(r nccplResource, row map[string]any) string {
	if r.Symbol == "" {
		return ""
	}
	if v, ok := row[r.Symbol]; ok {
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
	return ""
}

// nccplLatestDate resolves a resource's own most recent published date.
func nccplLatestDate(ctx context.Context, c *client.Client, r nccplResource) (string, error) {
	raw, err := c.Get(ctx, "/api/"+r.Segment+"/latest-date", nil)
	if err != nil {
		return "", fmt.Errorf("%s latest-date: %w", r.Name, err)
	}
	var resp struct {
		Success    bool   `json:"success"`
		LatestDate string `json:"latest_date"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", fmt.Errorf("%s latest-date decode: %w", r.Name, err)
	}
	if resp.LatestDate == "" {
		return "", fmt.Errorf("%s latest-date: empty response", r.Name)
	}
	return resp.LatestDate, nil
}

// nccplFetchDate POSTs one settlement date and returns its rows plus the decoded
// objects. An empty slice with a nil error is a legitimate "published nothing that
// day" result and must be recorded as coverage, not discarded as a failure.
func nccplFetchDate(ctx context.Context, c *client.Client, r nccplResource, date string) ([]store.NCCPLRow, []map[string]any, error) {
	raw, status, err := c.Post(ctx, "/api/"+r.Segment+"/data", nccplRequestBody(r, date))
	if err != nil {
		return nil, nil, fmt.Errorf("%s %s: %w", r.Name, date, err)
	}
	if status >= 400 {
		return nil, nil, fmt.Errorf("%s %s: HTTP %d", r.Name, date, status)
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, nil, fmt.Errorf("%s %s: decode envelope: %w", r.Name, date, err)
	}
	payload, ok := envelope[r.Envelope]
	if !ok {
		// Naming the expected key matters: the envelope key is not uniform across
		// this API, so a missing key is far more likely to be drift than no data.
		available := make([]string, 0, len(envelope))
		for k := range envelope {
			available = append(available, k)
		}
		sort.Strings(available)
		return nil, nil, fmt.Errorf("%s %s: response has no %q key (got: %s)",
			r.Name, date, r.Envelope, strings.Join(available, ", "))
	}

	var objs []map[string]any
	if err := json.Unmarshal(payload, &objs); err != nil {
		return nil, nil, fmt.Errorf("%s %s: decode %s: %w", r.Name, date, r.Envelope, err)
	}

	rows := make([]store.NCCPLRow, 0, len(objs))
	seen := map[string]bool{}
	for i, o := range objs {
		encoded, err := json.Marshal(o)
		if err != nil {
			return nil, nil, fmt.Errorf("%s %s: encode row %d: %w", r.Name, date, i, err)
		}
		rows = append(rows, store.NCCPLRow{Key: nccplRowKey(r, o, i, seen), Payload: string(encoded)})
	}
	return rows, objs, nil
}

// nccplSessionDates lists candidate settlement dates in [from, to], skipping weekends.
// Pakistani market holidays are not enumerated: a holiday simply returns zero rows and
// is recorded as fetched-and-empty, which is the honest representation.
func nccplSessionDates(from, to string) ([]string, error) {
	start, err := time.Parse("2006-01-02", from)
	if err != nil {
		return nil, fmt.Errorf("invalid --from %q: want YYYY-MM-DD", from)
	}
	end, err := time.Parse("2006-01-02", to)
	if err != nil {
		return nil, fmt.Errorf("invalid --to %q: want YYYY-MM-DD", to)
	}
	if end.Before(start) {
		return nil, fmt.Errorf("--from %s is after --to %s", from, to)
	}
	out := make([]string, 0)
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
			continue
		}
		out = append(out, d.Format("2006-01-02"))
	}
	return out, nil
}

// nccplDecodePayload parses a stored row payload back into an object.
func nccplDecodePayload(payload string) (map[string]any, error) {
	var m map[string]any
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		return nil, err
	}
	return m, nil
}
