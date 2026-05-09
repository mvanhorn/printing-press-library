package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// USASpendingClient handles USASpending.gov API calls.
type USASpendingClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewUSASpendingClient creates a USASpending.gov API client.
func NewUSASpendingClient(baseURL string) *USASpendingClient {
	return &USASpendingClient{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 45 * time.Second,
		},
	}
}

// AwardType codes for grants and contracts.
var (
	GrantAwardTypes    = []string{"02", "03", "04", "05"}
	ContractAwardTypes = []string{"A", "B", "C", "D"}
)

// AwardSearchRequest is the body for /api/v2/search/spending_by_award/
type AwardSearchRequest struct {
	Filters AwardFilters `json:"filters"`
	Fields  []string     `json:"fields"`
	Sort    string       `json:"sort"`
	Order   string       `json:"order"`
	Limit   int          `json:"limit"`
	Page    int          `json:"page"`
}

// AwardFilters holds filter criteria.
type AwardFilters struct {
	AwardTypeCodes []string        `json:"award_type_codes"`
	Agencies       []AgencyFilter  `json:"agencies,omitempty"`
	CFDANumbers    []string        `json:"program_numbers,omitempty"`
	TimeRange      []TimeRangeFilter `json:"time_period,omitempty"`
}

// AgencyFilter specifies an agency filter.
type AgencyFilter struct {
	Type string `json:"type"`
	Tier string `json:"tier"`
	Name string `json:"name"`
}

// TimeRangeFilter specifies a date range.
type TimeRangeFilter struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

// AwardResult is a single award from the search response.
type AwardResult struct {
	AwardID       string  `json:"Award ID"`
	RecipientName string  `json:"Recipient Name"`
	Agency        string  `json:"Awarding Agency"`
	SubAgency     string  `json:"Awarding Sub Agency"`
	AwardAmount   float64 `json:"Award Amount"`
	StartDate     string  `json:"Start Date"`
	EndDate       string  `json:"End Date"`
	Description   string  `json:"Description"`
	CFDANumber    string  `json:"CFDA Number"`
}

// AwardSearchResponse is the top-level response from /spending_by_award/
type AwardSearchResponse struct {
	Results    []AwardResult `json:"results"`
	Page       int           `json:"page"`
	Limit      int           `json:"limit"`
	Total      int           `json:"total_metadata"`
}

// SearchAwards queries USASpending.gov for awards.
// USASpending requires award_type_codes to come from a single group — you cannot
// mix contracts (A/B/C/D) and grants (02/03/04/05) in one request.
// When no types are specified we make two separate calls and merge.
func (c *USASpendingClient) SearchAwards(agencies []string, cfda []string, awardTypes []string, limit int) (*AwardSearchResponse, error) {
	if limit <= 0 {
		limit = 25
	}

	if len(awardTypes) == 0 {
		// Two separate calls, each with half the limit, then merge.
		half := limit/2 + limit%2
		grants, err := c.searchAwardGroup(agencies, cfda, GrantAwardTypes, half)
		if err != nil {
			// Grants failed — try contracts only.
			return c.searchAwardGroup(agencies, cfda, ContractAwardTypes, limit)
		}
		contracts, err := c.searchAwardGroup(agencies, cfda, ContractAwardTypes, half)
		if err != nil {
			// Contracts failed — return grants only.
			return grants, nil
		}
		merged := &AwardSearchResponse{
			Results: append(grants.Results, contracts.Results...),
			Limit:   limit,
		}
		return merged, nil
	}

	return c.searchAwardGroup(agencies, cfda, awardTypes, limit)
}

func (c *USASpendingClient) searchAwardGroup(agencies []string, cfda []string, awardTypes []string, limit int) (*AwardSearchResponse, error) {
	filters := AwardFilters{
		AwardTypeCodes: awardTypes,
	}
	for _, a := range agencies {
		filters.Agencies = append(filters.Agencies, AgencyFilter{
			Type: "awarding",
			Tier: "toptier",
			Name: a,
		})
	}
	if len(cfda) > 0 {
		filters.CFDANumbers = cfda
	}
	filters.TimeRange = []TimeRangeFilter{
		{
			StartDate: time.Now().AddDate(-3, 0, 0).Format("2006-01-02"),
			EndDate:   time.Now().Format("2006-01-02"),
		},
	}

	req := AwardSearchRequest{
		Filters: filters,
		Fields:  []string{"Award ID", "Recipient Name", "Awarding Agency", "Awarding Sub Agency", "Award Amount", "Start Date", "End Date", "Description", "CFDA Number"},
		Sort:    "Award Amount",
		Order:   "desc",
		Limit:   limit,
		Page:    1,
	}

	return c.postAwardSearch(req)
}

func (c *USASpendingClient) postAwardSearch(req AwardSearchRequest) (*AwardSearchResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	url := c.BaseURL + "/search/spending_by_award/"
	resp, err := c.HTTPClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("usaspending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("USASpending.gov rate limit exceeded")
	}
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("USASpending HTTP %d: %s", resp.StatusCode, string(data))
	}

	var result AwardSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

// RecipientResult is a recipient from the recipient search.
type RecipientResult struct {
	RecipientID   string  `json:"id"`
	Name          string  `json:"name"`
	State         string  `json:"state_province"`
	TotalAmount   float64 `json:"amount"`
	AwardCount    int     `json:"award_count"`
	EntityType    string  `json:"entity_type"`
}

// RecipientSearchResponse is the response from the recipient endpoint.
type RecipientSearchResponse struct {
	Results []RecipientResult `json:"results"`
}

// SearchRecipient looks up a recipient by name.
func (c *USASpendingClient) SearchRecipient(name string, limit int) (*RecipientSearchResponse, error) {
	if limit <= 0 {
		limit = 10
	}

	type recipReq struct {
		Keyword string `json:"keyword"`
		Limit   int    `json:"limit"`
	}

	req := recipReq{Keyword: name, Limit: limit}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	url := c.BaseURL + "/recipient/"
	resp, err := c.HTTPClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("recipient request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("USASpending recipient HTTP %d: %s", resp.StatusCode, string(data))
	}

	var result RecipientSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode recipient response: %w", err)
	}
	return &result, nil
}

// SpendingOverTimeRequest is the body for /spending_over_time/
type SpendingOverTimeRequest struct {
	Filters      AwardFilters `json:"filters"`
	GroupBy      string       `json:"group"`
}

// SpendingOverTimeResult is a single time-period result.
type SpendingOverTimeResult struct {
	Period         interface{} `json:"time_period"`
	AggregatedAmount float64   `json:"aggregated_amount"`
}

// SpendingOverTimeResponse is the top-level response.
type SpendingOverTimeResponse struct {
	GroupName string                   `json:"group"`
	Results   []SpendingOverTimeResult  `json:"results"`
}

// SpendingTrends fetches year-over-year spending for an agency.
func (c *USASpendingClient) SpendingTrends(agency string, years int) (*SpendingOverTimeResponse, error) {
	if years <= 0 {
		years = 5
	}

	// Use grants group only for spending trends (avoids mixed-group 422 error).
	filters := AwardFilters{
		AwardTypeCodes: GrantAwardTypes,
		TimeRange: []TimeRangeFilter{
			{
				StartDate: time.Now().AddDate(-years, 0, 0).Format("2006-01-02"),
				EndDate:   time.Now().Format("2006-01-02"),
			},
		},
	}
	if agency != "" {
		filters.Agencies = []AgencyFilter{
			{Type: "awarding", Tier: "toptier", Name: agency},
		}
	}

	req := SpendingOverTimeRequest{
		Filters: filters,
		GroupBy: "fiscal_year",
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	url := c.BaseURL + "/search/spending_over_time/"
	resp, err := c.HTTPClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("spending trends request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("USASpending trends HTTP %d: %s", resp.StatusCode, string(data))
	}

	var result SpendingOverTimeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode trends response: %w", err)
	}
	return &result, nil
}
