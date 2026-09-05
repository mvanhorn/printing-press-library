package cli

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/nccpl/internal/store"
)

// HAR parsing for browser-captured NCCPL responses.
//
// A DevTools export records both the request body (which carries the settlement date
// the page asked for) and the response body (the rows). Pairing them lets an ingest
// file the rows under the right resource and date with no operator input.

type harFile struct {
	Log struct {
		Entries []harEntry `json:"entries"`
	} `json:"log"`
}

type harEntry struct {
	Request struct {
		Method   string `json:"method"`
		URL      string `json:"url"`
		PostData struct {
			Text string `json:"text"`
		} `json:"postData"`
	} `json:"request"`
	Response struct {
		Status  int `json:"status"`
		Content struct {
			Text     string `json:"text"`
			Encoding string `json:"encoding"`
		} `json:"content"`
	} `json:"response"`
}

// nccplBatchesFromHAR extracts every recognisable /api/<resource>/data exchange.
func nccplBatchesFromHAR(raw []byte) ([]nccplIngestBatch, error) {
	var har harFile
	if err := json.Unmarshal(raw, &har); err != nil {
		return nil, fmt.Errorf("not a valid HAR: %w", err)
	}
	out := make([]nccplIngestBatch, 0)
	for _, e := range har.Log.Entries {
		res, ok := nccplResourceForURL(e.Request.URL)
		if !ok || e.Response.Status != 200 {
			continue
		}
		date := nccplDateFromRequestBody(e.Request.PostData.Text, res)
		if date == "" {
			continue
		}
		body := []byte(e.Response.Content.Text)
		if strings.EqualFold(e.Response.Content.Encoding, "base64") {
			decoded, err := base64.StdEncoding.DecodeString(e.Response.Content.Text)
			if err != nil {
				continue
			}
			body = decoded
		}
		rows, err := nccplRowsFromEnvelope(body, res)
		if err != nil || len(rows) == 0 {
			continue
		}
		out = append(out, nccplIngestBatch{Resource: res.Name, Date: date, Rows: rows})
	}
	return out, nil
}

// nccplResourceForURL maps a captured URL back to a registry resource.
func nccplResourceForURL(url string) (nccplResource, bool) {
	if !strings.Contains(url, "/api/") || !strings.HasSuffix(strings.SplitN(url, "?", 2)[0], "/data") {
		return nccplResource{}, false
	}
	for _, r := range nccplResources {
		if strings.Contains(url, "/api/"+r.Segment+"/data") {
			return r, true
		}
	}
	return nccplResource{}, false
}

// nccplDateFromRequestBody recovers the settlement date the page requested.
//
// All three of this API's date encodings have to be handled in reverse: the single-date
// endpoints send YYYY-MM-DD in `date`, fipi/lipi send DD/MM/YYYY in `fromDate`, and the
// sector-wise endpoints send YYYY-MM-DD in `fromDate`. Whatever the wire format, the
// store always holds YYYY-MM-DD.
func nccplDateFromRequestBody(body string, res nccplResource) string {
	if strings.TrimSpace(body) == "" {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		return ""
	}
	pick := func(keys ...string) string {
		for _, k := range keys {
			if v, ok := m[k]; ok {
				if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
					return strings.TrimSpace(s)
				}
			}
		}
		return ""
	}
	raw := pick("date", "fromDate", "start_date")
	if raw == "" {
		return ""
	}
	return nccplNormalizeCapturedDate(raw)
}

// nccplNormalizeCapturedDate converts either wire format to the stored YYYY-MM-DD.
func nccplNormalizeCapturedDate(raw string) string {
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		return t.Format("2006-01-02")
	}
	if t, err := time.Parse("02/01/2006", raw); err == nil {
		return t.Format("2006-01-02")
	}
	return ""
}

var _ = store.NCCPLRow{}
