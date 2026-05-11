// Copyright 2026 chris-drit. Licensed under Apache-2.0. See LICENSE.
// Hand-authored — extends the generated CLI with SEC-specific helpers
// used by novel-feature commands. Preserved across `printing-press generate
// --force`.

package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// padCIK normalizes an incoming CIK string to 10 digits, zero-padded.
// Accepts: "320193", "0000320193", "CIK0000320193", "cik320193".
func padCIK(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(strings.ToUpper(s), "CIK")
	s = strings.TrimLeft(s, "0")
	if s == "" {
		s = "0"
	}
	if _, err := strconv.Atoi(s); err != nil {
		return "", fmt.Errorf("CIK must be numeric, got %q", raw)
	}
	if len(s) > 10 {
		return "", fmt.Errorf("CIK %q has more than 10 digits after zero-strip", raw)
	}
	return strings.Repeat("0", 10-len(s)) + s, nil
}

// parseSince parses a relative duration string ("7d", "30d", "24h", "last friday")
// into an absolute time (now-d). For simple Nd / Nh / Nm forms this is exact;
// for "last friday" we compute the previous Friday at 00:00 UTC.
func parseSince(raw string, now time.Time) (time.Time, error) {
	s := strings.TrimSpace(strings.ToLower(raw))
	if s == "" {
		return time.Time{}, fmt.Errorf("empty since value")
	}

	if s == "last friday" {
		offset := int(now.Weekday()) - int(time.Friday)
		if offset <= 0 {
			offset += 7
		}
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).
			AddDate(0, 0, -offset), nil
	}

	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.UTC(), nil
	}

	re := regexp.MustCompile(`^(\d+)([dhm])$`)
	m := re.FindStringSubmatch(s)
	if m == nil {
		return time.Time{}, fmt.Errorf("unrecognized --since value %q; use Nd / Nh / Nm or YYYY-MM-DD or 'last friday'", raw)
	}
	n, _ := strconv.Atoi(m[1])
	var d time.Duration
	switch m[2] {
	case "d":
		d = time.Duration(n) * 24 * time.Hour
	case "h":
		d = time.Duration(n) * time.Hour
	case "m":
		d = time.Duration(n) * time.Minute
	}
	return now.Add(-d), nil
}

// parseCSV returns a non-empty trimmed list of comma-separated values, dropping
// empties.
func parseCSV(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

// fetchSECJSON does a GET against an absolute URL and decodes JSON. Uses the
// generated client so the User-Agent header (auth.in=header) is sent.
func fetchSECJSON(c clientLike, absURL string, out any) error {
	raw, err := c.Get(absURL, nil)
	if err != nil {
		return err
	}
	if len(raw) == 0 {
		return fmt.Errorf("empty response from %s", absURL)
	}
	return json.Unmarshal(raw, out)
}

// fetchSECRaw does a GET and returns the raw body without JSON decoding (for
// Atom XML, HTML, etc.).
func fetchSECRaw(c clientLike, absURL string) ([]byte, error) {
	raw, err := c.Get(absURL, nil)
	if err != nil {
		return nil, err
	}
	return []byte(raw), nil
}

// clientLike captures only the GET surface we need from *client.Client so
// helpers stay testable.
type clientLike interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
}

// efts URL builder. Returns an absolute https URL pointing at
// efts.sec.gov/LATEST/search-index with the provided EFTS query parameters.
type EFTSQuery struct {
	Q     string   // free-text query
	Forms []string // form types (10-K, 4, NT 10-K, ...)
	CIKs  []string // 10-digit zero-padded CIKs
	Start string   // YYYY-MM-DD (date filed start)
	End   string   // YYYY-MM-DD (date filed end)
	From  int      // pagination offset
}

func (q EFTSQuery) URL() string {
	v := url.Values{}
	if q.Q != "" {
		v.Set("q", q.Q)
	}
	if len(q.Forms) > 0 {
		v.Set("forms", strings.Join(q.Forms, ","))
	}
	if len(q.CIKs) > 0 {
		// EFTS accepts CIKs as zero-padded 10-digit values (no "CIK" prefix).
		v.Set("ciks", strings.Join(q.CIKs, ","))
	}
	if q.Start != "" && q.End != "" {
		v.Set("dateRange", "custom")
		v.Set("startdt", q.Start)
		v.Set("enddt", q.End)
	}
	if q.From > 0 {
		v.Set("from", strconv.Itoa(q.From))
	}
	return "https://efts.sec.gov/LATEST/search-index?" + v.Encode()
}

// EFTSHit is a flattened view of one filing in an EFTS response.
type EFTSHit struct {
	Accession    string   `json:"accession"`
	Form         string   `json:"form"`
	FileDate     string   `json:"file_date"`
	PeriodEnding string   `json:"period_ending,omitempty"`
	CIKs         []string `json:"ciks"`
	DisplayNames []string `json:"display_names"`
	SICs         []string `json:"sics,omitempty"`
	BizStates    []string `json:"biz_states,omitempty"`
	Items        []string `json:"items,omitempty"`
	FilingURL    string   `json:"filing_url"`
}

// EFTSResponse models the raw efts.sec.gov/LATEST/search-index payload.
type EFTSResponse struct {
	Hits struct {
		Total struct {
			Value    int    `json:"value"`
			Relation string `json:"relation"`
		} `json:"total"`
		Hits []struct {
			ID     string `json:"_id"`
			Source struct {
				Form         string   `json:"form"`
				FileDate     string   `json:"file_date"`
				PeriodEnding string   `json:"period_ending"`
				CIKs         []string `json:"ciks"`
				DisplayNames []string `json:"display_names"`
				Adsh         string   `json:"adsh"`
				SICs         []string `json:"sics"`
				BizStates    []string `json:"biz_states"`
				Items        []string `json:"items"`
			} `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

// Flatten converts an EFTSResponse into a slice of EFTSHit, building the
// filing-index URL for each row.
func (r EFTSResponse) Flatten() []EFTSHit {
	out := make([]EFTSHit, 0, len(r.Hits.Hits))
	for _, h := range r.Hits.Hits {
		adsh := h.Source.Adsh
		if adsh == "" {
			// _id is "<adsh>:<primary-doc>", split on colon
			if i := strings.Index(h.ID, ":"); i > 0 {
				adsh = h.ID[:i]
			} else {
				adsh = h.ID
			}
		}
		cik := ""
		if len(h.Source.CIKs) > 0 {
			cik = h.Source.CIKs[0]
		}
		filingURL := ""
		if cik != "" && adsh != "" {
			noDashes := strings.ReplaceAll(adsh, "-", "")
			cikInt, _ := strconv.Atoi(strings.TrimLeft(cik, "0"))
			filingURL = fmt.Sprintf("https://www.sec.gov/Archives/edgar/data/%d/%s/%s-index.htm",
				cikInt, noDashes, adsh)
		}
		out = append(out, EFTSHit{
			Accession:    adsh,
			Form:         h.Source.Form,
			FileDate:     h.Source.FileDate,
			PeriodEnding: h.Source.PeriodEnding,
			CIKs:         h.Source.CIKs,
			DisplayNames: h.Source.DisplayNames,
			SICs:         h.Source.SICs,
			BizStates:    h.Source.BizStates,
			Items:        h.Source.Items,
			FilingURL:    filingURL,
		})
	}
	return out
}

// archiveBase returns the Archives/edgar/data base URL for a CIK + accession,
// e.g. https://www.sec.gov/Archives/edgar/data/320193/000032019324000123/.
func archiveBase(cik, accession string) string {
	cikInt, _ := strconv.Atoi(strings.TrimLeft(cik, "0"))
	noDashes := strings.ReplaceAll(accession, "-", "")
	return fmt.Sprintf("https://www.sec.gov/Archives/edgar/data/%d/%s/", cikInt, noDashes)
}
