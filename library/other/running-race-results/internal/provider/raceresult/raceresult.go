// internal/provider/raceresult/raceresult.go
package raceresult

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/running-race-results/internal/domain"
	"github.com/mvanhorn/printing-press-library/library/other/running-race-results/internal/provider"
)

// userAgent is a browser-like UA; the RaceResult hosts may reject the default
// Go-http-client UA.
const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

// Client is the RaceResult provider adapter.
type Client struct {
	BaseURL     string // for the config call; default "https://my.raceresult.com"
	DataBaseURL string // for list calls; if empty, uses "https://{config.server}"
	HTTP        *http.Client
}

// New returns a Client configured for the RaceResult production API.
func New() *Client {
	return &Client{
		BaseURL: "https://my.raceresult.com",
		HTTP:    &http.Client{Timeout: 15 * time.Second},
	}
}

// Name implements provider.Provider.
func (c *Client) Name() string { return "raceresult" }

// configResponse is the JSON envelope returned by the config endpoint.
type configResponse struct {
	Key      string            `json:"key"`
	Server   string            `json:"server"`
	Contests map[string]string `json:"contests"`
	Tab      struct {
		Config struct {
			Lists []struct {
				Name string `json:"Name"`
			} `json:"Lists"`
		} `json:"Config"`
	} `json:"Tab"`
}

// cell holds one result-table cell. The list endpoint returns most cells as
// JSON strings but some as JSON numbers, so a permissive decoder is required —
// a strict [][]string would fail to decode the whole list on a single number.
type cell string

func (c *cell) UnmarshalJSON(b []byte) error {
	switch {
	case len(b) == 0 || string(b) == "null":
		*c = ""
	case b[0] == '"':
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*c = cell(s)
	default:
		*c = cell(strings.TrimSpace(string(b)))
	}
	return nil
}

// listResponse is the JSON envelope returned by the list endpoint.
type listResponse struct {
	DataFields []string            `json:"DataFields"`
	Data       map[string][][]cell `json:"data"`
}

var nonDigit = regexp.MustCompile(`\D`)

// get issues a GET with a browser User-Agent.
func (c *Client) get(ctx context.Context, rawURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	return c.HTTP.Do(req)
}

// Lookup implements provider.Provider. A RaceResult event exposes several
// result lists (teams/individual × gender), and a given bib lives in exactly
// one of them, so each list is queried until the bib is found.
func (c *Client) Lookup(ctx context.Context, ev domain.Event, bib string) (domain.Result, error) {
	configURL := fmt.Sprintf("%s/%s/results/config?lang=en", c.BaseURL, ev.ID)
	resp, err := c.get(ctx, configURL)
	if err != nil {
		return domain.Result{}, fmt.Errorf("raceresult: config request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return domain.Result{}, fmt.Errorf("raceresult: config status %d", resp.StatusCode)
	}
	var cfg configResponse
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		return domain.Result{}, fmt.Errorf("raceresult: decode config: %w", err)
	}
	if len(cfg.Tab.Config.Lists) == 0 {
		return domain.Result{}, fmt.Errorf("raceresult: no lists in config")
	}

	contestID := "1"
	if len(cfg.Contests) > 0 {
		keys := make([]string, 0, len(cfg.Contests))
		for k := range cfg.Contests {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		contestID = keys[0]
	}
	dataBase := c.DataBaseURL
	if dataBase == "" {
		dataBase = "https://" + cfg.Server
	}

	anyOK := false
	for _, l := range cfg.Tab.Config.Lists {
		res, found, ok := c.searchList(ctx, dataBase, ev, cfg.Key, l.Name, contestID, bib)
		if ok {
			anyOK = true
		}
		if found {
			return res, nil
		}
	}
	if !anyOK {
		return domain.Result{}, fmt.Errorf("raceresult: all result lists failed to load")
	}
	return domain.Result{}, provider.ErrBibNotFound
}

// searchList queries one result list and returns the mapped result if the bib
// is present, along with two booleans:
//   - found: the bib was located in the list
//   - ok: the list was successfully fetched and decoded (network/non-200/decode
//     failures return ok=false; missing BIB column also returns ok=false since
//     the list is not usable). A list that was fetched/decoded but lacks
//     AnzeigeName or TIME1 returns ok=true, found=false.
func (c *Client) searchList(ctx context.Context, dataBase string, ev domain.Event, key, listName, contestID, bib string) (domain.Result, bool /*found*/, bool /*ok*/) {
	listURL := fmt.Sprintf(
		"%s/%s/results/list?key=%s&listname=%s&page=results&contest=%s&r=leaders&l=50&fav=&openedGroups=%%7B%%7D&term=%s",
		dataBase, ev.ID, key, url.QueryEscape(listName), contestID, url.QueryEscape(bib),
	)
	resp, err := c.get(ctx, listURL)
	if err != nil {
		return domain.Result{}, false, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return domain.Result{}, false, false
	}
	var lr listResponse
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		return domain.Result{}, false, false
	}

	idx := make(map[string]int, len(lr.DataFields))
	for i, f := range lr.DataFields {
		idx[f] = i
	}
	bibIdx, hasBib := idx["BIB"]
	if !hasBib {
		// List has no BIB column — not usable at all.
		return domain.Result{}, false, false
	}
	nameIdx, hasName := idx["AnzeigeName"]
	timeIdx, hasTime := idx["TIME1"]
	rankIdx, hasRank := idx["RANK2p"]
	// Only individual result lists carry the runner name + finish time. Team
	// and organisation lists also contain the bib but lack these columns, so
	// skip them — otherwise we'd return a partial (name/time-less) result.
	// The list was successfully fetched, so ok=true even if we skip it.
	if !hasName || !hasTime {
		return domain.Result{}, false, true
	}

	for _, rows := range lr.Data {
		for _, row := range rows {
			if len(row) <= bibIdx || string(row[bibIdx]) != bib {
				continue
			}
			res := domain.Result{
				Provider:  "raceresult",
				RaceName:  ev.Name,
				Year:      ev.Year,
				Bib:       bib,
				SourceURL: "https://my.raceresult.com/" + ev.ID + "/results",
			}
			if hasName && len(row) > nameIdx {
				res.Runner = string(row[nameIdx])
			}
			if hasTime && len(row) > timeIdx {
				res.NetTime = string(row[timeIdx])
			}
			if hasRank && len(row) > rankIdx {
				if n, err := strconv.Atoi(nonDigit.ReplaceAllString(string(row[rankIdx]), "")); err == nil {
					res.OverallPlace = n
				}
			}
			return res, true, true
		}
	}
	return domain.Result{}, false, true
}

// nameRows queries one result list for name-based search. It fetches the list
// with term=name and returns all rows from individual lists (those with
// AnzeigeName + TIME1) whose AnzeigeName contains the name (case-insensitive).
// Returns nil rows and ok=false on fetch/decode failure.
func (c *Client) nameRows(ctx context.Context, dataBase string, ev domain.Event, key, listName, contestID, name string) ([]domain.Result, bool /*ok*/) {
	listURL := fmt.Sprintf(
		"%s/%s/results/list?key=%s&listname=%s&page=results&contest=%s&r=leaders&l=50&fav=&openedGroups=%%7B%%7D&term=%s",
		dataBase, ev.ID, key, url.QueryEscape(listName), contestID, url.QueryEscape(name),
	)
	resp, err := c.get(ctx, listURL)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, false
	}
	var lr listResponse
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		return nil, false
	}

	idx := make(map[string]int, len(lr.DataFields))
	for i, f := range lr.DataFields {
		idx[f] = i
	}
	bibIdx, hasBib := idx["BIB"]
	nameIdx, hasName := idx["AnzeigeName"]
	timeIdx, hasTime := idx["TIME1"]
	rankIdx, hasRank := idx["RANK2p"]
	if !hasBib || !hasName || !hasTime {
		return nil, true // individual list required; skip team/org lists
	}

	q := strings.ToLower(name)
	var out []domain.Result
	for _, rows := range lr.Data {
		for _, row := range rows {
			if len(row) <= nameIdx {
				continue
			}
			if !strings.Contains(strings.ToLower(string(row[nameIdx])), q) {
				continue
			}
			res := domain.Result{
				Provider:  "raceresult",
				RaceName:  ev.Name,
				Year:      ev.Year,
				Runner:    string(row[nameIdx]),
				SourceURL: "https://my.raceresult.com/" + ev.ID + "/results",
			}
			if hasBib && len(row) > bibIdx {
				res.Bib = string(row[bibIdx])
			}
			if hasTime && len(row) > timeIdx {
				res.NetTime = string(row[timeIdx])
			}
			if hasRank && len(row) > rankIdx {
				if n, err := strconv.Atoi(nonDigit.ReplaceAllString(string(row[rankIdx]), "")); err == nil {
					res.OverallPlace = n
				}
			}
			out = append(out, res)
		}
	}
	return out, true
}

// SearchByName implements provider.NameSearcher. It iterates the event's
// individual result lists and collects rows whose AnzeigeName contains name.
func (c *Client) SearchByName(ctx context.Context, ev domain.Event, name string) ([]domain.Result, error) {
	configURL := fmt.Sprintf("%s/%s/results/config?lang=en", c.BaseURL, ev.ID)
	resp, err := c.get(ctx, configURL)
	if err != nil {
		return nil, fmt.Errorf("raceresult: config request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("raceresult: config status %d", resp.StatusCode)
	}
	var cfg configResponse
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("raceresult: decode config: %w", err)
	}
	if len(cfg.Tab.Config.Lists) == 0 {
		return nil, fmt.Errorf("raceresult: no lists in config")
	}

	contestID := "1"
	if len(cfg.Contests) > 0 {
		keys := make([]string, 0, len(cfg.Contests))
		for k := range cfg.Contests {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		contestID = keys[0]
	}
	dataBase := c.DataBaseURL
	if dataBase == "" {
		dataBase = "https://" + cfg.Server
	}

	var out []domain.Result
	seen := make(map[string]bool)
	anyOK := false
	for _, l := range cfg.Tab.Config.Lists {
		rows, ok := c.nameRows(ctx, dataBase, ev, cfg.Key, l.Name, contestID, name)
		if ok {
			anyOK = true
		}
		for _, r := range rows {
			if r.Bib != "" {
				if seen[r.Bib] {
					continue // same runner appears in multiple lists
				}
				seen[r.Bib] = true
			}
			out = append(out, r)
		}
	}
	if !anyOK {
		return nil, fmt.Errorf("raceresult: all result lists failed to load")
	}
	return out, nil
}
