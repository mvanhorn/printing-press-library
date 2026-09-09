// Copyright 2026 Paul Siola and contributors. Licensed under Apache-2.0. See LICENSE.

// Hand-authored shared helpers for the SEEK novel commands. Not generator-emitted.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/seek/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/seek/internal/store"
)

const seekSearchPath = "/api/jobsearch/v5/search"

// seekJob is the subset of a /api/jobsearch/v5/search `data[]` element the
// novel commands read. Unmapped fields are preserved for the local store via
// the raw upsert.
type seekJob struct {
	ID                 string            `json:"id"`
	Title              string            `json:"title"`
	Teaser             string            `json:"teaser"`
	CompanyName        string            `json:"companyName"`
	SalaryLabel        string            `json:"salaryLabel"`
	ListingDate        string            `json:"listingDate"`
	ListingDateDisplay string            `json:"listingDateDisplay"`
	RoleID             string            `json:"roleId"`
	Advertiser         seekAdvertiser    `json:"advertiser"`
	Classifications    []seekClassifPair `json:"classifications"`
	Locations          []seekLocation    `json:"locations"`
	WorkTypes          []string          `json:"workTypes"`
	WorkArrangements   seekWorkArr       `json:"workArrangements"`
	raw                json.RawMessage
}

type seekAdvertiser struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

type seekClassifPair struct {
	Classification    seekIDName `json:"classification"`
	Subclassification seekIDName `json:"subclassification"`
}

type seekIDName struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

type seekLocation struct {
	Label string `json:"label"`
}

type seekWorkArr struct {
	Data []struct {
		ID    string `json:"id"`
		Label struct {
			Text string `json:"text"`
		} `json:"label"`
	} `json:"data"`
}

// seekSearchResponse is the top level of a search response.
type seekSearchResponse struct {
	Data       []json.RawMessage `json:"data"`
	TotalCount int               `json:"totalCount"`
}

// seekSearchOpts are the wire params for one search request.
type seekSearchOpts struct {
	Keywords          string
	Where             string
	SiteKey           string
	Locale            string
	Classification    string
	Subclassification string
	Worktype          string
	Salaryrange       string
	Salarytype        string
	Workarrangement   string
	PageSize          int
}

func (o seekSearchOpts) params(page int) map[string]string {
	p := map[string]string{
		"siteKey":      firstNonEmpty(o.SiteKey, "AU-Main"),
		"sourcesystem": "houston",
		"locale":       firstNonEmpty(o.Locale, "en-AU"),
		"page":         strconv.Itoa(page),
	}
	ps := o.PageSize
	if ps <= 0 {
		ps = 100
	}
	p["pageSize"] = strconv.Itoa(ps)
	setIf(p, "keywords", o.Keywords)
	setIf(p, "where", o.Where)
	setIf(p, "classification", o.Classification)
	setIf(p, "subclassification", o.Subclassification)
	setIf(p, "worktype", o.Worktype)
	setIf(p, "salaryrange", o.Salaryrange)
	setIf(p, "salarytype", o.Salarytype)
	setIf(p, "workarrangement", o.Workarrangement)
	return p
}

func setIf(m map[string]string, k, v string) {
	if strings.TrimSpace(v) != "" {
		m[k] = v
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// scanSearch pages through the SEEK search endpoint up to maxPages, decoding
// each result into a seekJob. It stops early once the API returns an empty
// page. The bool return reports whether the scan cap was hit before the
// result set was exhausted.
func scanSearch(ctx context.Context, c *client.Client, opts seekSearchOpts, maxPages int) (jobs []seekJob, total int, capHit bool, err error) {
	if maxPages < 1 {
		maxPages = 1
	}
	capHit = true
	for page := 1; page <= maxPages; page++ {
		raw, gErr := c.Get(ctx, seekSearchPath, opts.params(page))
		if gErr != nil {
			return jobs, total, capHit, fmt.Errorf("search page %d: %w", page, gErr)
		}
		var resp seekSearchResponse
		if uErr := json.Unmarshal(raw, &resp); uErr != nil {
			return jobs, total, capHit, fmt.Errorf("decoding search page %d: %w", page, uErr)
		}
		if page == 1 {
			total = resp.TotalCount
		}
		if len(resp.Data) == 0 {
			capHit = false
			break
		}
		for _, item := range resp.Data {
			var j seekJob
			if json.Unmarshal(item, &j) != nil {
				continue
			}
			j.raw = item
			jobs = append(jobs, j)
		}
		if len(jobs) >= total && total > 0 {
			capHit = false
			break
		}
	}
	return jobs, total, capHit, nil
}

// cacheJobs upserts each scanned job into the local `listings` table. Errors
// are collected but never fatal — caching is a side benefit, not the point of
// the command.
func cacheJobs(dbPath string, jobs []seekJob) (int, error) {
	if dbPath == "" || len(jobs) == 0 {
		return 0, nil
	}
	db, err := store.Open(dbPath)
	if err != nil {
		return 0, err
	}
	defer db.Close()
	n := 0
	for _, j := range jobs {
		if j.ID == "" || len(j.raw) == 0 {
			continue
		}
		if err := db.UpsertListings(j.raw); err == nil {
			n++
		}
	}
	return n, nil
}

func (j seekJob) topClassification() (id, name string) {
	if len(j.Classifications) == 0 {
		return "", ""
	}
	return j.Classifications[0].Classification.ID, j.Classifications[0].Classification.Description
}

func (j seekJob) subClassification() (id, name string) {
	if len(j.Classifications) == 0 {
		return "", ""
	}
	return j.Classifications[0].Subclassification.ID, j.Classifications[0].Subclassification.Description
}

func (j seekJob) region() string {
	if len(j.Locations) == 0 {
		return ""
	}
	label := j.Locations[0].Label
	// "Macquarie Park, Sydney NSW" -> "Sydney NSW"
	if i := strings.LastIndex(label, ", "); i >= 0 {
		return strings.TrimSpace(label[i+2:])
	}
	return label
}

func (j seekJob) workArrangement() string {
	if len(j.WorkArrangements.Data) == 0 {
		return ""
	}
	return j.WorkArrangements.Data[0].Label.Text
}

func (j seekJob) workType() string {
	if len(j.WorkTypes) == 0 {
		return ""
	}
	return j.WorkTypes[0]
}

const seekGraphQLPath = "/graphql"

// errNoSeekSession is the typed auth failure (exit 4) the me/* novel commands
// return when SEEK reports no viewer session.
var errNoSeekSession = authErr(fmt.Errorf("SEEK session required for this command; run: seek-pp-cli auth login --chrome"))

// seekJobDetails is the subset of the GraphQL jobDetails(id) payload the novel
// commands consume.
type seekJobDetails struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	SalaryLabel string `json:"salaryLabel"`
	Content     string `json:"content"`
	Advertiser  struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"advertiser"`
	Location struct {
		Label string `json:"label"`
	} `json:"location"`
	CompanyProfile json.RawMessage `json:"companyProfile"`
	CompanySearch  string          `json:"companySearchUrl"`
}

const jobDetailsQuery = `query jobDetails($id: ID!, $zone: Zone!, $locale: Locale!, $languageCode: LanguageCodeIso!) { jobDetails(id: $id) { job { id title content(platform: WEB) salary { label } advertiser { id name(locale: $locale) } location { label(locale: $locale) } } companyProfile(zone: $zone) { id name companyNameSlug shouldDisplayReviews overview { description { paragraphs } industry size { description } website { url } } } companySearchUrl(zone: $zone, languageCode: $languageCode) } }`

func zoneForSite(site string) (zone, locale string) {
	if strings.Contains(site, "NZ") {
		return "anz-2", "en-NZ"
	}
	return "anz-1", "en-AU"
}

// fetchJobDetails runs the GraphQL jobDetails query for one job ID.
func fetchJobDetails(ctx context.Context, c *client.Client, id, site string) (seekJobDetails, error) {
	var out seekJobDetails
	zone, locale := zoneForSite(site)
	body := map[string]any{
		"operationName": "jobDetails",
		"query":         jobDetailsQuery,
		"variables": map[string]any{
			"id": id, "zone": zone, "locale": locale, "languageCode": "en",
		},
	}
	raw, status, err := c.PostWithParams(ctx, seekGraphQLPath, map[string]string{}, body)
	if err != nil {
		return out, err
	}
	if status < 200 || status >= 300 {
		return out, fmt.Errorf("graphql jobDetails returned HTTP %d", status)
	}
	var env struct {
		Data struct {
			JobDetails struct {
				Job struct {
					ID      string `json:"id"`
					Title   string `json:"title"`
					Content string `json:"content"`
					Salary  struct {
						Label string `json:"label"`
					} `json:"salary"`
					Advertiser struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					} `json:"advertiser"`
					Location struct {
						Label string `json:"label"`
					} `json:"location"`
				} `json:"job"`
				CompanyProfile json.RawMessage `json:"companyProfile"`
				CompanySearch  string          `json:"companySearchUrl"`
			} `json:"jobDetails"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return out, fmt.Errorf("decoding jobDetails: %w", err)
	}
	if len(env.Errors) > 0 {
		return out, fmt.Errorf("graphql jobDetails: %s", env.Errors[0].Message)
	}
	j := env.Data.JobDetails
	out.ID = j.Job.ID
	out.Title = j.Job.Title
	out.Content = j.Job.Content
	out.SalaryLabel = j.Job.Salary.Label
	out.Advertiser.ID = j.Job.Advertiser.ID
	out.Advertiser.Name = j.Job.Advertiser.Name
	out.Location.Label = j.Job.Location.Label
	out.CompanyProfile = j.CompanyProfile
	out.CompanySearch = j.CompanySearch
	return out, nil
}

// seekSavedSearch is one row of viewer.apacSavedSearches.
type seekSavedSearch struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Query struct {
		SearchQueryString string `json:"searchQueryString"`
	} `json:"query"`
	CountryCode string `json:"countryCode"`
	CreatedDate struct {
		DateTimeUtc string `json:"dateTimeUtc"`
	} `json:"createdDate"`
	NewToYouCountLabel string `json:"newToYouCountLabel"`
	SubscribeToNewJobs bool   `json:"subscribeToNewJobs"`
}

const savedSearchesQuery = `query GetSavedSearches($languageCode: LanguageCodeIso!) { viewer { apacSavedSearches { id name(languageCode: $languageCode) countryCode createdDate { dateTimeUtc } query(languageCode: $languageCode) { searchQueryString } newToYouCountLabel subscribeToNewJobs } } }`

// fetchSavedSearches runs the authenticated viewer.apacSavedSearches query.
func fetchSavedSearches(ctx context.Context, c *client.Client) ([]seekSavedSearch, error) {
	body := map[string]any{
		"operationName": "GetSavedSearches",
		"query":         savedSearchesQuery,
		"variables":     map[string]any{"languageCode": "en"},
	}
	raw, status, err := c.PostWithParams(ctx, seekGraphQLPath, map[string]string{}, body)
	if err != nil {
		return nil, err
	}
	if status == 401 || status == 403 {
		return nil, errNoSeekSession
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("graphql saved searches returned HTTP %d", status)
	}
	var env struct {
		Data struct {
			Viewer *struct {
				ApacSavedSearches []seekSavedSearch `json:"apacSavedSearches"`
			} `json:"viewer"`
		} `json:"data"`
		Errors []struct {
			Message    string `json:"message"`
			Extensions struct {
				Code string `json:"code"`
			} `json:"extensions"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("decoding saved searches: %w", err)
	}
	// SEEK returns HTTP 200 with an error body + code UNAUTHENTICATED (and
	// viewer: null) when there is no session, rather than a 401.
	for _, e := range env.Errors {
		if e.Extensions.Code == "UNAUTHENTICATED" || strings.Contains(strings.ToLower(e.Message), "unauth") {
			return nil, errNoSeekSession
		}
	}
	if env.Data.Viewer == nil {
		return nil, errNoSeekSession
	}
	if len(env.Errors) > 0 {
		return nil, fmt.Errorf("graphql saved searches: %s", env.Errors[0].Message)
	}
	return env.Data.Viewer.ApacSavedSearches, nil
}

// knownJobIDs returns the set of job IDs already in the local listings table.
func knownJobIDs(dbPath string) (map[string]bool, error) {
	seen := map[string]bool{}
	if _, err := os.Stat(dbPath); err != nil {
		return seen, nil
	}
	db, err := store.OpenReadOnly(dbPath)
	if err != nil {
		return seen, err
	}
	defer db.Close()
	rows, err := db.DB().Query(`SELECT id FROM listings`)
	if err != nil {
		return seen, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil && id != "" {
			seen[id] = true
		}
	}
	return seen, rows.Err()
}
