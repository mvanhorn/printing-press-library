package nyrr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/other/running-race-results/internal/domain"
)

// runnerSearchRequest is the body for /api/v2/runners/search.
type runnerSearchRequest struct {
	SearchString   string  `json:"searchString"`
	PageIndex      int     `json:"pageIndex"`
	PageSize       int     `json:"pageSize"`
	SortColumn     *string `json:"sortColumn"`
	SortDescending bool    `json:"sortDescending"`
}

// runnerItem is one entry in the runners/search response.
type runnerItem struct {
	RunnerID      int    `json:"runnerId"`
	FirstName     string `json:"firstName"`
	LastName      string `json:"lastName"`
	Age           int    `json:"age"`
	City          string `json:"city"`
	StateProvince string `json:"stateProvince"`
}

// runnerSearchResponse wraps the runners/search items.
type runnerSearchResponse struct {
	TotalItems int          `json:"totalItems"`
	Items      []runnerItem `json:"items"`
}

// raceHistoryRequest is the body for /api/v2/runners/races.
type raceHistoryRequest struct {
	RunnerID       string `json:"runnerId"`
	PageIndex      int    `json:"pageIndex"`
	PageSize       int    `json:"pageSize"`
	SortColumn     string `json:"sortColumn"`
	SortDescending bool   `json:"sortDescending"`
}

// raceHistoryItem is one entry in the runners/races response.
type raceHistoryItem struct {
	EventName     string `json:"eventName"`
	StartDateTime string `json:"startDateTime"`
	DistanceName  string `json:"distanceName"`
	ActualTime    string `json:"actualTime"`
	Bib           string `json:"bib"`
	EventCode     string `json:"eventCode"`
}

// raceHistoryResponse wraps the runners/races items.
type raceHistoryResponse struct {
	TotalItems int               `json:"totalItems"`
	Items      []raceHistoryItem `json:"items"`
}

// postJSON sends a POST request with the given body and Referer header, checks
// the status code, and decodes the response. It strips a "_meta" key from the
// top-level JSON object before unmarshaling into dst.
func (c *Client) postJSON(ctx context.Context, path string, body any, dst any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("nyrr: marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("nyrr: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Referer", "https://results.nyrr.org/")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("nyrr: http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("nyrr: unexpected status %d", resp.StatusCode)
	}

	// Strip _meta key before decoding.
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return fmt.Errorf("nyrr: decode response envelope: %w", err)
	}
	delete(raw, "_meta")
	cleaned, err := json.Marshal(raw)
	if err != nil {
		return fmt.Errorf("nyrr: re-marshal response: %w", err)
	}
	if err := json.Unmarshal(cleaned, dst); err != nil {
		return fmt.Errorf("nyrr: decode response: %w", err)
	}
	return nil
}

// FindAthletes implements provider.AthleteSearcher.
func (c *Client) FindAthletes(ctx context.Context, name string) ([]domain.Athlete, error) {
	req := runnerSearchRequest{
		SearchString:   name,
		PageIndex:      1,
		PageSize:       51,
		SortColumn:     nil,
		SortDescending: false,
	}
	var resp runnerSearchResponse
	if err := c.postJSON(ctx, "/api/v2/runners/search", req, &resp); err != nil {
		return nil, err
	}

	seen := make(map[int]struct{}, len(resp.Items))
	out := make([]domain.Athlete, 0, len(resp.Items))
	for _, it := range resp.Items {
		if it.RunnerID == 0 {
			continue // no usable athlete id
		}
		if _, dup := seen[it.RunnerID]; dup {
			continue
		}
		seen[it.RunnerID] = struct{}{}
		out = append(out, domain.Athlete{
			Provider:  "nyrr",
			ID:        strconv.Itoa(it.RunnerID),
			Name:      it.FirstName + " " + it.LastName,
			City:      it.City,
			StateProv: it.StateProvince,
			Age:       it.Age,
		})
	}
	return out, nil
}

// AthleteHistory implements provider.AthleteSearcher. NYRR caps pageSize at
// 100; larger values return HTTP 400, so full history is paged in 100-race
// chunks.
func (c *Client) AthleteHistory(ctx context.Context, racerID string) ([]domain.Result, error) {
	const pageSize = 100
	var out []domain.Result
	for page := 1; ; page++ {
		req := raceHistoryRequest{
			RunnerID:       racerID,
			PageIndex:      page,
			PageSize:       pageSize,
			SortColumn:     "EventDate",
			SortDescending: true,
		}
		var resp raceHistoryResponse
		if err := c.postJSON(ctx, "/api/v2/runners/races", req, &resp); err != nil {
			return nil, err
		}
		for _, it := range resp.Items {
			date := it.StartDateTime
			if len(date) >= 10 {
				date = date[:10]
			}
			out = append(out, domain.Result{
				Provider:  "nyrr",
				RaceName:  it.EventName,
				Date:      date,
				Distance:  it.DistanceName,
				NetTime:   it.ActualTime,
				Bib:       it.Bib,
				SourceURL: "https://results.nyrr.org/races/" + it.EventCode + "/results",
			})
		}
		if len(resp.Items) < pageSize || len(out) >= resp.TotalItems {
			break
		}
	}
	return out, nil
}
