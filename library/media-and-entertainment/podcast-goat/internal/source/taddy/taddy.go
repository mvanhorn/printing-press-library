// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH: v0.1 taddy.org GraphQL paid bulk adapter.

package taddy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/config"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/source"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/transcript"
)

const (
	adapterName = "taddy"
	endpoint    = "https://api.taddy.org/"
)

type Adapter struct {
	Client *http.Client
	APIKey string
	UserID string
}

func New() *Adapter {
	return &Adapter{
		Client: &http.Client{Timeout: 30 * time.Second},
		APIKey: config.Resolve("TADDY_API_KEY"),
		UserID: config.Resolve("TADDY_USER_ID"),
	}
}

func (a *Adapter) Name() string          { return adapterName }
func (a *Adapter) Tier() transcript.Tier { return transcript.TierPaid }

// Match: tier-9 fallback for any URL the cheaper adapters don't claim. The
// dispatcher decides when to invoke this based on tier order — we accept
// any episode-shaped URL.
func (a *Adapter) Match(url string) bool {
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
}

type gqlRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

type gqlError struct {
	Message string `json:"message"`
}

func (a *Adapter) Fetch(ctx context.Context, episodeURL string) (*transcript.Transcript, error) {
	if a.APIKey == "" {
		return nil, &source.KeyMissingError{EnvVar: "TADDY_API_KEY", URL: "https://taddy.org/developers"}
	}
	if a.UserID == "" {
		return nil, &source.KeyMissingError{EnvVar: "TADDY_USER_ID", URL: "https://taddy.org/developers"}
	}
	uuid, err := a.resolveEpisodeUUID(ctx, episodeURL)
	if err != nil {
		return nil, err
	}
	return a.fetchTranscript(ctx, episodeURL, uuid)
}

func (a *Adapter) gql(ctx context.Context, req gqlRequest, out interface{}) error {
	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-KEY", a.APIKey)
	httpReq.Header.Set("X-USER-ID", a.UserID)
	resp, err := a.Client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("taddy graphql: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return &source.KeyMissingError{EnvVar: "TADDY_API_KEY", URL: "https://taddy.org/developers"}
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("taddy HTTP %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}
	var env struct {
		Data   json.RawMessage `json:"data"`
		Errors []gqlError      `json:"errors"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("taddy decode: %w", err)
	}
	if len(env.Errors) > 0 {
		return fmt.Errorf("taddy error: %s", env.Errors[0].Message)
	}
	if out != nil {
		return json.Unmarshal(env.Data, out)
	}
	return nil
}

func (a *Adapter) resolveEpisodeUUID(ctx context.Context, episodeURL string) (string, error) {
	query := `query($q: String!) {
		getPodcastEpisode(searchTerm: $q) {
			uuid
			name
		}
	}`
	var data struct {
		GetPodcastEpisode struct {
			UUID string `json:"uuid"`
			Name string `json:"name"`
		} `json:"getPodcastEpisode"`
	}
	if err := a.gql(ctx, gqlRequest{
		Query:     query,
		Variables: map[string]interface{}{"q": episodeURL},
	}, &data); err != nil {
		return "", err
	}
	if data.GetPodcastEpisode.UUID == "" {
		return "", &source.NotApplicableError{Source: adapterName, URL: episodeURL, Reason: "taddy has no record for this URL"}
	}
	return data.GetPodcastEpisode.UUID, nil
}

func (a *Adapter) fetchTranscript(ctx context.Context, episodeURL, uuid string) (*transcript.Transcript, error) {
	query := `query($id: ID!) {
		getEpisodeTranscript(uuid: $id) {
			id
			text
			speaker
			startTimecode
			endTimecode
		}
	}`
	var data struct {
		GetEpisodeTranscript []struct {
			ID            string `json:"id"`
			Text          string `json:"text"`
			Speaker       string `json:"speaker"`
			StartTimecode int    `json:"startTimecode"`
		} `json:"getEpisodeTranscript"`
	}
	if err := a.gql(ctx, gqlRequest{
		Query:     query,
		Variables: map[string]interface{}{"id": uuid},
	}, &data); err != nil {
		return nil, err
	}
	if len(data.GetEpisodeTranscript) == 0 {
		return nil, &source.NotApplicableError{Source: adapterName, URL: episodeURL, Reason: "taddy returned 0 transcript segments"}
	}
	segs := make([]transcript.Segment, 0, len(data.GetEpisodeTranscript))
	for _, s := range data.GetEpisodeTranscript {
		speaker := strings.TrimSpace(s.Speaker)
		if speaker == "" {
			speaker = "Speaker"
		}
		segs = append(segs, transcript.Segment{
			TsSec:   s.StartTimecode / 1000,
			Speaker: speaker,
			Text:    strings.TrimSpace(s.Text),
		})
	}
	return &transcript.Transcript{
		ID:        transcript.IDFor(episodeURL),
		Source:    adapterName,
		Tier:      transcript.TierPaid,
		URL:       episodeURL,
		Provider:  adapterName,
		Segments:  segs,
		FetchedAt: time.Now().UTC(),
	}, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..." + strconv.Itoa(len(s)-n) + "B truncated"
}

var _ source.Adapter = (*Adapter)(nil)
