// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package uberjobs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// The Oracle fallback reads the same requisitions from Uber's public Oracle
// Recruiting Cloud candidate-experience API (site UberCareers, siteNumber
// CX_1). It runs only after jobs.uber.com refused a request. Paths and finder
// syntax come from Oracle's REST reference (docs.oracle.com, farws) and match
// the three owner-approved requests measured on 2026-10-05.
const (
	OracleDefaultBase = "https://iaziqy.fa.ocs.oraclecloud.com"
	OracleSiteNumber  = "CX_1"
	// OracleUserAgent is the identity measured 3/3 at 200 on this tenant
	// (owner decision; never changed after a refusal).
	OracleUserAgent  = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36"
	oracleListPath   = "/hcmRestApi/resources/11.13.18.05/recruitingCEJobRequisitions"
	oracleDetailPath = "/hcmRestApi/resources/11.13.18.05/recruitingCEJobRequisitionDetails"
	oraclePageLimit  = 200 // Oracle caps limit at 200
	oracleMaxPages   = 10
)

// OracleEnabled reports whether a fallback host is configured.
func (c *Client) OracleEnabled() bool { return c != nil && c.OracleBase != "" }

// oracleFinderValue escapes a finder string the way the measured requests sent
// it: ; , = stay literal; quotes, spaces, and other unsafe bytes are encoded.
func oracleFinderValue(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch >= 'a' && ch <= 'z', ch >= 'A' && ch <= 'Z', ch >= '0' && ch <= '9':
			b.WriteByte(ch)
		case strings.IndexByte(";,=_-.", ch) >= 0:
			b.WriteByte(ch)
		default:
			fmt.Fprintf(&b, "%%%02X", ch)
		}
	}
	return b.String()
}

func oracleQuote(v string) string {
	v = strings.ReplaceAll(v, `\`, `\\`)
	v = strings.ReplaceAll(v, `"`, `\"`)
	return `"` + v + `"`
}

type oracleLocation struct {
	Name        LooseString `json:"Name"`
	CountryCode LooseString `json:"CountryCode"`
}

// OracleRow is one requisitionList entry. Only listing fields are read.
type OracleRow struct {
	ID                     LooseString      `json:"Id"`
	Title                  LooseString      `json:"Title"`
	PostedDate             LooseString      `json:"PostedDate"`
	PrimaryLocation        LooseString      `json:"PrimaryLocation"`
	PrimaryLocationCountry LooseString      `json:"PrimaryLocationCountry"`
	WorkplaceType          LooseString      `json:"WorkplaceType"`
	SecondaryLocations     []oracleLocation `json:"secondaryLocations"`
}

// OracleResult is a complete fallback read.
type OracleResult struct {
	Rows       []OracleRow
	Total      int
	Complete   bool
	ScanCapHit bool
	Requests   int
}

// OracleSearchAll pages the requisition list (limit inside the finder, stop on
// an empty page) and dedupes by Id. keyword narrows server-side when set.
func (c *Client) OracleSearchAll(ctx context.Context, keyword string) (*OracleResult, error) {
	if !c.OracleEnabled() {
		return nil, fmt.Errorf("oracle fallback is not configured")
	}
	res := &OracleResult{}
	seen := map[string]bool{}
	firstTotal, drift := -1, false
	kw := strings.TrimSpace(keyword)
	for page := 0; page < oracleMaxPages; page++ {
		finder := fmt.Sprintf("findReqs;siteNumber=%s,limit=%d,offset=%d,sortBy=POSTING_DATES_DESC", OracleSiteNumber, oraclePageLimit, page*oraclePageLimit)
		if kw != "" {
			finder += ",keyword=" + oracleQuote(kw)
		}
		u := c.OracleBase + oracleListPath + "?onlyData=true&expand=requisitionList.secondaryLocations&finder=" + oracleFinderValue(finder)
		resp, err := c.get(ctx, u, "application/json", OracleUserAgent)
		if err != nil {
			return nil, err
		}
		if !resp.CacheHit {
			res.Requests++
		}
		var env struct {
			Items []struct {
				TotalJobsCount  *int        `json:"TotalJobsCount"`
				RequisitionList []OracleRow `json:"requisitionList"`
			} `json:"items"`
		}
		trimmed := bytes.TrimSpace(resp.Body)
		if len(trimmed) == 0 || trimmed[0] != '{' || json.Unmarshal(trimmed, &env) != nil || env.Items == nil {
			return nil, &ContentError{URL: u, Reason: "oracle reply is not {items:[...]}"}
		}
		if len(env.Items) == 0 || env.Items[0].TotalJobsCount == nil {
			return nil, &ContentError{URL: u, Reason: "oracle reply has no TotalJobsCount"}
		}
		res.Total = *env.Items[0].TotalJobsCount
		if kw == "" && res.Total == 0 {
			return nil, &ContentError{URL: u, Reason: "the unfiltered requisition list reported 0 postings; refusing to treat an empty list as a complete read"}
		}
		rememberProjection(c, resp, env)
		// The list can shift between pages; a total that moves mid-walk
		// means a posting may have slid past a page boundary unseen.
		if firstTotal < 0 {
			firstTotal = res.Total
		} else if res.Total != firstTotal {
			drift = true
		}
		list := env.Items[0].RequisitionList
		if len(list) == 0 {
			res.Complete = !drift && len(res.Rows) >= res.Total
			return res, nil
		}
		for _, r := range list {
			id := strings.TrimSpace(string(r.ID))
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			res.Rows = append(res.Rows, r)
		}
	}
	res.ScanCapHit = true
	return res, nil
}

// rememberProjection caches the decoded, whitelisted view of an Oracle reply
// in place of its raw body; a cache hit decodes the same way.
func rememberProjection(c *Client, resp *response, env any) {
	// A cache hit is already stored; re-writing it would refresh its stamp
	// and keep it fresh forever.
	if resp.CacheHit {
		return
	}
	clean, err := json.Marshal(env)
	if err != nil {
		return
	}
	c.remember("application/json", &response{Body: clean, Header: resp.Header, Status: resp.Status, URL: resp.URL})
}

// OracleDetail is the detail record used by get when the site refused.
type OracleDetail struct {
	ID                      string
	Title                   string
	Category                string
	Description             string
	Qualifications          string
	Responsibilities        string
	ExternalPostedStartDate string
	PrimaryLocation         string
	PrimaryLocationCountry  string
	JobSchedule             string
}

// OracleGet reads one requisition's detail. An empty items array means the id
// is not listed. HiringManager, contact, and Internal* fields are never read.
func (c *Client) OracleGet(ctx context.Context, id string) (*OracleDetail, error) {
	if !c.OracleEnabled() {
		return nil, fmt.Errorf("oracle fallback is not configured")
	}
	finder := fmt.Sprintf("ById;Id=%s,siteNumber=%s", oracleQuote(id), OracleSiteNumber)
	u := c.OracleBase + oracleDetailPath + "?onlyData=true&expand=all&finder=" + oracleFinderValue(finder)
	resp, err := c.get(ctx, u, "application/json", OracleUserAgent)
	if err != nil {
		return nil, err
	}
	var env struct {
		Items []struct {
			ID                      LooseString `json:"Id"`
			Title                   LooseString `json:"Title"`
			Category                LooseString `json:"Category"`
			ExternalDescriptionStr  LooseString `json:"ExternalDescriptionStr"`
			ExternalQualifications  LooseString `json:"ExternalQualificationsStr"`
			ExternalResponsibility  LooseString `json:"ExternalResponsibilitiesStr"`
			ExternalPostedStartDate LooseString `json:"ExternalPostedStartDate"`
			PrimaryLocation         LooseString `json:"PrimaryLocation"`
			PrimaryLocationCountry  LooseString `json:"PrimaryLocationCountry"`
			JobSchedule             LooseString `json:"JobSchedule"`
		} `json:"items"`
	}
	trimmed := bytes.TrimSpace(resp.Body)
	if len(trimmed) == 0 || trimmed[0] != '{' || json.Unmarshal(trimmed, &env) != nil || env.Items == nil {
		return nil, &ContentError{URL: u, Reason: "oracle detail reply is not {items:[...]}"}
	}
	// Only the decoded projection is cached: the raw expand=all reply
	// carries HiringManager, ExternalContact* and Internal* fields, which
	// are never stored.
	rememberProjection(c, resp, env)
	if len(env.Items) == 0 {
		return nil, &NotFoundError{ID: id}
	}
	it := env.Items[0]
	if strings.TrimSpace(string(it.ID)) != id {
		return nil, &NotFoundError{ID: id}
	}
	return &OracleDetail{
		ID:                      string(it.ID),
		Title:                   string(it.Title),
		Category:                string(it.Category),
		Description:             StripHTML(string(it.ExternalDescriptionStr)),
		Qualifications:          StripHTML(string(it.ExternalQualifications)),
		Responsibilities:        StripHTML(string(it.ExternalResponsibility)),
		ExternalPostedStartDate: string(it.ExternalPostedStartDate),
		PrimaryLocation:         string(it.PrimaryLocation),
		PrimaryLocationCountry:  string(it.PrimaryLocationCountry),
		JobSchedule:             string(it.JobSchedule),
	}, nil
}

// NormalizeOracleDetail maps a fallback detail record onto the contract. The
// description comes from Oracle's external description. job_category stays
// null: Oracle's Category is its own taxonomy, not the site's Teams.
func NormalizeOracleDetail(d *OracleDetail, siteBase string) Posting {
	if siteBase == "" {
		siteBase = DefaultBaseURL
	}
	id := strings.TrimSpace(d.ID)
	p := Posting{
		ID:       id,
		Title:    strings.TrimSpace(d.Title),
		Employer: "uber",
		Source:   SourceOracle,
	}
	p.PostedRaw = strPtr(d.ExternalPostedStartDate)
	p.PostedOn, p.PostedDate, p.PostedDateIsFloor = postingDates(d.ExternalPostedStartDate)
	primary := Location{Address: strPtr(d.PrimaryLocation)}
	if cc := strings.TrimSpace(d.PrimaryLocationCountry); cc != "" {
		iso := ToISO3(cc)
		primary.CountryCode = &iso
		if name, ok := SiteNameForISO3(iso); ok {
			primary.Country = &name
		}
	}
	p.Locations = []Location{primary}
	p.CountryCode = primary.CountryCode
	p.Country = primary.Country
	p.Location = primary.Address
	p.NormalizedLocation = primary.Address
	p.Description = strPtr(d.Description)
	p.ContractType = strPtr(d.JobSchedule)
	p.SalaryRanges = []SalaryRange{}
	p.JobPath = "/en/jobs/" + id + "/"
	p.URL = strings.TrimRight(siteBase, "/") + p.JobPath
	return p
}

// NormalizeOracleRow maps a fallback listing row onto the contract. Fields
// the Oracle listing does not carry (description, category) stay null.
func NormalizeOracleRow(r OracleRow, siteBase string) Posting {
	if siteBase == "" {
		siteBase = DefaultBaseURL
	}
	id := strings.TrimSpace(string(r.ID))
	p := Posting{
		ID:       id,
		Title:    strings.TrimSpace(string(r.Title)),
		Employer: "uber",
		Source:   SourceOracle,
	}
	p.PostedRaw = strPtr(string(r.PostedDate))
	p.PostedOn, p.PostedDate, p.PostedDateIsFloor = postingDates(string(r.PostedDate))
	primary := Location{Address: strPtr(string(r.PrimaryLocation))}
	if cc := strings.TrimSpace(string(r.PrimaryLocationCountry)); cc != "" {
		iso := ToISO3(cc)
		primary.CountryCode = &iso
		if name, ok := SiteNameForISO3(iso); ok {
			primary.Country = &name
		}
	}
	p.Locations = []Location{primary}
	for _, s := range r.SecondaryLocations {
		loc := Location{Address: strPtr(string(s.Name))}
		if cc := strings.TrimSpace(string(s.CountryCode)); cc != "" {
			iso := ToISO3(cc)
			loc.CountryCode = &iso
			if name, ok := SiteNameForISO3(iso); ok {
				loc.Country = &name
			}
		}
		p.Locations = append(p.Locations, loc)
	}
	p.CountryCode = primary.CountryCode
	p.Country = primary.Country
	p.Location = primary.Address
	p.NormalizedLocation = primary.Address
	p.SalaryRanges = []SalaryRange{}
	p.JobPath = "/en/jobs/" + id + "/"
	p.URL = strings.TrimRight(siteBase, "/") + p.JobPath
	return p
}
