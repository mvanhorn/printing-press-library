// Package omdb provides a small client for the Open Movie Database (OMDb) API,
// used as a per-row enrichment source for IMDb / Rotten Tomatoes / Metacritic
// ratings on top of TMDb's primary data. The client uses cliutil.AdaptiveLimiter
// to pace requests against OMDb's free tier (~1 req/sec) and surfaces a
// *cliutil.RateLimitError when retries are exhausted, in line with the
// printing-press per-source rate-limiting contract.
package omdb

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/internal/cliutil"
)

const (
	baseURL    = "https://www.omdbapi.com/"
	maxRetries = 3
)

// Rating represents a single rating source (e.g. Rotten Tomatoes, Metacritic).
type Rating struct {
	Source string `json:"Source"`
	Value  string `json:"Value"`
}

// Result holds the full OMDb API response for a single title.
type Result struct {
	Title      string   `json:"Title"`
	Year       string   `json:"Year"`
	Rated      string   `json:"Rated"`
	Released   string   `json:"Released"`
	Runtime    string   `json:"Runtime"`
	Genre      string   `json:"Genre"`
	Director   string   `json:"Director"`
	Writer     string   `json:"Writer"`
	Actors     string   `json:"Actors"`
	Plot       string   `json:"Plot"`
	Language   string   `json:"Language"`
	Country    string   `json:"Country"`
	Awards     string   `json:"Awards"`
	Poster     string   `json:"Poster"`
	Ratings    []Rating `json:"Ratings"`
	Metascore  string   `json:"Metascore"`
	ImdbRating string   `json:"imdbRating"`
	ImdbVotes  string   `json:"imdbVotes"`
	BoxOffice  string   `json:"BoxOffice"`
	Response   string   `json:"Response"`
	Error      string   `json:"Error"`
}

// Client is the OMDb HTTP client. One AdaptiveLimiter is held per Client so
// concurrent fan-outs share the same per-host pacing budget.
type Client struct {
	HTTP    *http.Client
	limiter *cliutil.AdaptiveLimiter
}

// NewClient returns an OMDb client with a 10s timeout and a ~1 req/sec
// adaptive limiter (OMDb free tier is 1000/day with ~1/sec recommended pacing).
func NewClient() *Client {
	return &Client{
		HTTP:    &http.Client{Timeout: 10 * time.Second},
		limiter: cliutil.NewAdaptiveLimiter(1.0),
	}
}

// Fetch retrieves OMDb data for the given IMDb ID. Returns nil without error
// if apiKey or imdbID is empty (graceful degradation when OMDB_API_KEY is not
// set or the source title has no imdb_id mapping). Surfaces *cliutil.RateLimitError
// when retries are exhausted on 429.
func (c *Client) Fetch(imdbID string, apiKey string) (*Result, error) {
	if apiKey == "" || imdbID == "" {
		return nil, nil
	}

	url := fmt.Sprintf("%s?i=%s&apikey=%s&plot=full", baseURL, imdbID, apiKey)

	var lastResp *http.Response
	var lastBody string
	for attempt := 0; attempt <= maxRetries; attempt++ {
		c.limiter.Wait()
		resp, err := c.HTTP.Get(url)
		if err != nil {
			return nil, fmt.Errorf("omdb request failed: %w", err)
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			c.limiter.OnRateLimit()
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastResp = resp
			lastBody = string(body)
			if attempt >= maxRetries {
				break
			}
			wait := cliutil.RetryAfter(resp)
			time.Sleep(wait)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("reading omdb response: %w", err)
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("omdb returned HTTP %d: %s", resp.StatusCode, string(body))
		}

		c.limiter.OnSuccess()

		var result Result
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("parsing omdb response: %w", err)
		}

		if result.Response == "False" {
			// OMDb returned a logical error ("Movie not found!"); not a hard
			// failure — callers expect graceful degradation.
			return nil, nil
		}
		return &result, nil
	}

	// Retries exhausted on 429. Surface a structured rate-limit error so
	// callers can distinguish "no data exists" from "throttled".
	rerr := &cliutil.RateLimitError{URL: url, Body: lastBody}
	if lastResp != nil {
		rerr.RetryAfter = cliutil.RetryAfter(lastResp)
	}
	return nil, rerr
}

// Fetch is a package-level convenience wrapper that constructs a one-shot
// Client. Returns (nil, nil) on empty apiKey for graceful degradation.
func Fetch(imdbID string, apiKey string) (*Result, error) {
	if apiKey == "" {
		return nil, nil
	}
	return NewClient().Fetch(imdbID, apiKey)
}

// RatingBySource returns the value for a specific rating source, or empty
// string if not found.
func (r *Result) RatingBySource(source string) string {
	if r == nil {
		return ""
	}
	for _, rating := range r.Ratings {
		if rating.Source == source {
			return rating.Value
		}
	}
	return ""
}

// IsRateLimit reports whether err is a cliutil.RateLimitError.
func IsRateLimit(err error) bool {
	var rerr *cliutil.RateLimitError
	return errors.As(err, &rerr)
}
