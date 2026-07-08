package slack

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestHistoryFetchSummarizesLatestMessages(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/conversations.history" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer xoxb-test" {
			t.Fatalf("missing Slack authorization header: %q", r.Header.Get("Authorization"))
		}
		return jsonResponse(http.StatusOK, `{"ok":true,"messages":[{"ts":"2000.000","user":"U1","text":"latest","reactions":[{"name":"thumbsup","count":3}]},{"ts":"1000.000","user":"U2","text":"older"}]}`), nil
	})}

	histories, availability, err := HistoryFetch(context.Background(), []Linkage{{ChannelID: "C012ACME"}}, HistoryOptions{
		BotToken:   "xoxb-test",
		Start:      time.Unix(1000, 0),
		End:        time.Unix(2000, 0),
		HTTPClient: httpClient,
		BaseURL:    "https://slack.test",
	})
	if err != nil {
		t.Fatalf("HistoryFetch: %v", err)
	}
	if !availability.Available {
		t.Fatalf("expected available: %+v", availability)
	}
	if len(histories) != 1 || len(histories[0].Messages) != 2 {
		t.Fatalf("unexpected histories: %#v", histories)
	}
	if histories[0].Messages[0].Text != "latest" || histories[0].Messages[0].ReactionCounts["thumbsup"] != 3 {
		t.Fatalf("unexpected summary: %#v", histories[0].Messages[0])
	}
}

func TestHistoryFetchRateLimitWarnsAndContinues(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusTooManyRequests, `{"ok":false,"error":"ratelimited"}`), nil
	})}

	histories, availability, err := HistoryFetch(context.Background(), []Linkage{{ChannelID: "C012ACME"}}, HistoryOptions{
		BotToken:   "xoxb-test",
		HTTPClient: httpClient,
		BaseURL:    "https://slack.test",
	})
	if err != nil {
		t.Fatalf("HistoryFetch: %v", err)
	}
	if availability.Warning != "rate_limited" {
		t.Fatalf("expected rate limit warning: %+v", availability)
	}
	if len(histories) != 1 || histories[0].Warning != "rate_limited" {
		t.Fatalf("expected linkage-only warning history: %#v", histories)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
