package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// SAMClient handles SAM.gov API calls.
type SAMClient struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// NewSAMClient creates a SAM.gov API client.
func NewSAMClient(baseURL, apiKey string) *SAMClient {
	return &SAMClient{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetAsideCodes maps human-readable names to SAM.gov API codes.
var SetAsideCodes = map[string]string{
	"sba":     "SBA",
	"sdvosb":  "SBP",
	"wosb":    "WOSB",
	"8a":      "8AN",
	"hubzone": "HZC",
}

// SAMOpportunity is a single opportunity from the SAM.gov API.
type SAMOpportunity struct {
	NoticeID             string `json:"noticeId"`
	Title                string `json:"title"`
	FullParentPathName   string `json:"fullParentPathName"`
	Department           string `json:"department"`
	SubTier              string `json:"subTier"`
	Office               string `json:"office"`
	NAICSCode            string `json:"naicsCode"`
	TypeOfSetAsideCode   string `json:"typeOfSetAsideCode"`
	TypeOfSetAside       string `json:"typeOfSetAside"`
	ResponseDeadLine     string `json:"responseDeadLine"`
	PostedDate           string `json:"postedDate"`
	Description          string `json:"description"`
	SolicitationNumber   string `json:"solicitationNumber"`
	PointOfContact       []struct {
		FullName string `json:"fullName"`
		Email    string `json:"email"`
		Phone    string `json:"phone"`
	} `json:"pointOfContact"`
}

// SAMSearchResponse is the top-level SAM.gov search response.
type SAMSearchResponse struct {
	TotalRecords     int              `json:"totalRecords"`
	OpportunitiesData []SAMOpportunity `json:"opportunitiesData"`
}

// Search queries SAM.gov for opportunities.
func (c *SAMClient) Search(query, setAside string, limit, offset int) (*SAMSearchResponse, error) {
	if c.APIKey == "" {
		return nil, fmt.Errorf("SAM API key not configured — run: govprocure-pp-cli auth set-key --sam YOUR_KEY")
	}
	if limit <= 0 {
		limit = 25
	}

	params := url.Values{}
	params.Set("api_key", c.APIKey)
	if query != "" {
		params.Set("q", query)
	}
	params.Set("limit", strconv.Itoa(limit))
	params.Set("offset", strconv.Itoa(offset))
	params.Set("postedFrom", time.Now().AddDate(0, -3, 0).Format("01/02/2006"))
	params.Set("postedTo", time.Now().Format("01/02/2006"))
	if setAside != "" {
		params.Set("ntype", setAside)
	}

	reqURL := c.BaseURL + "?" + params.Encode()
	resp, err := c.HTTPClient.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("SAM.gov request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("SAM.gov auth error (HTTP %d): check your API key", resp.StatusCode)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("SAM.gov rate limit exceeded (HTTP 429)")
	}
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("SAM.gov HTTP %d: %s", resp.StatusCode, string(data))
	}

	var result SAMSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode SAM response: %w", err)
	}
	return &result, nil
}

// GetNotice fetches a single SAM notice by noticeId.
// SAM.gov v2 doesn't have a direct get-by-ID endpoint; we search and match.
func (c *SAMClient) GetNotice(noticeID string) (*SAMOpportunity, error) {
	resp, err := c.Search(noticeID, "", 10, 0)
	if err != nil {
		return nil, err
	}
	for i := range resp.OpportunitiesData {
		if resp.OpportunitiesData[i].NoticeID == noticeID {
			return &resp.OpportunitiesData[i], nil
		}
	}
	return nil, nil
}

// SyncAll paginates through all recent SAM opportunities.
func (c *SAMClient) SyncAll(query, setAside string, maxRecords int) ([]SAMOpportunity, error) {
	if maxRecords <= 0 {
		maxRecords = 500
	}
	const pageSize = 100
	var all []SAMOpportunity
	offset := 0

	for {
		resp, err := c.Search(query, setAside, pageSize, offset)
		if err != nil {
			return all, err
		}
		all = append(all, resp.OpportunitiesData...)
		offset += len(resp.OpportunitiesData)
		if len(resp.OpportunitiesData) < pageSize || offset >= maxRecords || offset >= resp.TotalRecords {
			break
		}
	}
	return all, nil
}

// AgencyFromSAM extracts a clean agency name from a SAM opportunity.
func AgencyFromSAM(s *SAMOpportunity) string {
	if s.Department != "" {
		return s.Department
	}
	if s.SubTier != "" {
		return s.SubTier
	}
	if s.FullParentPathName != "" {
		return s.FullParentPathName
	}
	return ""
}
