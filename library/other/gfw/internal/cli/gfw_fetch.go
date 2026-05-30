// Hand-authored — NOT generated. Shared GFW fetch + cache core for the novel
// due-diligence commands (vessel dossier/risk/ports/gaps, encounters network,
// watch refresh/since). Wraps the generated client with GFW dataset IDs and
// flattens the nested vessel identity for the local cache.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/gfw/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/gfw/internal/store"
)

// Canonical GFW v3 dataset IDs (extracted from the official v3 Postman collection).
const (
	dsVesselIdentity = "public-global-vessel-identity:latest"
	dsFishing        = "public-global-fishing-events:latest"
	dsEncounters     = "public-global-encounters-events:latest"
	dsLoitering      = "public-global-loitering-events:latest"
	dsPortVisits     = "public-global-port-visits-events:latest"
	dsGaps           = "public-global-gaps-events:latest"
)

// allEventDatasets is the set queried for a full behavioral picture (dossier).
var allEventDatasets = []string{dsEncounters, dsPortVisits, dsLoitering, dsGaps, dsFishing}

// vesselIdentity is the flattened identity cached under resource_type="vessel".
type vesselIdentity struct {
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	Flag      string `json:"flag,omitempty"`
	SSVID     string `json:"ssvid,omitempty"`
	IMO       string `json:"imo,omitempty"`
	CallSign  string `json:"call_sign,omitempty"`
	ShipType  string `json:"ship_type,omitempty"`
	SourceURL string `json:"source_url,omitempty"`
	FetchedAt string `json:"fetched_at,omitempty"`
}

func gfwVesselURL(id string) string {
	return "https://gateway.api.globalfishingwatch.org/v3/vessels/" + id
}

// fetchVesselByID GETs a single vessel by GFW id. The get-by-id endpoint takes
// a singular `dataset` string (not the `datasets[0]` array the search endpoint
// uses) and 422s on the array form.
func fetchVesselByID(ctx context.Context, c *client.Client, id string) (json.RawMessage, error) {
	return c.Get(ctx, "/v3/vessels/"+id, map[string]string{
		"dataset": dsVesselIdentity,
	})
}

// searchVessels GETs /v3/vessels/search with a free-text query (registry + AIS).
func searchVessels(ctx context.Context, c *client.Client, query string, limit int) (json.RawMessage, error) {
	if limit <= 0 {
		limit = 20
	}
	// GFW free-text search uses the `query` param (name/MMSI/IMO/callsign);
	// `where` is for structured field filters and rejects free text with 422.
	return c.Get(ctx, "/v3/vessels/search", map[string]string{
		"datasets[0]": dsVesselIdentity,
		"query":       query,
		"limit":       strconv.Itoa(limit),
	})
}

// fetchEvents GETs /v3/events for a vessel across the given event datasets.
// sinceISO (optional, YYYY-MM-DD or RFC3339) filters by start date.
func fetchEvents(ctx context.Context, c *client.Client, vesselID string, datasets []string, limit int, sinceISO string) (json.RawMessage, error) {
	if limit <= 0 {
		limit = 50
	}
	params := map[string]string{
		"vessels[0]": vesselID,
		"limit":      strconv.Itoa(limit),
		"offset":     "0", // GFW 422s on limit without offset
	}
	for i, ds := range datasets {
		params[fmt.Sprintf("datasets[%d]", i)] = ds
	}
	if sinceISO != "" {
		params["start-date"] = sinceISO
	}
	return c.Get(ctx, "/v3/events", params)
}

// fetchInsights POSTs /v3/insights/vessels for a vessel. Best-effort: callers
// treat a non-nil error as "insights unavailable" rather than fatal.
func fetchInsights(ctx context.Context, c *client.Client, vesselID string, includes []string, startDate, endDate string) (json.RawMessage, error) {
	if len(includes) == 0 {
		// Valid GFW insight types (others 422). VESSEL-IDENTITY-* cover flag
		// changes, IUU listing, and MOU (PSC) listing — all DD-relevant.
		includes = []string{"FISHING", "GAP", "COVERAGE", "VESSEL-IDENTITY-FLAG-CHANGES", "VESSEL-IDENTITY-IUU-VESSEL-LIST", "VESSEL-IDENTITY-MOU-LIST"}
	}
	body := map[string]any{
		"includes":  includes,
		"startDate": startDate,
		"endDate":   endDate,
		"vessels": []map[string]string{
			{"datasetId": dsVesselIdentity, "vesselId": vesselID},
		},
	}
	data, status, err := c.Post(ctx, "/v3/insights/vessels", body)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("insights returned HTTP %d", status)
	}
	return data, nil
}

// entriesOf extracts the `entries` array from a GFW list response. A response
// that carries an `entries` key is a list envelope — return it as-is, even when
// empty (total:0). Only when there is no `entries` key at all (a bare
// single-vessel get-by-id object) do we wrap the whole payload as one entry.
// (The earlier version's fallback fired on empty-entries list envelopes,
// fabricating a phantom entry from the envelope itself.)
func entriesOf(raw json.RawMessage) []json.RawMessage {
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return nil
	}
	if entriesRaw, ok := obj["entries"]; ok {
		var entries []json.RawMessage
		_ = json.Unmarshal(entriesRaw, &entries)
		return entries
	}
	if len(obj) > 0 {
		return []json.RawMessage{raw}
	}
	return nil
}

// extractVesselIdentity flattens a GFW vessel object (or list response) into a
// vesselIdentity, preferring selfReportedInfo then registryInfo. fallbackID is
// used when the payload carries no id (e.g. the requested id on a get-by-id).
func extractVesselIdentity(raw json.RawMessage, fallbackID string) vesselIdentity {
	id := vesselIdentity{ID: fallbackID, SourceURL: gfwVesselURL(fallbackID), FetchedAt: time.Now().UTC().Format(time.RFC3339)}
	entries := entriesOf(raw)
	if len(entries) == 0 {
		return id
	}
	var v map[string]any
	if json.Unmarshal(entries[0], &v) != nil {
		return id
	}
	// Prefer selfReportedInfo[0], then registryInfo[0].
	src := firstObjInList(v, "selfReportedInfo")
	if src == nil {
		src = firstObjInList(v, "registryInfo")
	}
	if src != nil {
		if s := mapStr(src, "id"); s != "" {
			id.ID = s
		}
		id.Name = mapStr(src, "shipname")
		id.Flag = mapStr(src, "flag")
		id.SSVID = mapStr(src, "ssvid")
		id.IMO = mapStr(src, "imo")
		id.CallSign = mapStr(src, "callsign")
	}
	// Ship type from combinedSourcesInfo[0].shiptypes if present.
	if cs := firstObjInList(v, "combinedSourcesInfo"); cs != nil {
		id.ShipType = firstStrInList(cs, "shiptypes")
		if id.ShipType == "" {
			id.ShipType = firstStrInList(cs, "geartypes")
		}
	}
	id.SourceURL = gfwVesselURL(id.ID)
	return id
}

// cacheVesselIdentity best-effort upserts a flattened vessel identity into the
// local store (resource_type="vessel", keyed by GFW vessel id). Skipped on
// --no-cache or empty id. A write failure is returned for the caller to warn.
func cacheVesselIdentity(ctx context.Context, flags *rootFlags, id vesselIdentity) error {
	if (flags != nil && flags.noCache) || strings.TrimSpace(id.ID) == "" {
		return nil
	}
	data, err := json.Marshal(id)
	if err != nil {
		return err
	}
	db, err := store.OpenWithContext(ctx, defaultDBPath("gfw-pp-cli"))
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Upsert("vessel", id.ID, data)
}

// --- small JSON traversal helpers (defensive; GFW nests deeply) ---

func mapStr(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		switch t := v.(type) {
		case string:
			return t
		case float64:
			return strconv.FormatFloat(t, 'f', -1, 64)
		case json.Number:
			return t.String()
		}
	}
	return ""
}

func firstObjInList(m map[string]any, key string) map[string]any {
	if lst, ok := m[key].([]any); ok && len(lst) > 0 {
		if obj, ok := lst[0].(map[string]any); ok {
			return obj
		}
	}
	return nil
}

func firstStrInList(m map[string]any, key string) string {
	if lst, ok := m[key].([]any); ok && len(lst) > 0 {
		if s, ok := lst[0].(string); ok {
			return s
		}
	}
	return ""
}

// defaultInsightDates returns a [start, end] YYYY-MM-DD window of the last year.
func defaultInsightDates() (string, string) {
	end := time.Now().UTC()
	start := end.AddDate(-1, 0, 0)
	return start.Format("2006-01-02"), end.Format("2006-01-02")
}
