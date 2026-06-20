// internal/provider/nyrr/nyrr.go
package nyrr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/running-race-results/internal/domain"
	"github.com/mvanhorn/printing-press-library/library/other/running-race-results/internal/provider"
)

// Client is the NYRR provider adapter.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// New returns a Client configured for the NYRR production API.
func New() *Client {
	return &Client{
		BaseURL: "https://rmsprodapi.nyrr.org",
		HTTP:    &http.Client{Timeout: 15 * time.Second},
	}
}

// Name implements provider.Provider.
func (c *Client) Name() string { return "nyrr" }

// searchRequest is the body sent to the finishers-filter endpoint.
type searchRequest struct {
	EventCode      string `json:"eventCode"`
	SearchString   string `json:"searchString"`
	PageIndex      int    `json:"pageIndex"`
	PageSize       int    `json:"pageSize"`
	SortColumn     string `json:"sortColumn"`
	SortDescending bool   `json:"sortDescending"`
}

// searchResponse is the JSON envelope returned by the API.
type searchResponse struct {
	TotalItems int    `json:"totalItems"`
	Items      []item `json:"items"`
}

type item struct {
	RunnerID        int     `json:"runnerId"`
	FirstName       string  `json:"firstName"`
	LastName        string  `json:"lastName"`
	Bib             string  `json:"bib"`
	Age             int     `json:"age"`
	Gender          string  `json:"gender"`
	City            string  `json:"city"`
	StateProvince   string  `json:"stateProvince"`
	CountryCode     string  `json:"countryCode"`
	OverallPlace    int     `json:"overallPlace"`
	OverallTime     string  `json:"overallTime"`
	Pace            string  `json:"pace"`
	GenderPlace     int     `json:"genderPlace"`
	AgeGradeTime    string  `json:"ageGradeTime"`
	AgeGradePlace   int     `json:"ageGradePlace"`
	AgeGradePercent float64 `json:"ageGradePercent"`
	RacesCount      int     `json:"racesCount"`
}

const nyrrPageSize = 100

// fetchPage calls one page of the finishers-filter endpoint.
func (c *Client) fetchPage(ctx context.Context, ev domain.Event, searchString string, pageIndex int) (searchResponse, error) {
	reqBody := searchRequest{
		EventCode:      ev.ID,
		SearchString:   searchString,
		PageIndex:      pageIndex,
		PageSize:       nyrrPageSize,
		SortColumn:     "overallTime",
		SortDescending: false,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return searchResponse{}, fmt.Errorf("nyrr: marshal request: %w", err)
	}

	url := c.BaseURL + "/api/v2/runners/finishers-filter"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return searchResponse{}, fmt.Errorf("nyrr: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return searchResponse{}, fmt.Errorf("nyrr: http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return searchResponse{}, fmt.Errorf("nyrr: unexpected status %d", resp.StatusCode)
	}

	var sr searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return searchResponse{}, fmt.Errorf("nyrr: decode response: %w", err)
	}
	return sr, nil
}

// fetch calls the first page of the finishers-filter endpoint with the given
// searchString and returns the page's items.
func (c *Client) fetch(ctx context.Context, ev domain.Event, searchString string) ([]item, error) {
	sr, err := c.fetchPage(ctx, ev, searchString, 1)
	if err != nil {
		return nil, err
	}
	return sr.Items, nil
}

// mapItem converts an item to a domain.Result.
func (c *Client) mapItem(ev domain.Event, it item) domain.Result {
	return domain.Result{
		Provider:     "nyrr",
		RaceName:     ev.Name,
		Year:         ev.Year,
		Runner:       it.FirstName + " " + it.LastName,
		Bib:          it.Bib,
		NetTime:      it.OverallTime,
		OverallPlace: it.OverallPlace,
		GenderPlace:  it.GenderPlace,
		SourceURL:    "https://results.nyrr.org/races/" + ev.ID + "/results",
	}
}

// Lookup implements provider.Provider.
func (c *Client) Lookup(ctx context.Context, ev domain.Event, bib string) (domain.Result, error) {
	seen := 0
	for page := 1; ; page++ {
		sr, err := c.fetchPage(ctx, ev, bib, page)
		if err != nil {
			return domain.Result{}, err
		}
		for _, it := range sr.Items {
			if it.Bib != bib {
				continue
			}
			return c.mapItem(ev, it), nil
		}
		seen += len(sr.Items)
		if len(sr.Items) == 0 || seen >= sr.TotalItems {
			break
		}
	}

	return domain.Result{}, provider.ErrBibNotFound
}

// SearchByName implements provider.NameSearcher. It posts the finishers-filter
// endpoint with the name as the search string and returns all items whose full
// name contains the query (case-insensitive substring match).
func (c *Client) SearchByName(ctx context.Context, ev domain.Event, name string) ([]domain.Result, error) {
	items, err := c.fetch(ctx, ev, name)
	if err != nil {
		return nil, err
	}
	var out []domain.Result
	q := strings.ToLower(name)
	for _, it := range items {
		full := strings.ToLower(it.FirstName + " " + it.LastName)
		if strings.Contains(full, q) {
			out = append(out, c.mapItem(ev, it))
		}
	}
	return out, nil
}
