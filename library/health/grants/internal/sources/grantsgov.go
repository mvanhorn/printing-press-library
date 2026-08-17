package sources

import (
	"encoding/json"
	"fmt"
	"html"
)

// Grants.gov Search2 API — open federal funding opportunities.
// Keyless. Dates in the results use MM/DD/YYYY.

const (
	grantsSearchURL = "https://api.grants.gov/v1/api/search2"
	grantsFetchURL  = "https://api.grants.gov/v1/api/fetchOpportunity"
)

type Opportunity struct {
	ID         json.Number `json:"id"`
	Number     string      `json:"number"`
	Title      string      `json:"title"`
	AgencyCode string      `json:"agencyCode"`
	Agency     string      `json:"agency"`
	OpenDate   string      `json:"openDate"`
	CloseDate  string      `json:"closeDate"`
	Status     string      `json:"oppStatus"`

	// Populated from fetchOpportunity, only for --details/--min-award/--eligibility.
	Details *OppDetails `json:"details,omitempty"`
}

type OppDetails struct {
	AwardCeiling     int64    `json:"awardCeiling"`
	AwardFloor       int64    `json:"awardFloor"`
	EstimatedFunding int64    `json:"estimatedFunding,omitempty"`
	ResponseDate     string   `json:"responseDate,omitempty"`
	ApplicantTypes   []string `json:"applicantTypes,omitempty"`
}

// AwardCap returns the best available amount for filtering and display: the
// award ceiling when present, otherwise the estimated total funding.
func (d OppDetails) AwardCap() int64 {
	if d.AwardCeiling > 0 {
		return d.AwardCeiling
	}
	return d.EstimatedFunding
}

type grantsSearchResp struct {
	ErrorCode int    `json:"errorcode"`
	Msg       string `json:"msg"`
	Data      struct {
		HitCount int           `json:"hitCount"`
		OppHits  []Opportunity `json:"oppHits"`
	} `json:"data"`
}

// SearchOpportunities lists posted (open) opportunities matching keyword.
func SearchOpportunities(keyword, agencyCode string, rows int) ([]Opportunity, int, error) {
	payload := map[string]any{
		"keyword":        keyword,
		"oppStatuses":    "posted",
		"rows":           rows,
		"startRecordNum": 0,
	}
	if agencyCode != "" {
		payload["agencies"] = agencyCode
	}
	var resp grantsSearchResp
	if err := postJSON(grantsSearchURL, payload, &resp); err != nil {
		return nil, 0, fmt.Errorf("grants.gov search: %w", err)
	}
	if resp.ErrorCode != 0 {
		return nil, 0, fmt.Errorf("grants.gov search: errorcode %d: %s", resp.ErrorCode, resp.Msg)
	}
	opps := resp.Data.OppHits
	for i := range opps {
		opps[i].Title = html.UnescapeString(opps[i].Title) // titles contain entities such as &ndash;
	}
	return opps, resp.Data.HitCount, nil
}

type grantsFetchResp struct {
	ErrorCode int    `json:"errorcode"`
	Msg       string `json:"msg"`
	Data      struct {
		Synopsis struct {
			AwardCeiling     any    `json:"awardCeiling"`
			AwardFloor       any    `json:"awardFloor"`
			EstimatedFunding any    `json:"estimatedFunding"`
			ResponseDate     string `json:"responseDate"`
			ApplicantTypes   []struct {
				Description string `json:"description"`
			} `json:"applicantTypes"`
		} `json:"synopsis"`
	} `json:"data"`
}

// FetchDetails loads award ceiling/floor + eligible applicant types for one opportunity.
func FetchDetails(id json.Number) (*OppDetails, error) {
	var resp grantsFetchResp
	if err := postJSON(grantsFetchURL, map[string]any{"opportunityId": id.String()}, &resp); err != nil {
		return nil, fmt.Errorf("grants.gov fetchOpportunity %s: %w", id, err)
	}
	if resp.ErrorCode != 0 {
		return nil, fmt.Errorf("grants.gov fetchOpportunity %s: errorcode %d: %s", id, resp.ErrorCode, resp.Msg)
	}
	d := &OppDetails{
		AwardCeiling:     ParseMoney(resp.Data.Synopsis.AwardCeiling),
		AwardFloor:       ParseMoney(resp.Data.Synopsis.AwardFloor),
		EstimatedFunding: ParseMoney(resp.Data.Synopsis.EstimatedFunding),
		ResponseDate:     resp.Data.Synopsis.ResponseDate,
	}
	for _, t := range resp.Data.Synopsis.ApplicantTypes {
		if t.Description != "" {
			d.ApplicantTypes = append(d.ApplicantTypes, t.Description)
		}
	}
	return d, nil
}
