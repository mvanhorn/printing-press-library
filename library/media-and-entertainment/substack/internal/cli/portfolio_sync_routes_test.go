package cli

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack/internal/config"
)

type captureTransport struct {
	urls []string
}

func (t *captureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.urls = append(t.urls, req.URL.String())
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`[]`)),
		Request:    req,
	}, nil
}

func (t *captureTransport) firstDraftURL() string {
	for _, u := range t.urls {
		if strings.Contains(u, "/api/v1/drafts") {
			return u
		}
	}
	return ""
}

func TestPortfolioFetchDraftsUsesGlobalPublicationIDRoute(t *testing.T) {
	transport := &captureTransport{}
	c := client.New(&config.Config{BaseURL: "https://trevinsays.substack.com/api/v1"}, 0, 0)
	c.NoCache = true
	c.HTTPClient.Transport = transport

	fetchDrafts(context.Background(), c, "7019888", io.Discard)

	if got, want := transport.firstDraftURL(), "https://substack.com/api/v1/drafts?publication_id=7019888"; got != want {
		t.Fatalf("fetchDrafts URL = %q, want %q", got, want)
	}
}

func TestSyncDraftsUsesGlobalPublicationIDRoute(t *testing.T) {
	path, err := syncResourcePath("drafts")
	if err != nil {
		t.Fatalf("syncResourcePath: %v", err)
	}
	if got, want := path, "https://substack.com/api/v1/drafts"; got != want {
		t.Fatalf("sync drafts path = %q, want %q", got, want)
	}

	flags := &rootFlags{publicationID: "7019888"}
	params, err := syncResourceExtraParams(context.Background(), nil, flags, "drafts")
	if err != nil {
		t.Fatalf("syncResourceExtraParams: %v", err)
	}
	if got, want := params["publication_id"], "7019888"; got != want {
		t.Fatalf("sync drafts publication_id = %q, want %q", got, want)
	}
}
