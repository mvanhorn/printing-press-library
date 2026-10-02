// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/pinecone/internal/client"
	"github.com/mvanhorn/printing-press-library/library/ai/pinecone/internal/config"
)

type pineconeHostRoundTrip func(*http.Request) (*http.Response, error)

func (f pineconeHostRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDataPlanePathResolvesDifferentHostsWithoutMutatingSharedClient(t *testing.T) {
	cfg := &config.Config{BaseURL: "https://api.pinecone.io", PineconeApiKey: "test-key", TemplateVars: map[string]string{"index_host": "index_host_placeholder"}}
	c := client.New(cfg, time.Second, 0)
	c.NoCache = true
	c.HTTPClient.Transport = pineconeHostRoundTrip(func(r *http.Request) (*http.Response, error) {
		name := strings.TrimPrefix(r.URL.Path, "/indexes/")
		if r.URL.Host != "api.pinecone.io" || (name != "index-a" && name != "index-b") {
			return nil, fmt.Errorf("unexpected host lookup %s", r.URL.Redacted())
		}
		body := fmt.Sprintf(`{"host":"%s.svc.pinecone.io"}`, name)
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	const workers = 20
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		name := "index-a"
		if i%2 == 1 {
			name = "index-b"
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := dataPlanePath(context.Background(), c, name, "/query")
			if err == nil && got != "https://"+name+".svc.pinecone.io/query" {
				err = fmt.Errorf("index %s resolved to %s", name, got)
			}
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if cfg.TemplateVars["index_host"] != "index_host_placeholder" {
		t.Fatalf("shared index host changed to %q", cfg.TemplateVars["index_host"])
	}
}
