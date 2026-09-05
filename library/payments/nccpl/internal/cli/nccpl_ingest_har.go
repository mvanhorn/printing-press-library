package cli

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
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
//
// The second return value lists the entries that looked like NCCPL data and were
// refused anyway, each with the reason. They are reported rather than dropped: a
// capture that silently ingested nothing looks exactly like a day the market
// published nothing, and only one of those is something the operator can fix.
func nccplBatchesFromHAR(raw []byte) ([]nccplIngestBatch, []string, error) {
	var har harFile
	if err := json.Unmarshal(raw, &har); err != nil {
		return nil, nil, fmt.Errorf("not a valid HAR: %w", err)
	}
	out := make([]nccplIngestBatch, 0)
	skipped := make([]string, 0)
	seen := make(map[string]bool)
	note := func(format string, args ...any) {
		msg := fmt.Sprintf(format, args...)
		if seen[msg] {
			return
		}
		seen[msg] = true
		skipped = append(skipped, msg)
	}

	for _, e := range har.Log.Entries {
		res, refusal, ok := nccplResourceForURL(e.Request.URL)
		if !ok {
			// refusal is empty for the ordinary unrelated traffic that fills a
			// HAR (images, scripts, analytics); those stay silent.
			if refusal != "" {
				note("%s", refusal)
			}
			continue
		}
		label := nccplCapturedURLLabel(e.Request.URL)
		if e.Response.Status != 200 {
			note("%s: HTTP %d, so it carries no data to store", label, e.Response.Status)
			continue
		}
		date := nccplDateFromRequestBody(e.Request.PostData.Text, res)
		if date == "" {
			note("%s: request body names no settlement date, so its rows cannot be dated", label)
			continue
		}
		body := []byte(e.Response.Content.Text)
		if strings.EqualFold(e.Response.Content.Encoding, "base64") {
			decoded, err := base64.StdEncoding.DecodeString(e.Response.Content.Text)
			if err != nil {
				note("%s (%s): response body is not decodable base64", label, date)
				continue
			}
			body = decoded
		}
		rows, err := nccplRowsFromEnvelope(body, res)
		if err != nil {
			note("%s (%s): %v", label, date, err)
			continue
		}
		if len(rows) == 0 {
			// A genuinely empty publication, not a refusal.
			continue
		}
		out = append(out, nccplIngestBatch{Resource: res.Name, Date: date, Rows: rows})
	}
	return out, skipped, nil
}

// nccplOriginDomain is NCCPL's registrable domain. Only this domain and its
// subdomains are treated as NCCPL speaking.
const nccplOriginDomain = "nccpl.com.pk"

// nccplResourceForURL maps a captured URL back to a registry resource.
//
// Three results, distinguished by the second and third return values:
//   - (resource, "", true)  the entry is NCCPL data for that resource;
//   - (zero, reason, false) it named a known data endpoint but did not come from
//     NCCPL over https, so it is refused with a reason the caller reports;
//   - (zero, "", false)     ordinary unrelated traffic, silently ignored.
//
// The path shape alone is not evidence of provenance. "Save all as HAR with content"
// exports EVERY request the browser made, so a capture routinely holds other origins
// -- a corporate proxy, a mock, a local dev server at
// http://localhost:3000/api/var-margins/data. Matching on the URL as a string filed
// all of those as authoritative NCCPL market data. So the URL is parsed and the host
// is compared against the registrable domain: a substring test would accept both
// "nccpl.com.pk.evil.com" and "notnccpl.com.pk", neither of which is NCCPL.
func nccplResourceForURL(rawURL string) (nccplResource, string, bool) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Host == "" {
		return nccplResource{}, "", false
	}
	res, ok := nccplResourceForAPIPath(u.Path)
	if !ok {
		return nccplResource{}, "", false
	}
	label := nccplCapturedURLLabel(rawURL)
	if !strings.EqualFold(u.Scheme, "https") {
		return nccplResource{}, fmt.Sprintf(
			"%s: ignored, NCCPL data is only accepted over https (this entry used %q)", label, u.Scheme), false
	}
	if !nccplIsNCCPLHost(u.Hostname()) {
		return nccplResource{}, fmt.Sprintf(
			"%s: ignored, host %q is not %s or a subdomain of it", label, u.Hostname(), nccplOriginDomain), false
	}
	return res, "", true
}

// nccplResourceForAPIPath matches a URL path against the registry's data endpoints.
// The comparison is exact rather than a substring, so "/api/var-margins/data-mock"
// and "/proxy/api/var-margins/data" are not this API.
func nccplResourceForAPIPath(path string) (nccplResource, bool) {
	p := strings.TrimSuffix(path, "/")
	for _, r := range nccplResources {
		// `flows` is republished by a different origin and carries no NCCPL
		// segment, so it can never be recognised from a captured NCCPL URL.
		if r.External || r.Segment == "" {
			continue
		}
		if p == "/api/"+r.Segment+"/data" {
			return r, true
		}
	}
	return nccplResource{}, false
}

// nccplIsNCCPLHost reports whether a hostname is NCCPL's own: the registrable domain
// itself, or a label-aligned subdomain of it. Matching on the dot boundary is what
// rejects the lookalikes a Contains check lets through.
func nccplIsNCCPLHost(host string) bool {
	h := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	return h == nccplOriginDomain || strings.HasSuffix(h, "."+nccplOriginDomain)
}

// nccplCapturedURLLabel renders a captured URL for a skip message: scheme, host and
// path only. The query is dropped so a capture's session tokens never reach a report.
func nccplCapturedURLLabel(rawURL string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Host == "" {
		return "(unparseable URL)"
	}
	return u.Scheme + "://" + u.Host + u.Path
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
