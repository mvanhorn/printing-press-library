package athlinks

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/other/running-race-results/internal/domain"
)

// athReq issues a GET with the headers the Athlinks API expects. Auth is optional
// (see setAuth): the athlete endpoints are public, so no token is required.
func (c *Client) athReq(ctx context.Context, rawURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	c.setAuth(req)
	return c.HTTP.Do(req)
}

func (c *Client) FindAthletes(ctx context.Context, name string) ([]domain.Athlete, error) {
	u := fmt.Sprintf("%s/athletes/api/find?searchTerm=%s&limit=10&running=true",
		c.AlaskaURL, url.QueryEscape(name))
	resp, err := c.athReq(ctx, u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if err := c.authStatusErr(resp.StatusCode); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("athlinks: athlete search status %d", resp.StatusCode)
	}
	var body struct {
		Result struct {
			Athletes []struct {
				RacerID     json.Number `json:"racerId"`
				DisplayName string      `json:"displayName"`
				City        string      `json:"city"`
				StateProv   string      `json:"stateProv"`
				Age         int         `json:"age"`
				ProfileURL  string      `json:"profileUrl"`
			} `json:"athletes"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("athlinks: decode athlete search: %w", err)
	}
	out := make([]domain.Athlete, 0, len(body.Result.Athletes))
	for _, a := range body.Result.Athletes {
		id := a.RacerID.String()
		profile := a.ProfileURL
		if profile == "" && id != "" {
			profile = "https://www.athlinks.com/athletes/" + id
		}
		out = append(out, domain.Athlete{
			Provider: "athlinks", ID: id, Name: a.DisplayName,
			City: a.City, StateProv: a.StateProv, Age: a.Age, ProfileURL: profile,
		})
	}
	return out, nil
}

func (c *Client) AthleteHistory(ctx context.Context, racerID string) ([]domain.Result, error) {
	u := fmt.Sprintf("%s/athletes/api/%s/Races", c.AlaskaURL, url.PathEscape(racerID))
	resp, err := c.athReq(ctx, u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if err := c.authStatusErr(resp.StatusCode); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("athlinks: athlete races status %d", resp.StatusCode)
	}
	var body struct {
		Result struct {
			RaceEntries struct {
				List []struct {
					BibNum      string `json:"BibNum"`
					RankO       int    `json:"RankO"`
					RankG       int    `json:"RankG"`
					RankA       int    `json:"RankA"`
					TicksString string `json:"TicksString"`
					Race        struct {
						RaceName string `json:"RaceName"`
						RaceDate string `json:"RaceDate"`
						RaceID   int    `json:"RaceID"`
						Courses  []struct {
							CourseName string `json:"CourseName"`
						} `json:"Courses"`
					} `json:"Race"`
				} `json:"List"`
			} `json:"raceEntries"`
		} `json:"Result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("athlinks: decode athlete races: %w", err)
	}
	rows := body.Result.RaceEntries.List
	out := make([]domain.Result, 0, len(rows))
	for _, e := range rows {
		date := e.Race.RaceDate
		if len(date) >= 10 {
			date = date[:10] // trim "2023-06-03T..." to "2023-06-03"
		}
		distance := ""
		if len(e.Race.Courses) > 0 {
			distance = e.Race.Courses[0].CourseName
		}
		out = append(out, domain.Result{
			Provider:      "athlinks",
			RaceName:      e.Race.RaceName,
			Date:          date,
			Distance:      distance,
			Bib:           e.BibNum,
			NetTime:       e.TicksString,
			OverallPlace:  e.RankO,
			GenderPlace:   e.RankG,
			AgeGroupPlace: e.RankA,
			SourceURL:     fmt.Sprintf("https://www.athlinks.com/event/%s/results", strconv.Itoa(e.Race.RaceID)),
		})
	}
	return out, nil
}
