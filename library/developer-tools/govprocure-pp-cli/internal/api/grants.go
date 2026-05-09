package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// GrantsClient handles grants.gov API calls.
type GrantsClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewGrantsClient creates a grants.gov API client.
func NewGrantsClient(baseURL string) *GrantsClient {
	return &GrantsClient{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GrantsSearchRequest is the POST body for the grants.gov search endpoint.
type GrantsSearchRequest struct {
	Keyword        string `json:"keyword"`
	Rows           int    `json:"rows"`
	StartRecordNum int    `json:"startRecordNum"`
	OppStatuses    string `json:"oppStatuses"`
}

// GrantsOpportunity is a single opportunity from the grants.gov API.
type GrantsOpportunity struct {
	ID              string   `json:"id"`
	Number          string   `json:"number"`
	Title           string   `json:"title"`
	Agency          string   `json:"agency"`
	AgencyCode      string   `json:"agencyCode"`
	OpenDate        string   `json:"openDate"`
	CloseDate       string   `json:"closeDate"`
	CFDAList        []string `json:"cfdaList"`
	Synopsis        string   `json:"synopsis"`
	EligApplicants  string   `json:"eligApplicants"`
	AwardFloor      float64  `json:"awardFloor"`
	AwardCeiling    float64  `json:"awardCeiling"`
}

// GrantsSearchResponse is the top-level API response.
// The actual grants.gov REST API returns oppHits at the top level (not nested under "data").
type GrantsSearchResponse struct {
	HitCount  int                 `json:"hitCount"`
	OppHits   []GrantsOpportunity `json:"oppHits"`
	ErrorMsgs []string            `json:"errorMsgs"`
	// Legacy nested form (some endpoint versions)
	Data *struct {
		OppHits []GrantsOpportunity `json:"oppHits"`
		Total   int                 `json:"totalOpportunities"`
	} `json:"data"`
}

// Search queries grants.gov with the given keyword.
func (c *GrantsClient) Search(keyword string, rows, offset int) (*GrantsSearchResponse, error) {
	if rows <= 0 {
		rows = 25
	}
	req := GrantsSearchRequest{
		Keyword:        keyword,
		Rows:           rows,
		StartRecordNum: offset,
		OppStatuses:    "posted",
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	resp, err := c.HTTPClient.Post(c.BaseURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("grants.gov request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("grants.gov HTTP %d: %s", resp.StatusCode, string(data))
	}

	var result GrantsSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Normalize: if top-level OppHits is empty but nested Data has results, promote them
	if len(result.OppHits) == 0 && result.Data != nil && len(result.Data.OppHits) > 0 {
		result.OppHits = result.Data.OppHits
		result.HitCount = result.Data.Total
	}

	return &result, nil
}

// GetOpportunity fetches a single opportunity by ID.
// grants.gov doesn't have a dedicated single-record endpoint, so we search by number.
func (c *GrantsClient) GetOpportunity(opportunityID string) (*GrantsOpportunity, error) {
	resp, err := c.Search(opportunityID, 5, 0)
	if err != nil {
		return nil, err
	}
	for i := range resp.OppHits {
		if resp.OppHits[i].Number == opportunityID || resp.OppHits[i].ID == opportunityID {
			return &resp.OppHits[i], nil
		}
	}
	return nil, nil
}

// SyncAll fetches all recently posted opportunities (paginates).
func (c *GrantsClient) SyncAll(keyword string, maxRecords int) ([]GrantsOpportunity, error) {
	if keyword == "" {
		keyword = "technology"
	}
	if maxRecords <= 0 {
		maxRecords = 500
	}

	const pageSize = 25
	var all []GrantsOpportunity
	offset := 0

	for {
		resp, err := c.Search(keyword, pageSize, offset)
		if err != nil {
			return all, err
		}
		all = append(all, resp.OppHits...)
		offset += len(resp.OppHits)
		if len(resp.OppHits) < pageSize || offset >= maxRecords || offset >= resp.HitCount {
			break
		}
	}
	return all, nil
}

// CFDAString joins a CFDA list to a comma-separated string.
func CFDAString(list []string) string {
	if len(list) == 0 {
		return ""
	}
	result := ""
	for i, v := range list {
		if i > 0 {
			result += ","
		}
		result += v
	}
	return result
}
